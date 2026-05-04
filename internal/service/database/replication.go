package database

import (
	"net/http"

	"github.com/gin-gonic/gin"
	dbbiz "github.com/ydcloud-dy/opshub/internal/biz/database"
	rbacservice "github.com/ydcloud-dy/opshub/internal/service/rbac"
	"github.com/ydcloud-dy/opshub/pkg/response"
)

func (s *Service) ListReplicas(c *gin.Context) {
	var req dbbiz.DatabaseInstanceReplicaListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionTopology)
	if !ok {
		return
	}
	applyAllowedInstanceScope(&req, scope)
	list, total, err := s.useCase.ListInstanceReplicas(c.Request.Context(), &req)
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

func (s *Service) ListInstanceReplicas(c *gin.Context) {
	instanceID, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, instanceID, dbbiz.DatabasePermissionTopology) {
		return
	}
	var req dbbiz.DatabaseInstanceReplicaListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	req.InstanceID = instanceID
	list, total, err := s.useCase.ListInstanceReplicas(c.Request.Context(), &req)
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

func (s *Service) DeleteReplicaRelation(c *gin.Context) {
	replicaID, ok := parseUintParam(c, "id", "副本关系ID")
	if !ok {
		return
	}
	target, err := s.useCase.GetInstanceReplica(c.Request.Context(), replicaID)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	if target == nil || !s.ensureReplicaRecordPermission(c, target.PrimaryInstanceID, target.ReplicaInstanceID) {
		return
	}
	err = s.useCase.DeleteInstanceReplica(c.Request.Context(), replicaID, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "删除失败: ", err)
		return
	}
	response.SuccessWithMessage(c, "副本关系记录已删除", gin.H{"id": replicaID})
}

func (s *Service) ListReplicationChecks(c *gin.Context) {
	var req dbbiz.DatabaseReplicationCheckListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionTopology)
	if !ok {
		return
	}
	applyAllowedInstanceScope(&req, scope)
	list, total, err := s.useCase.ListReplicationChecks(c.Request.Context(), &req)
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

func (s *Service) ListReplicaProtections(c *gin.Context) {
	var req dbbiz.DatabaseReplicaProtectionListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionTopology)
	if !ok {
		return
	}
	applyAllowedInstanceScope(&req, scope)
	list, total, err := s.useCase.ListReplicaProtections(c.Request.Context(), &req)
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

func (s *Service) CreateReplicaIncidentGuide(c *gin.Context) {
	var req dbbiz.DatabaseReplicaIncidentGuideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !s.ensureInstancePermission(c, req.InstanceID, dbbiz.DatabasePermissionTopology) {
		return
	}
	result, err := s.useCase.CreateReplicaIncidentGuide(c.Request.Context(), &req, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "生成失败: ", err)
		return
	}
	response.SuccessWithMessage(c, "事故指引已生成", result)
}

func (s *Service) ListReplicaIncidentGuides(c *gin.Context) {
	var req dbbiz.DatabaseReplicaIncidentGuideListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionTopology)
	if !ok {
		return
	}
	applyAllowedInstanceScope(&req, scope)
	list, total, err := s.useCase.ListReplicaIncidentGuides(c.Request.Context(), &req)
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

func (s *Service) GetReplicaIncidentGuide(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "事故指引ID")
	if !ok {
		return
	}
	result, err := s.useCase.GetReplicaIncidentGuide(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, result.InstanceID, dbbiz.DatabasePermissionTopology) {
		return
	}
	response.Success(c, result)
}

func (s *Service) DeleteReplicaIncidentGuide(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "事故指引ID")
	if !ok {
		return
	}
	result, err := s.useCase.GetReplicaIncidentGuide(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	if !s.ensureInstancePermission(c, result.InstanceID, dbbiz.DatabasePermissionTopology) {
		return
	}
	err = s.useCase.DeleteReplicaIncidentGuide(c.Request.Context(), id, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "删除失败: ", err)
		return
	}
	response.SuccessWithMessage(c, "事故指引已删除", gin.H{"id": id})
}

func (s *Service) ListReplicaActions(c *gin.Context) {
	var req dbbiz.DatabaseReplicaActionListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionTopology)
	if !ok {
		return
	}
	applyAllowedInstanceScope(&req, scope)
	list, total, err := s.useCase.ListReplicaActions(c.Request.Context(), &req)
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

func (s *Service) DeleteReplicaAction(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "Apply 操作记录ID")
	if !ok {
		return
	}
	target, err := s.useCase.GetReplicaAction(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	if target == nil || !s.ensureReplicaRecordPermission(c, target.PrimaryInstanceID, target.ReplicaInstanceID) {
		return
	}
	err = s.useCase.DeleteReplicaAction(c.Request.Context(), id, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "删除失败: ", err)
		return
	}
	response.SuccessWithMessage(c, "Apply 操作记录已删除", gin.H{"id": id})
}

func (s *Service) PauseReplicaApply(c *gin.Context) {
	s.executeReplicaApplyAction(c, true)
}

func (s *Service) ResumeReplicaApply(c *gin.Context) {
	s.executeReplicaApplyAction(c, false)
}

func (s *Service) executeReplicaApplyAction(c *gin.Context, pause bool) {
	replicaID, ok := parseUintParam(c, "id", "副本关系ID")
	if !ok {
		return
	}
	target, err := s.useCase.GetInstanceReplica(c.Request.Context(), replicaID)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	if target == nil || target.ReplicaInstanceID == 0 {
		response.ErrorCode(c, http.StatusBadRequest, "副本关系缺少目标实例")
		return
	}
	if !s.ensureInstancePermission(c, target.ReplicaInstanceID, dbbiz.DatabasePermissionTopology) {
		return
	}
	if target.PrimaryInstanceID > 0 && !s.ensureInstancePermission(c, target.PrimaryInstanceID, dbbiz.DatabasePermissionTopology) {
		return
	}
	var req dbbiz.DatabaseReplicaActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	operator := dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	}
	var result *dbbiz.DatabaseReplicaActionVO
	if pause {
		result, err = s.useCase.PauseReplicaApply(c.Request.Context(), replicaID, &req, operator)
	} else {
		result, err = s.useCase.ResumeReplicaApply(c.Request.Context(), replicaID, &req, operator)
	}
	if err != nil {
		prefix := "执行失败: "
		if pause {
			prefix = "暂停 apply 失败: "
		} else {
			prefix = "恢复 apply 失败: "
		}
		writeDatabaseError(c, prefix, err)
		return
	}
	message := "apply 控制已执行"
	if pause {
		message = "已暂停 apply/replay"
	} else {
		message = "已恢复 apply/replay"
	}
	response.SuccessWithMessage(c, message, result)
}

func (s *Service) ensureReplicaRecordPermission(c *gin.Context, primaryInstanceID, replicaInstanceID uint) bool {
	if primaryInstanceID == 0 && replicaInstanceID == 0 {
		response.ErrorCode(c, http.StatusBadRequest, "副本记录缺少关联实例")
		return false
	}
	scope, ok := s.databasePermissionScope(c, dbbiz.DatabasePermissionTopology)
	if !ok {
		return false
	}
	if !scope.enforced || scope.admin {
		return true
	}
	for _, allowedID := range scope.allowedIDs {
		if allowedID == primaryInstanceID || allowedID == replicaInstanceID {
			return true
		}
	}
	response.ErrorCode(c, http.StatusForbidden, "权限不足：无权操作该副本记录")
	return false
}

func (s *Service) GetInstanceReplicationStatus(c *gin.Context) {
	instanceID, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, instanceID, dbbiz.DatabasePermissionTopology) {
		return
	}
	result, err := s.useCase.GetInstanceReplicationStatus(c.Request.Context(), instanceID)
	if err != nil {
		writeDatabaseError(c, "查询失败: ", err)
		return
	}
	response.Success(c, result)
}

func (s *Service) CheckInstanceReplication(c *gin.Context) {
	instanceID, ok := parseUintParam(c, "id", "实例ID")
	if !ok {
		return
	}
	if !s.ensureInstancePermission(c, instanceID, dbbiz.DatabasePermissionTopology) {
		return
	}
	result, err := s.useCase.CheckInstanceReplication(c.Request.Context(), instanceID, dbbiz.QueryOperator{
		ID:       rbacservice.GetUserID(c),
		Username: rbacservice.GetUsername(c),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		writeDatabaseError(c, "采集失败: ", err)
		return
	}
	response.SuccessWithMessage(c, "副本状态采集完成", result)
}
