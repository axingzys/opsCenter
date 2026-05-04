package database

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const (
	DatabaseProtectionIssueMissingFullBackup       = "missing_full_backup"
	DatabaseProtectionIssueIncrementalChainBroken  = "incremental_chain_broken"
	DatabaseProtectionIssueLogChainGap             = "log_chain_gap"
	DatabaseProtectionIssueRunnerOffline           = "runner_offline"
	DatabaseProtectionIssueRunnerToolMissing       = "runner_tool_missing"
	DatabaseProtectionIssueStoragePostureFailed    = "storage_posture_failed"
	DatabaseProtectionIssueRestoreDrillMissing     = "restore_drill_missing"
	DatabaseProtectionIssueRestoreDrillFailed      = "restore_drill_failed"
	DatabaseProtectionIssueReplicaDelayUnavailable = "replica_delay_unavailable"
	DatabaseProtectionIssueArchiveLagHigh          = "archive_lag_high"
)

type DatabaseProtectionRiskListRequest struct {
	Page               int    `form:"page"`
	PageSize           int    `form:"pageSize"`
	Keyword            string `form:"keyword"`
	InstanceID         uint   `form:"instanceId"`
	Engine             string `form:"engine"`
	RiskLevel          string `form:"riskLevel"`
	IssueType          string `form:"issueType"`
	ProductionOnly     string `form:"productionOnly"`
	RestrictToAllowed  bool   `form:"-" json:"-"`
	AllowedInstanceIDs []uint `form:"-" json:"-"`
}

type DatabaseProtectionRiskVO struct {
	ID                      string `json:"id"`
	ProfileID               string `json:"profileId"`
	InstanceID              uint   `json:"instanceId"`
	InstanceName            string `json:"instanceName"`
	Engine                  string `json:"engine"`
	EngineText              string `json:"engineText"`
	Endpoint                string `json:"endpoint"`
	Environment             string `json:"environment"`
	BusinessSystem          string `json:"businessSystem"`
	Owner                   string `json:"owner"`
	InstanceRole            string `json:"instanceRole"`
	InstanceRoleText        string `json:"instanceRoleText"`
	ReplicaRole             string `json:"replicaRole"`
	ReplicaRoleText         string `json:"replicaRoleText"`
	PrimaryInstanceID       uint   `json:"primaryInstanceId"`
	PrimaryInstanceName     string `json:"primaryInstanceName"`
	PrimaryEndpoint         string `json:"primaryEndpoint"`
	BackupRequirement       string `json:"backupRequirement"`
	BackupRequirementText   string `json:"backupRequirementText"`
	InheritedProtection     bool   `json:"inheritedProtection"`
	InheritedProtectionText string `json:"inheritedProtectionText"`
	ProtectionMode          string `json:"protectionMode"`
	ProtectionModeText      string `json:"protectionModeText"`
	ProtectionLevel         string `json:"protectionLevel"`
	ProtectionLevelText     string `json:"protectionLevelText"`
	RiskLevel               string `json:"riskLevel"`
	RiskLevelText           string `json:"riskLevelText"`
	IssueType               string `json:"issueType"`
	IssueTypeText           string `json:"issueTypeText"`
	Message                 string `json:"message"`
	Action                  string `json:"action"`
	ActionText              string `json:"actionText"`
	Blocking                bool   `json:"blocking"`
	RecoverableUntil        string `json:"recoverableUntil"`
	LastFullAt              string `json:"lastFullAt"`
	LastLogArchiveAt        string `json:"lastLogArchiveAt"`
	CheckedAt               string `json:"checkedAt"`
}

func (uc *UseCase) ListProtectionRisks(ctx context.Context, req *DatabaseProtectionRiskListRequest) ([]*DatabaseProtectionRiskVO, int64, error) {
	normalizeProtectionRiskListRequest(req)
	if req == nil {
		req = &DatabaseProtectionRiskListRequest{Page: 1, PageSize: 10}
	}
	profileReq := &DatabaseProtectionProfileListRequest{
		Page:               1,
		PageSize:           100,
		Keyword:            req.Keyword,
		InstanceID:         req.InstanceID,
		Engine:             req.Engine,
		RiskLevel:          req.RiskLevel,
		RestrictToAllowed:  req.RestrictToAllowed,
		AllowedInstanceIDs: req.AllowedInstanceIDs,
	}
	risks := make([]*DatabaseProtectionRiskVO, 0, 128)
	for {
		profiles, _, err := uc.ListProtectionProfiles(ctx, profileReq)
		if err != nil {
			return nil, 0, err
		}
		for _, profile := range profiles {
			if !protectionRiskProfileMatches(profile, req) {
				continue
			}
			risks = append(risks, expandProtectionRisks(profile, req.IssueType)...)
		}
		if len(profiles) < profileReq.PageSize {
			break
		}
		profileReq.Page++
		if profileReq.Page > 200 {
			break
		}
	}
	sort.SliceStable(risks, func(i, j int) bool {
		left, right := risks[i], risks[j]
		if protectionRiskRank(left.RiskLevel) != protectionRiskRank(right.RiskLevel) {
			return protectionRiskRank(left.RiskLevel) > protectionRiskRank(right.RiskLevel)
		}
		if left.Blocking != right.Blocking {
			return left.Blocking
		}
		return left.InstanceID > right.InstanceID
	})
	total := int64(len(risks))
	start := (req.Page - 1) * req.PageSize
	if start >= len(risks) {
		return []*DatabaseProtectionRiskVO{}, total, nil
	}
	end := start + req.PageSize
	if end > len(risks) {
		end = len(risks)
	}
	return risks[start:end], total, nil
}

func expandProtectionRisks(profile *DatabaseProtectionProfileVO, issueFilter string) []*DatabaseProtectionRiskVO {
	if profile == nil || len(profile.RiskMessages) == 0 {
		return nil
	}
	actions := profile.RecommendedActions
	if len(actions) == 0 {
		actions = []DatabaseProtectionActionVO{{Level: profile.RiskLevel, Action: "inspect_profile", Text: "查看保护概览", Blocking: false}}
	}
	risks := make([]*DatabaseProtectionRiskVO, 0, len(profile.RiskMessages))
	for index, message := range profile.RiskMessages {
		action := protectionActionForMessage(message, actions)
		issueType := protectionIssueTypeForRisk(message, action)
		if issueFilter != "" && issueType != issueFilter {
			continue
		}
		level := firstNonEmpty(action.Level, profile.RiskLevel)
		item := &DatabaseProtectionRiskVO{
			ID:                      fmt.Sprintf("%s:%s:%d", profile.ProfileID, issueType, index),
			ProfileID:               profile.ProfileID,
			InstanceID:              profile.InstanceID,
			InstanceName:            profile.InstanceName,
			Engine:                  profile.Engine,
			EngineText:              profile.EngineText,
			Endpoint:                profile.Endpoint,
			Environment:             profile.Environment,
			BusinessSystem:          profile.BusinessSystem,
			Owner:                   profile.Owner,
			InstanceRole:            profile.InstanceRole,
			InstanceRoleText:        profile.InstanceRoleText,
			ReplicaRole:             profile.ReplicaRole,
			ReplicaRoleText:         profile.ReplicaRoleText,
			PrimaryInstanceID:       profile.PrimaryInstanceID,
			PrimaryInstanceName:     profile.PrimaryInstanceName,
			PrimaryEndpoint:         profile.PrimaryEndpoint,
			BackupRequirement:       profile.BackupRequirement,
			BackupRequirementText:   profile.BackupRequirementText,
			InheritedProtection:     profile.InheritedProtection,
			InheritedProtectionText: profile.InheritedProtectionText,
			ProtectionMode:          profile.ProtectionMode,
			ProtectionModeText:      profile.ProtectionModeText,
			ProtectionLevel:         profile.ProtectionLevel,
			ProtectionLevelText:     profile.ProtectionLevelText,
			RiskLevel:               level,
			RiskLevelText:           RiskLevelText(level),
			IssueType:               issueType,
			IssueTypeText:           ProtectionIssueTypeText(issueType),
			Message:                 message,
			Action:                  action.Action,
			ActionText:              action.Text,
			Blocking:                action.Blocking,
			RecoverableUntil:        profile.RecoverableUntil,
			LastFullAt:              profile.LastFullAt,
			LastLogArchiveAt:        profile.LastLogArchiveAt,
			CheckedAt:               profile.ValidatedAt,
		}
		risks = append(risks, item)
	}
	return risks
}

func protectionActionForMessage(message string, actions []DatabaseProtectionActionVO) DatabaseProtectionActionVO {
	issue := protectionIssueTypeForRisk(message, DatabaseProtectionActionVO{})
	for _, action := range actions {
		if protectionIssueTypeForRisk(message, action) == issue {
			return action
		}
	}
	for _, action := range actions {
		if action.Blocking {
			return action
		}
	}
	if len(actions) > 0 {
		return actions[0]
	}
	return DatabaseProtectionActionVO{}
}

func protectionIssueTypeForRisk(message string, action DatabaseProtectionActionVO) string {
	actionName := strings.TrimSpace(action.Action)
	switch actionName {
	case "run_full_backup", "enable_protection", "enable_backup_policy", "configure_barman_server":
		return DatabaseProtectionIssueMissingFullBackup
	case "validate_backup_chain", "inspect_backup_failure":
		return DatabaseProtectionIssueIncrementalChainBroken
	case "start_log_archive_stream", "inspect_log_archive_stream", "resume_log_archive_stream", "sync_barman_wal":
		return DatabaseProtectionIssueLogChainGap
	case "configure_runner", "repair_runner":
		return DatabaseProtectionIssueRunnerOffline
	case "check_storage_posture":
		return DatabaseProtectionIssueStoragePostureFailed
	case "run_restore_drill":
		if strings.Contains(message, "失败") {
			return DatabaseProtectionIssueRestoreDrillFailed
		}
		return DatabaseProtectionIssueRestoreDrillMissing
	case "inspect_archive_lag":
		return DatabaseProtectionIssueArchiveLagHigh
	case "check_barman_server", "sync_barman_catalog":
		return DatabaseProtectionIssueRunnerToolMissing
	}
	normalized := strings.ToLower(message)
	switch {
	case strings.Contains(message, "缺少可用全量") || strings.Contains(message, "没有启用备份") || strings.Contains(message, "未纳管 PostgreSQL Barman"):
		return DatabaseProtectionIssueMissingFullBackup
	case strings.Contains(message, "断链") || strings.Contains(message, "备份链") || strings.Contains(message, "物理备份失败"):
		return DatabaseProtectionIssueIncrementalChainBroken
	case strings.Contains(message, "归档") || strings.Contains(message, "WAL") || strings.Contains(message, "binlog") || strings.Contains(message, "timeline"):
		if strings.Contains(message, "延迟超过") {
			return DatabaseProtectionIssueArchiveLagHigh
		}
		return DatabaseProtectionIssueLogChainGap
	case strings.Contains(message, "Runner"):
		if strings.Contains(message, "工具") || strings.Contains(normalized, "barman") {
			return DatabaseProtectionIssueRunnerToolMissing
		}
		return DatabaseProtectionIssueRunnerOffline
	case strings.Contains(message, "存储"):
		return DatabaseProtectionIssueStoragePostureFailed
	case strings.Contains(message, "恢复演练") || strings.Contains(message, "proof"):
		if strings.Contains(message, "失败") {
			return DatabaseProtectionIssueRestoreDrillFailed
		}
		return DatabaseProtectionIssueRestoreDrillMissing
	case strings.Contains(message, "延迟副本"):
		return DatabaseProtectionIssueReplicaDelayUnavailable
	default:
		return DatabaseProtectionIssueIncrementalChainBroken
	}
}

func ProtectionIssueTypeText(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseProtectionIssueMissingFullBackup:
		return "缺少全量基线"
	case DatabaseProtectionIssueIncrementalChainBroken:
		return "增量链异常"
	case DatabaseProtectionIssueLogChainGap:
		return "日志链缺口"
	case DatabaseProtectionIssueRunnerOffline:
		return "Runner 不可用"
	case DatabaseProtectionIssueRunnerToolMissing:
		return "Runner 工具异常"
	case DatabaseProtectionIssueStoragePostureFailed:
		return "存储姿态异常"
	case DatabaseProtectionIssueRestoreDrillMissing:
		return "缺少恢复演练"
	case DatabaseProtectionIssueRestoreDrillFailed:
		return "恢复演练失败"
	case DatabaseProtectionIssueReplicaDelayUnavailable:
		return "延迟副本不可用"
	case DatabaseProtectionIssueArchiveLagHigh:
		return "归档延迟过高"
	default:
		return "保护风险"
	}
}

func normalizeProtectionRiskListRequest(req *DatabaseProtectionRiskListRequest) {
	if req == nil {
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	req.Keyword = strings.TrimSpace(req.Keyword)
	req.Engine = normalizeDBType(req.Engine)
	req.RiskLevel = strings.TrimSpace(req.RiskLevel)
	req.IssueType = strings.TrimSpace(req.IssueType)
	req.ProductionOnly = strings.TrimSpace(req.ProductionOnly)
}

func protectionRiskProfileMatches(profile *DatabaseProtectionProfileVO, req *DatabaseProtectionRiskListRequest) bool {
	if profile == nil || req == nil {
		return true
	}
	if req.ProductionOnly == "true" || req.ProductionOnly == "1" {
		env := strings.ToLower(strings.TrimSpace(profile.Environment))
		if env != "prod" && env != "production" && env != "生产" {
			return false
		}
	}
	return true
}
