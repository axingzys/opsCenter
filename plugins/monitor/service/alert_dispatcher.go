package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ydcloud-dy/opshub/plugins/monitor/model"
	"gorm.io/gorm"
)

// AlertDispatcher 负责装载通道、接收人和发送告警。
type AlertDispatcher struct {
	db           *gorm.DB
	alertService *AlertService
}

func NewAlertDispatcher(db *gorm.DB) *AlertDispatcher {
	return &AlertDispatcher{
		db:           db,
		alertService: NewAlertService(),
	}
}

func (d *AlertDispatcher) Dispatch(message AlertMessage, channelIDs ...uint) (string, error) {
	channels, err := d.loadEnabledChannels(channelIDs)
	if err != nil {
		return "", err
	}
	if len(channels) == 0 {
		return "", fmt.Errorf("未配置启用的告警通道")
	}

	receivers, relations, err := d.loadReceiversAndRelations(channels)
	if err != nil {
		return summarizeChannelTypes(channels), err
	}

	emailReceivers := collectEmailReceivers(receivers)
	hasDirectTarget := hasDirectWebhookChannel(channels)
	if len(emailReceivers) == 0 && len(relations) == 0 && !hasDirectTarget {
		return summarizeChannelTypes(channels), fmt.Errorf("未配置可用的告警接收目标")
	}

	var dispatchErrors []error
	var successCount int

	for _, channel := range channels {
		channelConfig := AlertChannelConfig{}
		if err := applyAlertChannelConfig(&channelConfig, &channel); err != nil {
			dispatchErrors = append(dispatchErrors, fmt.Errorf("%s 配置解析失败: %w", channel.Name, err))
			continue
		}

		switch channel.ChannelType {
		case "email":
			if len(emailReceivers) == 0 {
				dispatchErrors = append(dispatchErrors, fmt.Errorf("%s 未配置邮件接收人", channel.Name))
				continue
			}
			if err := d.alertService.sendEmail(message, channelConfig, emailReceivers); err != nil {
				dispatchErrors = append(dispatchErrors, fmt.Errorf("%s 发送失败: %w", channel.Name, err))
				continue
			}
		case "webhook":
			if err := d.alertService.sendWebhook(message, channelConfig); err != nil {
				dispatchErrors = append(dispatchErrors, fmt.Errorf("%s 发送失败: %w", channel.Name, err))
				continue
			}
		case "wechat":
			targets := filterReceiverTargets(relations, channel.ID, "wechat")
			if err := d.alertService.sendWeChat(message, channelConfig, targets); err != nil {
				dispatchErrors = append(dispatchErrors, fmt.Errorf("%s 发送失败: %w", channel.Name, err))
				continue
			}
		case "dingtalk":
			targets := filterReceiverTargets(relations, channel.ID, "dingtalk")
			if err := d.alertService.sendDingTalk(message, channelConfig, targets); err != nil {
				dispatchErrors = append(dispatchErrors, fmt.Errorf("%s 发送失败: %w", channel.Name, err))
				continue
			}
		case "feishu":
			targets := filterReceiverTargets(relations, channel.ID, "feishu")
			if err := d.alertService.sendFeishu(message, channelConfig, targets); err != nil {
				dispatchErrors = append(dispatchErrors, fmt.Errorf("%s 发送失败: %w", channel.Name, err))
				continue
			}
		default:
			dispatchErrors = append(dispatchErrors, fmt.Errorf("%s 使用了不支持的通道类型 %s", channel.Name, channel.ChannelType))
			continue
		}
		successCount++
	}

	if successCount == 0 && len(dispatchErrors) > 0 {
		return summarizeChannelTypes(channels), fmt.Errorf("所有告警通道发送失败: %v", dispatchErrors)
	}
	return summarizeChannelTypes(channels), nil
}

func (d *AlertDispatcher) loadEnabledChannels(channelIDs []uint) ([]model.AlertChannel, error) {
	channelIDs = sanitizeDispatcherChannelIDs(channelIDs)
	var channels []model.AlertChannel
	query := d.db.Where("enabled = ?", true)
	if len(channelIDs) > 0 {
		query = query.Where("id IN ?", channelIDs)
	}
	if err := query.Order("id ASC").Find(&channels).Error; err != nil {
		return nil, fmt.Errorf("获取告警通道失败: %w", err)
	}
	return channels, nil
}

func (d *AlertDispatcher) loadReceiversAndRelations(channels []model.AlertChannel) ([]model.AlertReceiver, []ReceiverChannelRelation, error) {
	var receivers []model.AlertReceiver
	if err := d.db.Find(&receivers).Error; err != nil {
		return nil, nil, fmt.Errorf("获取告警接收人失败: %w", err)
	}

	if len(receivers) == 0 {
		return []model.AlertReceiver{}, []ReceiverChannelRelation{}, nil
	}

	channelMap := make(map[uint]*model.AlertChannel, len(channels))
	channelIDs := make([]uint, 0, len(channels))
	for i := range channels {
		channelMap[channels[i].ID] = &channels[i]
		channelIDs = append(channelIDs, channels[i].ID)
	}

	var receiverChannels []model.AlertReceiverChannel
	if len(channelIDs) > 0 {
		if err := d.db.Where("channel_id IN ?", channelIDs).Find(&receiverChannels).Error; err != nil {
			return nil, nil, fmt.Errorf("获取接收人通道关联失败: %w", err)
		}
	}

	receiverMap := make(map[uint]*model.AlertReceiver, len(receivers))
	for i := range receivers {
		receiverMap[receivers[i].ID] = &receivers[i]
	}

	relations := make([]ReceiverChannelRelation, 0, len(receiverChannels))
	for _, rc := range receiverChannels {
		receiver, receiverExists := receiverMap[rc.ReceiverID]
		channel, channelExists := channelMap[rc.ChannelID]
		if !receiverExists || !channelExists {
			continue
		}

		shouldAdd := false
		switch channel.ChannelType {
		case "email":
			shouldAdd = receiver.EnableEmail && strings.TrimSpace(receiver.Email) != ""
		case "feishu":
			shouldAdd = receiver.EnableFeishu && strings.TrimSpace(receiver.FeishuID) != ""
		case "dingtalk":
			shouldAdd = receiver.EnableDingTalk && (strings.TrimSpace(receiver.DingTalkID) != "" || strings.TrimSpace(receiver.Phone) != "")
		case "wechat":
			shouldAdd = receiver.EnableWeChat && strings.TrimSpace(receiver.WeChatID) != ""
		}
		if !shouldAdd {
			continue
		}

		relations = append(relations, ReceiverChannelRelation{
			ReceiverID:  rc.ReceiverID,
			ChannelID:   rc.ChannelID,
			ChannelType: channel.ChannelType,
			Receiver: ReceiverInfo{
				ID:         receiver.ID,
				Name:       receiver.Name,
				Email:      receiver.Email,
				Phone:      receiver.Phone,
				FeishuID:   receiver.FeishuID,
				DingTalkID: receiver.DingTalkID,
				WeChatID:   receiver.WeChatID,
			},
			ChannelConfig: rc.Config,
		})
	}
	return receivers, relations, nil
}

func applyAlertChannelConfig(cfg *AlertChannelConfig, channel *model.AlertChannel) error {
	if cfg == nil || channel == nil {
		return nil
	}
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(channel.Config), &config); err != nil {
		return err
	}

	switch channel.ChannelType {
	case "email":
		if smtpHost, ok := config["smtpHost"].(string); ok {
			cfg.SMTPHost = smtpHost
		}
		if smtpPort, ok := config["smtpPort"].(float64); ok {
			cfg.SMTPPort = int(smtpPort)
		}
		if smtpUser, ok := config["smtpUser"].(string); ok {
			cfg.SMTPUser = smtpUser
		}
		if smtpPassword, ok := config["smtpPassword"].(string); ok {
			cfg.SMTPPassword = smtpPassword
		}
		if fromEmail, ok := config["fromEmail"].(string); ok {
			cfg.FromEmail = fromEmail
		}
		if fromName, ok := config["fromName"].(string); ok {
			cfg.FromName = fromName
		}
	case "webhook":
		if webhookURL, ok := config["webhookUrl"].(string); ok {
			cfg.WebhookURL = webhookURL
		}
	case "wechat":
		if webhook, ok := config["wechatWebhook"].(string); ok {
			cfg.WeChatWebhook = webhook
		}
	case "dingtalk":
		if webhook, ok := config["dingtalkWebhook"].(string); ok {
			cfg.DingTalkWebhook = webhook
		}
		if secret, ok := config["dingtalkSecret"].(string); ok {
			cfg.DingTalkSecret = secret
		}
	case "feishu":
		if webhook, ok := config["feishuWebhook"].(string); ok {
			cfg.FeishuWebhook = webhook
		}
	}
	return nil
}

func collectEmailReceivers(receivers []model.AlertReceiver) []string {
	items := make([]string, 0, len(receivers))
	for _, receiver := range receivers {
		if receiver.EnableEmail && strings.TrimSpace(receiver.Email) != "" {
			items = append(items, strings.TrimSpace(receiver.Email))
		}
	}
	return items
}

func hasDirectWebhookChannel(channels []model.AlertChannel) bool {
	for _, channel := range channels {
		switch strings.TrimSpace(channel.ChannelType) {
		case "webhook", "wechat", "dingtalk", "feishu":
			return true
		}
	}
	return false
}

func filterReceiverTargets(relations []ReceiverChannelRelation, channelID uint, channelType string) *[]ReceiverInfo {
	items := make([]ReceiverInfo, 0)
	for _, relation := range relations {
		if relation.ChannelID != channelID || relation.ChannelType != channelType {
			continue
		}
		items = append(items, relation.Receiver)
	}
	if len(items) == 0 {
		return nil
	}
	return &items
}

func sanitizeDispatcherChannelIDs(channelIDs []uint) []uint {
	seen := make(map[uint]struct{}, len(channelIDs))
	items := make([]uint, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		if channelID == 0 {
			continue
		}
		if _, ok := seen[channelID]; ok {
			continue
		}
		seen[channelID] = struct{}{}
		items = append(items, channelID)
	}
	return items
}

func summarizeChannelTypes(channels []model.AlertChannel) string {
	if len(channels) == 0 {
		return ""
	}
	if len(channels) == 1 {
		return channels[0].ChannelType
	}
	types := make([]string, 0, len(channels))
	for _, channel := range channels {
		if strings.TrimSpace(channel.ChannelType) == "" {
			continue
		}
		types = append(types, channel.ChannelType)
	}
	if len(types) == 0 {
		return "multiple"
	}
	return strings.Join(types, ",")
}
