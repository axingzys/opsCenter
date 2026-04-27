package messagequeue

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	mqbiz "github.com/ydcloud-dy/opshub/internal/biz/messagequeue"
	rbacservice "github.com/ydcloud-dy/opshub/internal/service/rbac"
	"github.com/ydcloud-dy/opshub/pkg/response"
)

const permMQHighRisk = "messagequeue:operation:high-risk"

type MenuPermissionChecker func(ctx context.Context, userID uint, code string) (bool, error)

type Service struct {
	useCase               *mqbiz.UseCase
	permissionRepo        mqbiz.PermissionRepo
	menuPermissionChecker MenuPermissionChecker
}

func NewService(useCase *mqbiz.UseCase, permissionRepo mqbiz.PermissionRepo, menuPermissionChecker MenuPermissionChecker) *Service {
	return &Service{useCase: useCase, permissionRepo: permissionRepo, menuPermissionChecker: menuPermissionChecker}
}

type permissionScope struct {
	enforced   bool
	admin      bool
	userID     uint
	allowedIDs []uint
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

func writeError(c *gin.Context, prefix string, err error) {
	statusCode := http.StatusInternalServerError
	message := err.Error()
	switch {
	case strings.Contains(message, "不存在"):
		statusCode = http.StatusNotFound
	case strings.Contains(message, "不能为空"),
		strings.Contains(message, "请选择"),
		strings.Contains(message, "不支持"),
		strings.Contains(message, "不可用"),
		strings.Contains(message, "未授权"),
		strings.Contains(message, "已禁用"),
		strings.Contains(message, "范围"),
		strings.Contains(message, "确认"),
		strings.Contains(message, "未开启"),
		strings.Contains(message, "参数"):
		statusCode = http.StatusBadRequest
	}
	response.ErrorCode(c, statusCode, prefix+message)
}

func normalizePermissionMask(permissions uint) uint {
	return permissions & mqbiz.PermissionAll
}

func (s *Service) permissionScope(c *gin.Context, required uint) (*permissionScope, bool) {
	scope := &permissionScope{userID: rbacservice.GetUserID(c)}
	if s.permissionRepo == nil {
		return scope, true
	}
	if scope.userID == 0 {
		response.ErrorCode(c, http.StatusUnauthorized, "未登录")
		return nil, false
	}
	hasRules, err := s.permissionRepo.HasAnyRules(c.Request.Context())
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "MQ实例权限检查失败")
		return nil, false
	}
	if !hasRules {
		return scope, true
	}
	scope.enforced = true
	admin, err := s.permissionRepo.IsAdmin(c.Request.Context(), scope.userID)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "MQ实例权限检查失败")
		return nil, false
	}
	if admin {
		scope.admin = true
		return scope, true
	}
	allowedIDs, err := s.permissionRepo.GetUserAccessibleInstanceIDs(c.Request.Context(), scope.userID, required)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "MQ实例权限检查失败")
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
	scope, ok := s.permissionScope(c, required)
	if !ok {
		return false
	}
	if !scope.enforced || scope.admin {
		return true
	}
	permissions, err := s.permissionRepo.GetUserInstancePermissions(c.Request.Context(), scope.userID, instanceID)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "MQ实例权限检查失败")
		return false
	}
	if permissions&required == 0 {
		response.ErrorCode(c, http.StatusForbidden, "权限不足：无权操作该MQ实例")
		return false
	}
	return true
}

func (s *Service) ensureHighRiskPermission(c *gin.Context, instanceID uint) bool {
	userID := rbacservice.GetUserID(c)
	if userID == 0 {
		response.ErrorCode(c, http.StatusUnauthorized, "未登录")
		return false
	}
	if s.menuPermissionChecker == nil {
		response.ErrorCode(c, http.StatusInternalServerError, "权限检查未初始化")
		return false
	}
	ok, err := s.menuPermissionChecker(c.Request.Context(), userID, permMQHighRisk)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "MQ高危菜单权限检查失败")
		return false
	}
	if !ok {
		response.ErrorCode(c, http.StatusForbidden, "权限不足：缺少MQ高危操作菜单权限")
		return false
	}
	if s.permissionRepo == nil {
		return true
	}
	hasRules, err := s.permissionRepo.HasAnyRules(c.Request.Context())
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "MQ实例权限检查失败")
		return false
	}
	if !hasRules {
		admin, err := s.permissionRepo.IsAdmin(c.Request.Context(), userID)
		if err != nil {
			response.ErrorCode(c, http.StatusInternalServerError, "MQ实例权限检查失败")
			return false
		}
		if !admin {
			response.ErrorCode(c, http.StatusForbidden, "权限不足：无权对该MQ实例执行高危操作")
			return false
		}
		return true
	}
	permissions, err := s.permissionRepo.GetUserInstancePermissions(c.Request.Context(), userID, instanceID)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "MQ实例权限检查失败")
		return false
	}
	if permissions&mqbiz.PermissionHighRisk == 0 {
		response.ErrorCode(c, http.StatusForbidden, "权限不足：无权对该MQ实例执行高危操作")
		return false
	}
	return true
}

func (s *Service) userHasHighRiskAccess(c *gin.Context, instanceID uint) bool {
	userID := rbacservice.GetUserID(c)
	if userID == 0 || s.menuPermissionChecker == nil {
		return false
	}
	ok, err := s.menuPermissionChecker(c.Request.Context(), userID, permMQHighRisk)
	if err != nil || !ok {
		return false
	}
	if s.permissionRepo == nil {
		return true
	}
	hasRules, err := s.permissionRepo.HasAnyRules(c.Request.Context())
	if err != nil {
		return false
	}
	if !hasRules {
		admin, err := s.permissionRepo.IsAdmin(c.Request.Context(), userID)
		return err == nil && admin
	}
	permissions, err := s.permissionRepo.GetUserInstancePermissions(c.Request.Context(), userID, instanceID)
	if err != nil {
		return false
	}
	return permissions&mqbiz.PermissionHighRisk != 0
}

func applyInstancePermissionScope(req *mqbiz.InstanceListRequest, scope *permissionScope) {
	if req == nil || scope == nil || !scope.enforced || scope.admin {
		return
	}
	req.RestrictToAllowed = true
	req.AllowedIDs = scope.allowedIDs
}

func applyJobPermissionScope(req *mqbiz.JobListRequest, scope *permissionScope) {
	if req == nil || scope == nil || !scope.enforced || scope.admin {
		return
	}
	req.RestrictToAllowed = true
	req.AllowedIDs = scope.allowedIDs
}

func applyAuditPermissionScope(req *mqbiz.AuditListRequest, scope *permissionScope) {
	if req == nil || scope == nil || !scope.enforced || scope.admin {
		return
	}
	req.RestrictToAllowed = true
	req.AllowedIDs = scope.allowedIDs
}

func applyGovernancePermissionScope(req *mqbiz.GovernanceReportRequest, scope *permissionScope) {
	if req == nil || scope == nil || !scope.enforced || scope.admin {
		return
	}
	req.RestrictToAllowed = true
	req.AllowedIDs = scope.allowedIDs
}

func applyDLQPermissionScope(req *mqbiz.DLQAnalysisRequest, scope *permissionScope) {
	if req == nil || scope == nil || !scope.enforced || scope.admin {
		return
	}
	req.RestrictToAllowed = true
	req.AllowedIDs = scope.allowedIDs
}

func applyAuditChainPermissionScope(req *mqbiz.AuditChainVerifyRequest, scope *permissionScope) {
	if req == nil || scope == nil || !scope.enforced || scope.admin {
		return
	}
	req.RestrictToAllowed = true
	req.AllowedIDs = scope.allowedIDs
}

func (s *Service) decorateInstancePermissions(c *gin.Context, list []*mqbiz.InstanceVO, scope *permissionScope) bool {
	if len(list) == 0 {
		return true
	}
	if scope != nil && !scope.enforced && !scope.admin && s.permissionRepo != nil && scope.userID > 0 {
		admin, err := s.permissionRepo.IsAdmin(c.Request.Context(), scope.userID)
		if err != nil {
			response.ErrorCode(c, http.StatusInternalServerError, "MQ实例权限检查失败")
			return false
		}
		permissions := mqbiz.PermissionAll
		if !admin {
			permissions &^= mqbiz.PermissionHighRisk
		}
		for _, item := range list {
			item.Permissions = permissions
		}
		return true
	}
	if scope == nil || !scope.enforced || scope.admin || s.permissionRepo == nil {
		for _, item := range list {
			item.Permissions = mqbiz.PermissionAll
		}
		return true
	}
	for _, item := range list {
		permissions, err := s.permissionRepo.GetUserInstancePermissions(c.Request.Context(), scope.userID, item.ID)
		if err != nil {
			response.ErrorCode(c, http.StatusInternalServerError, "MQ实例权限检查失败")
			return false
		}
		item.Permissions = permissions
	}
	return true
}

func currentOperator(c *gin.Context) mqbiz.Operator {
	return mqbiz.Operator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	}
}

func (s *Service) GetSupportedTypes(c *gin.Context) {
	response.Success(c, s.useCase.SupportedTypes())
}

func (s *Service) ListInstancePermissions(c *gin.Context) {
	var req mqbiz.InstancePermissionListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	list, total, err := s.permissionRepo.List(c.Request.Context(), &req)
	if err != nil {
		writeError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total, "page": req.Page, "pageSize": req.PageSize})
}

func (s *Service) UpsertInstancePermission(c *gin.Context) {
	var req mqbiz.InstancePermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	permissions := normalizePermissionMask(req.Permissions)
	if permissions == 0 {
		response.ErrorCode(c, http.StatusBadRequest, "请选择MQ实例权限")
		return
	}
	if err := s.permissionRepo.ValidateTarget(c.Request.Context(), req.RoleID, req.InstanceID); err != nil {
		writeError(c, "保存失败: ", err)
		return
	}
	before, err := s.permissionRepo.GetByRoleInstance(c.Request.Context(), req.RoleID, req.InstanceID)
	if err != nil {
		writeError(c, "保存失败: ", err)
		return
	}
	if err := s.permissionRepo.Upsert(c.Request.Context(), &mqbiz.MQInstancePermission{
		RoleID:      req.RoleID,
		InstanceID:  req.InstanceID,
		Permissions: permissions,
	}); err != nil {
		writeError(c, "保存失败: ", err)
		return
	}
	s.useCase.AuditInstancePermission(c.Request.Context(), mqbiz.AuditActionPermissionSet, gin.H{
		"before": before,
		"after":  req,
	}, currentOperator(c))
	response.SuccessWithMessage(c, "保存成功", nil)
}

func (s *Service) DeleteInstancePermission(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "权限ID")
	if !ok {
		return
	}
	before, err := s.permissionRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		writeError(c, "删除失败: ", err)
		return
	}
	if before == nil {
		response.ErrorCode(c, http.StatusNotFound, "权限配置不存在")
		return
	}
	if err := s.permissionRepo.Delete(c.Request.Context(), id); err != nil {
		writeError(c, "删除失败: ", err)
		return
	}
	s.useCase.AuditInstancePermission(c.Request.Context(), mqbiz.AuditActionPermissionDel, before, currentOperator(c))
	response.SuccessWithMessage(c, "删除成功", nil)
}

func (s *Service) ListInstances(c *gin.Context) {
	var req mqbiz.InstanceListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.permissionScope(c, mqbiz.PermissionView)
	if !ok {
		return
	}
	applyInstancePermissionScope(&req, scope)
	list, total, err := s.useCase.ListInstances(c.Request.Context(), &req)
	if err != nil {
		writeError(c, "查询失败: ", err)
		return
	}
	if !s.decorateInstancePermissions(c, list, scope) {
		return
	}
	response.Success(c, gin.H{"list": list, "total": total, "page": req.Page, "pageSize": req.PageSize})
}

func (s *Service) CreateInstance(c *gin.Context) {
	var req mqbiz.InstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	item, err := s.useCase.CreateInstance(c.Request.Context(), &req)
	if err != nil {
		writeError(c, "创建失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) GetInstance(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, id, mqbiz.PermissionView) {
		return
	}
	item, err := s.useCase.GetInstance(c.Request.Context(), id)
	if err != nil {
		writeError(c, "获取失败: ", err)
		return
	}
	response.Success(c, item)
}

func (s *Service) UpdateInstance(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	var req mqbiz.InstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, id, mqbiz.PermissionManage) {
		return
	}
	req.ID = id
	if err := s.useCase.UpdateInstance(c.Request.Context(), &req); err != nil {
		writeError(c, "更新失败: ", err)
		return
	}
	response.SuccessWithMessage(c, "更新成功", nil)
}

func (s *Service) DeleteInstance(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, id, mqbiz.PermissionManage) {
		return
	}
	if err := s.useCase.DeleteInstance(c.Request.Context(), id); err != nil {
		writeError(c, "删除失败: ", err)
		return
	}
	response.SuccessWithMessage(c, "删除成功", nil)
}

func (s *Service) EnableInstance(c *gin.Context) {
	s.setInstanceStatus(c, mqbiz.InstanceStatusEnabled, "启用成功")
}

func (s *Service) DisableInstance(c *gin.Context) {
	s.setInstanceStatus(c, mqbiz.InstanceStatusDisabled, "禁用成功")
}

func (s *Service) setInstanceStatus(c *gin.Context, status, message string) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, id, mqbiz.PermissionManage) {
		return
	}
	if err := s.useCase.SetInstanceStatus(c.Request.Context(), id, status); err != nil {
		writeError(c, "操作失败: ", err)
		return
	}
	response.SuccessWithMessage(c, message, nil)
}

func (s *Service) TestInstance(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, id, mqbiz.PermissionManage) {
		return
	}
	data, err := s.useCase.TestInstance(c.Request.Context(), id, currentOperator(c))
	if err != nil {
		writeError(c, "连接测试失败: ", err)
		return
	}
	response.Success(c, data)
}

func (s *Service) SyncMetadata(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, id, mqbiz.PermissionManage) {
		return
	}
	data, err := s.useCase.SyncMetadata(c.Request.Context(), id, currentOperator(c))
	if err != nil {
		writeError(c, "同步失败: ", err)
		return
	}
	response.Success(c, data)
}

func (s *Service) ListJobs(c *gin.Context) {
	var req mqbiz.JobListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.permissionScope(c, mqbiz.PermissionDiagnose)
	if !ok {
		return
	}
	if req.InstanceID > 0 && !s.ensureInstancePermission(c, req.InstanceID, mqbiz.PermissionDiagnose) {
		return
	}
	applyJobPermissionScope(&req, scope)
	list, total, err := s.useCase.ListJobs(c.Request.Context(), &req)
	if err != nil {
		writeError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total, "page": req.Page, "pageSize": req.PageSize})
}

func (s *Service) GetProductionDashboard(c *gin.Context) {
	scope, ok := s.permissionScope(c, mqbiz.PermissionDiagnose)
	if !ok {
		return
	}
	req := &mqbiz.GovernanceReportRequest{Page: 1, PageSize: 10}
	applyGovernancePermissionScope(req, scope)
	data, err := s.useCase.GetProductionDashboard(c.Request.Context(), req)
	if err != nil {
		writeError(c, "查询失败: ", err)
		return
	}
	response.Success(c, data)
}

func (s *Service) GetGovernanceReport(c *gin.Context) {
	var req mqbiz.GovernanceReportRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.permissionScope(c, mqbiz.PermissionDiagnose)
	if !ok {
		return
	}
	if req.InstanceID > 0 && !s.ensureInstancePermission(c, req.InstanceID, mqbiz.PermissionDiagnose) {
		return
	}
	applyGovernancePermissionScope(&req, scope)
	data, err := s.useCase.GetGovernanceReport(c.Request.Context(), &req)
	if err != nil {
		writeError(c, "查询失败: ", err)
		return
	}
	response.Success(c, data)
}

func (s *Service) GetDLQAnalysis(c *gin.Context) {
	var req mqbiz.DLQAnalysisRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.permissionScope(c, mqbiz.PermissionDiagnose)
	if !ok {
		return
	}
	if req.InstanceID > 0 && !s.ensureInstancePermission(c, req.InstanceID, mqbiz.PermissionDiagnose) {
		return
	}
	applyDLQPermissionScope(&req, scope)
	data, err := s.useCase.GetDLQAnalysis(c.Request.Context(), &req)
	if err != nil {
		writeError(c, "查询失败: ", err)
		return
	}
	response.Success(c, data)
}

func (s *Service) UpsertDLQRecord(c *gin.Context) {
	var req mqbiz.DLQRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, req.InstanceID, mqbiz.PermissionDiagnose) {
		return
	}
	if err := s.useCase.UpsertDLQRecord(c.Request.Context(), &req, currentOperator(c)); err != nil {
		writeError(c, "保存失败: ", err)
		return
	}
	response.Success(c, gin.H{"message": "DLQ处理记录已保存"})
}

func (s *Service) VerifyAuditChain(c *gin.Context) {
	var req mqbiz.AuditChainVerifyRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.permissionScope(c, mqbiz.PermissionAudit)
	if !ok {
		return
	}
	if req.InstanceID > 0 && !s.ensureInstancePermission(c, req.InstanceID, mqbiz.PermissionAudit) {
		return
	}
	applyAuditChainPermissionScope(&req, scope)
	data, err := s.useCase.VerifyAuditChain(c.Request.Context(), &req)
	if err != nil {
		writeError(c, "校验失败: ", err)
		return
	}
	response.Success(c, data)
}

func (s *Service) GetJob(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "任务ID")
	if !ok {
		return
	}
	item, err := s.useCase.GetJob(c.Request.Context(), id)
	if err != nil {
		writeError(c, "获取失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, item.InstanceID, mqbiz.PermissionDiagnose) {
		return
	}
	response.Success(c, item)
}

func (s *Service) ListBrokers(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok || !s.ensureInstancePermission(c, id, mqbiz.PermissionDiagnose) {
		return
	}
	list, err := s.useCase.ListBrokers(c.Request.Context(), id)
	if err != nil {
		writeError(c, "查询失败: ", err)
		return
	}
	response.Success(c, list)
}

func (s *Service) ListResources(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok || !s.ensureInstancePermission(c, id, mqbiz.PermissionView) {
		return
	}
	var req mqbiz.ResourceListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	list, total, err := s.useCase.ListResources(c.Request.Context(), id, &req)
	if err != nil {
		writeError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total, "page": req.Page, "pageSize": req.PageSize})
}

func (s *Service) ListBindings(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok || !s.ensureInstancePermission(c, id, mqbiz.PermissionView) {
		return
	}
	list, err := s.useCase.ListBindings(c.Request.Context(), id)
	if err != nil {
		writeError(c, "查询失败: ", err)
		return
	}
	response.Success(c, list)
}

func (s *Service) ListConsumerGroups(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok || !s.ensureInstancePermission(c, id, mqbiz.PermissionDiagnose) {
		return
	}
	var req mqbiz.ConsumerGroupListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	list, total, err := s.useCase.ListConsumerGroups(c.Request.Context(), id, &req)
	if err != nil {
		writeError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total, "page": req.Page, "pageSize": req.PageSize})
}

func (s *Service) ListPartitions(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok || !s.ensureInstancePermission(c, id, mqbiz.PermissionDiagnose) {
		return
	}
	var req mqbiz.PartitionListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	list, total, err := s.useCase.ListPartitions(c.Request.Context(), id, &req)
	if err != nil {
		writeError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total, "page": req.Page, "pageSize": req.PageSize})
}

func (s *Service) GetOverview(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok || !s.ensureInstancePermission(c, id, mqbiz.PermissionDiagnose) {
		return
	}
	data, err := s.useCase.GetOverview(c.Request.Context(), id)
	if err != nil {
		writeError(c, "查询失败: ", err)
		return
	}
	response.Success(c, data)
}

func (s *Service) GetCapacityForecast(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok || !s.ensureInstancePermission(c, id, mqbiz.PermissionDiagnose) {
		return
	}
	var req mqbiz.CapacityForecastRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	data, err := s.useCase.GetCapacityForecast(c.Request.Context(), id, &req)
	if err != nil {
		writeError(c, "预测失败: ", err)
		return
	}
	response.Success(c, data)
}

func (s *Service) GetCapabilities(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok || !s.ensureInstancePermission(c, id, mqbiz.PermissionView) {
		return
	}
	data, err := s.useCase.GetInstanceCapabilities(c.Request.Context(), id)
	if err != nil {
		writeError(c, "查询失败: ", err)
		return
	}
	response.Success(c, data)
}

func (s *Service) GetTopology(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok || !s.ensureInstancePermission(c, id, mqbiz.PermissionDiagnose) {
		return
	}
	data, err := s.useCase.GetTopology(c.Request.Context(), id)
	if err != nil {
		writeError(c, "查询失败: ", err)
		return
	}
	response.Success(c, data)
}

func (s *Service) BuildConfigClonePlan(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok || !s.ensureInstancePermission(c, id, mqbiz.PermissionResourceManage) {
		return
	}
	var req mqbiz.ConfigClonePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, req.TargetInstanceID, mqbiz.PermissionResourceManage) {
		return
	}
	data, err := s.useCase.BuildConfigClonePlan(c.Request.Context(), id, &req, currentOperator(c))
	if err != nil {
		writeError(c, "生成失败: ", err)
		return
	}
	response.Success(c, data)
}

func (s *Service) ListOperationActions(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok || !s.ensureInstancePermission(c, id, mqbiz.PermissionResourceManage) {
		return
	}
	data, err := s.useCase.ListOperationActions(c.Request.Context(), id)
	if err != nil {
		writeError(c, "查询失败: ", err)
		return
	}
	if !s.userHasHighRiskAccess(c, id) {
		for _, item := range data {
			if item != nil && item.RequiresHighRiskAck {
				item.Enabled = false
				if item.DisabledReason == "" {
					item.DisabledReason = "当前用户缺少MQ高危操作权限"
				}
			}
		}
	}
	response.Success(c, data)
}

func (s *Service) CollectMetricSnapshot(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok || !s.ensureInstancePermission(c, id, mqbiz.PermissionDiagnose) {
		return
	}
	data, err := s.useCase.CollectMetricSnapshot(c.Request.Context(), id, currentOperator(c))
	if err != nil {
		writeError(c, "采集失败: ", err)
		return
	}
	response.Success(c, data)
}

func (s *Service) ListMetricSnapshots(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok || !s.ensureInstancePermission(c, id, mqbiz.PermissionDiagnose) {
		return
	}
	var req mqbiz.MetricSnapshotListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	list, total, err := s.useCase.ListMetricSnapshots(c.Request.Context(), id, &req)
	if err != nil {
		writeError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total, "page": req.Page, "pageSize": req.PageSize})
}

func (s *Service) GenerateInspectionReport(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok || !s.ensureInstancePermission(c, id, mqbiz.PermissionDiagnose) {
		return
	}
	data, err := s.useCase.GenerateInspectionReport(c.Request.Context(), id, currentOperator(c))
	if err != nil {
		writeError(c, "巡检失败: ", err)
		return
	}
	response.Success(c, data)
}

func (s *Service) SampleMessages(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok || !s.ensureInstancePermission(c, id, mqbiz.PermissionMessageRead) {
		return
	}
	var req mqbiz.MessageSampleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	data, err := s.useCase.SampleMessages(c.Request.Context(), id, &req, currentOperator(c))
	if err != nil {
		writeError(c, "消息采样失败: ", err)
		return
	}
	response.Success(c, data)
}

func (s *Service) InspectMessageSchema(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok || !s.ensureInstancePermission(c, id, mqbiz.PermissionMessageRead) {
		return
	}
	var req mqbiz.MessageSchemaInspectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	data, err := s.useCase.InspectMessageSchema(c.Request.Context(), id, &req, currentOperator(c))
	if err != nil {
		writeError(c, "Schema检查失败: ", err)
		return
	}
	response.Success(c, data)
}

func (s *Service) PrepareMessageReplayApplication(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok || !s.ensureInstancePermission(c, id, mqbiz.PermissionMessageWrite) {
		return
	}
	var req mqbiz.MessageReplayApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	data, err := s.useCase.PrepareMessageReplayApplication(c.Request.Context(), id, &req, currentOperator(c))
	if err != nil {
		writeError(c, "重放申请失败: ", err)
		return
	}
	response.Success(c, data)
}

func (s *Service) ValidateResourceOperation(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok || !s.ensureInstancePermission(c, id, mqbiz.PermissionResourceManage) {
		return
	}
	var req mqbiz.ResourceOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	data, err := s.useCase.ValidateResourceOperation(c.Request.Context(), id, &req)
	if err != nil {
		writeError(c, "校验失败: ", err)
		return
	}
	if data.Supported && mqbiz.IsHighRiskLevel(data.RiskLevel) && !s.ensureHighRiskPermission(c, id) {
		return
	}
	response.Success(c, data)
}

func (s *Service) ExecuteResourceOperation(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "实例ID")
	if !ok || !s.ensureInstancePermission(c, id, mqbiz.PermissionResourceManage) {
		return
	}
	var req mqbiz.ResourceOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	validation, err := s.useCase.ValidateResourceOperation(c.Request.Context(), id, &req)
	if err != nil {
		writeError(c, "校验失败: ", err)
		return
	}
	if validation.Supported && mqbiz.IsHighRiskLevel(validation.RiskLevel) && !s.ensureHighRiskPermission(c, id) {
		return
	}
	data, err := s.useCase.ExecuteResourceOperation(c.Request.Context(), id, &req, currentOperator(c))
	if err != nil {
		writeError(c, "执行失败: ", err)
		return
	}
	response.Success(c, data)
}

func (s *Service) ListOperationAudits(c *gin.Context) {
	var req mqbiz.AuditListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.permissionScope(c, mqbiz.PermissionAudit)
	if !ok {
		return
	}
	if req.InstanceID > 0 && !s.ensureInstancePermission(c, req.InstanceID, mqbiz.PermissionAudit) {
		return
	}
	applyAuditPermissionScope(&req, scope)
	list, total, err := s.useCase.ListOperationAudits(c.Request.Context(), &req)
	if err != nil {
		writeError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total, "page": req.Page, "pageSize": req.PageSize})
}

func (s *Service) ListMessageAudits(c *gin.Context) {
	var req mqbiz.AuditListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.permissionScope(c, mqbiz.PermissionAudit)
	if !ok {
		return
	}
	if req.InstanceID > 0 && !s.ensureInstancePermission(c, req.InstanceID, mqbiz.PermissionAudit) {
		return
	}
	applyAuditPermissionScope(&req, scope)
	list, total, err := s.useCase.ListMessageAudits(c.Request.Context(), &req)
	if err != nil {
		writeError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total, "page": req.Page, "pageSize": req.PageSize})
}
