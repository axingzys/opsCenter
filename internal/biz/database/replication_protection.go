package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	defaultReplicaLagWarningSeconds            = 300
	defaultReplicaLagCriticalSeconds           = 1800
	defaultReplicaRemainingDelayWarningSeconds = 300
)

type replicaProtectionThresholds struct {
	lagWarningSeconds            int
	lagCriticalSeconds           int
	remainingDelayWarningSeconds int
	relayLogBacklogWarningBytes  int64
	walBacklogWarningBytes       int64
}

func (uc *UseCase) ListReplicaProtections(ctx context.Context, req *DatabaseReplicaProtectionListRequest) ([]*DatabaseReplicaProtectionVO, int64, error) {
	if uc.instanceRepo == nil || uc.instanceReplicaRepo == nil || uc.replicationCheckRepo == nil {
		return nil, 0, errors.New("副本治理仓储未配置")
	}
	instances, err := uc.instanceRepo.ListEnabled(ctx)
	if err != nil {
		return nil, 0, err
	}
	replicas, _, err := uc.instanceReplicaRepo.List(ctx, &DatabaseInstanceReplicaListRequest{Page: 1, PageSize: 10000})
	if err != nil {
		return nil, 0, err
	}

	allowed := map[uint]bool{}
	restrict := req != nil && req.RestrictToAllowed
	if restrict {
		for _, id := range req.AllowedInstanceIDs {
			allowed[id] = true
		}
	}
	instanceByID := make(map[uint]*DatabaseInstance, len(instances))
	for _, item := range instances {
		if item == nil {
			continue
		}
		instanceByID[item.ID] = item
	}

	replicasByPrimary := make(map[uint][]*DatabaseInstanceReplica)
	replicaInstanceIDs := make(map[uint]bool)
	for _, replica := range replicas {
		if replica == nil || replica.PrimaryInstanceID == 0 {
			continue
		}
		replicaInstanceIDs[replica.ReplicaInstanceID] = true
		if restrict && !allowed[replica.PrimaryInstanceID] && !allowed[replica.ReplicaInstanceID] {
			continue
		}
		replicasByPrimary[replica.PrimaryInstanceID] = append(replicasByPrimary[replica.PrimaryInstanceID], replica)
	}

	thresholds := normalizeReplicaProtectionThresholds(req)
	now := time.Now()
	result := make([]*DatabaseReplicaProtectionVO, 0, len(instances))
	for _, primary := range instances {
		if primary == nil || !isReplicaGovernanceEngine(primary.DBType) {
			continue
		}
		if replicaInstanceIDs[primary.ID] {
			continue
		}
		if !matchesReplicaProtectionRequest(primary, req) {
			continue
		}
		if restrict && !allowed[primary.ID] {
			ownedReplica := false
			for _, replica := range replicasByPrimary[primary.ID] {
				if allowed[replica.ReplicaInstanceID] {
					ownedReplica = true
					break
				}
			}
			if !ownedReplica {
				continue
			}
		}
		vo := uc.buildReplicaProtectionVO(ctx, primary, replicasByPrimary[primary.ID], instanceByID, thresholds, now)
		if req != nil {
			if status := strings.TrimSpace(req.ProtectionStatus); status != "" && vo.ProtectionStatus != status {
				continue
			}
			if risk := strings.TrimSpace(req.RiskLevel); risk != "" && vo.RiskLevel != risk {
				continue
			}
		}
		result = append(result, vo)
	}

	sort.SliceStable(result, func(i, j int) bool {
		left, right := result[i], result[j]
		if protectionRank(left.ProtectionStatus) != protectionRank(right.ProtectionStatus) {
			return protectionRank(left.ProtectionStatus) < protectionRank(right.ProtectionStatus)
		}
		if riskRank(left.RiskLevel) != riskRank(right.RiskLevel) {
			return riskRank(left.RiskLevel) > riskRank(right.RiskLevel)
		}
		return left.PrimaryInstanceID > right.PrimaryInstanceID
	})

	total := int64(len(result))
	page, pageSize := normalizeListPage(reqProtectionPage(req), reqProtectionPageSize(req))
	start := (page - 1) * pageSize
	if start >= len(result) {
		return []*DatabaseReplicaProtectionVO{}, total, nil
	}
	end := start + pageSize
	if end > len(result) {
		end = len(result)
	}
	return result[start:end], total, nil
}

func (uc *UseCase) buildReplicaProtectionVO(
	ctx context.Context,
	primary *DatabaseInstance,
	replicas []*DatabaseInstanceReplica,
	instanceByID map[uint]*DatabaseInstance,
	thresholds replicaProtectionThresholds,
	now time.Time,
) *DatabaseReplicaProtectionVO {
	vo := &DatabaseReplicaProtectionVO{
		PrimaryInstanceID:            primary.ID,
		PrimaryInstanceName:          primary.Name,
		PrimaryEndpoint:              netJoinHostPort(primary.Host, primary.Port),
		Engine:                       normalizeDBType(primary.DBType),
		EngineText:                   DBTypeText(primary.DBType),
		RemainingDelaySeconds:        -1,
		ProtectionStatus:             DatabaseReplicaProtectionUnprotected,
		ProtectionStatusText:         ReplicaProtectionStatusText(DatabaseReplicaProtectionUnprotected),
		RiskLevel:                    DatabaseReplicaHealthWarning,
		RiskLevelText:                ReplicaHealthText(DatabaseReplicaHealthWarning),
		LagWarningSeconds:            thresholds.lagWarningSeconds,
		LagCriticalSeconds:           thresholds.lagCriticalSeconds,
		RemainingDelayWarningSeconds: thresholds.remainingDelayWarningSeconds,
		RelayLogBacklogWarningBytes:  thresholds.relayLogBacklogWarningBytes,
		WALBacklogWarningBytes:       thresholds.walBacklogWarningBytes,
	}
	if len(replicas) == 0 {
		vo.RiskMessages = []string{"当前主库没有已登记副本"}
		vo.RiskFlagsJSON = marshalReplicaJSON(vo.RiskMessages)
		return vo
	}
	vo.HasReplica = true

	var preferred *DatabaseInstanceReplica
	var preferredCheck *DatabaseReplicationCheck
	delayedCount := 0
	for _, replica := range replicas {
		if replica == nil || replica.ReplicaRole != DatabaseReplicaRoleDelayed {
			continue
		}
		delayedCount++
		check := latestReplicationCheck(ctx, uc.replicationCheckRepo, replica.ReplicaInstanceID)
		if preferred == nil || delayedReplicaCandidateScore(replica, check) > delayedReplicaCandidateScore(preferred, preferredCheck) {
			preferred = replica
			preferredCheck = check
		}
	}
	vo.DelayedReplicaCount = delayedCount
	vo.HasDelayedReplica = delayedCount > 0
	if preferred == nil {
		vo.RiskMessages = []string{"当前主库没有延迟副本，缺少短窗口误删保护"}
		vo.RiskFlagsJSON = marshalReplicaJSON(vo.RiskMessages)
		return vo
	}

	vo.PreferredReplicaID = preferred.ID
	vo.PreferredReplicaInstanceID = preferred.ReplicaInstanceID
	vo.PreferredReplicaStatus = preferred.Status
	vo.PreferredReplicaStatusText = ReplicaHealthText(preferred.Status)
	vo.ConfiguredDelaySeconds = preferred.ConfiguredDelaySeconds
	if replicaInstance := instanceByID[preferred.ReplicaInstanceID]; replicaInstance != nil {
		vo.PreferredReplicaInstanceName = replicaInstance.Name
		vo.PreferredReplicaEndpoint = netJoinHostPort(replicaInstance.Host, replicaInstance.Port)
	}
	if preferredCheck != nil {
		vo.LastCheckID = preferredCheck.ID
		vo.LastCheckedAt = formatTime(preferredCheck.CheckedAt)
		vo.RemainingDelaySeconds = preferredCheck.RemainingDelaySeconds
		if preferredCheck.Engine == DBTypePostgreSQL {
			vo.RemainingDelayEstimated = true
			if preferredCheck.PGLastXactReplayTimestamp != nil {
				applyLag := int(now.Sub(*preferredCheck.PGLastXactReplayTimestamp).Seconds())
				if applyLag < 0 {
					applyLag = 0
				}
				vo.ApplyLagSeconds = applyLag
				vo.ApplyTime = formatTime(preferredCheck.PGLastXactReplayTimestamp)
				vo.RemainingDelaySeconds = preferredCheck.ConfiguredDelaySeconds - applyLag
				if vo.RemainingDelaySeconds < 0 {
					vo.RemainingDelaySeconds = 0
				}
			}
		}
	}

	vo.RiskMessages = replicaProtectionRiskMessages(preferred, preferredCheck, vo, thresholds)
	vo.RiskFlagsJSON = marshalReplicaJSON(vo.RiskMessages)
	vo.RiskLevel = replicaProtectionRiskLevel(vo.RiskMessages, preferred, preferredCheck)
	vo.RiskLevelText = ReplicaHealthText(vo.RiskLevel)
	vo.ProtectionStatus = replicaProtectionStatus(vo.RiskLevel, vo)
	vo.ProtectionStatusText = ReplicaProtectionStatusText(vo.ProtectionStatus)
	return vo
}

func replicaProtectionRiskMessages(replica *DatabaseInstanceReplica, check *DatabaseReplicationCheck, vo *DatabaseReplicaProtectionVO, thresholds replicaProtectionThresholds) []string {
	messages := make([]string, 0, 6)
	if replica == nil {
		return []string{"当前主库没有延迟副本，缺少短窗口误删保护"}
	}
	if strings.TrimSpace(replica.Status) == DatabaseReplicaHealthCritical {
		messages = append(messages, "延迟副本状态异常，需先排障")
	} else if strings.TrimSpace(replica.Status) == DatabaseReplicaHealthWarning {
		messages = append(messages, "延迟副本存在警告，请检查风险明细")
	}
	if check == nil {
		messages = append(messages, "延迟副本尚无采集记录，保护窗口未知")
		return messages
	}
	messages = append(messages, decodeReplicaRiskFlags(check.RiskFlagsJSON)...)
	if check.Engine == DBTypePostgreSQL && vo.RemainingDelayEstimated {
		messages = append(messages, "PostgreSQL remaining delay 为估算值，主从时钟不一致会影响判断")
	}
	if check.ConfiguredDelaySeconds > 0 && vo.RemainingDelaySeconds < 0 {
		messages = append(messages, "剩余保护窗口未知")
	}
	if check.ConfiguredDelaySeconds <= 0 && check.SecondsBehindSource >= thresholds.lagCriticalSeconds {
		messages = append(messages, "复制延迟达到严重阈值")
	} else if check.ConfiguredDelaySeconds <= 0 && check.SecondsBehindSource >= thresholds.lagWarningSeconds {
		messages = append(messages, "复制延迟超过告警阈值")
	}
	if thresholds.relayLogBacklogWarningBytes > 0 && check.RelayLogBytes >= thresholds.relayLogBacklogWarningBytes {
		messages = append(messages, "relay log 积压超过告警阈值")
	}
	if thresholds.walBacklogWarningBytes > 0 && check.WALBacklogBytes >= thresholds.walBacklogWarningBytes {
		messages = append(messages, "WAL 积压超过告警阈值")
	}
	return uniqueStrings(messages)
}

func replicaProtectionRiskLevel(messages []string, replica *DatabaseInstanceReplica, check *DatabaseReplicationCheck) string {
	if replica == nil {
		return DatabaseReplicaHealthWarning
	}
	if replica.Status == DatabaseReplicaHealthCritical {
		return DatabaseReplicaHealthCritical
	}
	if check != nil && check.HealthStatus == DatabaseReplicaHealthCritical {
		return DatabaseReplicaHealthCritical
	}
	for _, msg := range messages {
		if strings.Contains(msg, "异常") || strings.Contains(msg, "严重") || strings.Contains(msg, "失败") {
			return DatabaseReplicaHealthCritical
		}
	}
	if len(messages) > 0 || replica.Status == DatabaseReplicaHealthWarning || (check != nil && check.HealthStatus == DatabaseReplicaHealthWarning) {
		return DatabaseReplicaHealthWarning
	}
	return DatabaseReplicaHealthHealthy
}

func replicaProtectionStatus(riskLevel string, vo *DatabaseReplicaProtectionVO) string {
	if vo == nil || !vo.HasDelayedReplica {
		return DatabaseReplicaProtectionUnprotected
	}
	if riskLevel == DatabaseReplicaHealthCritical {
		return DatabaseReplicaProtectionDegraded
	}
	if riskLevel == DatabaseReplicaHealthWarning {
		return DatabaseReplicaProtectionDegraded
	}
	return DatabaseReplicaProtectionProtected
}

func delayedReplicaCandidateScore(replica *DatabaseInstanceReplica, check *DatabaseReplicationCheck) int {
	if replica == nil {
		return -1
	}
	score := 0
	switch replica.Status {
	case DatabaseReplicaHealthHealthy:
		score += 1000
	case DatabaseReplicaHealthWarning:
		score += 500
	}
	if check != nil {
		switch check.HealthStatus {
		case DatabaseReplicaHealthHealthy:
			score += 1000
		case DatabaseReplicaHealthWarning:
			score += 500
		}
		if check.RemainingDelaySeconds > 0 {
			score += check.RemainingDelaySeconds
		}
		if check.ConfiguredDelaySeconds > 0 {
			score += check.ConfiguredDelaySeconds / 10
		}
	}
	return score
}

func latestReplicationCheck(ctx context.Context, repo ReplicationCheckRepo, instanceID uint) *DatabaseReplicationCheck {
	if repo == nil || instanceID == 0 {
		return nil
	}
	check, err := repo.LatestByInstanceID(ctx, instanceID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return check
}

func normalizeReplicaProtectionThresholds(req *DatabaseReplicaProtectionListRequest) replicaProtectionThresholds {
	thresholds := replicaProtectionThresholds{
		lagWarningSeconds:            defaultReplicaLagWarningSeconds,
		lagCriticalSeconds:           defaultReplicaLagCriticalSeconds,
		remainingDelayWarningSeconds: defaultReplicaRemainingDelayWarningSeconds,
	}
	if req == nil {
		return thresholds
	}
	if req.LagWarningSeconds > 0 {
		thresholds.lagWarningSeconds = req.LagWarningSeconds
	}
	if req.LagCriticalSeconds > 0 {
		thresholds.lagCriticalSeconds = req.LagCriticalSeconds
	}
	if req.RemainingDelayWarningSeconds > 0 {
		thresholds.remainingDelayWarningSeconds = req.RemainingDelayWarningSeconds
	}
	if thresholds.lagCriticalSeconds < thresholds.lagWarningSeconds {
		thresholds.lagCriticalSeconds = thresholds.lagWarningSeconds
	}
	thresholds.relayLogBacklogWarningBytes = req.RelayLogBacklogWarningBytes
	thresholds.walBacklogWarningBytes = req.WALBacklogWarningBytes
	return thresholds
}

func matchesReplicaProtectionRequest(item *DatabaseInstance, req *DatabaseReplicaProtectionListRequest) bool {
	if item == nil || req == nil {
		return true
	}
	if req.InstanceID > 0 && item.ID != req.InstanceID {
		return false
	}
	if engine := strings.TrimSpace(req.Engine); engine != "" && normalizeDBType(item.DBType) != engine {
		return false
	}
	return true
}

func isReplicaGovernanceEngine(dbType string) bool {
	switch normalizeDBType(dbType) {
	case DBTypeMySQL, DBTypeMariaDB, DBTypePostgreSQL:
		return true
	default:
		return false
	}
}

func reqProtectionPage(req *DatabaseReplicaProtectionListRequest) int {
	if req == nil {
		return 1
	}
	return req.Page
}

func reqProtectionPageSize(req *DatabaseReplicaProtectionListRequest) int {
	if req == nil {
		return 10
	}
	return req.PageSize
}

func normalizeListPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func decodeReplicaRiskFlags(raw string) []string {
	var flags []string
	if err := json.Unmarshal([]byte(raw), &flags); err != nil {
		return nil
	}
	return flags
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func protectionRank(status string) int {
	switch status {
	case DatabaseReplicaProtectionUnprotected:
		return 0
	case DatabaseReplicaProtectionDegraded:
		return 1
	case DatabaseReplicaProtectionUnknown:
		return 2
	case DatabaseReplicaProtectionProtected:
		return 3
	default:
		return 4
	}
}

func riskRank(level string) int {
	switch level {
	case DatabaseReplicaHealthCritical:
		return 3
	case DatabaseReplicaHealthWarning:
		return 2
	case DatabaseReplicaHealthHealthy:
		return 1
	default:
		return 0
	}
}

func ReplicaProtectionStatusText(status string) string {
	switch strings.TrimSpace(status) {
	case DatabaseReplicaProtectionProtected:
		return "有保护窗口"
	case DatabaseReplicaProtectionDegraded:
		return "保护降级"
	case DatabaseReplicaProtectionUnprotected:
		return "无延迟保护"
	default:
		return "未知"
	}
}

func netJoinHostPort(host string, port int) string {
	if strings.TrimSpace(host) == "" || port <= 0 {
		return strings.TrimSpace(host)
	}
	return net.JoinHostPort(strings.TrimSpace(host), fmt.Sprintf("%d", port))
}
