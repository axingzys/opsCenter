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
	"time"

	"gorm.io/gorm"
)

// TerminalSession SSH终端会话模型
type TerminalSession struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deletedAt,omitempty"`
	HostID        uint           `gorm:"column:host_id;not null;comment:主机ID" json:"hostId"`
	HostName      string         `gorm:"type:varchar(100);comment:主机名称" json:"hostName"`
	HostIP        string         `gorm:"type:varchar(50);comment:主机IP" json:"hostIp"`
	UserID        uint           `gorm:"column:user_id;not null;comment:操作用户ID" json:"userId"`
	Username      string         `gorm:"type:varchar(100);comment:用户名" json:"username"`
	RecordingPath string         `gorm:"type:varchar(500);comment:录制文件路径" json:"recordingPath"`
	Duration      int            `gorm:"type:int;comment:会话时长(秒)" json:"duration"`
	FileSize      int64          `gorm:"type:bigint;comment:文件大小(字节)" json:"fileSize"`
	Status        string         `gorm:"type:varchar(20);default:'recording';comment:会话状态 recording/completed/failed/timeout" json:"status"`
	StartedAt     *time.Time     `gorm:"column:started_at;comment:开始时间" json:"startedAt,omitempty"`
	EndedAt       *time.Time     `gorm:"column:ended_at;comment:结束时间" json:"endedAt,omitempty"`
	CloseReason   string         `gorm:"type:varchar(100);comment:结束原因" json:"closeReason,omitempty"`
}

// TableName 表名
func (TerminalSession) TableName() string {
	return "ssh_terminal_sessions"
}

// TerminalCommandEvent SSH高危命令事件
type TerminalCommandEvent struct {
	ID                uint           `gorm:"primarykey" json:"id"`
	CreatedAt         time.Time      `json:"createdAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"deletedAt,omitempty"`
	SessionID         uint           `gorm:"column:session_id;not null;index;comment:终端会话ID" json:"sessionId"`
	HostID            uint           `gorm:"column:host_id;not null;index;comment:主机ID" json:"hostId"`
	HostName          string         `gorm:"type:varchar(100);comment:主机名称" json:"hostName"`
	HostIP            string         `gorm:"type:varchar(50);comment:主机IP" json:"hostIp"`
	UserID            uint           `gorm:"column:user_id;not null;index;comment:操作用户ID" json:"userId"`
	Username          string         `gorm:"type:varchar(100);comment:用户名" json:"username"`
	CommandText       string         `gorm:"type:text;comment:原始命令" json:"commandText"`
	NormalizedCommand string         `gorm:"type:text;comment:归一化命令" json:"normalizedCommand"`
	RiskLevel         string         `gorm:"type:varchar(20);index;comment:风险等级 high/medium" json:"riskLevel"`
	RuleCode          string         `gorm:"type:varchar(100);comment:规则编码" json:"ruleCode"`
	RuleName          string         `gorm:"type:varchar(200);comment:规则名称" json:"ruleName"`
	RuleDescription   string         `gorm:"type:varchar(500);comment:规则说明" json:"ruleDescription"`
	Source            string         `gorm:"type:varchar(20);default:'input';comment:来源 input" json:"source"`
	Confidence        string         `gorm:"type:varchar(20);default:'medium';comment:置信度 low/medium/high" json:"confidence"`
	ExecutedAt        time.Time      `gorm:"column:executed_at;index;comment:执行时间" json:"executedAt"`
}

// TableName 表名
func (TerminalCommandEvent) TableName() string {
	return "ssh_terminal_command_events"
}

// TerminalSessionInfo 终端会话信息VO
type TerminalSessionInfo struct {
	ID                 uint      `json:"id"`
	HostID             uint      `json:"hostId"`
	HostName           string    `json:"hostName"`
	HostIP             string    `json:"hostIp"`
	UserID             uint      `json:"userId"`
	Username           string    `json:"username"`
	Duration           int       `json:"duration"`
	DurationText       string    `json:"durationText"` // 格式化的时长，如 "1m 30s"
	FileSize           int64     `json:"fileSize"`
	FileSizeText       string    `json:"fileSizeText"` // 格式化的文件大小，如 "1.5 MB"
	Status             string    `json:"status"`
	StatusText         string    `json:"statusText"`
	RecordingAvailable bool      `json:"recordingAvailable"`
	RecordingIssue     string    `json:"recordingIssue"`
	HighRiskCount      int       `json:"highRiskCount"`
	MediumRiskCount    int       `json:"mediumRiskCount"`
	TopRiskLevel       string    `json:"topRiskLevel"`
	TopRiskLevelText   string    `json:"topRiskLevelText"`
	CreatedAt          time.Time `json:"createdAt"`
	CreatedAtText      string    `json:"createdAtText"` // 兼容旧字段
	StartedAt          time.Time `json:"startedAt"`
	StartedAtText      string    `json:"startedAtText"`
	EndedAt            time.Time `json:"endedAt"`
	EndedAtText        string    `json:"endedAtText"`
	CloseReason        string    `json:"closeReason"`
}

// TerminalCommandEventInfo 高危命令事件VO
type TerminalCommandEventInfo struct {
	ID                uint      `json:"id"`
	SessionID         uint      `json:"sessionId"`
	HostID            uint      `json:"hostId"`
	HostName          string    `json:"hostName"`
	HostIP            string    `json:"hostIp"`
	UserID            uint      `json:"userId"`
	Username          string    `json:"username"`
	CommandText       string    `json:"commandText"`
	NormalizedCommand string    `json:"normalizedCommand"`
	RiskLevel         string    `json:"riskLevel"`
	RiskLevelText     string    `json:"riskLevelText"`
	RuleCode          string    `json:"ruleCode"`
	RuleName          string    `json:"ruleName"`
	RuleDescription   string    `json:"ruleDescription"`
	Source            string    `json:"source"`
	Confidence        string    `json:"confidence"`
	ExecutedAt        time.Time `json:"executedAt"`
	ExecutedAtText    string    `json:"executedAtText"`
}

// TerminalSessionListRequest 终端会话列表请求
type TerminalSessionListRequest struct {
	Page     int    `form:"page" binding:"required,min=1"`
	PageSize int    `form:"pageSize" binding:"required,min=1,max=100"`
	Keyword  string `form:"keyword"` // 搜索关键词（主机名、IP）
}

// TerminalSessionListResponse 终端会话列表响应
type TerminalSessionListResponse struct {
	Total int64                  `json:"total"`
	List  []*TerminalSessionInfo `json:"list"`
}
