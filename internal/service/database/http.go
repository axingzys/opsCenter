package database

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	dbbiz "github.com/ydcloud-dy/opshub/internal/biz/database"
	rbacservice "github.com/ydcloud-dy/opshub/internal/service/rbac"
	"github.com/ydcloud-dy/opshub/pkg/response"
)

type Service struct {
	useCase                *dbbiz.UseCase
	permissionRepo         dbbiz.DatabasePermissionRepo
	permissionModeResolver func(ctx context.Context) (string, error)
}

func NewService(useCase *dbbiz.UseCase, permissionRepo dbbiz.DatabasePermissionRepo, permissionModeResolvers ...func(ctx context.Context) (string, error)) *Service {
	var resolver func(ctx context.Context) (string, error)
	if len(permissionModeResolvers) > 0 {
		resolver = permissionModeResolvers[0]
	}
	return &Service{useCase: useCase, permissionRepo: permissionRepo, permissionModeResolver: resolver}
}

type databasePermissionScope struct {
	enforced   bool
	admin      bool
	userID     uint
	allowedIDs []uint
}

func (s *Service) databasePermissionMode(ctx context.Context) (string, error) {
	if s == nil || s.permissionModeResolver == nil {
		return "compat", nil
	}
	mode, err := s.permissionModeResolver(ctx)
	if err != nil {
		return "", err
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "whitelist":
		return "whitelist", nil
	default:
		return "compat", nil
	}
}

func databasePermissionModeText(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "whitelist":
		return "白名单模式"
	default:
		return "兼容模式"
	}
}

func parseUintParam(c *gin.Context, key, name string) (uint, bool) {
	raw := c.Param(key)
	id, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || id == 0 {
		response.ErrorCode(c, http.StatusBadRequest, "无效的"+name)
		return 0, false
	}
	return uint(id), true
}

func writeDatabaseError(c *gin.Context, prefix string, err error) {
	statusCode := http.StatusInternalServerError
	message := err.Error()
	switch {
	case strings.Contains(message, "不存在"):
		statusCode = http.StatusNotFound
	case strings.Contains(message, "鉴权失败"):
		statusCode = http.StatusUnauthorized
	case strings.Contains(message, "不能为空"),
		strings.Contains(message, "请选择"),
		strings.Contains(message, "需要"),
		strings.Contains(message, "不支持"),
		strings.Contains(message, "格式"),
		strings.Contains(message, "表达式"),
		strings.Contains(message, "执行中"),
		strings.Contains(message, "后续批次"),
		strings.Contains(message, "不支持"),
		strings.Contains(message, "仅允许"),
		strings.Contains(message, "禁止"),
		strings.Contains(message, "不能"),
		strings.Contains(message, "生产"),
		strings.Contains(message, "已禁用"),
		strings.Contains(message, "未确认"),
		strings.Contains(message, "未开启"),
		strings.Contains(message, "范围"):
		statusCode = http.StatusBadRequest
	}
	response.ErrorCode(c, statusCode, prefix+message)
}

func normalizeDatabasePermissionMask(permissions uint) uint {
	return permissions & dbbiz.DatabasePermissionAll
}

func (s *Service) databasePermissionScope(c *gin.Context, required uint) (*databasePermissionScope, bool) {
	scope := &databasePermissionScope{
		userID: rbacservice.GetUserID(c),
	}
	if s.permissionRepo == nil {
		return scope, true
	}
	if scope.userID == 0 {
		response.ErrorCode(c, http.StatusUnauthorized, "未登录")
		return nil, false
	}
	hasRules, err := s.permissionRepo.HasAnyRules(c.Request.Context())
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "数据库实例权限检查失败")
		return nil, false
	}
	mode, err := s.databasePermissionMode(c.Request.Context())
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "数据库实例权限模式读取失败")
		return nil, false
	}
	admin, err := s.permissionRepo.IsAdmin(c.Request.Context(), scope.userID)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "数据库实例权限检查失败")
		return nil, false
	}
	scope.admin = admin
	if !hasRules && mode != "whitelist" {
		return scope, true
	}
	scope.enforced = true

	if scope.admin {
		return scope, true
	}

	allowedIDs, err := s.permissionRepo.GetUserAccessibleInstanceIDs(c.Request.Context(), scope.userID, required)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "数据库实例权限检查失败")
		return nil, false
	}
	scope.allowedIDs = allowedIDs
	return scope, true
}

func (s *Service) ensureInstancePermission(c *gin.Context, instanceID uint, required uint) bool {
	if instanceID == 0 {
		response.ErrorCode(c, http.StatusBadRequest, "无效的实例ID")
		return false
	}
	scope, ok := s.databasePermissionScope(c, required)
	if !ok {
		return false
	}
	if !scope.enforced || scope.admin {
		return true
	}
	permissions, err := s.permissionRepo.GetUserInstancePermissions(c.Request.Context(), scope.userID, instanceID)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "数据库实例权限检查失败")
		return false
	}
	if permissions&required == 0 {
		response.ErrorCode(c, http.StatusForbidden, "权限不足：无权操作该数据库实例")
		return false
	}
	return true
}

func (s *Service) allowUnlimitedQueryRows(c *gin.Context, instanceID uint) bool {
	return s.allowExplicitInstancePermission(c, instanceID, dbbiz.DatabasePermissionQueryUnlimited, "权限不足：无不限行数查询权限")
}

func (s *Service) allowWriteExplainQuery(c *gin.Context, instanceID uint) bool {
	return s.allowExplicitInstancePermission(c, instanceID, dbbiz.DatabasePermissionWriteExplain, "权限不足：无写 SQL 执行计划权限")
}

func (s *Service) allowExplicitInstancePermission(c *gin.Context, instanceID uint, required uint, deniedMessage string) bool {
	if instanceID == 0 || s.permissionRepo == nil {
		return false
	}
	userID := rbacservice.GetUserID(c)
	if userID == 0 {
		response.ErrorCode(c, http.StatusUnauthorized, "未登录")
		return false
	}
	permissions, err := s.permissionRepo.GetUserInstancePermissions(c.Request.Context(), userID, instanceID)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "数据库实例权限检查失败")
		return false
	}
	if permissions&required == 0 {
		if strings.TrimSpace(deniedMessage) == "" {
			deniedMessage = "权限不足：无权操作该数据库实例"
		}
		response.ErrorCode(c, http.StatusForbidden, deniedMessage)
		return false
	}
	return true
}

func applyInstancePermissionScope(req *dbbiz.DatabaseInstanceListRequest, scope *databasePermissionScope) {
	if req == nil || scope == nil || !scope.enforced || scope.admin {
		return
	}
	req.RestrictToAllowed = true
	req.AllowedIDs = scope.allowedIDs
}

func applyAllowedInstanceScope(req interface{}, scope *databasePermissionScope) {
	if req == nil || scope == nil || !scope.enforced || scope.admin {
		return
	}
	switch item := req.(type) {
	case *dbbiz.DatabaseQueryAuditListRequest:
		item.RestrictToAllowed = true
		item.AllowedInstanceIDs = scope.allowedIDs
	case *dbbiz.DatabaseBackupTaskListRequest:
		item.RestrictToAllowed = true
		item.AllowedInstanceIDs = scope.allowedIDs
	case *dbbiz.DatabaseBackupRecordListRequest:
		item.RestrictToAllowed = true
		item.AllowedInstanceIDs = scope.allowedIDs
	case *dbbiz.DatabaseRestoreJobListRequest:
		item.RestrictToAllowed = true
		item.AllowedInstanceIDs = scope.allowedIDs
	case *dbbiz.DatabaseLogArchiveStreamListRequest:
		item.RestrictToAllowed = true
		item.AllowedInstanceIDs = scope.allowedIDs
	case *dbbiz.DatabaseLogArchiveListRequest:
		item.RestrictToAllowed = true
		item.AllowedInstanceIDs = scope.allowedIDs
	case *dbbiz.DatabaseLogArchiveEventListRequest:
		item.RestrictToAllowed = true
		item.AllowedInstanceIDs = scope.allowedIDs
	case *dbbiz.DatabaseRestorePlanListRequest:
		item.RestrictToAllowed = true
		item.AllowedInstanceIDs = scope.allowedIDs
	case *dbbiz.DatabaseBarmanServerListRequest:
		item.RestrictToAllowed = true
		item.AllowedInstanceIDs = scope.allowedIDs
	case *dbbiz.DatabaseInspectionReportListRequest:
		item.RestrictToAllowed = true
		item.AllowedInstanceIDs = scope.allowedIDs
	case *dbbiz.DatabaseQueryHistoryRequest:
		item.RestrictToAllowed = true
		item.AllowedInstanceIDs = scope.allowedIDs
	case *dbbiz.DatabaseInstanceReplicaListRequest:
		item.RestrictToAllowed = true
		item.AllowedInstanceIDs = scope.allowedIDs
	case *dbbiz.DatabaseReplicationCheckListRequest:
		item.RestrictToAllowed = true
		item.AllowedInstanceIDs = scope.allowedIDs
	case *dbbiz.DatabaseReplicaProtectionListRequest:
		item.RestrictToAllowed = true
		item.AllowedInstanceIDs = scope.allowedIDs
	}
}

func (s *Service) decorateInstancePermissions(c *gin.Context, list []*dbbiz.DatabaseInstanceVO, scope *databasePermissionScope) bool {
	if len(list) == 0 {
		return true
	}
	if scope == nil || scope.admin || s.permissionRepo == nil {
		for _, item := range list {
			item.Permissions = dbbiz.DatabasePermissionAll
		}
		return true
	}
	if !scope.enforced {
		for _, item := range list {
			item.Permissions = dbbiz.DatabasePermissionAll &^ (dbbiz.DatabasePermissionQueryUnlimited | dbbiz.DatabasePermissionWriteExplain | dbbiz.DatabasePermissionDDL)
		}
		return true
	}
	for _, item := range list {
		permissions, err := s.permissionRepo.GetUserInstancePermissions(c.Request.Context(), scope.userID, item.ID)
		if err != nil {
			response.ErrorCode(c, http.StatusInternalServerError, "数据库实例权限检查失败")
			return false
		}
		item.Permissions = permissions
	}
	return true
}

// GetSupportedTypes 获取支持的数据库类型
// @Summary 获取支持的数据库类型
// @Description 返回数据库管理一期支持的类型和能力标记
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/databases/supported-types [get]
func (s *Service) GetSupportedTypes(c *gin.Context) {
	response.Success(c, s.useCase.SupportedTypes())
}

// ListInstancePermissions 获取数据库实例对象级权限配置
func (s *Service) ListInstancePermissions(c *gin.Context) {
	if s.permissionRepo == nil {
		response.ErrorCode(c, http.StatusInternalServerError, "数据库实例权限仓库未配置")
		return
	}
	var req dbbiz.DatabaseInstancePermissionListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	list, total, err := s.permissionRepo.List(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	mode, err := s.databasePermissionMode(c.Request.Context())
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	hasRules, err := s.permissionRepo.HasAnyRules(c.Request.Context())
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{
		"list":                   list,
		"total":                  total,
		"page":                   req.Page,
		"pageSize":               req.PageSize,
		"permissionMode":         mode,
		"permissionModeText":     databasePermissionModeText(mode),
		"permissionModeEnforced": mode == "whitelist" || hasRules,
		"permissionRulesEnabled": hasRules,
	})
}

// UpsertInstancePermission 创建或更新数据库实例对象级权限配置
func (s *Service) UpsertInstancePermission(c *gin.Context) {
	if s.permissionRepo == nil {
		response.ErrorCode(c, http.StatusInternalServerError, "数据库实例权限仓库未配置")
		return
	}
	var req dbbiz.DatabaseInstancePermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	permissions := normalizeDatabasePermissionMask(req.Permissions)
	if permissions == 0 {
		response.ErrorCode(c, http.StatusBadRequest, "请选择数据库实例权限")
		return
	}
	if err := s.permissionRepo.ValidateTarget(c.Request.Context(), req.RoleID, req.InstanceID); err != nil {
		writeDatabaseError(c, "保存失败: ", err)
		return
	}
	var before *dbbiz.DatabaseInstancePermissionVO
	if s.useCase != nil {
		var err error
		before, err = s.permissionRepo.GetByRoleInstance(c.Request.Context(), req.RoleID, req.InstanceID)
		if err != nil {
			writeDatabaseError(c, "保存失败: ", err)
			return
		}
	}
	if err := s.permissionRepo.Upsert(c.Request.Context(), &dbbiz.DatabaseInstancePermission{
		RoleID:      req.RoleID,
		InstanceID:  req.InstanceID,
		Permissions: permissions,
	}); err != nil {
		writeDatabaseError(c, "保存失败: ", err)
		return
	}
	if s.useCase != nil {
		after, err := s.permissionRepo.GetByRoleInstance(c.Request.Context(), req.RoleID, req.InstanceID)
		if err != nil {
			writeDatabaseError(c, "保存成功但记录审计失败: ", err)
			return
		}
		auditReq := buildInstancePermissionAuditRequest(before, after, req.RoleID, req.InstanceID, permissions)
		if err := s.useCase.RecordInstancePermissionAudit(c.Request.Context(), dbbiz.DatabaseAuditActionPermissionUpsert, auditReq, dbbiz.QueryOperator{
			ID:       rbacservice.GetUserID(c),
			Username: rbacservice.GetUsername(c),
			ClientIP: c.ClientIP(),
		}); err != nil {
			writeDatabaseError(c, "保存成功但记录审计失败: ", err)
			return
		}
	}
	response.SuccessWithMessage(c, "保存成功", nil)
}

// DeleteInstancePermission 删除数据库实例对象级权限配置
func (s *Service) DeleteInstancePermission(c *gin.Context) {
	if s.permissionRepo == nil {
		response.ErrorCode(c, http.StatusInternalServerError, "数据库实例权限仓库未配置")
		return
	}
	id, ok := parseUintParam(c, "id", "权限ID")
	if !ok {
		return
	}
	existing, err := s.permissionRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "删除失败: ", err)
		return
	}
	if existing == nil {
		response.ErrorCode(c, http.StatusNotFound, "数据库实例权限不存在")
		return
	}
	if err := s.permissionRepo.Delete(c.Request.Context(), id); err != nil {
		writeDatabaseError(c, "删除失败: ", err)
		return
	}
	if s.useCase != nil {
		auditReq := buildInstancePermissionAuditRequest(existing, nil, existing.RoleID, existing.InstanceID, 0)
		if err := s.useCase.RecordInstancePermissionAudit(c.Request.Context(), dbbiz.DatabaseAuditActionPermissionDelete, auditReq, dbbiz.QueryOperator{
			ID:       rbacservice.GetUserID(c),
			Username: rbacservice.GetUsername(c),
			ClientIP: c.ClientIP(),
		}); err != nil {
			writeDatabaseError(c, "删除成功但记录审计失败: ", err)
			return
		}
	}
	response.SuccessWithMessage(c, "删除成功", nil)
}

func buildInstancePermissionAuditRequest(before, after *dbbiz.DatabaseInstancePermissionVO, roleID, instanceID, afterPermissions uint) *dbbiz.DatabaseInstancePermissionAuditRequest {
	result := &dbbiz.DatabaseInstancePermissionAuditRequest{
		RoleID:           roleID,
		InstanceID:       instanceID,
		AfterPermissions: afterPermissions,
	}
	if before != nil {
		result.RoleID = before.RoleID
		result.RoleName = before.RoleName
		result.RoleCode = before.RoleCode
		result.InstanceID = before.InstanceID
		result.InstanceName = before.InstanceName
		result.BeforePermissions = before.Permissions
	}
	if after != nil {
		result.RoleID = after.RoleID
		result.RoleName = after.RoleName
		result.RoleCode = after.RoleCode
		result.InstanceID = after.InstanceID
		result.InstanceName = after.InstanceName
		result.AfterPermissions = after.Permissions
	}
	return result
}

// ListQueryHistory 获取当前用户最近 SQL 历史
// @Summary 获取 SQL 历史
// @Description 获取当前登录用户最近执行的 SQL 历史，最多返回 50 条
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Router /api/v1/databases/query-history [get]
func (s *Service) ListQueryHistory(c *gin.Context) {
	var req dbbiz.DatabaseQueryHistoryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionQuery)
	if !ok {
		return
	}
	applyAllowedInstanceScope(&req, scope)
	list, err := s.useCase.ListQueryHistory(c.Request.Context(), &req, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{
		"list":  list,
		"limit": req.Limit,
	})
}

// ListQueryAudits 获取查询审计列表
// @Summary 获取查询审计列表
// @Description 分页查询 SQL 查询审计
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param keyword query string false "关键字"
// @Param instanceId query int false "实例ID"
// @Param status query string false "状态"
// @Param riskLevel query string false "风险等级"
// @Param sqlType query string false "SQL类型"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/databases/query-audits [get]
func (s *Service) ListQueryAudits(c *gin.Context) {
	var req dbbiz.DatabaseQueryAuditListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionView)
	if !ok {
		return
	}
	applyAllowedInstanceScope(&req, scope)
	list, total, err := s.useCase.ListQueryAudits(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{
		"list":     list,
		"total":    total,
		"page":     req.Page,
		"pageSize": req.PageSize,
	})
}

// ExportQueryAudits 导出查询审计 CSV
// @Summary 导出查询审计
// @Description 按筛选条件导出 SQL 查询审计 CSV，最多导出 5000 条
// @Tags 数据库管理
// @Accept json
// @Produce text/csv
// @Security Bearer
// @Router /api/v1/databases/query-audits/export [get]
func (s *Service) ExportQueryAudits(c *gin.Context) {
	var req dbbiz.DatabaseQueryAuditListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionView)
	if !ok {
		return
	}
	applyAllowedInstanceScope(&req, scope)
	list, _, err := s.useCase.ExportQueryAudits(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "导出失败: ", err)
		return
	}

	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(&buf)
	header := []string{"审计ID", "执行时间", "实例", "Schema", "操作者", "审计动作", "SQL类型", "风险", "状态", "返回行", "耗时ms", "客户端IP", "SQL", "错误信息", "SQL指纹"}
	if err := writer.Write(header); err != nil {
		writeDatabaseError(c, "导出失败: ", err)
		return
	}
	for _, item := range list {
		record := []string{
			strconv.FormatUint(uint64(item.ID), 10),
			item.CreatedAt,
			safeCSVCell(item.InstanceName),
			safeCSVCell(item.SchemaName),
			safeCSVCell(item.OperatorName),
			safeCSVCell(item.ActionText),
			safeCSVCell(item.SQLType),
			safeCSVCell(item.RiskLevelText),
			safeCSVCell(item.StatusText),
			strconv.Itoa(item.RowsReturned),
			strconv.FormatInt(item.DurationMs, 10),
			safeCSVCell(item.ClientIP),
			safeCSVCell(item.SQLText),
			safeCSVCell(item.ErrorMessage),
			safeCSVCell(item.SQLFingerprint),
		}
		if err := writer.Write(record); err != nil {
			writeDatabaseError(c, "导出失败: ", err)
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		writeDatabaseError(c, "导出失败: ", err)
		return
	}

	filename := "database-query-audits-" + time.Now().Format("20060102150405") + ".csv"
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}

// ListBackupTasks 获取备份任务列表
// @Summary 获取备份任务列表
// @Description 分页查询数据库备份任务配置
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Router /api/v1/databases/backup-tasks [get]
func (s *Service) ListBackupTasks(c *gin.Context) {
	var req dbbiz.DatabaseBackupTaskListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionBackup)
	if !ok {
		return
	}
	applyAllowedInstanceScope(&req, scope)
	list, total, err := s.useCase.ListBackupTasks(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{
		"list":     list,
		"total":    total,
		"page":     req.Page,
		"pageSize": req.PageSize,
	})
}

// CreateBackupTask 创建备份任务
// @Summary 创建备份任务
// @Description 创建数据库逻辑备份任务
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Router /api/v1/databases/backup-tasks [post]
func (s *Service) CreateBackupTask(c *gin.Context) {
	var req dbbiz.DatabaseBackupTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, req.InstanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	item, err := s.useCase.CreateBackupTask(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "创建失败: ", err)
		return
	}
	response.Success(c, item)
}

// UpdateBackupTask 更新备份任务
// @Summary 更新备份任务
// @Description 更新数据库逻辑备份任务
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Router /api/v1/databases/backup-tasks/{id} [put]
func (s *Service) UpdateBackupTask(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "任务ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseBackupTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	existingInstanceID, err := s.useCase.GetBackupTaskInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "更新失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, existingInstanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	if req.InstanceID != existingInstanceID && !s.ensureInstancePermission(c, req.InstanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	item, err := s.useCase.UpdateBackupTask(c.Request.Context(), id, &req)
	if err != nil {
		writeDatabaseError(c, "更新失败: ", err)
		return
	}
	response.Success(c, item)
}

// DeleteBackupTask 删除备份任务
// @Summary 删除备份任务
// @Description 删除数据库备份任务配置
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Router /api/v1/databases/backup-tasks/{id} [delete]
func (s *Service) DeleteBackupTask(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "任务ID")
	if !ok {
		return
	}
	instanceID, err := s.useCase.GetBackupTaskInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "删除失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, instanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	if err := s.useCase.DeleteBackupTask(c.Request.Context(), id); err != nil {
		writeDatabaseError(c, "删除失败: ", err)
		return
	}
	response.SuccessWithMessage(c, "删除成功", nil)
}

// RunBackupTask 手动触发备份任务
// @Summary 手动触发备份任务
// @Description 创建一条待执行备份记录并写入统一审计
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Router /api/v1/databases/backup-tasks/{id}/run [post]
func (s *Service) RunBackupTask(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "任务ID")
	if !ok {
		return
	}
	instanceID, err := s.useCase.GetBackupTaskInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "触发失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, instanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	item, err := s.useCase.RunBackupTask(c.Request.Context(), id, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "触发失败: ", err)
		return
	}
	response.Success(c, item)
}

// ListBackupRecords 获取备份记录列表
// @Summary 获取备份记录列表
// @Description 分页查询数据库备份执行记录
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Router /api/v1/databases/backup-records [get]
func (s *Service) ListBackupRecords(c *gin.Context) {
	var req dbbiz.DatabaseBackupRecordListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionBackup)
	if !ok {
		return
	}
	applyAllowedInstanceScope(&req, scope)
	list, total, err := s.useCase.ListBackupRecords(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{
		"list":     list,
		"total":    total,
		"page":     req.Page,
		"pageSize": req.PageSize,
	})
}

func (s *Service) RegisterExternalBackupRecord(c *gin.Context) {
	var req dbbiz.DatabaseExternalBackupRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, req.InstanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	item, err := s.useCase.RegisterExternalBackupRecord(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "登记失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) ListStorageProfiles(c *gin.Context) {
	var req dbbiz.DatabaseStorageProfileListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	list, total, err := s.useCase.ListStorageProfiles(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total, "page": req.Page, "pageSize": req.PageSize})
}

func (s *Service) CreateStorageProfile(c *gin.Context) {
	var req dbbiz.DatabaseStorageProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	item, err := s.useCase.CreateStorageProfile(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "创建失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) CheckStorageProfilePosture(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "存储配置ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseStorageProfilePostureCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errorsIsEOF(err) {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	item, err := s.useCase.CheckStorageProfilePosture(c.Request.Context(), id, &req)
	if err != nil {
		writeDatabaseError(c, "检测失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) ListSecretProfiles(c *gin.Context) {
	var req dbbiz.DatabaseSecretProfileListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	list, total, err := s.useCase.ListSecretProfiles(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total, "page": req.Page, "pageSize": req.PageSize})
}

func (s *Service) CreateSecretProfile(c *gin.Context) {
	var req dbbiz.DatabaseSecretProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	item, err := s.useCase.CreateSecretProfile(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "创建失败: ", err)
		return
	}
	response.Success(c, item)
}

func errorsIsEOF(err error) bool {
	return err == io.EOF || strings.EqualFold(strings.TrimSpace(err.Error()), "EOF")
}

func (s *Service) ListRunnerHosts(c *gin.Context) {
	var req dbbiz.DatabaseRunnerHostListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	list, total, err := s.useCase.ListRunnerHosts(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total, "page": req.Page, "pageSize": req.PageSize})
}

func (s *Service) CreateRunnerHost(c *gin.Context) {
	var req dbbiz.DatabaseRunnerHostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	item, err := s.useCase.CreateRunnerHost(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "创建失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) UpdateRunnerHost(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "Runner 主机ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseRunnerHostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	item, err := s.useCase.UpdateRunnerHost(c.Request.Context(), id, &req)
	if err != nil {
		writeDatabaseError(c, "更新失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) TestRunnerHost(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "Runner 主机ID")
	if !ok {
		return
	}
	item, err := s.useCase.TestRunnerHost(c.Request.Context(), id, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "测试失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) ListRunnerJobs(c *gin.Context) {
	var req dbbiz.DatabaseRunnerJobListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	list, total, err := s.useCase.ListRunnerJobs(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total, "page": req.Page, "pageSize": req.PageSize})
}

func (s *Service) ListBarmanServers(c *gin.Context) {
	var req dbbiz.DatabaseBarmanServerListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionBackup)
	if !ok {
		return
	}
	applyAllowedInstanceScope(&req, scope)
	list, total, err := s.useCase.ListBarmanServers(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total, "page": req.Page, "pageSize": req.PageSize})
}

func (s *Service) CreateBarmanServer(c *gin.Context) {
	var req dbbiz.DatabaseBarmanServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, req.SourceInstanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	item, err := s.useCase.CreateBarmanServer(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "创建失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) UpdateBarmanServer(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "Barman Server ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseBarmanServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, req.SourceInstanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	item, err := s.useCase.UpdateBarmanServer(c.Request.Context(), id, &req)
	if err != nil {
		writeDatabaseError(c, "更新失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) DeleteBarmanServer(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "Barman Server ID")
	if !ok {
		return
	}
	instanceID, err := s.useCase.GetBarmanServerSourceInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "删除失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, instanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	if err := s.useCase.DeleteBarmanServer(c.Request.Context(), id); err != nil {
		writeDatabaseError(c, "删除失败: ", err)
		return
	}
	response.SuccessWithMessage(c, "删除成功", nil)
}

func (s *Service) CheckBarmanServer(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "Barman Server ID")
	if !ok {
		return
	}
	instanceID, err := s.useCase.GetBarmanServerSourceInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "检测失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, instanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	item, err := s.useCase.CheckBarmanServer(c.Request.Context(), id, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "检测失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) SyncBarmanCatalog(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "Barman Server ID")
	if !ok {
		return
	}
	instanceID, err := s.useCase.GetBarmanServerSourceInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "同步失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, instanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	item, err := s.useCase.SyncBarmanCatalog(c.Request.Context(), id, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "同步失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) SyncBarmanWAL(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "Barman Server ID")
	if !ok {
		return
	}
	instanceID, err := s.useCase.GetBarmanServerSourceInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "WAL 同步失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, instanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	item, err := s.useCase.SyncBarmanWAL(c.Request.Context(), id, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "WAL 同步失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) BackupBarmanServer(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "Barman Server ID")
	if !ok {
		return
	}
	instanceID, err := s.useCase.GetBarmanServerSourceInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "备份触发失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, instanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	item, err := s.useCase.BackupBarmanServer(c.Request.Context(), id, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "备份触发失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) ListLogArchiveStreams(c *gin.Context) {
	var req dbbiz.DatabaseLogArchiveStreamListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionBackup)
	if !ok {
		return
	}
	applyAllowedInstanceScope(&req, scope)
	list, total, err := s.useCase.ListLogArchiveStreams(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total, "page": req.Page, "pageSize": req.PageSize})
}

func (s *Service) CreateLogArchiveStream(c *gin.Context) {
	var req dbbiz.DatabaseLogArchiveStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, req.InstanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	if req.SourceInstanceID > 0 && req.SourceInstanceID != req.InstanceID && !s.ensureInstancePermission(c, req.SourceInstanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	item, err := s.useCase.CreateLogArchiveStream(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "创建失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) GetLogArchiveStreamStatus(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "日志归档流ID")
	if !ok {
		return
	}
	instanceID, err := s.useCase.GetLogArchiveStreamInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, instanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	item, err := s.useCase.GetLogArchiveStreamStatus(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) StartLogArchiveStream(c *gin.Context) {
	s.controlLogArchiveStream(c, "启动失败: ", s.useCase.StartLogArchiveStream)
}

func (s *Service) PauseLogArchiveStream(c *gin.Context) {
	s.controlLogArchiveStream(c, "暂停失败: ", s.useCase.PauseLogArchiveStream)
}

func (s *Service) ResumeLogArchiveStream(c *gin.Context) {
	s.controlLogArchiveStream(c, "恢复失败: ", s.useCase.ResumeLogArchiveStream)
}

func (s *Service) StopLogArchiveStream(c *gin.Context) {
	s.controlLogArchiveStream(c, "停止失败: ", s.useCase.StopLogArchiveStream)
}

func (s *Service) controlLogArchiveStream(
	c *gin.Context,
	prefix string,
	handler func(context.Context, uint, *dbbiz.DatabaseLogArchiveStreamControlRequest) (*dbbiz.DatabaseLogArchiveStreamVO, error),
) {
	id, ok := parseUintParam(c, "id", "日志归档流ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseLogArchiveStreamControlRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
			return
		}
	}
	instanceID, err := s.useCase.GetLogArchiveStreamInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, prefix, err)
		return
	}
	if !s.ensureInstancePermission(c, instanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	item, err := handler(c.Request.Context(), id, &req)
	if err != nil {
		writeDatabaseError(c, prefix, err)
		return
	}
	response.Success(c, item)
}

func (s *Service) ListLogArchives(c *gin.Context) {
	var req dbbiz.DatabaseLogArchiveListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionBackup)
	if !ok {
		return
	}
	applyAllowedInstanceScope(&req, scope)
	list, total, err := s.useCase.ListLogArchives(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total, "page": req.Page, "pageSize": req.PageSize})
}

func (s *Service) RegisterExternalLogArchive(c *gin.Context) {
	var req dbbiz.DatabaseExternalLogArchiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	instanceID, err := s.useCase.GetLogArchiveStreamInstanceID(c.Request.Context(), req.StreamID)
	if err != nil {
		writeDatabaseError(c, "登记失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, instanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	item, err := s.useCase.RegisterExternalLogArchive(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "登记失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) ListLogArchiveEvents(c *gin.Context) {
	var req dbbiz.DatabaseLogArchiveEventListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionBackup)
	if !ok {
		return
	}
	applyAllowedInstanceScope(&req, scope)
	list, total, err := s.useCase.ListLogArchiveEvents(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total, "page": req.Page, "pageSize": req.PageSize})
}

func (s *Service) RunLogArchiveOnce(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "日志归档流ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseRunLogArchiveOnceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	instanceID, err := s.useCase.GetLogArchiveStreamInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "归档失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, instanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	item, err := s.useCase.RunLogArchiveOnce(c.Request.Context(), id, &req, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "归档失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) RunLogArchiveCatchUp(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "日志归档流ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseRunLogArchiveCatchUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	instanceID, err := s.useCase.GetLogArchiveStreamInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "追平归档失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, instanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	item, err := s.useCase.RunLogArchiveCatchUp(c.Request.Context(), id, &req, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "追平归档失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) RunnerAgentHeartbeat(c *gin.Context) {
	runnerID := c.Param("runnerId")
	var req dbbiz.DatabaseRunnerAgentHeartbeatRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
			return
		}
	}
	item, err := s.useCase.RunnerAgentHeartbeat(c.Request.Context(), runnerID, runnerAgentAuthHeader(c), &req)
	if err != nil {
		writeDatabaseError(c, "Runner Agent 心跳失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) RunnerAgentListLogArchiveStreams(c *gin.Context) {
	runnerID := c.Param("runnerId")
	basePath := fmt.Sprintf("/api/v1/public/databases/runner-agents/%s", runnerID)
	item, err := s.useCase.RunnerAgentListLogArchiveStreams(c.Request.Context(), runnerID, runnerAgentAuthHeader(c), basePath)
	if err != nil {
		writeDatabaseError(c, "Runner Agent 拉取归档流失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) RunnerAgentCheckpointLogArchiveStream(c *gin.Context) {
	runnerID := c.Param("runnerId")
	id, ok := parseUintParam(c, "id", "日志归档流ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseRunnerAgentCheckpointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	item, err := s.useCase.RunnerAgentCheckpointLogArchiveStream(c.Request.Context(), runnerID, runnerAgentAuthHeader(c), id, &req)
	if err != nil {
		writeDatabaseError(c, "Runner Agent checkpoint 失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) RunnerAgentRegisterLogArchive(c *gin.Context) {
	runnerID := c.Param("runnerId")
	var req dbbiz.DatabaseExternalLogArchiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	item, err := s.useCase.RunnerAgentRegisterLogArchive(c.Request.Context(), runnerID, runnerAgentAuthHeader(c), &req)
	if err != nil {
		writeDatabaseError(c, "Runner Agent 登记日志归档失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) RunnerAgentCreateLogArchiveEvent(c *gin.Context) {
	runnerID := c.Param("runnerId")
	var req dbbiz.DatabaseRunnerAgentEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	item, err := s.useCase.RunnerAgentCreateLogArchiveEvent(c.Request.Context(), runnerID, runnerAgentAuthHeader(c), &req)
	if err != nil {
		writeDatabaseError(c, "Runner Agent 上报日志归档事件失败: ", err)
		return
	}
	response.Success(c, item)
}

func runnerAgentAuthHeader(c *gin.Context) string {
	value := strings.TrimSpace(c.GetHeader("X-OpsHub-Runner-Auth"))
	if value != "" {
		return value
	}
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
}

func (s *Service) ListRestorePlans(c *gin.Context) {
	var req dbbiz.DatabaseRestorePlanListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionRestore)
	if !ok {
		return
	}
	applyAllowedInstanceScope(&req, scope)
	list, total, err := s.useCase.ListRestorePlans(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total, "page": req.Page, "pageSize": req.PageSize})
}

func (s *Service) CreateRestorePlan(c *gin.Context) {
	var req dbbiz.DatabaseRestorePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, req.SourceInstanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	if req.TargetInstanceID > 0 && !s.ensureInstancePermission(c, req.TargetInstanceID, dbbiz.DatabasePermissionRestore) {
		return
	}
	item, err := s.useCase.CreateRestorePlan(c.Request.Context(), &req, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "生成失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) RunRestorePlan(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "恢复计划ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseRestorePlanRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	sourceInstanceID, err := s.useCase.GetRestorePlanSourceInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "执行失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, sourceInstanceID, dbbiz.DatabasePermissionRestore) {
		return
	}
	item, err := s.useCase.RunRestorePlan(c.Request.Context(), id, &req, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "执行失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) GetRestoreJob(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "恢复任务ID")
	if !ok {
		return
	}
	sourceInstanceID, err := s.useCase.GetRestoreJobSourceInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, sourceInstanceID, dbbiz.DatabasePermissionRestore) {
		return
	}
	item, err := s.useCase.GetRestoreJob(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) CancelRestoreJob(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "恢复任务ID")
	if !ok {
		return
	}
	sourceInstanceID, err := s.useCase.GetRestoreJobSourceInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "取消失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, sourceInstanceID, dbbiz.DatabasePermissionRestore) {
		return
	}
	item, err := s.useCase.CancelRestoreJob(c.Request.Context(), id, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "取消失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) CleanupRestoreJob(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "恢复任务ID")
	if !ok {
		return
	}
	sourceInstanceID, err := s.useCase.GetRestoreJobSourceInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "清理失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, sourceInstanceID, dbbiz.DatabasePermissionRestore) {
		return
	}
	item, err := s.useCase.CleanupRestoreJob(c.Request.Context(), id, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "清理失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) GetRestoreJobProof(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "恢复任务ID")
	if !ok {
		return
	}
	sourceInstanceID, err := s.useCase.GetRestoreJobSourceInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, sourceInstanceID, dbbiz.DatabasePermissionRestore) {
		return
	}
	proof, err := s.useCase.GetRestoreJobProof(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{"proofJson": proof})
}

// DownloadBackupRecord 下载备份文件
// @Summary 下载备份文件
// @Description 下载指定的成功备份文件，并写入统一审计
// @Tags 数据库管理
// @Accept json
// @Produce application/octet-stream
// @Security Bearer
// @Router /api/v1/databases/backup-records/{id}/download [get]
func (s *Service) DownloadBackupRecord(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "记录ID")
	if !ok {
		return
	}
	instanceID, err := s.useCase.GetBackupRecordInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "下载失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, instanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	data, err := s.useCase.DownloadBackupRecord(c.Request.Context(), id, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "下载失败: ", err)
		return
	}
	if data.ContentType != "" {
		c.Header("Content-Type", data.ContentType)
	}
	c.FileAttachment(data.FilePath, data.FileName)
}

// VerifyBackupRecord 手动校验备份文件
// @Summary 手动校验备份文件
// @Description 对成功备份记录执行文件存在性、大小和 checksum 校验，并写入统一审计
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Router /api/v1/databases/backup-records/{id}/verify [post]
func (s *Service) VerifyBackupRecord(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "记录ID")
	if !ok {
		return
	}
	instanceID, err := s.useCase.GetBackupRecordInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "校验失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, instanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	item, err := s.useCase.VerifyBackupRecord(c.Request.Context(), id, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "校验失败: ", err)
		return
	}
	response.Success(c, item)
}

// RunRestoreDryRun 发起恢复演练
// @Summary 发起恢复演练
// @Description 从成功备份记录恢复到非生产目标实例，并写入统一审计
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Router /api/v1/databases/backup-records/{id}/restore-dry-run [post]
func (s *Service) RunRestoreDryRun(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "备份记录ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseRestoreDryRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	sourceInstanceID, err := s.useCase.GetBackupRecordInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "恢复演练失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, sourceInstanceID, dbbiz.DatabasePermissionBackup) {
		return
	}
	if !s.ensureInstancePermission(c, req.TargetInstanceID, dbbiz.DatabasePermissionRestore) {
		return
	}
	item, err := s.useCase.RunRestoreDryRun(c.Request.Context(), id, &req, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "恢复演练失败: ", err)
		return
	}
	response.Success(c, item)
}

// ListRestoreJobs 获取恢复演练记录
// @Summary 获取恢复演练记录
// @Description 分页查询数据库恢复演练记录
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Router /api/v1/databases/restore-jobs [get]
func (s *Service) ListRestoreJobs(c *gin.Context) {
	var req dbbiz.DatabaseRestoreJobListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionRestore)
	if !ok {
		return
	}
	applyAllowedInstanceScope(&req, scope)
	list, total, err := s.useCase.ListRestoreJobs(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{
		"list":     list,
		"total":    total,
		"page":     req.Page,
		"pageSize": req.PageSize,
	})
}

// GetCapacityTrend 获取容量趋势
// @Summary 获取容量趋势
// @Description 获取实例容量采样趋势和 Top 对象
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Router /api/v1/databases/instances/{id}/capacity-trend [get]
func (s *Service) GetCapacityTrend(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseCapacityTrendRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionDiagnosis) {
		return
	}
	item, err := s.useCase.GetCapacityTrend(c.Request.Context(), id, &req, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, item)
}

// CollectCapacitySnapshot 手动采集容量快照
// @Summary 手动采集容量快照
// @Description 基于已同步元数据采集实例、Schema、表级容量快照
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Router /api/v1/databases/instances/{id}/capacity-snapshots [post]
func (s *Service) CollectCapacitySnapshot(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionDiagnosis) {
		return
	}
	item, err := s.useCase.CollectCapacitySnapshotForUser(c.Request.Context(), id, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "采集失败: ", err)
		return
	}
	response.Success(c, item)
}

// ListInspectionReports 获取巡检报告列表
// @Summary 获取巡检报告列表
// @Description 分页查询数据库巡检报告
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Router /api/v1/databases/inspection-reports [get]
func (s *Service) ListInspectionReports(c *gin.Context) {
	var req dbbiz.DatabaseInspectionReportListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionDiagnosis)
	if !ok {
		return
	}
	applyAllowedInstanceScope(&req, scope)
	list, total, err := s.useCase.ListInspectionReports(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{
		"list":     list,
		"total":    total,
		"page":     req.Page,
		"pageSize": req.PageSize,
	})
}

// GetInspectionReport 获取巡检报告详情
// @Summary 获取巡检报告详情
// @Description 获取数据库巡检报告详情
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Router /api/v1/databases/inspection-reports/{id} [get]
func (s *Service) GetInspectionReport(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "报告ID")
	if !ok {
		return
	}
	instanceID, err := s.useCase.GetInspectionReportInstanceID(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, instanceID, dbbiz.DatabasePermissionDiagnosis) {
		return
	}
	item, err := s.useCase.GetInspectionReport(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, item)
}

// GenerateInspectionReport 手动生成巡检报告
// @Summary 手动生成巡检报告
// @Description 聚合容量、性能、安全和备份状态生成巡检报告
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Router /api/v1/databases/inspection-reports [post]
func (s *Service) GenerateInspectionReport(c *gin.Context) {
	var req dbbiz.DatabaseInspectionReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, req.InstanceID, dbbiz.DatabasePermissionDiagnosis) {
		return
	}
	item, err := s.useCase.GenerateInspectionReport(c.Request.Context(), &req, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "生成失败: ", err)
		return
	}
	response.Success(c, item)
}

// GetTopology 获取数据库拓扑
// @Summary 获取数据库拓扑
// @Description 获取 Redis Cluster、MongoDB ReplicaSet、Elasticsearch / OpenSearch 拓扑和分片信息
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Router /api/v1/databases/instances/{id}/topology [get]
func (s *Service) GetTopology(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionTopology) {
		return
	}
	item, err := s.useCase.GetTopology(c.Request.Context(), id, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, item)
}

// ListInstances 获取数据库实例列表
// @Summary 获取数据库实例列表
// @Description 分页查询数据库实例
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param keyword query string false "关键字"
// @Param dbType query string false "数据库类型"
// @Param status query string false "状态"
// @Param environment query string false "环境"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/databases/instances [get]
func (s *Service) ListInstances(c *gin.Context) {
	var req dbbiz.DatabaseInstanceListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionView)
	if !ok {
		return
	}
	applyInstancePermissionScope(&req, scope)
	list, total, err := s.useCase.ListInstances(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	if !s.decorateInstancePermissions(c, list, scope) {
		return
	}
	response.Success(c, gin.H{
		"list":     list,
		"total":    total,
		"page":     req.Page,
		"pageSize": req.PageSize,
	})
}

// CreateInstance 创建数据库实例
// @Summary 创建数据库实例
// @Description 创建数据库实例连接配置
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body dbbiz.DatabaseInstanceRequest true "实例信息"
// @Success 200 {object} response.Response "创建成功"
// @Router /api/v1/databases/instances [post]
func (s *Service) CreateInstance(c *gin.Context) {
	var req dbbiz.DatabaseInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	item, err := s.useCase.CreateInstance(c.Request.Context(), &req)
	if err != nil {
		writeDatabaseError(c, "创建失败: ", err)
		return
	}
	response.Success(c, item)
}

// GetInstance 获取数据库实例详情
// @Summary 获取数据库实例详情
// @Description 获取单个数据库实例
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/databases/instances/{id} [get]
func (s *Service) GetInstance(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionView)
	if !ok {
		return
	}
	if scope.enforced && !scope.admin {
		permissions, err := s.permissionRepo.GetUserInstancePermissions(c.Request.Context(), scope.userID, id)
		if err != nil {
			response.ErrorCode(c, http.StatusInternalServerError, "数据库实例权限检查失败")
			return
		}
		if permissions&dbbiz.DatabasePermissionView == 0 {
			response.ErrorCode(c, http.StatusForbidden, "权限不足：无权操作该数据库实例")
			return
		}
	}
	item, err := s.useCase.GetInstance(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "获取失败: ", err)
		return
	}
	if !s.decorateInstancePermissions(c, []*dbbiz.DatabaseInstanceVO{item}, scope) {
		return
	}
	response.Success(c, item)
}

// UpdateInstance 更新数据库实例
// @Summary 更新数据库实例
// @Description 更新数据库实例连接配置
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Param body body dbbiz.DatabaseInstanceRequest true "实例信息"
// @Success 200 {object} response.Response "更新成功"
// @Router /api/v1/databases/instances/{id} [put]
func (s *Service) UpdateInstance(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionManage) {
		return
	}
	req.ID = id
	if err := s.useCase.UpdateInstance(c.Request.Context(), &req); err != nil {
		writeDatabaseError(c, "更新失败: ", err)
		return
	}
	response.SuccessWithMessage(c, "更新成功", nil)
}

// DeleteInstance 删除数据库实例
// @Summary 删除数据库实例
// @Description 删除指定数据库实例
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Success 200 {object} response.Response "删除成功"
// @Router /api/v1/databases/instances/{id} [delete]
func (s *Service) DeleteInstance(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionManage) {
		return
	}
	if err := s.useCase.DeleteInstance(c.Request.Context(), id); err != nil {
		writeDatabaseError(c, "删除失败: ", err)
		return
	}
	response.SuccessWithMessage(c, "删除成功", nil)
}

// EnableInstance 启用数据库实例
// @Summary 启用数据库实例
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Success 200 {object} response.Response "启用成功"
// @Router /api/v1/databases/instances/{id}/enable [post]
func (s *Service) EnableInstance(c *gin.Context) {
	s.setInstanceStatus(c, dbbiz.DatabaseInstanceStatusEnabled, "启用成功")
}

// DisableInstance 禁用数据库实例
// @Summary 禁用数据库实例
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Success 200 {object} response.Response "禁用成功"
// @Router /api/v1/databases/instances/{id}/disable [post]
func (s *Service) DisableInstance(c *gin.Context) {
	s.setInstanceStatus(c, dbbiz.DatabaseInstanceStatusDisabled, "禁用成功")
}

// TestInstance 测试数据库实例连接
// @Summary 测试数据库实例连接
// @Description 测试数据库实例连通性并更新版本信息
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Success 200 {object} response.Response "测试成功"
// @Router /api/v1/databases/instances/{id}/test [post]
func (s *Service) TestInstance(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionManage) {
		return
	}
	data, err := s.useCase.TestInstance(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "连接测试失败: ", err)
		return
	}
	response.Success(c, data)
}

// SyncMetadata 同步数据库元数据
// @Summary 同步数据库元数据
// @Description 拉取实例的 Schema、表、字段和索引元数据
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Success 200 {object} response.Response "同步成功"
// @Router /api/v1/databases/instances/{id}/sync-metadata [post]
func (s *Service) SyncMetadata(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionManage) {
		return
	}
	data, err := s.useCase.SyncMetadata(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "同步失败: ", err)
		return
	}
	response.Success(c, data)
}

// ListSchemas 获取实例 Schema 列表
// @Summary 获取实例 Schema 列表
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/databases/instances/{id}/schemas [get]
func (s *Service) ListSchemas(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionView) {
		return
	}
	data, err := s.useCase.ListSchemas(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, data)
}

// ListTables 获取表列表
// @Summary 获取表列表
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Param schemaName query string false "Schema名称"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/databases/instances/{id}/tables [get]
func (s *Service) ListTables(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionView) {
		return
	}
	data, err := s.useCase.ListTables(c.Request.Context(), id, c.Query("schemaName"))
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, data)
}

// ListColumns 获取字段列表
// @Summary 获取字段列表
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Param schemaName query string false "Schema名称"
// @Param tableName query string true "表名"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/databases/instances/{id}/columns [get]
func (s *Service) ListColumns(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionView) {
		return
	}
	data, err := s.useCase.ListColumns(c.Request.Context(), id, c.Query("schemaName"), c.Query("tableName"))
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, data)
}

// ListIndexes 获取索引列表
// @Summary 获取索引列表
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Param schemaName query string false "Schema名称"
// @Param tableName query string true "表名"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/databases/instances/{id}/indexes [get]
func (s *Service) ListIndexes(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionView) {
		return
	}
	data, err := s.useCase.ListIndexes(c.Request.Context(), id, c.Query("schemaName"), c.Query("tableName"))
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, data)
}

// GetTableDDL 获取表 DDL 预览
// @Summary 获取表 DDL 预览
// @Description 基于已同步元数据生成表结构 DDL 预览
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Param schemaName query string false "Schema名称"
// @Param tableName query string true "表名"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/databases/instances/{id}/ddl [get]
func (s *Service) GetTableDDL(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionView) {
		return
	}
	data, err := s.useCase.GetTableDDL(c.Request.Context(), id, c.Query("schemaName"), c.Query("tableName"))
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, data)
}

// ExportTableDictionary 导出表数据字典 CSV
// @Summary 导出表数据字典
// @Description 基于已同步元数据导出当前表的数据字典 CSV
// @Tags 数据库管理
// @Accept json
// @Produce text/csv
// @Security Bearer
// @Param id path int true "实例ID"
// @Param schemaName query string false "Schema名称"
// @Param tableName query string true "表名"
// @Router /api/v1/databases/instances/{id}/dictionary/export [get]
func (s *Service) ExportTableDictionary(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionExport) {
		return
	}
	data, err := s.useCase.ExportTableDictionary(c.Request.Context(), id, c.Query("schemaName"), c.Query("tableName"), dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "导出失败: ", err)
		return
	}

	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(&buf)
	header := []string{
		"导出时间", "实例", "数据库类型", "Schema", "表名", "表类型", "引擎", "表注释",
		"行数估算", "数据大小(B)", "索引大小(B)", "字段序号", "字段名", "数据类型", "可空",
		"默认值", "键类型", "敏感字段", "字段注释", "索引摘要",
	}
	if err := writer.Write(header); err != nil {
		writeDatabaseError(c, "导出失败: ", err)
		return
	}
	for _, row := range data.Rows {
		record := []string{
			data.ExportedAt,
			safeCSVCell(data.InstanceName),
			safeCSVCell(data.DBTypeText),
			safeCSVCell(row.SchemaName),
			safeCSVCell(row.TableName),
			safeCSVCell(row.TableType),
			safeCSVCell(row.Engine),
			safeCSVCell(row.TableComment),
			strconv.FormatInt(row.RowCount, 10),
			strconv.FormatInt(row.DataSizeBytes, 10),
			strconv.FormatInt(row.IndexSizeBytes, 10),
			strconv.Itoa(row.ColumnOrder),
			safeCSVCell(row.ColumnName),
			safeCSVCell(row.DataType),
			boolText(row.IsNullable),
			safeCSVCell(row.DefaultValue),
			safeCSVCell(row.ColumnKey),
			boolText(row.IsSensitive),
			safeCSVCell(row.ColumnComment),
			safeCSVCell(row.IndexSummary),
		}
		if err := writer.Write(record); err != nil {
			writeDatabaseError(c, "导出失败: ", err)
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		writeDatabaseError(c, "导出失败: ", err)
		return
	}

	filename := fmt.Sprintf(
		"database-dictionary-%s-%s-%s.csv",
		safeFilenamePart(data.SchemaName),
		safeFilenamePart(data.TableName),
		time.Now().Format("20060102150405"),
	)
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}

// GetDiagnosisMetrics 获取数据库诊断指标
// @Summary 获取数据库诊断指标
// @Description 获取当前实例的实时连接、活跃会话、对象数和容量信息
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/databases/instances/{id}/metrics [get]
func (s *Service) GetDiagnosisMetrics(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionDiagnosis) {
		return
	}
	data, err := s.useCase.GetDiagnosisMetrics(c.Request.Context(), id, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, data)
}

// ListDiagnosisSessions 获取数据库活跃会话
// @Summary 获取数据库活跃会话
// @Description 获取当前实例的活跃会话列表
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Param limit query int false "返回条数"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/databases/instances/{id}/sessions [get]
func (s *Service) ListDiagnosisSessions(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseDiagnosisListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionDiagnosis) {
		return
	}
	data, err := s.useCase.ListDiagnosisSessions(c.Request.Context(), id, &req, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, data)
}

// ListSlowQueries 获取数据库慢 SQL
// @Summary 获取数据库慢 SQL
// @Description 获取当前实例的慢 SQL 摘要列表，不可用时返回能力提示
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Param limit query int false "返回条数"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/databases/instances/{id}/slow-queries [get]
func (s *Service) ListSlowQueries(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseDiagnosisListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionDiagnosis) {
		return
	}
	data, err := s.useCase.ListSlowQueries(c.Request.Context(), id, &req, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, data)
}

// FormatQuerySQL 格式化只读 SQL
// @Summary 格式化只读 SQL
// @Description 对只读 SQL 做基础格式化和关键字规范化
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Param body body dbbiz.DatabaseQueryFormatRequest true "格式化请求"
// @Success 200 {object} response.Response "格式化成功"
// @Router /api/v1/databases/instances/{id}/query/format [post]
func (s *Service) FormatQuerySQL(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseQueryFormatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionQuery) {
		return
	}
	data, err := s.useCase.FormatQuerySQL(c.Request.Context(), id, &req)
	if err != nil {
		writeDatabaseError(c, "格式化失败: ", err)
		return
	}
	response.Success(c, data)
}

// ValidateWriteQuery 预检查写 SQL
// @Summary 预检查写 SQL
// @Description 对写 SQL 做风险识别、门禁校验和执行前确认项计算
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Param body body dbbiz.DatabaseWriteValidateRequest true "预检查请求"
// @Success 200 {object} response.Response "预检查完成"
// @Router /api/v1/databases/instances/{id}/query/write/validate [post]
func (s *Service) ValidateWriteQuery(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseWriteValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionWrite) {
		return
	}
	data, err := s.useCase.ValidateWriteQuery(c.Request.Context(), id, &req)
	if err != nil {
		writeDatabaseError(c, "预检查失败: ", err)
		return
	}
	response.Success(c, data)
}

// ExecuteWriteQuery 执行写 SQL
// @Summary 执行写 SQL
// @Description 执行受控单条写 SQL，并写入统一审计
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Param body body dbbiz.DatabaseWriteExecuteRequest true "写操作执行请求"
// @Success 200 {object} response.Response "执行成功"
// @Router /api/v1/databases/instances/{id}/query/write [post]
func (s *Service) ExecuteWriteQuery(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseWriteExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionWrite) {
		return
	}
	data, err := s.useCase.ExecuteWriteQuery(c.Request.Context(), id, &req, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "执行失败: ", err)
		return
	}
	response.Success(c, data)
}

// ValidateDDLQuery 预检查 DDL 结构变更
// @Summary 预检查 DDL 结构变更
// @Description 对 DDL SQL 做风险识别、门禁校验和执行前确认项计算
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Param body body dbbiz.DatabaseDDLValidateRequest true "DDL 预检查请求"
// @Success 200 {object} response.Response "预检查完成"
// @Router /api/v1/databases/instances/{id}/query/ddl/validate [post]
func (s *Service) ValidateDDLQuery(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseDDLValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionDDL) {
		return
	}
	data, err := s.useCase.ValidateDDLQuery(c.Request.Context(), id, &req)
	if err != nil {
		writeDatabaseError(c, "DDL 检查失败: ", err)
		return
	}
	response.Success(c, data)
}

// ExecuteDDLQuery 执行 DDL 结构变更
// @Summary 执行 DDL 结构变更
// @Description 执行受控单条 DDL SQL，并写入统一审计
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Param body body dbbiz.DatabaseWriteExecuteRequest true "DDL 执行请求"
// @Success 200 {object} response.Response "执行成功"
// @Router /api/v1/databases/instances/{id}/query/ddl [post]
func (s *Service) ExecuteDDLQuery(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseWriteExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionDDL) {
		return
	}
	data, err := s.useCase.ExecuteDDLQuery(c.Request.Context(), id, &req, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "DDL 执行失败: ", err)
		return
	}
	response.Success(c, data)
}

// ExecuteQuery 执行只读 SQL 查询
// @Summary 执行只读 SQL 查询
// @Description 执行 SELECT / SHOW / DESC / DESCRIBE / EXPLAIN / WITH 查询，并写入审计
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Param body body dbbiz.DatabaseQueryRequest true "查询请求"
// @Success 200 {object} response.Response "查询成功"
// @Router /api/v1/databases/instances/{id}/query [post]
func (s *Service) ExecuteQuery(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionQuery) {
		return
	}
	if req.UnlimitedRows {
		if !s.allowUnlimitedQueryRows(c, id) {
			return
		}
		req.UnlimitedRowsPermitted = true
	}
	data, err := s.useCase.ExecuteQuery(c.Request.Context(), id, &req, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, data)
}

// ExplainQuery 获取 SQL 执行计划
// @Summary 获取 SQL 执行计划
// @Description 对 SELECT / WITH 查询执行 EXPLAIN，并写入审计
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Param body body dbbiz.DatabaseQueryRequest true "查询请求"
// @Success 200 {object} response.Response "执行成功"
// @Router /api/v1/databases/instances/{id}/query/explain [post]
func (s *Service) ExplainQuery(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionQuery) {
		return
	}
	data, err := s.useCase.ExplainQuery(c.Request.Context(), id, &req, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "执行计划失败: ", err)
		return
	}
	response.Success(c, data)
}

// ExplainWriteQuery 获取写 SQL 执行计划
// @Summary 获取写 SQL 执行计划
// @Description 对受控写 SQL 执行 EXPLAIN，并写入审计；不会执行真实写入
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "实例ID"
// @Param body body dbbiz.DatabaseQueryRequest true "查询请求"
// @Success 200 {object} response.Response "执行成功"
// @Router /api/v1/databases/instances/{id}/query/write/explain [post]
func (s *Service) ExplainWriteQuery(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionQuery) {
		return
	}
	if !s.allowWriteExplainQuery(c, id) {
		return
	}
	data, err := s.useCase.ExplainWriteQuery(c.Request.Context(), id, &req, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "写 SQL 执行计划失败: ", err)
		return
	}
	response.Success(c, data)
}

// ExportQueryResult 导出 SQL 结果 CSV
// @Summary 导出 SQL 结果
// @Description 对只读 SQL 重新执行并导出结果 CSV，最多导出 5000 行
// @Tags 数据库管理
// @Accept json
// @Produce text/csv
// @Security Bearer
// @Param id path int true "实例ID"
// @Param body body dbbiz.DatabaseQueryRequest true "查询请求"
// @Router /api/v1/databases/instances/{id}/query/export [post]
func (s *Service) ExportQueryResult(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	var req dbbiz.DatabaseQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionExport) {
		return
	}
	data, err := s.useCase.ExportQuery(c.Request.Context(), id, &req, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "导出失败: ", err)
		return
	}

	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(&buf)
	if err := writer.Write(data.Columns); err != nil {
		writeDatabaseError(c, "导出失败: ", err)
		return
	}
	for _, row := range data.Rows {
		record := make([]string, 0, len(data.Columns))
		for _, column := range data.Columns {
			record = append(record, safeCSVCell(stringifyQueryValue(row[column])))
		}
		if err := writer.Write(record); err != nil {
			writeDatabaseError(c, "导出失败: ", err)
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		writeDatabaseError(c, "导出失败: ", err)
		return
	}

	filename := fmt.Sprintf("database-query-result-%s.csv", time.Now().Format("20060102150405"))
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}

func (s *Service) setInstanceStatus(c *gin.Context, status, message string) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, id, dbbiz.DatabasePermissionManage) {
		return
	}
	if err := s.useCase.SetInstanceStatus(c.Request.Context(), id, status); err != nil {
		writeDatabaseError(c, "状态更新失败: ", err)
		return
	}
	response.SuccessWithMessage(c, message, nil)
}

func safeCSVCell(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	switch value[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + value
	default:
		return value
	}
}

func boolText(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func safeFilenamePart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "default"
	}
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(value)
}

func stringifyQueryValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
