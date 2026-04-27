package messagequeue

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	mqGovernanceMaxInstances = 200
	mqGovernanceMaxItems     = 500
	mqTopologyNodeLimit      = 600
	mqTopologyEdgeLimit      = 1000
)

func (uc *UseCase) GetProductionDashboard(ctx context.Context, req *GovernanceReportRequest) (*ProductionDashboardVO, error) {
	if req == nil {
		req = &GovernanceReportRequest{}
	}
	normalizeGovernanceReportRequest(req)
	instances, err := uc.listGovernanceInstances(ctx, req)
	if err != nil {
		return nil, err
	}
	result := &ProductionDashboardVO{
		TypeStats:            make([]*CountStatVO, 0),
		EnvironmentStats:     make([]*CountStatVO, 0),
		HealthStats:          make([]*CountStatVO, 0),
		TopBacklogResources:  make([]*DashboardResourceVO, 0),
		TopLagConsumerGroups: make([]*DashboardConsumerGroupVO, 0),
		GeneratedAt:          nowText(),
	}
	typeCounts := make(map[string]int64)
	envCounts := make(map[string]int64)
	healthCounts := make(map[string]int64)
	for _, instance := range instances {
		if instance == nil {
			continue
		}
		result.InstanceTotal++
		typeCounts[NormalizeType(instance.MQType)]++
		envCounts[fallbackKey(instance.Environment, "unknown")]++
		health := fallbackKey(instance.HealthStatus, HealthStatusUnknown)
		healthCounts[health]++
		switch health {
		case HealthStatusHealthy:
			result.HealthyInstances++
		case HealthStatusWarning:
			result.WarningInstances++
		case HealthStatusCritical:
			result.CriticalInstances++
		default:
			result.UnknownInstances++
		}
		if strings.TrimSpace(instance.Owner) == "" {
			result.NoOwnerInstanceTotal++
		}
		if strings.TrimSpace(instance.BusinessSystem) == "" {
			result.NoBusinessInstanceTotal++
		}
		if uc.resourceRepo != nil {
			resourceSummary, _ := uc.resourceRepo.Summary(ctx, instance.ID)
			if resourceSummary != nil {
				result.ResourceTotal += resourceSummary.Count
				result.BacklogTotal += resourceSummary.Backlog
				result.DLQResourceTotal += resourceSummary.DLQResourceCount
				result.RetryResourceTotal += resourceSummary.RetryResourceCount
				if strings.TrimSpace(instance.Owner) == "" {
					result.NoOwnerResourceTotal += resourceSummary.Count
				}
			}
		}
		if uc.consumerGroupRepo != nil {
			groupSummary, _ := uc.consumerGroupRepo.Summary(ctx, instance.ID)
			if groupSummary != nil {
				result.ConsumerGroupTotal += groupSummary.Count
				result.LagTotal += groupSummary.Lag
			}
		}
		if uc.resourceRepo != nil {
			topResources, _ := uc.resourceRepo.TopBacklog(ctx, instance.ID, 5)
			for _, item := range topResources {
				result.TopBacklogResources = append(result.TopBacklogResources, toDashboardResource(instance, item))
			}
		}
		if uc.consumerGroupRepo != nil {
			groups, _, _ := uc.consumerGroupRepo.List(ctx, instance.ID, &ConsumerGroupListRequest{Page: 1, PageSize: 5, HasLag: "true"})
			for _, item := range groups {
				result.TopLagConsumerGroups = append(result.TopLagConsumerGroups, toDashboardConsumerGroup(instance, item))
			}
		}
	}
	result.TypeStats = countStats(typeCounts, TypeText)
	result.EnvironmentStats = countStats(envCounts, environmentText)
	result.HealthStats = countStats(healthCounts, HealthText)
	sort.Slice(result.TopBacklogResources, func(i, j int) bool {
		return result.TopBacklogResources[i].Backlog > result.TopBacklogResources[j].Backlog
	})
	sort.Slice(result.TopLagConsumerGroups, func(i, j int) bool {
		return result.TopLagConsumerGroups[i].Lag > result.TopLagConsumerGroups[j].Lag
	})
	if len(result.TopBacklogResources) > 10 {
		result.TopBacklogResources = result.TopBacklogResources[:10]
	}
	if len(result.TopLagConsumerGroups) > 10 {
		result.TopLagConsumerGroups = result.TopLagConsumerGroups[:10]
	}
	result.RecentHighRiskAudits = uc.recentHighRiskAudits(ctx, req)
	result.RecentFailedJobs = uc.recentFailedJobs(ctx, req)
	governance, _ := uc.GetGovernanceReport(ctx, &GovernanceReportRequest{
		Page:              1,
		PageSize:          10,
		RestrictToAllowed: req.RestrictToAllowed,
		AllowedIDs:        req.AllowedIDs,
	})
	if governance != nil {
		for _, item := range governance.Violations {
			if item != nil && (item.Severity == "critical" || item.Severity == "warning") {
				result.AlertCandidates = append(result.AlertCandidates, item)
			}
		}
	}
	return result, nil
}

func (uc *UseCase) GetGovernanceReport(ctx context.Context, req *GovernanceReportRequest) (*GovernanceReportVO, error) {
	if req == nil {
		req = &GovernanceReportRequest{}
	}
	normalizeGovernanceReportRequest(req)
	instances, err := uc.listGovernanceInstances(ctx, req)
	if err != nil {
		return nil, err
	}
	all := make([]*GovernanceViolationVO, 0)
	for _, instance := range instances {
		all = append(all, uc.instanceGovernanceViolations(ctx, instance)...)
	}
	filtered := filterGovernanceViolations(all, req)
	sort.Slice(filtered, func(i, j int) bool {
		if severityRank(filtered[i].Severity) == severityRank(filtered[j].Severity) {
			return filtered[i].InstanceName < filtered[j].InstanceName
		}
		return severityRank(filtered[i].Severity) > severityRank(filtered[j].Severity)
	})
	report := &GovernanceReportVO{Total: int64(len(filtered)), GeneratedAt: nowText()}
	for _, item := range filtered {
		switch item.Severity {
		case "critical":
			report.CriticalCount++
		case "warning":
			report.WarningCount++
		default:
			report.InfoCount++
		}
		switch item.Category {
		case "ownership":
			report.OwnerMissingCount++
		case "baseline":
			report.BaselineDriftCount++
		case "lifecycle":
			report.LifecycleRiskCount++
		}
	}
	start := (req.Page - 1) * req.PageSize
	if start < len(filtered) {
		end := start + req.PageSize
		if end > len(filtered) {
			end = len(filtered)
		}
		report.Violations = filtered[start:end]
	} else {
		report.Violations = []*GovernanceViolationVO{}
	}
	return report, nil
}

func (uc *UseCase) GetTopology(ctx context.Context, instanceID uint) (*TopologyVO, error) {
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("MQ实例不存在")
	}
	topology := &TopologyVO{
		InstanceID:   instance.ID,
		InstanceName: instance.Name,
		MQType:       instance.MQType,
		Nodes:        make([]*TopologyNodeVO, 0),
		Edges:        make([]*TopologyEdgeVO, 0),
		Warnings:     make([]string, 0),
		GeneratedAt:  nowText(),
	}
	nodeSeen := make(map[string]bool)
	addNode := func(node *TopologyNodeVO) {
		if node == nil || node.ID == "" || nodeSeen[node.ID] {
			return
		}
		if len(topology.Nodes) >= mqTopologyNodeLimit {
			topology.Warnings = appendUniqueString(topology.Warnings, fmt.Sprintf("拓扑节点超过 %d，结果已截断", mqTopologyNodeLimit))
			return
		}
		nodeSeen[node.ID] = true
		topology.Nodes = append(topology.Nodes, node)
	}
	addEdge := func(edge *TopologyEdgeVO) {
		if edge == nil || edge.From == "" || edge.To == "" || edge.From == edge.To {
			return
		}
		if len(topology.Edges) >= mqTopologyEdgeLimit {
			topology.Warnings = appendUniqueString(topology.Warnings, fmt.Sprintf("拓扑边超过 %d，结果已截断", mqTopologyEdgeLimit))
			return
		}
		if edge.ID == "" {
			edge.ID = fmt.Sprintf("%s:%s:%s", edge.Type, edge.From, edge.To)
		}
		topology.Edges = append(topology.Edges, edge)
	}
	instanceNodeID := fmt.Sprintf("instance:%d", instance.ID)
	addNode(&TopologyNodeVO{ID: instanceNodeID, Label: instance.Name, Type: "instance", Status: instance.HealthStatus, Metadata: map[string]any{"mqType": instance.MQType, "environment": instance.Environment, "owner": instance.Owner}})
	if uc.brokerRepo != nil {
		brokers, _ := uc.brokerRepo.ListByInstanceID(ctx, instance.ID)
		for _, broker := range brokers {
			id := fmt.Sprintf("broker:%d", broker.ID)
			addNode(&TopologyNodeVO{ID: id, Label: fallbackDash(broker.BrokerName), Type: ResourceTypeBroker, Status: broker.Status, Metadata: map[string]any{"host": broker.Host, "role": broker.Role, "version": broker.Version}})
			addEdge(&TopologyEdgeVO{From: instanceNodeID, To: id, Type: "contains", Label: "broker"})
		}
	}
	resources := []*MQResource{}
	if uc.resourceRepo != nil {
		resources, _, _ = uc.resourceRepo.List(ctx, instance.ID, &ResourceListRequest{Page: 1, PageSize: mqGovernanceMaxItems})
	}
	resourceNodes := make(map[string]string)
	namespaceNodes := make(map[string]string)
	for _, resource := range resources {
		nodeID := resourceTopologyNodeID(resource)
		resourceNodes[resourceLookupKey(resource.Namespace, resource.ResourceType, resource.Name)] = nodeID
		addNode(&TopologyNodeVO{
			ID:     nodeID,
			Label:  resourceTopologyLabel(resource),
			Type:   resource.ResourceType,
			Status: resourceStatus(resource),
			Metrics: map[string]any{
				"backlog":       resource.Backlog,
				"messageCount":  resource.MessageCount,
				"consumerCount": resource.ConsumerCount,
				"partitions":    resource.PartitionCount,
				"replicas":      resource.ReplicaCount,
			},
			Metadata: map[string]any{"namespace": resource.Namespace, "durable": resource.Durable},
		})
		if resource.Namespace != "" {
			nsID := namespaceNodes[resource.Namespace]
			if nsID == "" {
				nsID = "namespace:" + resource.Namespace
				namespaceNodes[resource.Namespace] = nsID
				addNode(&TopologyNodeVO{ID: nsID, Label: resource.Namespace, Type: ResourceTypeNamespace})
				addEdge(&TopologyEdgeVO{From: instanceNodeID, To: nsID, Type: "contains", Label: "namespace"})
			}
			addEdge(&TopologyEdgeVO{From: nsID, To: nodeID, Type: "contains", Label: ResourceTypeText(resource.ResourceType)})
		} else {
			addEdge(&TopologyEdgeVO{From: instanceNodeID, To: nodeID, Type: "contains", Label: ResourceTypeText(resource.ResourceType)})
		}
	}
	if uc.bindingRepo != nil {
		bindings, _ := uc.bindingRepo.ListByInstanceID(ctx, instance.ID)
		for _, binding := range bindings {
			sourceID := resourceNodes[resourceLookupKey(binding.VHost, ResourceTypeExchange, binding.Source)]
			if sourceID == "" {
				sourceID = "external:exchange:" + binding.VHost + ":" + binding.Source
				addNode(&TopologyNodeVO{ID: sourceID, Label: binding.Source, Type: ResourceTypeExchange, Metadata: map[string]any{"namespace": binding.VHost}})
			}
			destinationType := normalizeDestinationType(binding.DestinationType)
			destinationID := resourceNodes[resourceLookupKey(binding.VHost, destinationType, binding.Destination)]
			if destinationID == "" {
				destinationID = "external:" + destinationType + ":" + binding.VHost + ":" + binding.Destination
				addNode(&TopologyNodeVO{ID: destinationID, Label: binding.Destination, Type: destinationType, Metadata: map[string]any{"namespace": binding.VHost}})
			}
			addEdge(&TopologyEdgeVO{From: sourceID, To: destinationID, Type: ResourceTypeBinding, Label: binding.RoutingKey})
		}
	}
	if uc.consumerGroupRepo != nil {
		groups, _, _ := uc.consumerGroupRepo.List(ctx, instance.ID, &ConsumerGroupListRequest{Page: 1, PageSize: mqGovernanceMaxItems})
		for _, group := range groups {
			groupID := fmt.Sprintf("consumer_group:%d", group.ID)
			addNode(&TopologyNodeVO{
				ID:     groupID,
				Label:  group.GroupName,
				Type:   "consumer_group",
				Status: group.State,
				Metrics: map[string]any{
					"lag":                 group.Lag,
					"backlog":             group.Backlog,
					"consumerCount":       group.ConsumerCount,
					"activeConsumerCount": group.ActiveConsumerCount,
				},
				Metadata: map[string]any{"resourceName": group.ResourceName, "namespace": group.Namespace},
			})
			resourceID := findResourceNodeForGroup(resourceNodes, group)
			if resourceID != "" {
				addEdge(&TopologyEdgeVO{From: resourceID, To: groupID, Type: "consume", Label: "consume", Metric: map[string]any{"lag": group.Lag, "backlog": group.Backlog}})
			} else {
				addEdge(&TopologyEdgeVO{From: instanceNodeID, To: groupID, Type: "consume", Label: "consume", Metric: map[string]any{"lag": group.Lag, "backlog": group.Backlog}})
			}
		}
	}
	return topology, nil
}

func (uc *UseCase) listGovernanceInstances(ctx context.Context, req *GovernanceReportRequest) ([]*MQInstance, error) {
	if req != nil && req.InstanceID > 0 {
		if req.RestrictToAllowed && !containsUint(req.AllowedIDs, req.InstanceID) {
			return []*MQInstance{}, nil
		}
		item, err := uc.instanceRepo.GetByID(ctx, req.InstanceID)
		if err != nil {
			return nil, fmt.Errorf("MQ实例不存在")
		}
		return []*MQInstance{item}, nil
	}
	result := make([]*MQInstance, 0)
	page := 1
	for {
		items, total, err := uc.instanceRepo.List(ctx, &InstanceListRequest{
			Page:              page,
			PageSize:          mqGovernanceMaxInstances,
			RestrictToAllowed: req != nil && req.RestrictToAllowed,
			AllowedIDs:        allowedIDsFromGovernanceReq(req),
		})
		if err != nil {
			return nil, err
		}
		result = append(result, items...)
		if len(result) >= int(total) || len(items) == 0 {
			break
		}
		page++
	}
	return result, nil
}

func (uc *UseCase) instanceGovernanceViolations(ctx context.Context, instance *MQInstance) []*GovernanceViolationVO {
	if instance == nil {
		return nil
	}
	items := make([]*GovernanceViolationVO, 0)
	add := func(severity, category, title, description, suggestion, resourceType, resourceName, namespace, metric string) {
		items = append(items, &GovernanceViolationVO{
			Severity:     severity,
			Category:     category,
			Title:        title,
			Description:  description,
			Suggestion:   suggestion,
			InstanceID:   instance.ID,
			InstanceName: instance.Name,
			MQType:       instance.MQType,
			ResourceType: resourceType,
			ResourceName: resourceName,
			Namespace:    namespace,
			MetricValue:  metric,
			CreatedAt:    nowText(),
		})
	}
	if strings.TrimSpace(instance.Owner) == "" {
		add("warning", "ownership", "实例未绑定负责人", "生产治理、告警路由和事故响应缺少负责人", "补充实例负责人，并保持与业务系统一致", "instance", instance.Name, "", "")
	}
	if strings.TrimSpace(instance.BusinessSystem) == "" {
		add("warning", "ownership", "实例未绑定业务系统", "无法按业务系统聚合影响面和治理报表", "补充业务系统字段", "instance", instance.Name, "", "")
	}
	if instance.CredentialID == 0 {
		add("info", "security", "实例未绑定凭据", "连接测试、同步和自动巡检可能依赖人工连接参数", "绑定统一凭据并定期轮换", "instance", instance.Name, "", "")
	}
	if strings.TrimSpace(instance.Status) != InstanceStatusEnabled {
		add("info", "lifecycle", "实例已禁用", "禁用实例不会参与正常同步和诊断", "确认是否为下线实例，必要时补充备注或清理", "instance", instance.Name, "", "")
	}
	if instance.HealthStatus == HealthStatusCritical {
		add("critical", "health", "实例健康异常", "最近连接测试、同步或指标判断为异常", "先执行连接测试和元数据同步，再查看巡检报告", "instance", instance.Name, "", instance.HealthStatus)
	} else if instance.HealthStatus == HealthStatusWarning || instance.HealthStatus == HealthStatusUnknown {
		add("warning", "health", "实例健康存在风险", "实例处于警告或未知状态", "执行指标采集和巡检，确认具体原因", "instance", instance.Name, "", instance.HealthStatus)
	}
	if instance.LastSyncAt == nil || instance.LastSyncAt.IsZero() {
		add("warning", "lifecycle", "实例从未同步元数据", "页面资源、拓扑和治理判断缺少基础数据", "执行元数据同步", "instance", instance.Name, "", "")
	} else if age := time.Since(*instance.LastSyncAt); age > mqStaleSyncAge {
		add("warning", "lifecycle", "元数据同步过期", fmt.Sprintf("最近同步已超过 %d 分钟", int(mqStaleSyncAge.Minutes())), "重新同步元数据", "instance", instance.Name, "", fmt.Sprintf("%.0fmin", age.Minutes()))
	}
	if instance.LastMetricAt == nil || instance.LastMetricAt.IsZero() {
		add("warning", "lifecycle", "实例从未采集指标", "无法判断 backlog、lag 和消费速率趋势", "执行指标采集", "instance", instance.Name, "", "")
	} else if age := time.Since(*instance.LastMetricAt); age > mqStaleMetricAge {
		add("warning", "lifecycle", "指标快照过期", fmt.Sprintf("最近指标已超过 %d 分钟", int(mqStaleMetricAge.Minutes())), "重新采集指标", "instance", instance.Name, "", fmt.Sprintf("%.0fmin", age.Minutes()))
	}
	resources := []*MQResource{}
	if uc.resourceRepo != nil {
		resources, _, _ = uc.resourceRepo.List(ctx, instance.ID, &ResourceListRequest{Page: 1, PageSize: mqGovernanceMaxItems})
	}
	for _, resource := range resources {
		if resource == nil {
			continue
		}
		resourceName := firstNonEmpty(resource.FullName, resource.Name)
		if isQueueOrTopic(resource.ResourceType) && resource.ConsumerCount == 0 {
			add("warning", "lifecycle", "资源无消费者", "Topic/Queue 当前消费者数为 0", "确认是否为僵尸资源；生产核心资源应绑定消费方和告警", resource.ResourceType, resourceName, resource.Namespace, "consumer=0")
		}
		if resource.Backlog >= mqCriticalBacklogThreshold {
			add("critical", "capacity", "资源堆积严重", fmt.Sprintf("当前 Backlog %d", resource.Backlog), "立即排查消费者状态、下游依赖和分区热点", resource.ResourceType, resourceName, resource.Namespace, strconv.FormatInt(resource.Backlog, 10))
		} else if resource.Backlog >= mqWarningBacklogThreshold || resource.Backlog > 0 && isDLQOrRetry(resource.Name) {
			add("warning", "capacity", "资源存在堆积", fmt.Sprintf("当前 Backlog %d", resource.Backlog), "查看消费诊断并确认是否需要扩容或业务处理", resource.ResourceType, resourceName, resource.Namespace, strconv.FormatInt(resource.Backlog, 10))
		}
		if isDLQName(resource.Name) {
			add("warning", "lifecycle", "识别到 DLQ 资源", "死信资源需要负责人跟进处理闭环", "建立 DLQ 处理流程，避免长期堆积", resource.ResourceType, resourceName, resource.Namespace, "")
		} else if isRetryName(resource.Name) {
			add("info", "lifecycle", "识别到 Retry 资源", "重试资源需要关注增长趋势", "配置 retry 资源告警并确认关联原始资源", resource.ResourceType, resourceName, resource.Namespace, "")
		}
		uc.appendBaselineViolations(instance, resource, add)
	}
	groups := []*MQConsumerGroup{}
	if uc.consumerGroupRepo != nil {
		groups, _, _ = uc.consumerGroupRepo.List(ctx, instance.ID, &ConsumerGroupListRequest{Page: 1, PageSize: mqGovernanceMaxItems})
	}
	for _, group := range groups {
		if group == nil {
			continue
		}
		if group.Lag >= mqCriticalLagThreshold {
			add("critical", "consumer", "消费组延迟严重", fmt.Sprintf("当前 Lag %d", group.Lag), "检查消费者是否停滞、分区热点和下游处理耗时", "consumer_group", group.GroupName, group.Namespace, strconv.FormatInt(group.Lag, 10))
		} else if group.Lag >= mqWarningLagThreshold {
			add("warning", "consumer", "消费组存在延迟", fmt.Sprintf("当前 Lag %d", group.Lag), "观察消费速率并确认是否需要扩容消费者", "consumer_group", group.GroupName, group.Namespace, strconv.FormatInt(group.Lag, 10))
		}
		if group.ActiveConsumerCount == 0 && (group.Lag > 0 || group.Backlog > 0) {
			add("critical", "consumer", "消费停滞", "存在 lag/backlog 但无活跃消费者", "先恢复消费者，再评估是否需要 reset/skip 等高危操作", "consumer_group", group.GroupName, group.Namespace, fmt.Sprintf("lag=%d backlog=%d", group.Lag, group.Backlog))
		}
	}
	if uc.operationAuditRepo != nil {
		start := time.Now().Add(-7 * 24 * time.Hour).Format("2006-01-02 15:04:05")
		_, highTotal, _ := uc.operationAuditRepo.List(ctx, &AuditListRequest{Page: 1, PageSize: 1, InstanceID: instance.ID, RiskLevel: RiskLevelHigh, Status: AuditStatusFailed, StartTime: start})
		_, criticalTotal, _ := uc.operationAuditRepo.List(ctx, &AuditListRequest{Page: 1, PageSize: 1, InstanceID: instance.ID, RiskLevel: RiskLevelCritical, Status: AuditStatusFailed, StartTime: start})
		if highTotal+criticalTotal > 0 {
			add("warning", "audit", "近期存在高危操作失败", fmt.Sprintf("近 7 天高危/严重操作失败 %d 次", highTotal+criticalTotal), "查看操作审计并复盘失败原因", "audit", instance.Name, "", strconv.FormatInt(highTotal+criticalTotal, 10))
		}
	}
	return items
}

func (uc *UseCase) appendBaselineViolations(instance *MQInstance, resource *MQResource, add func(string, string, string, string, string, string, string, string, string)) {
	if instance == nil || resource == nil || strings.TrimSpace(instance.Environment) != "prod" {
		return
	}
	resourceName := firstNonEmpty(resource.FullName, resource.Name)
	switch NormalizeType(instance.MQType) {
	case MQTypeKafka:
		if resource.ResourceType == ResourceTypeTopic {
			if resource.ReplicaCount > 0 && resource.ReplicaCount < 3 {
				add("warning", "baseline", "生产 Topic 副本数偏低", fmt.Sprintf("当前副本数 %d，生产建议 >= 3", resource.ReplicaCount), "按业务等级调整副本数或记录豁免原因", resource.ResourceType, resourceName, resource.Namespace, strconv.Itoa(resource.ReplicaCount))
			}
			if resource.PartitionCount <= 0 {
				add("warning", "baseline", "Topic 分区信息缺失", "无法判断容量和分区热点", "重新同步元数据，确认分区数", resource.ResourceType, resourceName, resource.Namespace, "")
			}
			cfg := parseJSONStringMap(resource.ConfigJSON)
			retention := fmt.Sprint(cfg["retention.ms"])
			if retention == "" || retention == "<nil>" || retention == "-1" {
				add("warning", "baseline", "生产 Topic retention 未规范设置", "retention.ms 为空或永久保留会影响容量治理", "按业务基线设置 retention.ms，并记录特殊豁免", resource.ResourceType, resourceName, resource.Namespace, retention)
			}
			minISR := fmt.Sprint(cfg["min.insync.replicas"])
			if minISR == "" || minISR == "<nil>" {
				add("info", "baseline", "生产 Topic 未显式设置 min.insync.replicas", "写入可靠性基线缺少明确配置", "按副本数设置 min.insync.replicas", resource.ResourceType, resourceName, resource.Namespace, "")
			}
		}
	case MQTypeRabbitMQ:
		if resource.ResourceType == ResourceTypeQueue && !resource.Durable {
			add("warning", "baseline", "生产 Queue 非持久化", "durable=false 可能导致 Broker 重启后队列丢失", "核心生产队列建议 durable=true", resource.ResourceType, resourceName, resource.Namespace, "durable=false")
		}
	case MQTypePulsar:
		if resource.ResourceType == ResourceTypeNamespace {
			cfg := parseJSONStringMap(resource.ConfigJSON)
			if len(cfg) == 0 {
				add("info", "baseline", "Namespace 策略信息缺失", "无法判断 retention、TTL 和 backlog quota 基线", "重新同步 namespace 策略并补充基线", resource.ResourceType, resourceName, resource.Namespace, "")
			}
		}
	}
}

func normalizeGovernanceReportRequest(req *GovernanceReportRequest) {
	if req == nil {
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	req.Severity = strings.TrimSpace(req.Severity)
	req.Category = strings.TrimSpace(req.Category)
	req.Keyword = strings.TrimSpace(req.Keyword)
}

func filterGovernanceViolations(items []*GovernanceViolationVO, req *GovernanceReportRequest) []*GovernanceViolationVO {
	result := make([]*GovernanceViolationVO, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if req != nil {
			if req.Severity != "" && item.Severity != req.Severity {
				continue
			}
			if req.Category != "" && item.Category != req.Category {
				continue
			}
			if kw := strings.ToLower(strings.TrimSpace(req.Keyword)); kw != "" {
				text := strings.ToLower(strings.Join([]string{item.Title, item.Description, item.Suggestion, item.InstanceName, item.ResourceName, item.Namespace, item.Category}, " "))
				if !strings.Contains(text, kw) {
					continue
				}
			}
		}
		result = append(result, item)
	}
	return result
}

func toDashboardResource(instance *MQInstance, item *MQResource) *DashboardResourceVO {
	if instance == nil || item == nil {
		return nil
	}
	return &DashboardResourceVO{
		InstanceID:   instance.ID,
		InstanceName: instance.Name,
		MQType:       instance.MQType,
		ResourceType: item.ResourceType,
		Namespace:    item.Namespace,
		ResourceName: firstNonEmpty(item.FullName, item.Name),
		Backlog:      item.Backlog,
		MessageCount: item.MessageCount,
		Consumer:     item.ConsumerCount,
		LastSyncAt:   formatTimePtr(item.LastSyncAt),
	}
}

func toDashboardConsumerGroup(instance *MQInstance, item *MQConsumerGroup) *DashboardConsumerGroupVO {
	if instance == nil || item == nil {
		return nil
	}
	return &DashboardConsumerGroupVO{
		InstanceID:          instance.ID,
		InstanceName:        instance.Name,
		MQType:              instance.MQType,
		Namespace:           item.Namespace,
		ResourceName:        item.ResourceName,
		GroupName:           item.GroupName,
		Lag:                 item.Lag,
		Backlog:             item.Backlog,
		ConsumerCount:       item.ConsumerCount,
		ActiveConsumerCount: item.ActiveConsumerCount,
		LastSyncAt:          formatTimePtr(item.LastSyncAt),
	}
}

func (uc *UseCase) recentHighRiskAudits(ctx context.Context, req *GovernanceReportRequest) []*AuditVO {
	if uc.operationAuditRepo == nil {
		return nil
	}
	audits := make([]*MQOperationAudit, 0)
	for _, risk := range []string{RiskLevelCritical, RiskLevelHigh} {
		list, _, err := uc.operationAuditRepo.List(ctx, &AuditListRequest{
			Page:              1,
			PageSize:          5,
			RiskLevel:         risk,
			RestrictToAllowed: req != nil && req.RestrictToAllowed,
			AllowedIDs:        allowedIDsFromGovernanceReq(req),
		})
		if err == nil {
			audits = append(audits, list...)
		}
	}
	sort.Slice(audits, func(i, j int) bool { return audits[i].CreatedAt.After(audits[j].CreatedAt) })
	if len(audits) > 5 {
		audits = audits[:5]
	}
	result := make([]*AuditVO, 0, len(audits))
	for _, item := range audits {
		result = append(result, operationAuditToVO(item))
	}
	return result
}

func (uc *UseCase) recentFailedJobs(ctx context.Context, req *GovernanceReportRequest) []*JobVO {
	if uc.jobRepo == nil {
		return nil
	}
	list, _, err := uc.jobRepo.List(ctx, &JobListRequest{
		Page:              1,
		PageSize:          5,
		Status:            JobStatusFailed,
		RestrictToAllowed: req != nil && req.RestrictToAllowed,
		AllowedIDs:        allowedIDsFromGovernanceReq(req),
	})
	if err != nil {
		return nil
	}
	result := make([]*JobVO, 0, len(list))
	for _, item := range list {
		result = append(result, toJobVO(item))
	}
	return result
}

func countStats(counts map[string]int64, nameFn func(string) string) []*CountStatVO {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]*CountStatVO, 0, len(keys))
	for _, key := range keys {
		result = append(result, &CountStatVO{Key: key, Name: nameFn(key), Count: counts[key]})
	}
	return result
}

func environmentText(value string) string {
	switch strings.TrimSpace(value) {
	case "prod":
		return "生产"
	case "staging":
		return "预发"
	case "test":
		return "测试"
	case "dev":
		return "开发"
	case "unknown", "":
		return "未知"
	default:
		return value
	}
}

func severityRank(value string) int {
	switch value {
	case "critical":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

func resourceTopologyNodeID(item *MQResource) string {
	if item == nil {
		return ""
	}
	if item.ID > 0 {
		return fmt.Sprintf("resource:%d", item.ID)
	}
	return "resource:" + resourceLookupKey(item.Namespace, item.ResourceType, item.Name)
}

func resourceTopologyLabel(item *MQResource) string {
	if item == nil {
		return ""
	}
	return firstNonEmpty(item.FullName, item.Name)
}

func resourceLookupKey(namespace, resourceType, name string) string {
	return strings.ToLower(strings.Join([]string{strings.TrimSpace(namespace), strings.TrimSpace(resourceType), strings.TrimSpace(name)}, "\x00"))
}

func resourceStatus(item *MQResource) string {
	if item == nil {
		return ""
	}
	switch {
	case item.Backlog >= mqCriticalBacklogThreshold:
		return HealthStatusCritical
	case item.Backlog > 0 || item.ConsumerCount == 0 && isQueueOrTopic(item.ResourceType):
		return HealthStatusWarning
	default:
		return HealthStatusHealthy
	}
}

func normalizeDestinationType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ResourceTypeExchange:
		return ResourceTypeExchange
	default:
		return ResourceTypeQueue
	}
}

func findResourceNodeForGroup(resources map[string]string, group *MQConsumerGroup) string {
	if group == nil {
		return ""
	}
	for _, resourceType := range []string{ResourceTypeTopic, ResourceTypeQueue, ResourceTypeSubscription} {
		if id := resources[resourceLookupKey(group.Namespace, resourceType, group.ResourceName)]; id != "" {
			return id
		}
	}
	for key, id := range resources {
		if strings.HasSuffix(key, "\x00"+strings.ToLower(strings.TrimSpace(group.ResourceName))) {
			return id
		}
	}
	return ""
}

func isQueueOrTopic(resourceType string) bool {
	return resourceType == ResourceTypeQueue || resourceType == ResourceTypeTopic
}

func isDLQOrRetry(name string) bool {
	return isDLQName(name) || isRetryName(name)
}

func isDLQName(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "dlq") || strings.Contains(lower, "dead") || strings.Contains(lower, "dead-letter") || strings.Contains(lower, "dead_letter")
}

func isRetryName(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "retry") || strings.Contains(lower, "reconsume")
}

func parseJSONStringMap(value string) map[string]any {
	value = strings.TrimSpace(value)
	if value == "" {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return map[string]any{}
	}
	return result
}

func fallbackKey(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func containsUint(items []uint, target uint) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func allowedIDsFromGovernanceReq(req *GovernanceReportRequest) []uint {
	if req == nil {
		return nil
	}
	return req.AllowedIDs
}

func nowText() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
