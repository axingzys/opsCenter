package asset

import (
	"time"

	"gorm.io/gorm"
)

// DesktopSession 远程桌面会话
type DesktopSession struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deletedAt,omitempty"`
	SessionUUID   string         `gorm:"column:session_uuid;type:varchar(64);not null;uniqueIndex;comment:会话UUID" json:"sessionUuid"`
	HostID        uint           `gorm:"column:host_id;not null;index;comment:主机ID" json:"hostId"`
	HostName      string         `gorm:"type:varchar(100);comment:主机名称" json:"hostName"`
	HostIP        string         `gorm:"type:varchar(50);comment:主机IP" json:"hostIp"`
	UserID        uint           `gorm:"column:user_id;not null;index;comment:操作用户ID" json:"userId"`
	Username      string         `gorm:"type:varchar(100);comment:用户名" json:"username"`
	Provider      string         `gorm:"type:varchar(20);default:'guacamole';comment:桌面网关" json:"provider"`
	Protocol      string         `gorm:"type:varchar(20);default:'rdp';comment:桌面协议" json:"protocol"`
	Status        string         `gorm:"type:varchar(20);default:'creating';comment:creating/active/closed/failed/timeout" json:"status"`
	ClientIP      string         `gorm:"type:varchar(64);comment:客户端IP" json:"clientIp"`
	Resolution    string         `gorm:"type:varchar(32);comment:分辨率" json:"resolution"`
	RecordingPath string         `gorm:"type:varchar(500);comment:录屏路径" json:"recordingPath"`
	StartedAt     *time.Time     `gorm:"column:started_at;comment:开始时间" json:"startedAt,omitempty"`
	EndedAt       *time.Time     `gorm:"column:ended_at;comment:结束时间" json:"endedAt,omitempty"`
	CloseReason   string         `gorm:"type:varchar(100);comment:结束原因" json:"closeReason,omitempty"`
}

// TableName 指定表名
func (DesktopSession) TableName() string {
	return "asset_desktop_sessions"
}

// DesktopLaunchRequest 创建桌面会话请求
type DesktopLaunchRequest struct {
	Width      int  `json:"width"`
	Height     int  `json:"height"`
	DPI        int  `json:"dpi"`
	Fullscreen bool `json:"fullscreen"`
}

// DesktopLaunchResponse 创建桌面会话响应
type DesktopLaunchResponse struct {
	SessionID   uint   `json:"sessionId"`
	SessionUUID string `json:"sessionUuid"`
	LaunchURL   string `json:"launchUrl"`
	Status      string `json:"status"`
}

// DesktopSessionInfo 桌面会话详情
type DesktopSessionInfo struct {
	ID                 uint   `json:"id"`
	SessionUUID        string `json:"sessionUuid"`
	HostID             uint   `json:"hostId"`
	HostName           string `json:"hostName"`
	HostIP             string `json:"hostIp"`
	UserID             uint   `json:"userId"`
	Username           string `json:"username"`
	Provider           string `json:"provider"`
	Protocol           string `json:"protocol"`
	Status             string `json:"status"`
	ClientIP           string `json:"clientIp"`
	Resolution         string `json:"resolution"`
	RecordingPath      string `json:"recordingPath"`
	RecordingAvailable bool   `json:"recordingAvailable"`
	FileSize           int64  `json:"fileSize"`
	DurationSeconds    int64  `json:"durationSeconds"`
	StartedAt          string `json:"startedAt,omitempty"`
	EndedAt            string `json:"endedAt,omitempty"`
	CloseReason        string `json:"closeReason,omitempty"`
	CreateTime         string `json:"createTime"`
	UpdateTime         string `json:"updateTime"`
}
