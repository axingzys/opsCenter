package model

import "time"

// HostAlertRule 主机阈值告警规则。
type HostAlertRule struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	Name           string    `gorm:"type:varchar(100);not null" json:"name"`
	HostID         *uint     `gorm:"index" json:"hostId"`
	Metric         string    `gorm:"type:varchar(50);not null;index" json:"metric"`
	Threshold      float64   `gorm:"type:double;not null" json:"threshold"`
	AlertInterval  int       `gorm:"type:int;default:600" json:"alertInterval"`
	Severity       string    `gorm:"type:varchar(20);default:'warning'" json:"severity"`
	Enabled        bool      `gorm:"type:tinyint(1);default:1" json:"enabled"`
	Description    string    `gorm:"type:varchar(255)" json:"description"`
	ChannelIDsJSON string    `gorm:"column:channel_ids_json;type:json;comment:告警通道ID列表" json:"-"`
	ChannelIDs     []uint    `gorm:"-" json:"channelIds"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (HostAlertRule) TableName() string {
	return "host_alert_rules"
}
