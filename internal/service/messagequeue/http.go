package messagequeue

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	mqbiz "github.com/ydcloud-dy/opshub/internal/biz/messagequeue"
	rbacservice "github.com/ydcloud-dy/opshub/internal/service/rbac"
	"github.com/ydcloud-dy/opshub/pkg/response"
)

type Service struct {
	useCase        *mqbiz.UseCase
	permissionRepo mqbiz.PermissionRepo
}

func NewService(useCase *mqbiz.UseCase, permissionRepo mqbiz.PermissionRepo) *Service {
	return &Service{useCase: useCase, permissionRepo: permissionRepo}
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
		strings.Contains(message, "范围"):
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

func applyInstancePermissionScope(req *mqbiz.InstanceListRequest, scope *permissionScope) {
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

func (s *Service) ListOperationAudits(c *gin.Context) {
	var req mqbiz.AuditListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
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
	list, total, err := s.useCase.ListMessageAudits(c.Request.Context(), &req)
	if err != nil {
		writeError(c, "查询失败: ", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total, "page": req.Page, "pageSize": req.PageSize})
}
