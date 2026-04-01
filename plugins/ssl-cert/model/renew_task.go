package model

import (
	"time"

	"gorm.io/gorm"
)

// RenewTask 续期任务
type RenewTask struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	CertificateID uint   `gorm:"index;not null" json:"certificate_id"`
	TaskType      string `gorm:"type:varchar(20);not null" json:"task_type"`       // issue/renew/deploy
	Status        string `gorm:"type:varchar(20);default:'pending'" json:"status"` // pending/running/success/failed
	TriggerType   string `gorm:"type:varchar(20);not null" json:"trigger_type"`    // auto/manual

	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	ErrorMessage string     `gorm:"type:text" json:"error_message"`
	Result       string     `gorm:"type:text" json:"result"` // 结果JSON

	// 关联
	Certificate *SSLCertificate `gorm:"foreignKey:CertificateID" json:"certificate,omitempty"`
}

// TableName 表名
func (RenewTask) TableName() string {
	return "ssl_renew_tasks"
}

// 任务类型常量
const (
	TaskTypeIssue  = "issue"  // 签发证书
	TaskTypeRenew  = "renew"  // 续期证书
	TaskTypeDeploy = "deploy" // 部署证书
)

// 任务状态常量
const (
	TaskStatusPending = "pending" // 待执行
	TaskStatusRunning = "running" // 执行中
	TaskStatusSuccess = "success" // 成功
	TaskStatusFailed  = "failed"  // 失败
)

// 触发类型常量
const (
	TriggerTypeAuto   = "auto"   // 自动触发
	TriggerTypeManual = "manual" // 手动触发
)

// TaskResult 任务结果
type TaskResult struct {
	Success      bool     `json:"success"`
	Message      string   `json:"message"`
	Certificate  string   `json:"certificate,omitempty"`   // 新证书(issue/renew任务)
	DeployedTo   []string `json:"deployed_to,omitempty"`   // 部署目标列表
	DeployErrors []string `json:"deploy_errors,omitempty"` // 部署错误列表
}
