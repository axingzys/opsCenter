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
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	assetbiz "github.com/ydcloud-dy/opshub/internal/biz/asset"
	"github.com/ydcloud-dy/opshub/internal/conf"
	"github.com/ydcloud-dy/opshub/pkg/response"
	"gorm.io/gorm"
)

// TerminalAuditHandler 终端审计处理器
type TerminalAuditHandler struct {
	db             *gorm.DB
	recordingStore *terminalRecordingStore
}

// NewTerminalAuditHandler 创建终端审计处理器
func NewTerminalAuditHandler(db *gorm.DB, terminalCfg conf.TerminalConfig) *TerminalAuditHandler {
	return &TerminalAuditHandler{
		db:             db,
		recordingStore: newTerminalRecordingStore(terminalCfg),
	}
}

// ListTerminalSessions 获取终端会话列表
// @Summary 获取终端会话列表
// @Description 分页获取终端审计会话列表，支持搜索
// @Tags 终端审计
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(10)
// @Param keyword query string false "搜索关键字"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/terminal-sessions [get]
func (h *TerminalAuditHandler) ListTerminalSessions(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "10")
	keyword := c.Query("keyword")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 {
		pageSize = 10
	}

	query := h.db.Model(&assetbiz.TerminalSession{})
	if keyword != "" {
		query = query.Where("host_name LIKE ? OR host_ip LIKE ? OR username LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "查询失败")
		return
	}

	var sessions []*assetbiz.TerminalSession
	if err := query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&sessions).Error; err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "查询失败")
		return
	}

	riskSummary := h.loadRiskSummary(sessions)
	list := make([]*assetbiz.TerminalSessionInfo, 0, len(sessions))
	for _, session := range sessions {
		startAt, endAt, endedAtText := resolveTerminalSessionTimes(session)
		displayDuration := session.Duration
		if session.Status == "recording" && !startAt.IsZero() {
			displayDuration = int(time.Since(startAt).Seconds())
			if displayDuration < 0 {
				displayDuration = 0
			}
		}

		recordingAvailable, recordingIssue := h.inspectRecording(session)
		summary := riskSummary[session.ID]
		topRiskLevel := ""
		if summary.HighRiskCount > 0 {
			topRiskLevel = "high"
		} else if summary.MediumRiskCount > 0 {
			topRiskLevel = "medium"
		}

		info := &assetbiz.TerminalSessionInfo{
			ID:                 session.ID,
			HostID:             session.HostID,
			HostName:           session.HostName,
			HostIP:             session.HostIP,
			UserID:             session.UserID,
			Username:           session.Username,
			Duration:           displayDuration,
			DurationText:       formatDuration(displayDuration),
			FileSize:           session.FileSize,
			FileSizeText:       formatFileSize(session.FileSize),
			Status:             session.Status,
			StatusText:         getStatusText(session.Status),
			RecordingAvailable: recordingAvailable,
			RecordingIssue:     recordingIssue,
			HighRiskCount:      int(summary.HighRiskCount),
			MediumRiskCount:    int(summary.MediumRiskCount),
			TopRiskLevel:       topRiskLevel,
			TopRiskLevelText:   getRiskLevelText(topRiskLevel),
			CreatedAt:          session.CreatedAt,
			CreatedAtText:      session.CreatedAt.Format("2006-01-02 15:04:05"),
			StartedAt:          startAt,
			StartedAtText:      startAt.Format("2006-01-02 15:04:05"),
			EndedAt:            endAt,
			EndedAtText:        endedAtText,
			CloseReason:        session.CloseReason,
		}
		list = append(list, info)
	}

	response.Success(c, gin.H{
		"total": total,
		"list":  list,
	})
}

func resolveTerminalSessionTimes(session *assetbiz.TerminalSession) (time.Time, time.Time, string) {
	startAt := session.CreatedAt
	if session.StartedAt != nil && !session.StartedAt.IsZero() {
		startAt = *session.StartedAt
	}

	if session.EndedAt != nil && !session.EndedAt.IsZero() {
		endAt := *session.EndedAt
		return startAt, endAt, endAt.Format("2006-01-02 15:04:05")
	}

	if session.Duration > 0 && session.StartedAt == nil {
		endAt := session.CreatedAt
		startAt = endAt.Add(-time.Duration(session.Duration) * time.Second)
		return startAt, endAt, endAt.Format("2006-01-02 15:04:05")
	}

	if session.Status != "recording" && !session.UpdatedAt.IsZero() {
		return startAt, session.UpdatedAt, session.UpdatedAt.Format("2006-01-02 15:04:05")
	}

	return startAt, time.Time{}, "-"
}

// PlayTerminalSession 播放终端会话录制
// @Summary 播放终端会话
// @Description 获取终端会话的录制文件内容用于回放
// @Tags 终端审计
// @Accept json
// @Produce plain
// @Security Bearer
// @Param id path int true "会话ID"
// @Success 200 {string} string "录制文件内容"
// @Failure 404 {object} response.Response "会话不存在"
// @Router /api/v1/terminal-sessions/{id}/play [get]
func (h *TerminalAuditHandler) PlayTerminalSession(c *gin.Context) {
	session, ok := h.getSessionByParam(c)
	if !ok {
		return
	}

	content, _, err := h.recordingStore.ReadValidated(session.RecordingPath)
	if err != nil {
		h.writeRecordingError(c, err)
		return
	}

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, string(content))
}

// DownloadTerminalSession 下载终端会话录制文件
// @Summary 下载终端会话录制
// @Description 下载终端会话的录制文件
// @Tags 终端审计
// @Accept json
// @Produce octet-stream
// @Security Bearer
// @Param id path int true "会话ID"
// @Success 200 {file} file "录制文件"
// @Failure 404 {object} response.Response "会话不存在"
// @Router /api/v1/terminal-sessions/{id}/download [get]
func (h *TerminalAuditHandler) DownloadTerminalSession(c *gin.Context) {
	session, ok := h.getSessionByParam(c)
	if !ok {
		return
	}

	_, resolvedPath, err := h.recordingStore.ReadValidated(session.RecordingPath)
	if err != nil {
		h.writeRecordingError(c, err)
		return
	}

	fileName := filepath.Base(resolvedPath)
	if fileName == "." || fileName == string(filepath.Separator) {
		fileName = fmt.Sprintf("terminal-session-%d.cast", session.ID)
	}

	c.FileAttachment(resolvedPath, fileName)
}

// ListTerminalSessionEvents 获取会话高危命令记录
// @Summary 获取会话高危命令记录
// @Description 获取指定 SSH 会话的高危/中危命令事件
// @Tags 终端审计
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "会话ID"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/terminal-sessions/{id}/events [get]
func (h *TerminalAuditHandler) ListTerminalSessionEvents(c *gin.Context) {
	session, ok := h.getSessionByParam(c)
	if !ok {
		return
	}

	var events []*assetbiz.TerminalCommandEvent
	if err := h.db.Where("session_id = ?", session.ID).
		Order("executed_at DESC, id DESC").
		Find(&events).Error; err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "查询高危命令记录失败")
		return
	}

	list := make([]*assetbiz.TerminalCommandEventInfo, 0, len(events))
	highRiskCount := 0
	mediumRiskCount := 0
	for _, event := range events {
		if event.RiskLevel == "high" {
			highRiskCount++
		} else if event.RiskLevel == "medium" {
			mediumRiskCount++
		}
		list = append(list, &assetbiz.TerminalCommandEventInfo{
			ID:                event.ID,
			SessionID:         event.SessionID,
			HostID:            event.HostID,
			HostName:          event.HostName,
			HostIP:            event.HostIP,
			UserID:            event.UserID,
			Username:          event.Username,
			CommandText:       event.CommandText,
			NormalizedCommand: event.NormalizedCommand,
			RiskLevel:         event.RiskLevel,
			RiskLevelText:     getRiskLevelText(event.RiskLevel),
			RuleCode:          event.RuleCode,
			RuleName:          event.RuleName,
			RuleDescription:   event.RuleDescription,
			Source:            event.Source,
			Confidence:        event.Confidence,
			ExecutedAt:        event.ExecutedAt,
			ExecutedAtText:    event.ExecutedAt.Format("2006-01-02 15:04:05"),
		})
	}

	response.Success(c, gin.H{
		"list":            list,
		"highRiskCount":   highRiskCount,
		"mediumRiskCount": mediumRiskCount,
	})
}

// DeleteTerminalSession 删除终端会话
// @Summary 删除终端会话
// @Description 删除指定的终端会话记录及其录制文件
// @Tags 终端审计
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "会话ID"
// @Success 200 {object} response.Response "删除成功"
// @Failure 404 {object} response.Response "会话不存在"
// @Router /api/v1/terminal-sessions/{id} [delete]
func (h *TerminalAuditHandler) DeleteTerminalSession(c *gin.Context) {
	session, ok := h.getSessionByParam(c)
	if !ok {
		return
	}

	if resolvedPath, err := h.recordingStore.Resolve(session.RecordingPath); err == nil {
		_ = os.Remove(resolvedPath)
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", session.ID).Delete(&assetbiz.TerminalCommandEvent{}).Error; err != nil {
			return err
		}
		return tx.Delete(session).Error
	}); err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "删除失败")
		return
	}

	response.SuccessWithMessage(c, "删除成功", nil)
}

type terminalRiskSummary struct {
	SessionID       uint  `gorm:"column:session_id"`
	HighRiskCount   int64 `gorm:"column:high_risk_count"`
	MediumRiskCount int64 `gorm:"column:medium_risk_count"`
}

func (h *TerminalAuditHandler) loadRiskSummary(sessions []*assetbiz.TerminalSession) map[uint]terminalRiskSummary {
	result := make(map[uint]terminalRiskSummary, len(sessions))
	if len(sessions) == 0 {
		return result
	}

	ids := make([]uint, 0, len(sessions))
	for _, session := range sessions {
		ids = append(ids, session.ID)
	}

	var summaryRows []terminalRiskSummary
	if err := h.db.Model(&assetbiz.TerminalCommandEvent{}).
		Select("session_id, SUM(CASE WHEN risk_level = 'high' THEN 1 ELSE 0 END) AS high_risk_count, SUM(CASE WHEN risk_level = 'medium' THEN 1 ELSE 0 END) AS medium_risk_count").
		Where("session_id IN ?", ids).
		Group("session_id").
		Scan(&summaryRows).Error; err != nil {
		return result
	}

	for _, row := range summaryRows {
		result[row.SessionID] = row
	}
	return result
}

func (h *TerminalAuditHandler) inspectRecording(session *assetbiz.TerminalSession) (bool, string) {
	_, err := h.recordingStore.Resolve(session.RecordingPath)
	if err == nil {
		return true, ""
	}
	return false, recordingIssueMessage(err)
}

func (h *TerminalAuditHandler) getSessionByParam(c *gin.Context) (*assetbiz.TerminalSession, bool) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "无效的会话ID")
		return nil, false
	}

	var session assetbiz.TerminalSession
	if err := h.db.First(&session, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.ErrorCode(c, http.StatusNotFound, "会话不存在")
		} else {
			response.ErrorCode(c, http.StatusInternalServerError, "查询失败")
		}
		return nil, false
	}

	return &session, true
}

func (h *TerminalAuditHandler) writeRecordingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrTerminalRecordingPathEmpty):
		response.ErrorCode(c, http.StatusNotFound, recordingIssueMessage(err))
	case errors.Is(err, ErrTerminalRecordingMissing):
		response.ErrorCode(c, http.StatusNotFound, recordingIssueMessage(err))
	case errors.Is(err, ErrTerminalRecordingInvalid):
		response.ErrorCode(c, http.StatusUnprocessableEntity, recordingIssueMessage(err))
	default:
		response.ErrorCode(c, http.StatusInternalServerError, "读取录制文件失败")
	}
}

func formatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	} else if seconds < 3600 {
		minutes := seconds / 60
		secs := seconds % 60
		return fmt.Sprintf("%dm %ds", minutes, secs)
	} else {
		hours := seconds / 3600
		minutes := (seconds % 3600) / 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
}

func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func getStatusText(status string) string {
	statusMap := map[string]string{
		"recording": "录制中",
		"completed": "已完成",
		"failed":    "失败",
		"timeout":   "超时",
	}
	if text, ok := statusMap[status]; ok {
		return text
	}
	return status
}

func getRiskLevelText(level string) string {
	switch level {
	case "high":
		return "高危"
	case "medium":
		return "中危"
	default:
		return ""
	}
}

func recordingIssueMessage(err error) string {
	switch {
	case errors.Is(err, ErrTerminalRecordingPathEmpty):
		return "未生成录屏文件"
	case errors.Is(err, ErrTerminalRecordingMissing):
		return err.Error()
	case errors.Is(err, ErrTerminalRecordingInvalid):
		return "录屏文件格式损坏，无法播放"
	default:
		return "录屏文件读取失败"
	}
}
