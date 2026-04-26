// Copyright (c) 2026 DYCloud J.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package asset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"
)

type VirtualizationUseCase struct {
	platformRepo      VirtualizationPlatformRepo
	clusterRepo       VirtualizationClusterRepo
	hostRepo          VirtualizationHostRepo
	guestRepo         VirtualizationGuestRepo
	bindingRepo       VirtualizationGuestBindingRepo
	syncJobRepo       VirtualizationSyncJobRepo
	metricRepo        VirtualizationPlatformMetricRepo
	clusterMetricRepo VirtualizationClusterMetricRepo
	actionLogRepo     VirtualizationActionLogRepo
	policyRepo        VirtualizationPolicyRepo
	assetHostRepo     HostRepo
}

func NewVirtualizationUseCase(
	platformRepo VirtualizationPlatformRepo,
	clusterRepo VirtualizationClusterRepo,
	hostRepo VirtualizationHostRepo,
	guestRepo VirtualizationGuestRepo,
	bindingRepo VirtualizationGuestBindingRepo,
	syncJobRepo VirtualizationSyncJobRepo,
	metricRepo VirtualizationPlatformMetricRepo,
	clusterMetricRepo VirtualizationClusterMetricRepo,
	actionLogRepo VirtualizationActionLogRepo,
	policyRepo VirtualizationPolicyRepo,
	assetHostRepo HostRepo,
) *VirtualizationUseCase {
	return &VirtualizationUseCase{
		platformRepo:      platformRepo,
		clusterRepo:       clusterRepo,
		hostRepo:          hostRepo,
		guestRepo:         guestRepo,
		bindingRepo:       bindingRepo,
		syncJobRepo:       syncJobRepo,
		metricRepo:        metricRepo,
		clusterMetricRepo: clusterMetricRepo,
		actionLogRepo:     actionLogRepo,
		policyRepo:        policyRepo,
		assetHostRepo:     assetHostRepo,
	}
}

func (uc *VirtualizationUseCase) CreatePlatform(ctx context.Context, req *VirtualizationPlatformRequest) (*VirtualizationPlatformVO, error) {
	if strings.TrimSpace(req.Password) == "" {
		return nil, fmt.Errorf("密码不能为空")
	}

	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "enabled"
	}

	port := req.Port
	if port == 0 {
		port = defaultVirtualizationPort(req.Provider)
	}

	item := &VirtualizationPlatform{
		Name:               strings.TrimSpace(req.Name),
		Provider:           strings.TrimSpace(req.Provider),
		Endpoint:           strings.TrimSpace(req.Endpoint),
		Port:               port,
		Username:           strings.TrimSpace(req.Username),
		Password:           req.Password,
		InsecureSkipVerify: req.InsecureSkipVerify,
		Status:             status,
		LastSyncStatus:     "idle",
		Description:        strings.TrimSpace(req.Description),
	}

	if err := uc.platformRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	return uc.toPlatformVO(item), nil
}

func (uc *VirtualizationUseCase) UpdatePlatform(ctx context.Context, req *VirtualizationPlatformRequest) error {
	item, err := uc.platformRepo.GetByID(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("虚拟化平台不存在")
	}

	item.Name = strings.TrimSpace(req.Name)
	item.Provider = strings.TrimSpace(req.Provider)
	item.Endpoint = strings.TrimSpace(req.Endpoint)
	if req.Port > 0 {
		item.Port = req.Port
	}
	item.Username = strings.TrimSpace(req.Username)
	if strings.TrimSpace(req.Password) != "" {
		item.Password = req.Password
	}
	item.InsecureSkipVerify = req.InsecureSkipVerify
	if status := strings.TrimSpace(req.Status); status != "" {
		item.Status = status
	}
	item.Description = strings.TrimSpace(req.Description)

	return uc.platformRepo.Update(ctx, item)
}

func (uc *VirtualizationUseCase) DeletePlatform(ctx context.Context, id uint) error {
	return uc.platformRepo.Delete(ctx, id)
}

func (uc *VirtualizationUseCase) GetPlatformByID(ctx context.Context, id uint) (*VirtualizationPlatformVO, error) {
	item, err := uc.platformRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return uc.toPlatformVO(item), nil
}

func (uc *VirtualizationUseCase) ListPlatforms(ctx context.Context, page, pageSize int, keyword string) ([]*VirtualizationPlatformVO, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	items, total, err := uc.platformRepo.List(ctx, page, pageSize, keyword)
	if err != nil {
		return nil, 0, err
	}

	list := make([]*VirtualizationPlatformVO, 0, len(items))
	for _, item := range items {
		list = append(list, uc.toPlatformVO(item))
	}
	return list, total, nil
}

func (uc *VirtualizationUseCase) GetSettings(ctx context.Context) (*VirtualizationSettingsVO, error) {
	policy, err := uc.getConflictPolicy(ctx)
	if err != nil {
		return nil, err
	}
	writeEnabled, err := uc.getWriteOperationsEnabled(ctx)
	if err != nil {
		return nil, err
	}
	return &VirtualizationSettingsVO{
		ConflictPolicy:             policy,
		ConflictPolicyText:         virtualizationConflictPolicyText(policy),
		WriteOperationsEnabled:     writeEnabled,
		WriteOperationsEnabledText: virtualizationWriteOperationsEnabledText(writeEnabled),
	}, nil
}

func (uc *VirtualizationUseCase) UpdateSettings(ctx context.Context, req *VirtualizationSettingsRequest) (*VirtualizationSettingsVO, error) {
	if req == nil {
		return nil, fmt.Errorf("参数不能为空")
	}
	policy := strings.TrimSpace(req.ConflictPolicy)
	switch policy {
	case VirtualizationOnboardPolicyStrict, VirtualizationOnboardPolicyWarn:
	default:
		return nil, fmt.Errorf("不支持的冲突策略: %s", policy)
	}

	if err := uc.policyRepo.SaveOnboardConflictPolicy(ctx, policy); err != nil {
		return nil, err
	}
	if err := uc.policyRepo.SaveWriteOperationsEnabled(ctx, req.WriteOperationsEnabled); err != nil {
		return nil, err
	}
	return uc.GetSettings(ctx)
}

func (uc *VirtualizationUseCase) TestPlatformConnection(ctx context.Context, id uint) error {
	item, err := uc.platformRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("虚拟化平台不存在")
	}

	target, err := uc.buildPlatformDialAddr(item)
	if err != nil {
		return err
	}

	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return fmt.Errorf("连接测试失败: %w", err)
	}
	_ = conn.Close()
	return nil
}

func (uc *VirtualizationUseCase) TriggerPlatformSync(ctx context.Context, id uint, operatorID uint) (*VirtualizationSyncJobVO, error) {
	platform, err := uc.platformRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("虚拟化平台不存在")
	}

	now := time.Now()
	job := &VirtualizationSyncJob{
		PlatformID:  platform.ID,
		TriggerType: "manual",
		Status:      "running",
		StartedAt:   &now,
		OperatorID:  operatorID,
	}
	if err := uc.syncJobRepo.Create(ctx, job); err != nil {
		return nil, err
	}

	platform.LastSyncStatus = "running"
	platform.LastSyncMessage = "同步任务执行中"
	_ = uc.platformRepo.Update(ctx, platform)

	adapter, err := newVirtualizationAdapter(platform.Provider)
	if err != nil {
		return nil, err
	}

	snapshot, err := adapter.Collect(ctx, platform)
	if err != nil {
		finishedAt := time.Now()
		job.Status = "failed"
		job.FinishedAt = &finishedAt
		job.FailureReason = err.Error()
		_ = uc.syncJobRepo.Update(ctx, job)

		platform.LastSyncAt = &finishedAt
		platform.LastSyncStatus = "failed"
		platform.LastSyncMessage = err.Error()
		_ = uc.platformRepo.Update(ctx, platform)
		return nil, err
	}

	itemsTotal, err := uc.applySyncSnapshot(ctx, platform.ID, snapshot)
	if err != nil {
		finishedAt := time.Now()
		job.Status = "failed"
		job.FinishedAt = &finishedAt
		job.FailureReason = err.Error()
		_ = uc.syncJobRepo.Update(ctx, job)

		platform.LastSyncAt = &finishedAt
		platform.LastSyncStatus = "failed"
		platform.LastSyncMessage = err.Error()
		_ = uc.platformRepo.Update(ctx, platform)
		return nil, err
	}

	finishedAt := time.Now()
	if err := uc.recordPlatformMetricSnapshot(ctx, platform.ID, finishedAt); err != nil {
		job.Status = "failed"
		job.FinishedAt = &finishedAt
		job.FailureReason = err.Error()
		_ = uc.syncJobRepo.Update(ctx, job)

		platform.LastSyncAt = &finishedAt
		platform.LastSyncStatus = "failed"
		platform.LastSyncMessage = err.Error()
		_ = uc.platformRepo.Update(ctx, platform)
		return nil, err
	}
	if err := uc.recordClusterMetricSnapshots(ctx, platform.ID, finishedAt); err != nil {
		job.Status = "failed"
		job.FinishedAt = &finishedAt
		job.FailureReason = err.Error()
		_ = uc.syncJobRepo.Update(ctx, job)

		platform.LastSyncAt = &finishedAt
		platform.LastSyncStatus = "failed"
		platform.LastSyncMessage = err.Error()
		_ = uc.platformRepo.Update(ctx, platform)
		return nil, err
	}

	job.Status = "success"
	job.FinishedAt = &finishedAt
	job.ItemsTotal = itemsTotal
	job.ItemsCreated = 0
	job.ItemsUpdated = itemsTotal
	job.ItemsDeleted = 0
	job.FailureReason = ""
	if err := uc.syncJobRepo.Update(ctx, job); err != nil {
		return nil, err
	}

	platform.LastSyncAt = &finishedAt
	platform.LastSyncStatus = "success"
	platform.LastSyncMessage = fmt.Sprintf("同步完成：集群 %d，宿主机 %d，虚机 %d", len(snapshot.Clusters), len(snapshot.Hosts), len(snapshot.Guests))
	if err := uc.platformRepo.Update(ctx, platform); err != nil {
		return nil, err
	}

	return uc.toSyncJobVO(job), nil
}

func (uc *VirtualizationUseCase) ListPlatformSyncJobs(ctx context.Context, platformID uint, page, pageSize int) ([]*VirtualizationSyncJobVO, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	_, err := uc.platformRepo.GetByID(ctx, platformID)
	if err != nil {
		return nil, 0, fmt.Errorf("虚拟化平台不存在")
	}

	items, total, err := uc.syncJobRepo.ListByPlatformID(ctx, platformID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	list := make([]*VirtualizationSyncJobVO, 0, len(items))
	for _, item := range items {
		list = append(list, uc.toSyncJobVO(item))
	}
	return list, total, nil
}

func (uc *VirtualizationUseCase) GetPlatformTrend(ctx context.Context, platformID uint, rangeKey string) (*VirtualizationPlatformTrendVO, error) {
	platform, err := uc.platformRepo.GetByID(ctx, platformID)
	if err != nil {
		return nil, fmt.Errorf("虚拟化平台不存在")
	}

	window := resolveVirtualizationTrendWindow(rangeKey)
	result := &VirtualizationPlatformTrendVO{
		ScopeType:  "platform",
		ScopeID:    platformID,
		ScopeName:  platform.Name,
		PlatformID: platformID,
		Range:      window.Key,
		Start:      window.Start.Format("2006-01-02 15:04:05"),
		End:        window.End.Format("2006-01-02 15:04:05"),
		Points:     make([]*VirtualizationPlatformTrendPointVO, 0),
	}

	if uc.metricRepo == nil {
		return result, nil
	}

	items, err := uc.metricRepo.ListByPlatformID(ctx, platformID, window.Start, window.End)
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		current, snapshotErr := uc.buildPlatformMetricSnapshot(ctx, platformID, window.End)
		if snapshotErr == nil && current != nil {
			items = []*VirtualizationPlatformMetric{current}
		}
	}

	for _, item := range items {
		if item == nil || item.CollectedAt == nil {
			continue
		}
		result.Points = append(result.Points, &VirtualizationPlatformTrendPointVO{
			Timestamp:           item.CollectedAt.Unix(),
			Time:                item.CollectedAt.Format("2006-01-02 15:04:05"),
			GuestTotal:          item.GuestTotal,
			PoweredOnGuests:     item.PoweredOnGuests,
			PoweredOffGuests:    item.PoweredOffGuests,
			SuspendedGuests:     item.SuspendedGuests,
			BoundGuests:         item.BoundGuests,
			OnlineGuests:        item.OnlineGuests,
			OfflineGuests:       item.OfflineGuests,
			NotConfiguredGuests: item.NotConfiguredGuests,
			UnknownGuests:       item.UnknownGuests,
		})
	}

	return result, nil
}

func (uc *VirtualizationUseCase) GetClusterTrend(ctx context.Context, clusterID uint, rangeKey string) (*VirtualizationPlatformTrendVO, error) {
	cluster, err := uc.clusterRepo.GetByID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("虚拟化集群不存在")
	}

	window := resolveVirtualizationTrendWindow(rangeKey)
	result := &VirtualizationPlatformTrendVO{
		ScopeType:  "cluster",
		ScopeID:    clusterID,
		ScopeName:  cluster.Name,
		PlatformID: cluster.PlatformID,
		ClusterID:  clusterID,
		Range:      window.Key,
		Start:      window.Start.Format("2006-01-02 15:04:05"),
		End:        window.End.Format("2006-01-02 15:04:05"),
		Points:     make([]*VirtualizationPlatformTrendPointVO, 0),
	}

	if uc.clusterMetricRepo == nil {
		return result, nil
	}

	items, err := uc.clusterMetricRepo.ListByClusterID(ctx, clusterID, window.Start, window.End)
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		current, snapshotErr := uc.buildClusterMetricSnapshot(ctx, cluster, window.End)
		if snapshotErr == nil && current != nil {
			items = []*VirtualizationClusterMetric{current}
		}
	}

	for _, item := range items {
		if item == nil || item.CollectedAt == nil {
			continue
		}
		result.Points = append(result.Points, &VirtualizationPlatformTrendPointVO{
			Timestamp:           item.CollectedAt.Unix(),
			Time:                item.CollectedAt.Format("2006-01-02 15:04:05"),
			GuestTotal:          item.GuestTotal,
			PoweredOnGuests:     item.PoweredOnGuests,
			PoweredOffGuests:    item.PoweredOffGuests,
			SuspendedGuests:     item.SuspendedGuests,
			BoundGuests:         item.BoundGuests,
			OnlineGuests:        item.OnlineGuests,
			OfflineGuests:       item.OfflineGuests,
			NotConfiguredGuests: item.NotConfiguredGuests,
			UnknownGuests:       item.UnknownGuests,
		})
	}

	return result, nil
}

func (uc *VirtualizationUseCase) GetTopology(ctx context.Context, platformID uint) (*VirtualizationTopologyVO, error) {
	var platforms []*VirtualizationPlatform

	if platformID > 0 {
		item, err := uc.platformRepo.GetByID(ctx, platformID)
		if err != nil {
			return nil, fmt.Errorf("虚拟化平台不存在")
		}
		platforms = append(platforms, item)
	} else {
		items, _, err := uc.platformRepo.List(ctx, 1, 1000, "")
		if err != nil {
			return nil, err
		}
		platforms = items
	}

	result := &VirtualizationTopologyVO{
		Platforms: make([]*VirtualizationTopologyPlatformNode, 0, len(platforms)),
	}

	for _, platform := range platforms {
		clusters, err := uc.clusterRepo.ListByPlatformID(ctx, platform.ID)
		if err != nil {
			return nil, err
		}
		hosts, err := uc.hostRepo.ListByPlatformID(ctx, platform.ID)
		if err != nil {
			return nil, err
		}

		clusterMap := make(map[uint]*VirtualizationTopologyClusterNode, len(clusters))
		clusterNodes := make([]*VirtualizationTopologyClusterNode, 0, len(clusters)+1)

		for _, cluster := range clusters {
			node := &VirtualizationTopologyClusterNode{
				ID:         cluster.ID,
				Name:       cluster.Name,
				Datacenter: cluster.Datacenter,
				Status:     cluster.Status,
				HostCount:  cluster.HostCount,
				GuestCount: cluster.GuestCount,
				Hosts:      make([]*VirtualizationTopologyHostNode, 0),
			}
			clusterMap[cluster.ID] = node
			clusterNodes = append(clusterNodes, node)
		}

		orphan := &VirtualizationTopologyClusterNode{
			ID:         0,
			Name:       "未归属集群",
			Datacenter: "",
			Status:     "unknown",
			HostCount:  0,
			GuestCount: 0,
			Hosts:      make([]*VirtualizationTopologyHostNode, 0),
		}

		for _, host := range hosts {
			node := &VirtualizationTopologyHostNode{
				ID:           host.ID,
				Name:         host.Name,
				ClusterID:    host.ClusterID,
				ManagementIP: host.ManagementIP,
				Status:       host.Status,
				GuestCount:   host.GuestCount,
			}
			if host.ClusterID > 0 {
				if clusterNode, ok := clusterMap[host.ClusterID]; ok {
					clusterNode.Hosts = append(clusterNode.Hosts, node)
					continue
				}
			}
			orphan.Hosts = append(orphan.Hosts, node)
		}

		if len(orphan.Hosts) > 0 {
			orphan.HostCount = len(orphan.Hosts)
			clusterNodes = append(clusterNodes, orphan)
		}

		result.Platforms = append(result.Platforms, &VirtualizationTopologyPlatformNode{
			ID:           platform.ID,
			Name:         platform.Name,
			Provider:     platform.Provider,
			ProviderText: virtualizationProviderText(platform.Provider),
			Status:       platform.Status,
			LastSyncAt:   formatTime(platform.LastSyncAt),
			Clusters:     clusterNodes,
		})
	}

	return result, nil
}

func (uc *VirtualizationUseCase) ListGuests(ctx context.Context, req *VirtualizationGuestListRequest) ([]*VirtualizationGuestVO, int64, error) {
	if req == nil {
		req = &VirtualizationGuestListRequest{}
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.Bound == "" {
		req.Bound = "all"
	}

	items, total, err := uc.guestRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	list := make([]*VirtualizationGuestVO, 0, len(items))
	for _, item := range items {
		vo := uc.toGuestVO(item)
		binding, host := uc.findActiveBindingHost(ctx, item.ID)
		if binding != nil {
			vo.AssetHostID = binding.AssetHostID
			if host != nil {
				vo.AssetHostName = host.Name
			}
			uc.enrichGuestVOFromBindingHost(vo, item, host)
		}
		list = append(list, vo)
	}

	return list, total, nil
}

func (uc *VirtualizationUseCase) GetGuestByID(ctx context.Context, id uint) (*VirtualizationGuestDetailVO, error) {
	item, err := uc.guestRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("虚机不存在")
	}

	detail := &VirtualizationGuestDetailVO{
		Guest: uc.toGuestVO(item),
	}

	binding, host := uc.findActiveBindingHost(ctx, item.ID)
	if binding != nil {
		detail.Binding = uc.toBindingVO(binding)
		detail.Guest.AssetHostID = binding.AssetHostID
		if host != nil {
			detail.Guest.AssetHostName = host.Name
		}
		uc.enrichGuestVOFromBindingHost(detail.Guest, item, host)
	}

	return detail, nil
}

func (uc *VirtualizationUseCase) PrecheckGuestBinding(ctx context.Context, guestID uint, assetHostID uint) (*VirtualizationGuestOnboardPrecheckVO, error) {
	guest, err := uc.guestRepo.GetByID(ctx, guestID)
	if err != nil {
		return nil, fmt.Errorf("虚机不存在")
	}

	result := &VirtualizationGuestOnboardPrecheckVO{
		GuestID:        guest.ID,
		GuestName:      guest.Name,
		PrimaryIP:      strings.TrimSpace(guest.PrimaryIP),
		ConflictPolicy: VirtualizationOnboardPolicyStrict,
		RiskLevel:      "none",
		CandidateHosts: make([]*VirtualizationPrecheckHostVO, 0),
		Checks:         make([]*VirtualizationPrecheckCheckVO, 0),
	}

	if policy, policyErr := uc.getConflictPolicy(ctx); policyErr == nil {
		result.ConflictPolicy = policy
	}

	addCheck := func(code, level, message string) {
		result.Checks = append(result.Checks, &VirtualizationPrecheckCheckVO{
			Code:    code,
			Level:   level,
			Message: message,
		})
		result.RiskLevel = mergeRiskLevel(result.RiskLevel, level)
	}

	hostByID := make(map[uint]*Host)
	candidateByID := make(map[uint]*VirtualizationPrecheckHostVO)
	appendCandidate := func(host *Host, matchType string) {
		if host == nil || host.ID == 0 {
			return
		}
		if existing, ok := candidateByID[host.ID]; ok {
			existing.MatchType = mergeHostMatchType(existing.MatchType, matchType)
			return
		}
		vo := &VirtualizationPrecheckHostVO{
			ID:        host.ID,
			Name:      host.Name,
			IP:        host.IP,
			MatchType: matchType,
		}
		candidateByID[host.ID] = vo
		hostByID[host.ID] = host
		result.CandidateHosts = append(result.CandidateHosts, vo)
	}

	var ipMatchedHost *Host
	if ip := strings.TrimSpace(guest.PrimaryIP); ip != "" {
		host, hostErr := uc.assetHostRepo.GetByIP(ctx, ip)
		if hostErr == nil && host != nil {
			ipMatchedHost = host
			appendCandidate(host, "ip_exact")
			addCheck("ip_exact_match", "low", fmt.Sprintf("发现同IP资产主机：%s(%s)", host.Name, host.IP))
		} else {
			addCheck("ip_not_found", "medium", fmt.Sprintf("未找到IP为 %s 的资产主机，请确认后再纳管", ip))
		}
	} else {
		addCheck("guest_ip_missing", "medium", "虚机缺少主IP，无法做IP冲突校验")
	}

	if guestName := strings.TrimSpace(guest.Name); guestName != "" {
		hosts, _, listErr := uc.assetHostRepo.List(ctx, 1, 200, guestName, nil, nil, nil)
		if listErr == nil {
			matchCount := 0
			for _, host := range hosts {
				if strings.EqualFold(strings.TrimSpace(host.Name), guestName) {
					matchCount++
					appendCandidate(host, "name_exact")
				}
			}
			if matchCount > 1 {
				addCheck("name_conflict", "high", fmt.Sprintf("检测到 %d 台同名资产主机，存在同名冲突", matchCount))
			} else if matchCount == 1 {
				addCheck("name_exact_match", "low", "检测到同名资产主机，可作为纳管候选")
			}
		}
	}

	activeBinding, activeHost := uc.findActiveBindingHost(ctx, guest.ID)
	if activeBinding != nil {
		result.CurrentBinding = &VirtualizationPrecheckHostVO{
			ID:   activeBinding.AssetHostID,
			Name: "",
			IP:   "",
		}
		if activeHost != nil {
			result.CurrentBinding.Name = activeHost.Name
			result.CurrentBinding.IP = activeHost.IP
		}
	}

	if assetHostID > 0 {
		selectedHost, hostErr := uc.assetHostRepo.GetByID(ctx, assetHostID)
		if hostErr != nil || selectedHost == nil {
			return nil, fmt.Errorf("目标资产主机不存在")
		}
		result.SelectedHost = &VirtualizationPrecheckHostVO{
			ID:   selectedHost.ID,
			Name: selectedHost.Name,
			IP:   selectedHost.IP,
		}

		if activeBinding != nil && activeBinding.AssetHostID != selectedHost.ID {
			addCheck("already_bound_other", "high", fmt.Sprintf("当前虚机已绑定资产主机 ID=%d，改绑请谨慎确认", activeBinding.AssetHostID))
		}

		if ipMatchedHost != nil && ipMatchedHost.ID != selectedHost.ID {
			addCheck("selected_host_ip_mismatch", "high", fmt.Sprintf("所选主机(%s)与虚机IP匹配主机(%s)不一致", selectedHost.Name, ipMatchedHost.Name))
		}

		if ipMatchedHost != nil && ipMatchedHost.ID == selectedHost.ID {
			addCheck("selected_host_ip_exact", "low", "所选主机与虚机IP一致，建议直接纳管")
		}

		if ipMatchedHost == nil {
			addCheck("selected_host_manual_confirm", "medium", "未命中IP匹配，请手工确认该主机是否正确")
		}
	}

	result.SuggestedHost = uc.pickSuggestedPrecheckHost(result.CandidateHosts)
	if result.SuggestedHost == nil {
		addCheck("no_suggested_host", "medium", "未找到自动建议主机，请人工选择纳管目标")
	}

	return result, nil
}

func (uc *VirtualizationUseCase) BindGuest(ctx context.Context, guestID uint, req *VirtualizationGuestBindingRequest, operatorID uint) (*VirtualizationGuestBindingVO, error) {
	if req.AssetHostID == 0 {
		return nil, fmt.Errorf("资产主机ID不能为空")
	}

	guest, err := uc.guestRepo.GetByID(ctx, guestID)
	if err != nil {
		return nil, fmt.Errorf("虚机不存在")
	}

	selectedHost, err := uc.assetHostRepo.GetByID(ctx, req.AssetHostID)
	if err != nil || selectedHost == nil {
		return nil, fmt.Errorf("目标资产主机不存在")
	}

	if err := uc.checkBindAllowedByPolicy(ctx, guestID, req.AssetHostID); err != nil {
		return nil, err
	}

	now := time.Now()
	bindingType := strings.TrimSpace(req.BindingType)
	if bindingType == "" {
		bindingType = "manual"
	}

	binding, err := uc.bindingRepo.GetActiveByGuestID(ctx, guestID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) || binding == nil {
		// 兼容唯一键 guest_id：若存在历史 inactive 绑定，则直接复用更新，避免重复插入冲突。
		existing, findErr := uc.bindingRepo.GetByGuestID(ctx, guestID)
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return nil, findErr
		}
		if existing == nil || errors.Is(findErr, gorm.ErrRecordNotFound) {
			binding = &VirtualizationGuestBinding{
				GuestID:     guestID,
				AssetHostID: req.AssetHostID,
				BindingType: bindingType,
				Status:      "active",
				BoundBy:     operatorID,
				BoundAt:     &now,
				BindingNote: strings.TrimSpace(req.BindingNote),
			}
			if err := uc.bindingRepo.Create(ctx, binding); err != nil {
				return nil, err
			}
		} else {
			binding = existing
			binding.AssetHostID = req.AssetHostID
			binding.BindingType = bindingType
			binding.Status = "active"
			binding.BoundBy = operatorID
			binding.BoundAt = &now
			binding.UnboundAt = nil
			binding.BindingNote = strings.TrimSpace(req.BindingNote)
			if err := uc.bindingRepo.Update(ctx, binding); err != nil {
				return nil, err
			}
		}
	} else {
		binding.AssetHostID = req.AssetHostID
		binding.BindingType = bindingType
		binding.Status = "active"
		binding.BoundBy = operatorID
		binding.BoundAt = &now
		binding.UnboundAt = nil
		binding.BindingNote = strings.TrimSpace(req.BindingNote)
		if err := uc.bindingRepo.Update(ctx, binding); err != nil {
			return nil, err
		}
	}

	guest.BindingStatus = "bound"
	uc.hydrateGuestMetadataFromHost(guest, selectedHost)
	if err := uc.guestRepo.UpsertBatch(ctx, []*VirtualizationGuest{guest}); err != nil {
		return nil, err
	}

	return uc.toBindingVO(binding), nil
}

func (uc *VirtualizationUseCase) UnbindGuest(ctx context.Context, guestID uint, req *VirtualizationGuestUnbindRequest) error {
	guest, err := uc.guestRepo.GetByID(ctx, guestID)
	if err != nil {
		return fmt.Errorf("虚机不存在")
	}

	binding, err := uc.bindingRepo.GetActiveByGuestID(ctx, guestID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("该虚机尚未纳管")
		}
		return err
	}

	now := time.Now()
	binding.Status = "inactive"
	binding.UnboundAt = &now
	if req != nil {
		binding.BindingNote = strings.TrimSpace(req.BindingNote)
	}
	if err := uc.bindingRepo.Update(ctx, binding); err != nil {
		return err
	}

	guest.BindingStatus = "unbound"
	return uc.guestRepo.UpsertBatch(ctx, []*VirtualizationGuest{guest})
}

func (uc *VirtualizationUseCase) PowerGuest(ctx context.Context, guestID uint, req *VirtualizationGuestPowerRequest, operatorID uint, operatorName string) (*VirtualizationGuestPowerVO, error) {
	if req == nil {
		return nil, fmt.Errorf("请求参数不能为空")
	}

	guest, err := uc.guestRepo.GetByID(ctx, guestID)
	if err != nil {
		return nil, fmt.Errorf("虚机不存在")
	}

	platform, err := uc.platformRepo.GetByID(ctx, guest.PlatformID)
	if err != nil || platform == nil {
		return nil, fmt.Errorf("虚拟化平台不存在")
	}

	var host *VirtualizationHost
	if guest.HostID > 0 {
		host, err = uc.hostRepo.GetByID(ctx, guest.HostID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	now := time.Now()
	action := strings.TrimSpace(req.Action)
	logEntry := &VirtualizationActionLog{
		PlatformID:      guest.PlatformID,
		ClusterID:       guest.ClusterID,
		HostID:          guest.HostID,
		GuestID:         guest.ID,
		Action:          action,
		RiskLevel:       virtualizationActionRiskLevel(action),
		Status:          "pending",
		TargetType:      "guest",
		TargetName:      guest.Name,
		OperatorID:      operatorID,
		OperatorName:    strings.TrimSpace(operatorName),
		ConfirmRequired: true,
		Reason:          strings.TrimSpace(req.Reason),
		RequestPayload:  stringifyVirtualizationActionPayload(req),
		StartedAt:       &now,
	}
	if uc.actionLogRepo != nil {
		if err := uc.actionLogRepo.Create(ctx, logEntry); err != nil {
			return nil, err
		}
	}

	deny := func(message string) (*VirtualizationGuestPowerVO, error) {
		uc.finishActionLog(ctx, logEntry, "denied", message)
		return nil, errors.New(message)
	}
	fail := func(message string, cause error) (*VirtualizationGuestPowerVO, error) {
		uc.finishActionLog(ctx, logEntry, "failed", message)
		if cause != nil {
			return nil, cause
		}
		return nil, errors.New(message)
	}

	if strings.TrimSpace(platform.Status) != "enabled" {
		return deny("平台已禁用，不能执行虚机电源操作")
	}
	writeEnabled, err := uc.getWriteOperationsEnabled(ctx)
	if err != nil {
		return fail(err.Error(), err)
	}
	if !writeEnabled {
		return deny("当前未开启虚拟化写操作，仅允许只读能力")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return deny("操作原因不能为空")
	}
	if strings.TrimSpace(req.ConfirmText) != strings.TrimSpace(guest.Name) {
		return deny("二次确认未通过，请输入正确的虚机名称")
	}
	if err := validateVirtualizationGuestPowerAction(guest, action); err != nil {
		return deny(err.Error())
	}

	adapter, err := newVirtualizationAdapter(platform.Provider)
	if err != nil {
		return fail(err.Error(), err)
	}
	if err := adapter.PowerAction(ctx, platform, guest, host, action); err != nil {
		return fail(err.Error(), err)
	}

	finishedAt := time.Now()
	guest.PowerState = nextVirtualizationGuestPowerState(action, guest.PowerState)
	guest.LastCollectedAt = &finishedAt
	message := fmt.Sprintf("%s已提交", virtualizationActionText(action))
	if err := uc.guestRepo.UpsertBatch(ctx, []*VirtualizationGuest{guest}); err != nil {
		message = fmt.Sprintf("%s，但状态回写失败: %s", message, err.Error())
	}
	uc.finishActionLog(ctx, logEntry, "success", message)

	return &VirtualizationGuestPowerVO{
		AuditLogID:     logEntry.ID,
		GuestID:        guest.ID,
		GuestName:      guest.Name,
		Action:         action,
		ActionText:     virtualizationActionText(action),
		Status:         "success",
		StatusText:     virtualizationActionStatusText("success"),
		Message:        message,
		PowerState:     guest.PowerState,
		PowerStateText: virtualizationGuestPowerStateText(guest.PowerState),
		RequestedAt:    formatTime(logEntry.StartedAt),
		FinishedAt:     formatTime(logEntry.FinishedAt),
	}, nil
}

func (uc *VirtualizationUseCase) CreateGuestConsoleLink(ctx context.Context, guestID uint, operatorID uint, operatorName string) (*VirtualizationGuestConsoleLinkVO, error) {
	platform, guest, host, adapter, err := uc.buildGuestOperationContext(ctx, guestID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	logEntry := &VirtualizationActionLog{
		PlatformID:      guest.PlatformID,
		ClusterID:       guest.ClusterID,
		HostID:          guest.HostID,
		GuestID:         guest.ID,
		Action:          VirtualizationConsoleActionOpen,
		RiskLevel:       virtualizationActionRiskLevel(VirtualizationConsoleActionOpen),
		Status:          "pending",
		TargetType:      "guest",
		TargetName:      guest.Name,
		OperatorID:      operatorID,
		OperatorName:    strings.TrimSpace(operatorName),
		ConfirmRequired: false,
		StartedAt:       &now,
	}
	if uc.actionLogRepo != nil {
		if err := uc.actionLogRepo.Create(ctx, logEntry); err != nil {
			return nil, err
		}
	}

	deny := func(message string) (*VirtualizationGuestConsoleLinkVO, error) {
		uc.finishActionLog(ctx, logEntry, "denied", message)
		return nil, errors.New(message)
	}
	fail := func(message string, cause error) (*VirtualizationGuestConsoleLinkVO, error) {
		uc.finishActionLog(ctx, logEntry, "failed", message)
		if cause != nil {
			return nil, cause
		}
		return nil, errors.New(message)
	}

	if strings.TrimSpace(platform.Status) != "enabled" {
		return deny("平台已禁用，不能执行控制台跳转")
	}

	link, err := adapter.BuildConsoleLink(ctx, platform, guest, host)
	if err != nil {
		return fail(err.Error(), err)
	}
	if link == nil || strings.TrimSpace(link.URL) == "" {
		return fail("未生成可用的控制台链接", nil)
	}

	successMessage := strings.TrimSpace(link.Message)
	if successMessage == "" {
		successMessage = "控制台链接已生成"
	}
	uc.finishActionLog(ctx, logEntry, "success", successMessage)

	return &VirtualizationGuestConsoleLinkVO{
		AuditLogID:    logEntry.ID,
		GuestID:       guest.ID,
		GuestName:     guest.Name,
		Provider:      platform.Provider,
		ProviderText:  virtualizationProviderText(platform.Provider),
		Mode:          strings.TrimSpace(link.Mode),
		URL:           strings.TrimSpace(link.URL),
		RequiresLogin: link.RequiresLogin,
		Message:       successMessage,
		RequestedAt:   formatTime(logEntry.StartedAt),
		ExpiresAt:     formatTime(link.ExpiresAt),
	}, nil
}

func (uc *VirtualizationUseCase) ListGuestSnapshots(ctx context.Context, guestID uint) ([]*VirtualizationGuestSnapshotVO, error) {
	platform, guest, host, adapter, err := uc.buildGuestOperationContext(ctx, guestID)
	if err != nil {
		return nil, err
	}

	items, err := adapter.ListSnapshots(ctx, platform, guest, host)
	if err != nil {
		return nil, err
	}

	list := make([]*VirtualizationGuestSnapshotVO, 0, len(items))
	for _, item := range items {
		list = append(list, &VirtualizationGuestSnapshotVO{
			Name:          item.Name,
			Description:   item.Description,
			CreatedAt:     formatTime(item.CreatedAt),
			ParentName:    item.ParentName,
			Current:       item.Current,
			IncludeMemory: item.IncludeMemory,
			Quiesced:      item.Quiesced,
			PowerState:    item.PowerState,
			Children:      item.Children,
		})
	}
	return list, nil
}

func (uc *VirtualizationUseCase) CreateGuestSnapshot(ctx context.Context, guestID uint, req *VirtualizationGuestSnapshotCreateRequest, operatorID uint, operatorName string) error {
	if req == nil {
		return fmt.Errorf("请求参数不能为空")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return fmt.Errorf("操作原因不能为空")
	}

	platform, guest, host, adapter, err := uc.buildGuestOperationContext(ctx, guestID)
	if err != nil {
		return err
	}

	return uc.executeSnapshotAction(ctx, platform, guest, host, adapter, VirtualizationSnapshotActionCreate, strings.TrimSpace(req.Name), operatorID, operatorName, strings.TrimSpace(req.Reason), false, "", req, func() error {
		return adapter.CreateSnapshot(ctx, platform, guest, host, req)
	})
}

func (uc *VirtualizationUseCase) RollbackGuestSnapshot(ctx context.Context, guestID uint, req *VirtualizationGuestSnapshotActionRequest, operatorID uint, operatorName string) error {
	if req == nil {
		return fmt.Errorf("请求参数不能为空")
	}

	platform, guest, host, adapter, err := uc.buildGuestOperationContext(ctx, guestID)
	if err != nil {
		return err
	}

	return uc.executeSnapshotAction(ctx, platform, guest, host, adapter, VirtualizationSnapshotActionRollback, strings.TrimSpace(req.SnapshotName), operatorID, operatorName, strings.TrimSpace(req.Reason), true, strings.TrimSpace(req.ConfirmText), req, func() error {
		return adapter.RollbackSnapshot(ctx, platform, guest, host, strings.TrimSpace(req.SnapshotName))
	})
}

func (uc *VirtualizationUseCase) DeleteGuestSnapshot(ctx context.Context, guestID uint, req *VirtualizationGuestSnapshotActionRequest, operatorID uint, operatorName string) error {
	if req == nil {
		return fmt.Errorf("请求参数不能为空")
	}

	platform, guest, host, adapter, err := uc.buildGuestOperationContext(ctx, guestID)
	if err != nil {
		return err
	}

	return uc.executeSnapshotAction(ctx, platform, guest, host, adapter, VirtualizationSnapshotActionDelete, strings.TrimSpace(req.SnapshotName), operatorID, operatorName, strings.TrimSpace(req.Reason), true, strings.TrimSpace(req.ConfirmText), req, func() error {
		return adapter.DeleteSnapshot(ctx, platform, guest, host, strings.TrimSpace(req.SnapshotName))
	})
}

func (uc *VirtualizationUseCase) ListActionLogs(ctx context.Context, req *VirtualizationActionLogListRequest) ([]*VirtualizationActionLogVO, int64, error) {
	if req == nil {
		req = &VirtualizationActionLogListRequest{}
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	items, total, err := uc.actionLogRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	platformNameCache := make(map[uint]string)
	list := make([]*VirtualizationActionLogVO, 0, len(items))
	for _, item := range items {
		if item.PlatformID > 0 {
			if _, ok := platformNameCache[item.PlatformID]; !ok {
				platformNameCache[item.PlatformID] = ""
				if platform, platformErr := uc.platformRepo.GetByID(ctx, item.PlatformID); platformErr == nil && platform != nil {
					platformNameCache[item.PlatformID] = platform.Name
				}
			}
		}
		list = append(list, &VirtualizationActionLogVO{
			ID:              item.ID,
			PlatformID:      item.PlatformID,
			PlatformName:    platformNameCache[item.PlatformID],
			ClusterID:       item.ClusterID,
			HostID:          item.HostID,
			GuestID:         item.GuestID,
			Action:          item.Action,
			ActionText:      virtualizationActionText(item.Action),
			RiskLevel:       item.RiskLevel,
			Status:          item.Status,
			StatusText:      virtualizationActionStatusText(item.Status),
			TargetType:      item.TargetType,
			TargetName:      item.TargetName,
			OperatorID:      item.OperatorID,
			OperatorName:    item.OperatorName,
			ConfirmRequired: item.ConfirmRequired,
			Reason:          item.Reason,
			ResultMessage:   item.ResultMessage,
			StartedAt:       formatTime(item.StartedAt),
			FinishedAt:      formatTime(item.FinishedAt),
			CreateTime:      item.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return list, total, nil
}

func (uc *VirtualizationUseCase) toPlatformVO(item *VirtualizationPlatform) *VirtualizationPlatformVO {
	return &VirtualizationPlatformVO{
		ID:                 item.ID,
		Name:               item.Name,
		Provider:           item.Provider,
		ProviderText:       virtualizationProviderText(item.Provider),
		Endpoint:           item.Endpoint,
		Port:               item.Port,
		Username:           item.Username,
		InsecureSkipVerify: item.InsecureSkipVerify,
		Status:             item.Status,
		LastSyncAt:         formatTime(item.LastSyncAt),
		LastSyncStatus:     item.LastSyncStatus,
		LastSyncMessage:    item.LastSyncMessage,
		Description:        item.Description,
		CreateTime:         item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdateTime:         item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func (uc *VirtualizationUseCase) toSyncJobVO(item *VirtualizationSyncJob) *VirtualizationSyncJobVO {
	return &VirtualizationSyncJobVO{
		ID:            item.ID,
		PlatformID:    item.PlatformID,
		TriggerType:   item.TriggerType,
		Status:        item.Status,
		StartedAt:     formatTime(item.StartedAt),
		FinishedAt:    formatTime(item.FinishedAt),
		OperatorID:    item.OperatorID,
		ItemsTotal:    item.ItemsTotal,
		ItemsCreated:  item.ItemsCreated,
		ItemsUpdated:  item.ItemsUpdated,
		ItemsDeleted:  item.ItemsDeleted,
		FailureReason: item.FailureReason,
		CreateTime:    item.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func (uc *VirtualizationUseCase) toGuestVO(item *VirtualizationGuest) *VirtualizationGuestVO {
	return &VirtualizationGuestVO{
		ID:              item.ID,
		PlatformID:      item.PlatformID,
		ClusterID:       item.ClusterID,
		HostID:          item.HostID,
		ExternalID:      item.ExternalID,
		Name:            item.Name,
		OSType:          item.OSType,
		PowerState:      item.PowerState,
		CPUCount:        item.CPUCount,
		MemoryMB:        item.MemoryMB,
		PrimaryIP:       item.PrimaryIP,
		ToolsStatus:     item.ToolsStatus,
		BindingStatus:   item.BindingStatus,
		LastCollectedAt: formatTime(item.LastCollectedAt),
	}
}

func (uc *VirtualizationUseCase) toBindingVO(item *VirtualizationGuestBinding) *VirtualizationGuestBindingVO {
	return &VirtualizationGuestBindingVO{
		ID:          item.ID,
		GuestID:     item.GuestID,
		AssetHostID: item.AssetHostID,
		BindingType: item.BindingType,
		Status:      item.Status,
		BoundBy:     item.BoundBy,
		BoundAt:     formatTime(item.BoundAt),
		UnboundAt:   formatTime(item.UnboundAt),
		BindingNote: item.BindingNote,
	}
}

func (uc *VirtualizationUseCase) findActiveBindingHost(ctx context.Context, guestID uint) (*VirtualizationGuestBinding, *Host) {
	binding, err := uc.bindingRepo.GetActiveByGuestID(ctx, guestID)
	if err != nil {
		return nil, nil
	}

	if host, err := uc.assetHostRepo.GetByID(ctx, binding.AssetHostID); err == nil && host != nil {
		return binding, host
	}
	return binding, nil
}

func (uc *VirtualizationUseCase) buildPlatformDialAddr(item *VirtualizationPlatform) (string, error) {
	endpoint := strings.TrimSpace(item.Endpoint)
	if endpoint == "" {
		return "", fmt.Errorf("平台地址不能为空")
	}

	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("平台地址格式不正确")
	}
	if parsed.Hostname() == "" {
		return "", fmt.Errorf("平台地址格式不正确")
	}

	port := item.Port
	if port <= 0 {
		port = defaultVirtualizationPort(item.Provider)
	}
	return net.JoinHostPort(parsed.Hostname(), fmt.Sprintf("%d", port)), nil
}

func (uc *VirtualizationUseCase) applySyncSnapshot(ctx context.Context, platformID uint, snapshot *virtualizationSyncSnapshot) (int, error) {
	if snapshot == nil {
		return 0, fmt.Errorf("同步数据为空")
	}

	now := time.Now()
	clusterModels := make([]*VirtualizationCluster, 0, len(snapshot.Clusters))
	for _, item := range snapshot.Clusters {
		if strings.TrimSpace(item.ExternalID) == "" {
			continue
		}
		clusterModels = append(clusterModels, &VirtualizationCluster{
			PlatformID:       platformID,
			ExternalID:       strings.TrimSpace(item.ExternalID),
			Name:             firstNonEmpty([]string{item.Name, item.ExternalID}),
			Datacenter:       item.Datacenter,
			HostCount:        item.HostCount,
			GuestCount:       item.GuestCount,
			Status:           firstNonEmpty([]string{item.Status, "unknown"}),
			LastCollectedAt:  &now,
			RawPayloadDigest: "",
		})
	}
	if err := uc.clusterRepo.UpsertBatch(ctx, clusterModels); err != nil {
		return 0, fmt.Errorf("写入集群数据失败: %w", err)
	}

	clusterRows, err := uc.clusterRepo.ListByPlatformID(ctx, platformID)
	if err != nil {
		return 0, fmt.Errorf("读取集群数据失败: %w", err)
	}
	clusterIDByExternal := make(map[string]uint, len(clusterRows))
	for _, item := range clusterRows {
		clusterIDByExternal[item.ExternalID] = item.ID
	}

	hostModels := make([]*VirtualizationHost, 0, len(snapshot.Hosts))
	for _, item := range snapshot.Hosts {
		if strings.TrimSpace(item.ExternalID) == "" {
			continue
		}
		hostModels = append(hostModels, &VirtualizationHost{
			PlatformID:      platformID,
			ClusterID:       clusterIDByExternal[strings.TrimSpace(item.ClusterExternalID)],
			ExternalID:      strings.TrimSpace(item.ExternalID),
			Name:            firstNonEmpty([]string{item.Name, item.ExternalID}),
			ManagementIP:    item.ManagementIP,
			CPUModel:        item.CPUModel,
			CPUCores:        item.CPUCores,
			MemoryTotalMB:   item.MemoryTotalMB,
			MemoryUsedMB:    item.MemoryUsedMB,
			GuestCount:      item.GuestCount,
			Status:          firstNonEmpty([]string{item.Status, "unknown"}),
			LastCollectedAt: &now,
		})
	}
	if err := uc.hostRepo.UpsertBatch(ctx, hostModels); err != nil {
		return 0, fmt.Errorf("写入宿主机数据失败: %w", err)
	}

	hostRows, err := uc.hostRepo.ListByPlatformID(ctx, platformID)
	if err != nil {
		return 0, fmt.Errorf("读取宿主机数据失败: %w", err)
	}
	hostByExternal := make(map[string]*VirtualizationHost, len(hostRows))
	for _, item := range hostRows {
		hostByExternal[item.ExternalID] = item
	}

	existingGuests, err := uc.guestRepo.ListByPlatformID(ctx, platformID)
	if err != nil {
		return 0, fmt.Errorf("读取已有虚机数据失败: %w", err)
	}
	existingGuestByExternal := make(map[string]*VirtualizationGuest, len(existingGuests))
	for _, item := range existingGuests {
		existingGuestByExternal[item.ExternalID] = item
	}

	guestModels := make([]*VirtualizationGuest, 0, len(snapshot.Guests))
	for _, item := range snapshot.Guests {
		if strings.TrimSpace(item.ExternalID) == "" {
			continue
		}

		clusterID := clusterIDByExternal[strings.TrimSpace(item.ClusterExternalID)]
		hostID := uint(0)
		if host := hostByExternal[strings.TrimSpace(item.HostExternalID)]; host != nil {
			hostID = host.ID
			if clusterID == 0 {
				clusterID = host.ClusterID
			}
		}

		bindingStatus := "unbound"
		old := existingGuestByExternal[strings.TrimSpace(item.ExternalID)]
		if old != nil && strings.TrimSpace(old.BindingStatus) != "" {
			bindingStatus = old.BindingStatus
		}

		guestModel := &VirtualizationGuest{
			PlatformID:      platformID,
			ClusterID:       clusterID,
			HostID:          hostID,
			ExternalID:      strings.TrimSpace(item.ExternalID),
			Name:            firstNonEmpty([]string{item.Name, item.ExternalID}),
			OSType:          strings.TrimSpace(item.OSType),
			PowerState:      firstNonEmpty([]string{item.PowerState, "unknown"}),
			CPUCount:        item.CPUCount,
			MemoryMB:        item.MemoryMB,
			PrimaryIP:       strings.TrimSpace(item.PrimaryIP),
			ToolsStatus:     item.ToolsStatus,
			BindingStatus:   bindingStatus,
			LastCollectedAt: &now,
		}
		if old != nil {
			if guestModel.OSType == "" || (isGenericVirtualizationGuestOS(guestModel.OSType) && !isGenericVirtualizationGuestOS(old.OSType)) {
				guestModel.OSType = strings.TrimSpace(old.OSType)
			}
			if guestModel.PrimaryIP == "" {
				guestModel.PrimaryIP = strings.TrimSpace(old.PrimaryIP)
			}
		}
		if (guestModel.OSType == "" || guestModel.PrimaryIP == "") && old != nil && bindingStatus == "bound" {
			_, boundHost := uc.findActiveBindingHost(ctx, old.ID)
			uc.hydrateGuestMetadataFromHost(guestModel, boundHost)
		}
		guestModels = append(guestModels, guestModel)
	}
	if err := uc.guestRepo.UpsertBatch(ctx, guestModels); err != nil {
		return 0, fmt.Errorf("写入虚机数据失败: %w", err)
	}

	return len(clusterModels) + len(hostModels) + len(guestModels), nil
}

func defaultVirtualizationPort(provider string) int {
	switch strings.TrimSpace(provider) {
	case VirtualizationProviderPVE:
		return 8006
	default:
		return 443
	}
}

func virtualizationProviderText(provider string) string {
	switch provider {
	case VirtualizationProviderESXi:
		return "VMware ESXi/vCenter"
	case VirtualizationProviderPVE:
		return "Proxmox VE"
	default:
		return provider
	}
}

func virtualizationConflictPolicyText(policy string) string {
	switch strings.TrimSpace(policy) {
	case VirtualizationOnboardPolicyWarn:
		return "宽松（仅提示不拦截）"
	case VirtualizationOnboardPolicyStrict:
		fallthrough
	default:
		return "严格（高风险拦截）"
	}
}

func (uc *VirtualizationUseCase) enrichGuestVOFromBindingHost(vo *VirtualizationGuestVO, guest *VirtualizationGuest, host *Host) {
	if vo == nil || guest == nil || host == nil {
		return
	}

	mutable := &VirtualizationGuest{
		OSType:    vo.OSType,
		PrimaryIP: vo.PrimaryIP,
	}
	uc.hydrateGuestMetadataFromHost(mutable, host)
	vo.OSType = mutable.OSType
	vo.PrimaryIP = mutable.PrimaryIP
	vo.RuntimeStatus = virtualizationGuestRuntimeStatus(host)
	vo.RuntimeStatusText = CollectStatusText(vo.RuntimeStatus)
	vo.RuntimeStatusSource = "asset_host"
}

func (uc *VirtualizationUseCase) hydrateGuestMetadataFromHost(guest *VirtualizationGuest, host *Host) {
	if guest == nil || host == nil {
		return
	}
	if strings.TrimSpace(guest.PrimaryIP) == "" {
		guest.PrimaryIP = preferredVirtualizationGuestIP(host)
	}
	if strings.TrimSpace(guest.OSType) == "" || isGenericVirtualizationGuestOS(guest.OSType) {
		guest.OSType = preferredVirtualizationGuestOS(host)
	}
}

func (uc *VirtualizationUseCase) finishActionLog(ctx context.Context, item *VirtualizationActionLog, status, message string) {
	if uc.actionLogRepo == nil || item == nil || item.ID == 0 {
		return
	}
	finishedAt := time.Now()
	item.Status = status
	item.ResultMessage = strings.TrimSpace(message)
	item.FinishedAt = &finishedAt
	_ = uc.actionLogRepo.Update(ctx, item)
}

func (uc *VirtualizationUseCase) buildGuestOperationContext(ctx context.Context, guestID uint) (*VirtualizationPlatform, *VirtualizationGuest, *VirtualizationHost, virtualizationProviderAdapter, error) {
	guest, err := uc.guestRepo.GetByID(ctx, guestID)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("虚机不存在")
	}
	platform, err := uc.platformRepo.GetByID(ctx, guest.PlatformID)
	if err != nil || platform == nil {
		return nil, nil, nil, nil, fmt.Errorf("虚拟化平台不存在")
	}
	var host *VirtualizationHost
	if guest.HostID > 0 {
		host, err = uc.hostRepo.GetByID(ctx, guest.HostID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, nil, err
		}
	}
	adapter, err := newVirtualizationAdapter(platform.Provider)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return platform, guest, host, adapter, nil
}

func (uc *VirtualizationUseCase) executeSnapshotAction(
	ctx context.Context,
	platform *VirtualizationPlatform,
	guest *VirtualizationGuest,
	host *VirtualizationHost,
	adapter virtualizationProviderAdapter,
	action string,
	snapshotName string,
	operatorID uint,
	operatorName string,
	reason string,
	confirmRequired bool,
	confirmText string,
	payload any,
	execute func() error,
) error {
	if platform == nil || guest == nil || adapter == nil {
		return fmt.Errorf("虚机操作上下文不完整")
	}
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("操作原因不能为空")
	}
	snapshotName = strings.TrimSpace(snapshotName)
	if snapshotName == "" {
		return fmt.Errorf("快照名称不能为空")
	}
	if strings.TrimSpace(platform.Status) != "enabled" {
		return fmt.Errorf("平台已禁用，不能执行快照操作")
	}
	if confirmRequired && strings.TrimSpace(confirmText) != snapshotName {
		return fmt.Errorf("二次确认未通过，请输入正确的快照名称")
	}

	now := time.Now()
	logEntry := &VirtualizationActionLog{
		PlatformID:      guest.PlatformID,
		ClusterID:       guest.ClusterID,
		HostID:          guest.HostID,
		GuestID:         guest.ID,
		Action:          action,
		RiskLevel:       virtualizationActionRiskLevel(action),
		Status:          "pending",
		TargetType:      "snapshot",
		TargetName:      fmt.Sprintf("%s / %s", guest.Name, snapshotName),
		OperatorID:      operatorID,
		OperatorName:    strings.TrimSpace(operatorName),
		ConfirmRequired: confirmRequired,
		Reason:          reason,
		RequestPayload:  stringifyVirtualizationActionPayload(payload),
		StartedAt:       &now,
	}
	if uc.actionLogRepo != nil {
		if err := uc.actionLogRepo.Create(ctx, logEntry); err != nil {
			return err
		}
	}

	writeEnabled, err := uc.getWriteOperationsEnabled(ctx)
	if err != nil {
		uc.finishActionLog(ctx, logEntry, "failed", err.Error())
		return err
	}
	if !writeEnabled {
		uc.finishActionLog(ctx, logEntry, "denied", "当前未开启虚拟化写操作，仅允许只读能力")
		return fmt.Errorf("当前未开启虚拟化写操作，仅允许只读能力")
	}

	if err := execute(); err != nil {
		uc.finishActionLog(ctx, logEntry, "failed", err.Error())
		return err
	}

	successMessage := fmt.Sprintf("%s已提交", virtualizationActionText(action))
	if action == VirtualizationSnapshotActionDelete {
		successMessage = fmt.Sprintf("快照 %s 已删除", snapshotName)
	}
	uc.finishActionLog(ctx, logEntry, "success", successMessage)
	return nil
}

func stringifyVirtualizationActionPayload(payload any) string {
	if payload == nil {
		return ""
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

func validateVirtualizationGuestPowerAction(guest *VirtualizationGuest, action string) error {
	if guest == nil {
		return fmt.Errorf("虚机不存在")
	}
	switch action {
	case VirtualizationGuestPowerActionOn:
		if guest.PowerState == "powered_on" {
			return fmt.Errorf("当前虚机已经是开机状态")
		}
	case VirtualizationGuestPowerActionOff:
		if guest.PowerState == "powered_off" {
			return fmt.Errorf("当前虚机已经是关机状态")
		}
	case VirtualizationGuestPowerActionReboot:
		if guest.PowerState != "powered_on" {
			return fmt.Errorf("当前虚机不是开机状态，不能执行重启")
		}
	default:
		return fmt.Errorf("不支持的电源操作: %s", action)
	}
	return nil
}

func virtualizationActionRiskLevel(action string) string {
	switch action {
	case VirtualizationGuestPowerActionOff, VirtualizationGuestPowerActionReboot, VirtualizationSnapshotActionRollback, VirtualizationSnapshotActionDelete:
		return "high"
	case VirtualizationGuestPowerActionOn, VirtualizationSnapshotActionCreate:
		return "medium"
	case VirtualizationConsoleActionOpen:
		return "low"
	default:
		return "medium"
	}
}

func virtualizationActionText(action string) string {
	switch action {
	case VirtualizationGuestPowerActionOn:
		return "开机"
	case VirtualizationGuestPowerActionOff:
		return "关机"
	case VirtualizationGuestPowerActionReboot:
		return "重启"
	case VirtualizationSnapshotActionCreate:
		return "创建快照"
	case VirtualizationSnapshotActionRollback:
		return "回滚快照"
	case VirtualizationSnapshotActionDelete:
		return "删除快照"
	case VirtualizationConsoleActionOpen:
		return "控制台跳转"
	default:
		return action
	}
}

func virtualizationActionStatusText(status string) string {
	switch status {
	case "success":
		return "成功"
	case "failed":
		return "失败"
	case "denied":
		return "已拒绝"
	default:
		return "处理中"
	}
}

func virtualizationGuestPowerStateText(status string) string {
	switch status {
	case "powered_on":
		return "开机"
	case "powered_off":
		return "关机"
	case "suspended":
		return "挂起"
	default:
		return "未知"
	}
}

func nextVirtualizationGuestPowerState(action, currentState string) string {
	switch action {
	case VirtualizationGuestPowerActionOn:
		return "powered_on"
	case VirtualizationGuestPowerActionOff:
		return "powered_off"
	case VirtualizationGuestPowerActionReboot:
		if currentState == "powered_off" {
			return currentState
		}
		return "powered_on"
	default:
		return currentState
	}
}

func preferredVirtualizationGuestIP(host *Host) string {
	if host == nil {
		return ""
	}
	return firstNonEmpty([]string{
		strings.TrimSpace(host.PrimaryPrivateIP),
		strings.TrimSpace(host.IP),
		strings.TrimSpace(host.PrimaryPublicIP),
	})
}

func preferredVirtualizationGuestOS(host *Host) string {
	if host == nil {
		return ""
	}
	if osName := strings.TrimSpace(host.OS); osName != "" {
		return osName
	}
	switch strings.ToLower(strings.TrimSpace(host.OSType)) {
	case OSTypeWindows:
		return "Windows"
	case OSTypeLinux:
		fallthrough
	default:
		return "Linux"
	}
}

func isGenericVirtualizationGuestOS(osName string) bool {
	switch strings.ToLower(strings.TrimSpace(osName)) {
	case "", "linux", "windows", "other":
		return true
	default:
		return false
	}
}

func virtualizationGuestRuntimeStatus(host *Host) string {
	if host == nil {
		return CollectStatusUnknown
	}

	switch strings.TrimSpace(host.CollectStatus) {
	case CollectStatusOnline:
		return CollectStatusOnline
	case CollectStatusOffline:
		return CollectStatusOffline
	case CollectStatusNotConfigured:
		return CollectStatusNotConfigured
	}

	switch host.Status {
	case 1:
		return CollectStatusOnline
	case 0:
		return CollectStatusOffline
	default:
		return CollectStatusUnknown
	}
}

func (uc *VirtualizationUseCase) getConflictPolicy(ctx context.Context) (string, error) {
	if uc.policyRepo == nil {
		return VirtualizationOnboardPolicyStrict, nil
	}
	policy, err := uc.policyRepo.GetOnboardConflictPolicy(ctx)
	if err != nil {
		return "", err
	}
	policy = strings.TrimSpace(policy)
	switch policy {
	case VirtualizationOnboardPolicyStrict, VirtualizationOnboardPolicyWarn:
		return policy, nil
	default:
		return VirtualizationOnboardPolicyStrict, nil
	}
}

func (uc *VirtualizationUseCase) getWriteOperationsEnabled(ctx context.Context) (bool, error) {
	if uc.policyRepo == nil {
		return false, nil
	}
	return uc.policyRepo.GetWriteOperationsEnabled(ctx)
}

func (uc *VirtualizationUseCase) checkBindAllowedByPolicy(ctx context.Context, guestID, assetHostID uint) error {
	policy, err := uc.getConflictPolicy(ctx)
	if err != nil {
		return err
	}
	if policy == VirtualizationOnboardPolicyWarn {
		return nil
	}

	precheck, err := uc.PrecheckGuestBinding(ctx, guestID, assetHostID)
	if err != nil {
		return err
	}
	if precheck == nil {
		return nil
	}
	if precheck.RiskLevel != "high" {
		return nil
	}

	return fmt.Errorf("当前纳管命中高风险规则（严格模式已拦截），请先处理冲突或切换为宽松策略")
}

func virtualizationWriteOperationsEnabledText(enabled bool) string {
	if enabled {
		return "已启用"
	}
	return "已关闭"
}

func (uc *VirtualizationUseCase) buildPlatformMetricSnapshot(ctx context.Context, platformID uint, collectedAt time.Time) (*VirtualizationPlatformMetric, error) {
	guests, err := uc.guestRepo.ListByPlatformID(ctx, platformID)
	if err != nil {
		return nil, err
	}

	metric := &VirtualizationPlatformMetric{
		PlatformID:  platformID,
		CollectedAt: &collectedAt,
	}
	uc.applyGuestMetrics(ctx, guests, &metric.GuestTotal, &metric.PoweredOnGuests, &metric.PoweredOffGuests, &metric.SuspendedGuests, &metric.BoundGuests, &metric.OnlineGuests, &metric.OfflineGuests, &metric.NotConfiguredGuests, &metric.UnknownGuests)
	return metric, nil
}

func (uc *VirtualizationUseCase) buildClusterMetricSnapshot(ctx context.Context, cluster *VirtualizationCluster, collectedAt time.Time) (*VirtualizationClusterMetric, error) {
	if cluster == nil {
		return nil, nil
	}

	guests, err := uc.guestRepo.ListByPlatformID(ctx, cluster.PlatformID)
	if err != nil {
		return nil, err
	}

	metric := &VirtualizationClusterMetric{
		PlatformID:  cluster.PlatformID,
		ClusterID:   cluster.ID,
		CollectedAt: &collectedAt,
	}
	uc.applyGuestMetrics(ctx, filterGuestsByCluster(guests, cluster.ID), &metric.GuestTotal, &metric.PoweredOnGuests, &metric.PoweredOffGuests, &metric.SuspendedGuests, &metric.BoundGuests, &metric.OnlineGuests, &metric.OfflineGuests, &metric.NotConfiguredGuests, &metric.UnknownGuests)
	return metric, nil
}

func (uc *VirtualizationUseCase) recordPlatformMetricSnapshot(ctx context.Context, platformID uint, collectedAt time.Time) error {
	if uc.metricRepo == nil {
		return nil
	}
	snapshot, err := uc.buildPlatformMetricSnapshot(ctx, platformID, collectedAt)
	if err != nil {
		return err
	}
	return uc.metricRepo.Create(ctx, snapshot)
}

func (uc *VirtualizationUseCase) recordClusterMetricSnapshots(ctx context.Context, platformID uint, collectedAt time.Time) error {
	if uc.clusterMetricRepo == nil {
		return nil
	}

	clusters, err := uc.clusterRepo.ListByPlatformID(ctx, platformID)
	if err != nil {
		return err
	}
	if len(clusters) == 0 {
		return nil
	}

	guests, err := uc.guestRepo.ListByPlatformID(ctx, platformID)
	if err != nil {
		return err
	}

	items := make([]*VirtualizationClusterMetric, 0, len(clusters))
	for _, cluster := range clusters {
		if cluster == nil {
			continue
		}
		metric := &VirtualizationClusterMetric{
			PlatformID:  cluster.PlatformID,
			ClusterID:   cluster.ID,
			CollectedAt: &collectedAt,
		}
		uc.applyGuestMetrics(ctx, filterGuestsByCluster(guests, cluster.ID), &metric.GuestTotal, &metric.PoweredOnGuests, &metric.PoweredOffGuests, &metric.SuspendedGuests, &metric.BoundGuests, &metric.OnlineGuests, &metric.OfflineGuests, &metric.NotConfiguredGuests, &metric.UnknownGuests)
		items = append(items, metric)
	}
	return uc.clusterMetricRepo.CreateBatch(ctx, items)
}

func (uc *VirtualizationUseCase) applyGuestMetrics(
	ctx context.Context,
	guests []*VirtualizationGuest,
	guestTotal *int,
	poweredOnGuests *int,
	poweredOffGuests *int,
	suspendedGuests *int,
	boundGuests *int,
	onlineGuests *int,
	offlineGuests *int,
	notConfiguredGuests *int,
	unknownGuests *int,
) {
	for _, guest := range guests {
		if guest == nil {
			continue
		}

		*guestTotal = *guestTotal + 1
		switch strings.TrimSpace(guest.PowerState) {
		case "powered_on":
			*poweredOnGuests = *poweredOnGuests + 1
		case "suspended":
			*suspendedGuests = *suspendedGuests + 1
		default:
			*poweredOffGuests = *poweredOffGuests + 1
		}

		if strings.TrimSpace(guest.BindingStatus) == "bound" {
			*boundGuests = *boundGuests + 1
		}

		_, boundHost := uc.findActiveBindingHost(ctx, guest.ID)
		switch virtualizationGuestRuntimeStatus(boundHost) {
		case CollectStatusOnline:
			*onlineGuests = *onlineGuests + 1
		case CollectStatusOffline:
			*offlineGuests = *offlineGuests + 1
		case CollectStatusNotConfigured:
			*notConfiguredGuests = *notConfiguredGuests + 1
		default:
			*unknownGuests = *unknownGuests + 1
		}
	}
}

func filterGuestsByCluster(guests []*VirtualizationGuest, clusterID uint) []*VirtualizationGuest {
	result := make([]*VirtualizationGuest, 0)
	for _, guest := range guests {
		if guest == nil || guest.ClusterID != clusterID {
			continue
		}
		result = append(result, guest)
	}
	return result
}

type virtualizationTrendWindow struct {
	Key   string
	Start time.Time
	End   time.Time
}

func resolveVirtualizationTrendWindow(rangeKey string) virtualizationTrendWindow {
	now := time.Now()
	switch strings.TrimSpace(rangeKey) {
	case "24h":
		return virtualizationTrendWindow{
			Key:   "24h",
			Start: now.Add(-24 * time.Hour),
			End:   now,
		}
	case "15d":
		return virtualizationTrendWindow{
			Key:   "15d",
			Start: now.Add(-15 * 24 * time.Hour),
			End:   now,
		}
	case "7d":
		fallthrough
	default:
		return virtualizationTrendWindow{
			Key:   "7d",
			Start: now.Add(-7 * 24 * time.Hour),
			End:   now,
		}
	}
}

func formatTime(ts *time.Time) string {
	if ts == nil {
		return ""
	}
	return ts.Format("2006-01-02 15:04:05")
}

func mergeHostMatchType(a, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" {
		return b
	}
	if b == "" || a == b {
		return a
	}
	return a + "+" + b
}

func mergeRiskLevel(current, next string) string {
	order := map[string]int{
		"none":   0,
		"low":    1,
		"medium": 2,
		"high":   3,
	}
	if order[next] > order[current] {
		return next
	}
	return current
}

func (uc *VirtualizationUseCase) pickSuggestedPrecheckHost(candidates []*VirtualizationPrecheckHostVO) *VirtualizationPrecheckHostVO {
	if len(candidates) == 0 {
		return nil
	}
	for _, item := range candidates {
		if strings.Contains(item.MatchType, "ip_exact") {
			return item
		}
	}
	return candidates[0]
}
