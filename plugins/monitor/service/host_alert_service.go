package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	assetbiz "github.com/ydcloud-dy/opshub/internal/biz/asset"
	"github.com/ydcloud-dy/opshub/plugins/monitor/model"
	"gorm.io/gorm"
)

type HostAlertService struct {
	db         *gorm.DB
	dispatcher *AlertDispatcher
}

func NewHostAlertService(db *gorm.DB) *HostAlertService {
	return &HostAlertService{
		db:         db,
		dispatcher: NewAlertDispatcher(db),
	}
}

func (s *HostAlertService) ListRules(ctx context.Context) ([]model.HostAlertRule, error) {
	var rules []model.HostAlertRule
	if err := s.db.WithContext(ctx).Order("updated_at DESC").Find(&rules).Error; err != nil {
		return nil, err
	}
	for i := range rules {
		decodeHostAlertRuleChannelIDs(&rules[i])
	}
	return rules, nil
}

func (s *HostAlertService) GetRule(ctx context.Context, id uint) (*model.HostAlertRule, error) {
	var rule model.HostAlertRule
	if err := s.db.WithContext(ctx).First(&rule, id).Error; err != nil {
		return nil, err
	}
	decodeHostAlertRuleChannelIDs(&rule)
	return &rule, nil
}

func (s *HostAlertService) CreateRule(ctx context.Context, rule *model.HostAlertRule) error {
	rule.ChannelIDs = sanitizeChannelIDs(rule.ChannelIDs)
	rule.ChannelIDsJSON = encodeHostAlertRuleChannelIDs(rule.ChannelIDs)
	return s.db.WithContext(ctx).Create(rule).Error
}

func (s *HostAlertService) UpdateRule(ctx context.Context, id uint, req *model.HostAlertRule) (*model.HostAlertRule, error) {
	rule, err := s.GetRule(ctx, id)
	if err != nil {
		return nil, err
	}
	rule.Name = req.Name
	rule.HostID = req.HostID
	rule.Metric = req.Metric
	rule.Threshold = req.Threshold
	rule.AlertInterval = req.AlertInterval
	rule.Severity = req.Severity
	rule.Enabled = req.Enabled
	rule.Description = req.Description
	rule.ChannelIDs = sanitizeChannelIDs(req.ChannelIDs)
	rule.ChannelIDsJSON = encodeHostAlertRuleChannelIDs(rule.ChannelIDs)
	if err := s.db.WithContext(ctx).Save(rule).Error; err != nil {
		return nil, err
	}
	decodeHostAlertRuleChannelIDs(rule)
	return rule, nil
}

func (s *HostAlertService) DeleteRule(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&model.HostAlertRule{}, id).Error
}

func (s *HostAlertService) GetStats(ctx context.Context) (map[string]int64, error) {
	var total int64
	if err := s.db.WithContext(ctx).Model(&model.HostAlertRule{}).Count(&total).Error; err != nil {
		return nil, err
	}
	var enabled int64
	if err := s.db.WithContext(ctx).Model(&model.HostAlertRule{}).Where("enabled = ?", true).Count(&enabled).Error; err != nil {
		return nil, err
	}
	return map[string]int64{
		"total":    total,
		"enabled":  enabled,
		"disabled": total - enabled,
	}, nil
}

func (s *HostAlertService) ListAssignableHosts(ctx context.Context) ([]assetbiz.HostListVO, error) {
	var hosts []assetbiz.Host
	if err := applyAgentManagedHostScope(s.db.WithContext(ctx), "").
		Order("os_type ASC, name ASC").
		Find(&hosts).Error; err != nil {
		return nil, err
	}

	result := make([]assetbiz.HostListVO, 0, len(hosts))
	for _, host := range hosts {
		result = append(result, assetbiz.HostListVO{
			ID:               host.ID,
			Name:             host.Name,
			IP:               host.IP,
			Status:           host.Status,
			Port:             host.Port,
			SSHUser:          host.SSHUser,
			OSType:           host.OSType,
			ManagementMode:   host.ManagementMode,
			CollectStatus:    host.CollectStatus,
			DesktopEnabled:   host.DesktopEnabled,
			DesktopPort:      host.DesktopPort,
			OS:               host.OS,
			PrimaryPrivateIP: host.PrimaryPrivateIP,
			PrimaryPublicIP:  host.PrimaryPublicIP,
		})
	}
	return result, nil
}

func (s *HostAlertService) CheckRules(ctx context.Context) {
	var rules []model.HostAlertRule
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).Find(&rules).Error; err != nil {
		return
	}

	for _, rule := range rules {
		hosts, err := s.listRuleHosts(ctx, &rule)
		if err != nil {
			continue
		}
		for i := range hosts {
			host := &hosts[i]
			if !evaluateHostAlertRule(host, &rule) {
				continue
			}
			if s.shouldSuppress(ctx, &rule, host.ID) {
				continue
			}

			message := buildHostAlertMessage(host, &rule)
			channelSummary, err := s.dispatcher.Dispatch(message, rule.ChannelIDs...)
			status := "success"
			errorMsg := ""
			if err != nil {
				status = "failed"
				errorMsg = err.Error()
			}
			s.writeAlertLog(ctx, &rule, host, message, status, channelSummary, errorMsg)
		}
	}
}

func (s *HostAlertService) listRuleHosts(ctx context.Context, rule *model.HostAlertRule) ([]assetbiz.Host, error) {
	query := applyAgentManagedHostScope(s.db.WithContext(ctx).Model(&assetbiz.Host{}), "")
	if rule.HostID != nil && *rule.HostID > 0 {
		query = query.Where("id = ?", *rule.HostID)
	}

	var hosts []assetbiz.Host
	if err := query.Find(&hosts).Error; err != nil {
		return nil, err
	}
	return hosts, nil
}

func (s *HostAlertService) shouldSuppress(ctx context.Context, rule *model.HostAlertRule, hostID uint) bool {
	interval := time.Duration(rule.AlertInterval) * time.Second
	if interval <= 0 {
		interval = 10 * time.Minute
	}

	alertType := mapHostAlertType(rule.Metric)
	var count int64
	s.db.WithContext(ctx).Model(&model.AlertLog{}).
		Where("resource_type = ? AND resource_id = ? AND alert_rule_id = ? AND alert_type = ? AND status = ? AND sent_at >= ?",
			"host", hostID, rule.ID, alertType, "success", time.Now().Add(-interval)).
		Count(&count)
	return count > 0
}

func (s *HostAlertService) writeAlertLog(ctx context.Context, rule *model.HostAlertRule, host *assetbiz.Host, message AlertMessage, status, channel, errorMsg string) {
	currentValue := message.CurrentValue
	thresholdValue := message.ThresholdValue

	alertLog := &model.AlertLog{
		AlertType:       message.AlertType,
		ResourceType:    "host",
		ResourceID:      host.ID,
		ResourceName:    host.Name,
		ResourceTarget:  firstNonEmptyHostValue(host.PrimaryPrivateIP, host.IP, host.PrimaryPublicIP),
		Metric:          rule.Metric,
		Severity:        firstNonEmptyString(rule.Severity, "warning"),
		CurrentValue:    currentValue,
		ThresholdValue:  thresholdValue,
		AlertRuleID:     &rule.ID,
		DomainMonitorID: 0,
		Domain:          firstNonEmptyHostValue(host.Name, host.IP),
		Status:          status,
		Message:         message.Message,
		ChannelType:     channel,
		ErrorMsg:        errorMsg,
		SentAt:          time.Now(),
	}
	_ = s.db.WithContext(ctx).Create(alertLog).Error
}

func buildHostAlertMessage(host *assetbiz.Host, rule *model.HostAlertRule) AlertMessage {
	now := time.Now()
	alertType := mapHostAlertType(rule.Metric)
	resourceTarget := firstNonEmptyHostValue(host.PrimaryPrivateIP, host.IP, host.PrimaryPublicIP)
	message := AlertMessage{
		AlertType:      alertType,
		ResourceType:   "host",
		ResourceID:     host.ID,
		ResourceName:   host.Name,
		ResourceTarget: resourceTarget,
		Metric:         rule.Metric,
		Severity:       firstNonEmptyString(rule.Severity, "warning"),
		Status:         firstNonEmptyString(rule.Severity, "warning"),
		Timestamp:      now.Format("2006-01-02 15:04:05"),
		AlertRuleID:    &rule.ID,
	}

	switch rule.Metric {
	case "cpu_usage":
		current := host.CPUUsage
		threshold := rule.Threshold
		alertMessageWithValues(&message, current, threshold)
		message.Message = fmt.Sprintf("主机 %s CPU 使用率 %.2f%%，超过阈值 %.2f%%", host.Name, current, threshold)
	case "memory_usage":
		current := host.MemoryUsage
		threshold := rule.Threshold
		alertMessageWithValues(&message, current, threshold)
		message.Message = fmt.Sprintf("主机 %s 内存使用率 %.2f%%，超过阈值 %.2f%%", host.Name, current, threshold)
	case "disk_usage":
		current := host.DiskUsage
		threshold := rule.Threshold
		alertMessageWithValues(&message, current, threshold)
		message.Message = fmt.Sprintf("主机 %s 磁盘使用率 %.2f%%，超过阈值 %.2f%%", host.Name, current, threshold)
	case "agent_offline":
		current := computeOfflineSeconds(host)
		threshold := rule.Threshold
		message.Status = "offline"
		alertMessageWithValues(&message, current, threshold)
		message.Message = fmt.Sprintf("主机 %s Agent 已离线 %.0f 秒，超过阈值 %.0f 秒", host.Name, current, threshold)
	default:
		message.Message = fmt.Sprintf("主机 %s 触发告警规则 %s", host.Name, rule.Name)
	}

	return message
}

func alertMessageWithValues(message *AlertMessage, current, threshold float64) {
	message.CurrentValue = &current
	message.ThresholdValue = &threshold
}

func computeOfflineSeconds(host *assetbiz.Host) float64 {
	if host == nil {
		return 0
	}
	if host.AgentLastHeartbeatAt == nil {
		if host.AgentLastReportAt != nil {
			return time.Since(*host.AgentLastReportAt).Seconds()
		}
		if host.LastSeen != nil {
			return time.Since(*host.LastSeen).Seconds()
		}
		return 0
	}
	return time.Since(*host.AgentLastHeartbeatAt).Seconds()
}

func mapHostAlertType(metric string) string {
	switch metric {
	case "cpu_usage":
		return "high_cpu_usage"
	case "memory_usage":
		return "high_memory_usage"
	case "disk_usage":
		return "high_disk_usage"
	case "agent_offline":
		return "agent_offline"
	default:
		return metric
	}
}

func firstNonEmptyHostValue(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func decodeHostAlertRuleChannelIDs(rule *model.HostAlertRule) {
	if rule == nil {
		return
	}
	rule.ChannelIDs = sanitizeChannelIDs(rule.ChannelIDs)
	raw := strings.TrimSpace(rule.ChannelIDsJSON)
	if raw == "" {
		return
	}
	var channelIDs []uint
	if err := json.Unmarshal([]byte(raw), &channelIDs); err != nil {
		rule.ChannelIDs = []uint{}
		return
	}
	rule.ChannelIDs = sanitizeChannelIDs(channelIDs)
}

func encodeHostAlertRuleChannelIDs(channelIDs []uint) string {
	channelIDs = sanitizeChannelIDs(channelIDs)
	if len(channelIDs) == 0 {
		return "[]"
	}
	data, err := json.Marshal(channelIDs)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func sanitizeChannelIDs(channelIDs []uint) []uint {
	if len(channelIDs) == 0 {
		return []uint{}
	}
	seen := make(map[uint]struct{}, len(channelIDs))
	result := make([]uint, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		if channelID == 0 {
			continue
		}
		if _, ok := seen[channelID]; ok {
			continue
		}
		seen[channelID] = struct{}{}
		result = append(result, channelID)
	}
	return result
}
