package messagequeue

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

type KafkaAdapter struct{}

func NewKafkaAdapter() *KafkaAdapter {
	return &KafkaAdapter{}
}

func (a *KafkaAdapter) Type() string {
	return MQTypeKafka
}

func (a *KafkaAdapter) TestConnection(ctx context.Context, instance *MQInstance, credential *ConnectionCredential) (*MQConnectionTestResult, error) {
	conn, err := kafkaDial(ctx, instance)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	brokers, err := conn.Brokers()
	if err != nil {
		return nil, fmt.Errorf("读取Kafka Broker失败: %w", err)
	}
	version := "Kafka"
	if versions, err := conn.ApiVersions(); err == nil && len(versions) > 0 {
		version = fmt.Sprintf("Kafka API %d-%d", versions[0].MinVersion, versions[0].MaxVersion)
	}
	return &MQConnectionTestResult{
		Version:         version,
		Engine:          MQTypeKafka,
		BrokerCount:     len(brokers),
		ManagementReady: true,
		Message:         "Kafka Broker 连接成功",
	}, nil
}

func (a *KafkaAdapter) DiscoverMetadata(ctx context.Context, instance *MQInstance, credential *ConnectionCredential) (*MQMetadataSnapshot, error) {
	conn, err := kafkaDial(ctx, instance)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	now := time.Now()
	brokers, err := conn.Brokers()
	if err != nil {
		return nil, fmt.Errorf("读取Kafka Broker失败: %w", err)
	}
	partitions, err := conn.ReadPartitions()
	if err != nil {
		return nil, fmt.Errorf("读取Kafka Topic元数据失败: %w", err)
	}

	snapshot := &MQMetadataSnapshot{
		Version:      "Kafka",
		Engine:       MQTypeKafka,
		HealthStatus: HealthStatusHealthy,
		Message:      "Kafka 元数据同步成功",
		SyncedAt:     now,
	}
	for _, broker := range brokers {
		name := fmt.Sprintf("broker-%d", broker.ID)
		snapshot.Brokers = append(snapshot.Brokers, &MQBroker{
			InstanceID: instance.ID,
			BrokerName: name,
			BrokerID:   strconv.Itoa(broker.ID),
			Host:       broker.Host,
			Port:       broker.Port,
			Role:       "broker",
			Status:     "online",
			Rack:       broker.Rack,
			LastSyncAt: &now,
		})
	}

	topicPartitions := make(map[string][]kafka.Partition)
	for _, partition := range partitions {
		if strings.HasPrefix(partition.Topic, "__") {
			continue
		}
		topicPartitions[partition.Topic] = append(topicPartitions[partition.Topic], partition)
	}
	for topic, list := range topicPartitions {
		replicaCount := 0
		if len(list) > 0 {
			replicaCount = len(list[0].Replicas)
		}
		snapshot.Resources = append(snapshot.Resources, &MQResource{
			InstanceID:     instance.ID,
			ResourceType:   ResourceTypeTopic,
			Name:           topic,
			FullName:       topic,
			PartitionCount: len(list),
			ReplicaCount:   replicaCount,
			MetadataJSON:   mustJSON(list),
			LastSyncAt:     &now,
		})
		for _, partition := range list {
			snapshot.Partitions = append(snapshot.Partitions, &MQPartition{
				InstanceID:   instance.ID,
				ResourceName: topic,
				PartitionID:  partition.ID,
				Leader:       brokerName(partition.Leader),
				ReplicasJSON: mustJSON(partition.Replicas),
				ISRJSON:      mustJSON(partition.Isr),
				Status:       partitionStatus(partition),
				LastSyncAt:   &now,
			})
		}
	}
	return snapshot, nil
}

func (a *KafkaAdapter) SampleMessages(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *MessageSampleRequest) (*MessageSampleResultVO, error) {
	if req == nil || strings.TrimSpace(req.ResourceName) == "" {
		return nil, fmt.Errorf("Topic不能为空")
	}
	limit := req.Limit
	if limit <= 0 || limit > 10 {
		limit = 10
	}
	maxBytes := req.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 64 * 1024
	}
	brokers := kafkaBrokers(instance)
	if len(brokers) == 0 {
		return nil, fmt.Errorf("Kafka bootstrap地址不能为空")
	}
	partition := 0
	if req.PartitionID != nil {
		partition = *req.PartitionID
	}
	startOffset := kafka.FirstOffset
	if req.Offset != nil {
		startOffset = *req.Offset
	}
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       strings.TrimSpace(req.ResourceName),
		Partition:   partition,
		StartOffset: startOffset,
		MinBytes:    1,
		MaxBytes:    maxBytes,
		MaxWait:     2 * time.Second,
	})
	defer reader.Close()

	samples := make([]MessageSampleVO, 0, limit)
	readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for len(samples) < limit {
		message, err := reader.ReadMessage(readCtx)
		if err != nil {
			if len(samples) > 0 {
				break
			}
			return nil, fmt.Errorf("读取Kafka消息失败: %w", err)
		}
		payload, truncated, encoding := truncatePayload(message.Value, maxBytes)
		headers := make(map[string]string, len(message.Headers))
		for _, header := range message.Headers {
			value, _, _ := truncatePayload(header.Value, 512)
			headers[header.Key] = value
		}
		samples = append(samples, MessageSampleVO{
			Topic:       message.Topic,
			PartitionID: message.Partition,
			Offset:      message.Offset,
			Key:         string(message.Key),
			Timestamp:   message.Time.Format("2006-01-02 15:04:05"),
			Headers:     headers,
			Payload:     payload,
			PayloadSize: len(message.Value),
			Truncated:   truncated,
			Encoding:    encoding,
		})
	}
	return &MessageSampleResultVO{
		InstanceID:   instance.ID,
		MQType:       MQTypeKafka,
		ResourceName: strings.TrimSpace(req.ResourceName),
		Samples:      samples,
		SampleCount:  len(samples),
		Message:      "Kafka消息采样完成",
		SampledAt:    time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

func (a *KafkaAdapter) ValidateOperation(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	normalizeResourceOperationRequest(req)
	switch req.Action {
	case OperationActionKafkaTopicCreate:
		return a.validateTopicCreate(instance, req)
	case OperationActionKafkaPartitionsExpand:
		return a.validatePartitionsExpand(instance, req)
	case OperationActionKafkaTopicConfigUpdate:
		return a.validateTopicConfigUpdate(instance, req)
	case OperationActionKafkaTopicDelete:
		return a.validateTopicDelete(instance, req)
	default:
		return unsupportedOperationValidation(instance, req, "Kafka 不支持该资源操作"), nil
	}
}

func (a *KafkaAdapter) ApplyOperation(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *ResourceOperationRequest) (*ResourceOperationApplyResult, error) {
	validation, err := a.ValidateOperation(ctx, instance, credential, req)
	if err != nil {
		return nil, err
	}
	if !validation.Supported {
		return nil, fmt.Errorf("%s", validation.Message)
	}
	addr, err := kafkaControllerAddr(ctx, instance)
	if err != nil {
		return nil, err
	}
	client := kafkaClient(instance, addr)
	topic := validation.ResourceName
	params := validation.NormalizedParams
	switch req.Action {
	case OperationActionKafkaTopicCreate:
		topicConfig := kafka.TopicConfig{
			Topic:             topic,
			NumPartitions:     operationIntParam(params, 1, "partitions", "numPartitions"),
			ReplicationFactor: operationIntParam(params, 1, "replicationFactor"),
			ConfigEntries:     kafkaConfigEntries(operationStringMapParam(params, "configs")),
		}
		res, err := client.CreateTopics(ctx, &kafka.CreateTopicsRequest{Topics: []kafka.TopicConfig{topicConfig}})
		if err != nil {
			return nil, err
		}
		if err := kafkaTopicError(res.Errors, topic); err != nil {
			return nil, err
		}
		return &ResourceOperationApplyResult{
			ResourceType: ResourceTypeTopic,
			ResourceName: topic,
			Message:      "Kafka Topic 创建已提交",
			Result:       map[string]any{"topic": topic, "partitions": topicConfig.NumPartitions, "replicationFactor": topicConfig.ReplicationFactor, "configs": operationStringMapParam(params, "configs")},
		}, nil
	case OperationActionKafkaPartitionsExpand:
		count := operationIntParam(params, 0, "partitions", "count")
		res, err := client.CreatePartitions(ctx, &kafka.CreatePartitionsRequest{Topics: []kafka.TopicPartitionsConfig{{Name: topic, Count: int32(count)}}})
		if err != nil {
			return nil, err
		}
		if err := kafkaTopicError(res.Errors, topic); err != nil {
			return nil, err
		}
		return &ResourceOperationApplyResult{
			ResourceType: ResourceTypeTopic,
			ResourceName: topic,
			Message:      "Kafka Topic 分区扩容已提交",
			Result:       map[string]any{"topic": topic, "partitions": count},
		}, nil
	case OperationActionKafkaTopicConfigUpdate:
		configs := operationStringMapParam(params, "configs")
		reqConfigs := make([]kafka.IncrementalAlterConfigsRequestConfig, 0, len(configs))
		for key, value := range configs {
			reqConfigs = append(reqConfigs, kafka.IncrementalAlterConfigsRequestConfig{Name: key, Value: value, ConfigOperation: kafka.ConfigOperationSet})
		}
		res, err := client.IncrementalAlterConfigs(ctx, &kafka.IncrementalAlterConfigsRequest{
			Resources: []kafka.IncrementalAlterConfigsRequestResource{{
				ResourceType: kafka.ResourceTypeTopic,
				ResourceName: topic,
				Configs:      reqConfigs,
			}},
		})
		if err != nil {
			return nil, err
		}
		for _, resource := range res.Resources {
			if resource.ResourceName == topic && resource.Error != nil {
				return nil, resource.Error
			}
		}
		return &ResourceOperationApplyResult{
			ResourceType: ResourceTypeTopic,
			ResourceName: topic,
			Message:      "Kafka Topic 配置更新已提交",
			Result:       map[string]any{"topic": topic, "configs": configs},
		}, nil
	case OperationActionKafkaTopicDelete:
		res, err := client.DeleteTopics(ctx, &kafka.DeleteTopicsRequest{Topics: []string{topic}})
		if err != nil {
			return nil, err
		}
		if err := kafkaTopicError(res.Errors, topic); err != nil {
			return nil, err
		}
		return &ResourceOperationApplyResult{
			ResourceType: ResourceTypeTopic,
			ResourceName: topic,
			Message:      "Kafka Topic 删除已提交",
			Result:       map[string]any{"topic": topic},
		}, nil
	default:
		return nil, adapterNotSupported(instance.MQType, "资源操作")
	}
}

func (a *KafkaAdapter) validateTopicCreate(instance *MQInstance, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	validation := newOperationValidation(instance, req, RiskLevelLow)
	topic := firstNonEmpty(req.ResourceName, operationStringParam(req.Params, "topic", "name"))
	if err := ensureOperationRequired(topic, "Topic名称"); err != nil {
		return nil, err
	}
	partitions := operationIntParam(req.Params, 1, "partitions", "numPartitions")
	replicationFactor := operationIntParam(req.Params, 1, "replicationFactor")
	if partitions <= 0 {
		return nil, fmt.Errorf("分区数必须大于0")
	}
	if replicationFactor <= 0 {
		return nil, fmt.Errorf("副本数必须大于0")
	}
	configs := operationStringMapParam(req.Params, "configs")
	params := map[string]any{"partitions": partitions, "replicationFactor": replicationFactor, "configs": configs}
	req.ResourceType = ResourceTypeTopic
	req.ResourceName = topic
	validation.ResourceType = ResourceTypeTopic
	validation.ResourceName = topic
	validation.Message = "将通过 Kafka Controller 创建 Topic"
	validation.Impacts = []string{"Topic: " + topic, fmt.Sprintf("分区数: %d", partitions), fmt.Sprintf("副本数: %d", replicationFactor)}
	validation.Warnings = []string{"CreateTopics 具备幂等语义，已存在 Topic 通常不会重复创建"}
	setNormalizedParams(req, validation, params)
	return validation, nil
}

func (a *KafkaAdapter) validatePartitionsExpand(instance *MQInstance, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	validation := newOperationValidation(instance, req, RiskLevelMedium)
	topic := firstNonEmpty(req.ResourceName, operationStringParam(req.Params, "topic", "name"))
	if err := ensureOperationRequired(topic, "Topic名称"); err != nil {
		return nil, err
	}
	partitions := operationIntParam(req.Params, 0, "partitions", "count")
	if partitions <= 0 {
		return nil, fmt.Errorf("目标分区数必须大于0")
	}
	params := map[string]any{"partitions": partitions}
	req.ResourceType = ResourceTypeTopic
	req.ResourceName = topic
	validation.ResourceType = ResourceTypeTopic
	validation.ResourceName = topic
	validation.Message = "将通过 Kafka Controller 扩展 Topic 分区"
	validation.Impacts = []string{"Topic: " + topic, fmt.Sprintf("目标分区数: %d", partitions)}
	validation.Warnings = []string{"Kafka 分区只能增加不能减少，扩容会影响后续消息分布"}
	setNormalizedParams(req, validation, params)
	return validation, nil
}

func (a *KafkaAdapter) validateTopicConfigUpdate(instance *MQInstance, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	validation := newOperationValidation(instance, req, RiskLevelMedium)
	topic := firstNonEmpty(req.ResourceName, operationStringParam(req.Params, "topic", "name"))
	if err := ensureOperationRequired(topic, "Topic名称"); err != nil {
		return nil, err
	}
	configs := operationStringMapParam(req.Params, "configs")
	if len(configs) == 0 {
		return nil, fmt.Errorf("Topic配置不能为空")
	}
	for key := range configs {
		key = strings.TrimSpace(key)
		if !kafkaTopicConfigUpdateAllowed(key) {
			return nil, fmt.Errorf("Kafka Topic配置 %s 不在二期允许更新白名单内", key)
		}
	}
	params := map[string]any{"configs": configs}
	req.ResourceType = ResourceTypeTopic
	req.ResourceName = topic
	validation.ResourceType = ResourceTypeTopic
	validation.ResourceName = topic
	validation.Message = "将通过 Kafka IncrementalAlterConfigs 更新 Topic 配置"
	validation.Impacts = []string{"Topic: " + topic, fmt.Sprintf("配置项数量: %d", len(configs))}
	validation.Warnings = []string{"配置更新会影响 Topic 运行行为，请确认 retention、cleanup.policy 等关键参数"}
	setNormalizedParams(req, validation, params)
	return validation, nil
}

func kafkaTopicConfigUpdateAllowed(key string) bool {
	switch strings.TrimSpace(key) {
	case "retention.ms",
		"retention.bytes",
		"cleanup.policy",
		"compression.type",
		"max.message.bytes",
		"min.insync.replicas",
		"segment.ms":
		return true
	default:
		return false
	}
}

func (a *KafkaAdapter) validateTopicDelete(instance *MQInstance, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	validation := newOperationValidation(instance, req, RiskLevelHigh)
	topic := firstNonEmpty(req.ResourceName, operationStringParam(req.Params, "topic", "name"))
	if err := ensureOperationRequired(topic, "Topic名称"); err != nil {
		return nil, err
	}
	params := map[string]any{"topic": topic}
	req.ResourceType = ResourceTypeTopic
	req.ResourceName = topic
	validation.RequiredPermission = PermissionHighRisk
	validation.ResourceType = ResourceTypeTopic
	validation.ResourceName = topic
	validation.Message = "将通过 Kafka Controller 删除 Topic"
	validation.Impacts = []string{"Topic: " + topic, "Topic 删除后分区、消息和消费位点关联信息会失效"}
	validation.Warnings = []string{"删除 Topic 依赖 broker 开启 delete.topic.enable，执行后可能异步完成"}
	setNormalizedParams(req, validation, params)
	return validation, nil
}

func kafkaDial(ctx context.Context, instance *MQInstance) (*kafka.Conn, error) {
	address := firstEndpoint(instance, DefaultPort(MQTypeKafka))
	if address == "" {
		return nil, fmt.Errorf("Kafka bootstrap地址不能为空")
	}
	dialer := &kafka.Dialer{Timeout: 10 * time.Second, ClientID: "opshub-messagequeue"}
	if instance.TLSEnabled {
		dialer.TLS = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("连接Kafka失败: %w", err)
	}
	return conn, nil
}

func kafkaControllerAddr(ctx context.Context, instance *MQInstance) (net.Addr, error) {
	conn, err := kafkaDial(ctx, instance)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	controller, err := conn.Controller()
	if err != nil {
		return nil, fmt.Errorf("读取Kafka Controller失败: %w", err)
	}
	if strings.TrimSpace(controller.Host) == "" || controller.Port <= 0 {
		return kafka.TCP(firstEndpoint(instance, DefaultPort(MQTypeKafka))), nil
	}
	return kafka.TCP(net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port))), nil
}

func kafkaClient(instance *MQInstance, addr net.Addr) *kafka.Client {
	client := &kafka.Client{Addr: addr, Timeout: 10 * time.Second}
	if instance.TLSEnabled {
		client.Transport = &kafka.Transport{
			ClientID: "opshub-messagequeue",
			TLS:      &tls.Config{MinVersion: tls.VersionTLS12},
		}
	}
	return client
}

func kafkaConfigEntries(configs map[string]string) []kafka.ConfigEntry {
	entries := make([]kafka.ConfigEntry, 0, len(configs))
	for key, value := range configs {
		entries = append(entries, kafka.ConfigEntry{ConfigName: key, ConfigValue: value})
	}
	return entries
}

func kafkaTopicError(errors map[string]error, topic string) error {
	if len(errors) == 0 {
		return nil
	}
	if err := errors[topic]; err != nil {
		return err
	}
	return nil
}

func kafkaBrokers(instance *MQInstance) []string {
	endpoints := splitEndpoints(instance.Endpoint)
	result := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		result = append(result, normalizeAddress(endpoint, DefaultPort(MQTypeKafka)))
	}
	return result
}

func brokerName(broker kafka.Broker) string {
	host := broker.Host
	if host == "" {
		host = "broker"
	}
	if broker.Port > 0 {
		host = net.JoinHostPort(host, strconv.Itoa(broker.Port))
	}
	return fmt.Sprintf("%s#%d", host, broker.ID)
}

func partitionStatus(partition kafka.Partition) string {
	if partition.Error != nil {
		return "error"
	}
	if len(partition.OfflineReplicas) > 0 {
		return "degraded"
	}
	return "online"
}
