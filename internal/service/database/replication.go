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
