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
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	assetbiz "github.com/ydcloud-dy/opshub/internal/biz/asset"
	rbacService "github.com/ydcloud-dy/opshub/internal/service/rbac"
	"github.com/ydcloud-dy/opshub/pkg/response"
)

type VirtualizationService struct {
	useCase *assetbiz.VirtualizationUseCase
}

func NewVirtualizationService(useCase *assetbiz.VirtualizationUseCase) *VirtualizationService {
	return &VirtualizationService{useCase: useCase}
}

func (s *VirtualizationService) StartAutoSync(ctx context.Context, opts assetbiz.VirtualizationAutoSyncOptions) *assetbiz.VirtualizationAutoSyncScheduler {
	if s == nil || s.useCase == nil {
		return nil
	}
	return s.useCase.StartAutoSyncScheduler(ctx, opts)
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

func writeVirtualizationGuestOperationError(c *gin.Context, prefix string, err error) {
	statusCode := http.StatusInternalServerError
	message := err.Error()
	switch {
	case strings.Contains(message, "不存在"):
		statusCode = http.StatusNotFound
	case strings.Contains(message, "二次确认"),
		strings.Contains(message, "操作原因"),
		strings.Contains(message, "快照名称"),
		strings.Contains(message, "不能执行"),
		strings.Contains(message, "不支持"),
		strings.Contains(message, "已经是"),
		strings.Contains(message, "已禁用"),
		strings.Contains(message, "未开启虚拟化写操作"),
		strings.Contains(message, "不能为空"),
		strings.Contains(message, "缺少"),
		strings.Contains(message, "无法识别"),
		strings.Contains(message, "上下文不完整"):
		statusCode = http.StatusBadRequest
	}
	response.ErrorCode(c, statusCode, prefix+message)
}

// CreatePlatform 创建虚拟化平台
// @Summary 创建虚拟化平台
// @Description 创建 ESXi/PVE 平台连接配置
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body assetbiz.VirtualizationPlatformRequest true "平台信息"
// @Success 200 {object} response.Response "创建成功"
// @Router /api/v1/virtualization/platforms [post]
func (s *VirtualizationService) CreatePlatform(c *gin.Context) {
	var req assetbiz.VirtualizationPlatformRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	item, err := s.useCase.CreatePlatform(c.Request.Context(), &req)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}
	response.Success(c, item)
}

// UpdatePlatform 更新虚拟化平台
// @Summary 更新虚拟化平台
// @Description 更新虚拟化平台连接配置
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "平台ID"
// @Param body body assetbiz.VirtualizationPlatformRequest true "平台信息"
// @Success 200 {object} response.Response "更新成功"
// @Router /api/v1/virtualization/platforms/{id} [put]
func (s *VirtualizationService) UpdatePlatform(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "平台ID")
	if !ok {
		return
	}

	var req assetbiz.VirtualizationPlatformRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	req.ID = id

	if err := s.useCase.UpdatePlatform(c.Request.Context(), &req); err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}
	response.SuccessWithMessage(c, "更新成功", nil)
}

// DeletePlatform 删除虚拟化平台
// @Summary 删除虚拟化平台
// @Description 删除指定虚拟化平台
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "平台ID"
// @Success 200 {object} response.Response "删除成功"
// @Router /api/v1/virtualization/platforms/{id} [delete]
func (s *VirtualizationService) DeletePlatform(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "平台ID")
	if !ok {
		return
	}

	if err := s.useCase.DeletePlatform(c.Request.Context(), id); err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "删除失败: "+err.Error())
		return
	}
	response.SuccessWithMessage(c, "删除成功", nil)
}

// GetPlatform 获取虚拟化平台详情
// @Summary 获取虚拟化平台详情
// @Description 获取单个虚拟化平台详情
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "平台ID"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/virtualization/platforms/{id} [get]
func (s *VirtualizationService) GetPlatform(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "平台ID")
	if !ok {
		return
	}

	item, err := s.useCase.GetPlatformByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorCode(c, http.StatusNotFound, "平台不存在")
		return
	}
	response.Success(c, item)
}

// ListPlatforms 获取虚拟化平台列表
// @Summary 获取虚拟化平台列表
// @Description 分页获取虚拟化平台列表
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(10)
// @Param keyword query string false "搜索关键字"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/virtualization/platforms [get]
func (s *VirtualizationService) ListPlatforms(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	keyword := c.Query("keyword")

	list, total, err := s.useCase.ListPlatforms(c.Request.Context(), page, pageSize, keyword)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{
		"list":     list,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// GetSettings 获取虚拟化纳管策略配置
// @Summary 获取虚拟化纳管策略配置
// @Description 获取当前冲突策略（严格/宽松）
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/virtualization/settings [get]
func (s *VirtualizationService) GetSettings(c *gin.Context) {
	data, err := s.useCase.GetSettings(c.Request.Context())
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "获取配置失败: "+err.Error())
		return
	}
	response.Success(c, data)
}

// UpdateSettings 更新虚拟化纳管策略配置
// @Summary 更新虚拟化纳管策略配置
// @Description 更新冲突策略（严格/宽松）
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body assetbiz.VirtualizationSettingsRequest true "虚拟化配置"
// @Success 200 {object} response.Response "更新成功"
// @Router /api/v1/virtualization/settings [put]
func (s *VirtualizationService) UpdateSettings(c *gin.Context) {
	var req assetbiz.VirtualizationSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	data, err := s.useCase.UpdateSettings(c.Request.Context(), &req)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "更新配置失败: "+err.Error())
		return
	}
	response.Success(c, data)
}

// TestPlatform 测试虚拟化平台连通性
// @Summary 测试虚拟化平台连通性
// @Description 测试平台凭据与 API 连通性
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "平台ID"
// @Success 200 {object} response.Response "测试完成"
// @Router /api/v1/virtualization/platforms/{id}/test [post]
func (s *VirtualizationService) TestPlatform(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "平台ID")
	if !ok {
		return
	}

	if err := s.useCase.TestPlatformConnection(c.Request.Context(), id); err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "连接测试失败: "+err.Error())
		return
	}
	response.SuccessWithMessage(c, "连接测试成功", nil)
}

// SyncPlatform 手动触发平台同步
// @Summary 触发虚拟化平台同步
// @Description 手动触发一次平台同步
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "平台ID"
// @Success 200 {object} response.Response "触发成功"
// @Router /api/v1/virtualization/platforms/{id}/sync [post]
func (s *VirtualizationService) SyncPlatform(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "平台ID")
	if !ok {
		return
	}
	operatorID := rbacService.GetUserID(c)

	job, err := s.useCase.TriggerPlatformSync(c.Request.Context(), id, operatorID)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "触发同步失败: "+err.Error())
		return
	}
	response.Success(c, job)
}

// ListSyncJobs 获取同步任务列表
// @Summary 获取虚拟化平台同步任务列表
// @Description 查询指定平台的同步任务
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "平台ID"
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(10)
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/virtualization/platforms/{id}/sync-jobs [get]
func (s *VirtualizationService) ListSyncJobs(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "平台ID")
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	list, total, err := s.useCase.ListPlatformSyncJobs(c.Request.Context(), id, page, pageSize)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "查询同步任务失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{
		"list":     list,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// GetTopology 获取虚拟化拓扑
// @Summary 获取虚拟化拓扑
// @Description 获取平台/集群/宿主机拓扑
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param platformId query int false "平台ID"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/virtualization/topology [get]
func (s *VirtualizationService) GetTopology(c *gin.Context) {
	var platformID uint
	if idRaw := c.Query("platformId"); idRaw != "" {
		id, err := strconv.ParseUint(idRaw, 10, 32)
		if err != nil {
			response.ErrorCode(c, http.StatusBadRequest, "无效的平台ID")
			return
		}
		platformID = uint(id)
	}

	data, err := s.useCase.GetTopology(c.Request.Context(), platformID)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "获取拓扑失败: "+err.Error())
		return
	}
	response.Success(c, data)
}

// GetPlatformTrend 获取平台虚机状态趋势
// @Summary 获取平台虚机状态趋势
// @Description 获取按平台维度的虚机状态变化趋势
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "平台ID"
// @Param range query string false "时间范围 24h/7d/15d"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/virtualization/platforms/{id}/trend [get]
func (s *VirtualizationService) GetPlatformTrend(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "平台ID")
	if !ok {
		return
	}

	data, err := s.useCase.GetPlatformTrend(c.Request.Context(), id, c.DefaultQuery("range", "7d"))
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "获取趋势失败: "+err.Error())
		return
	}
	response.Success(c, data)
}

// GetClusterTrend 获取集群虚机状态趋势
// @Summary 获取集群虚机状态趋势
// @Description 获取按集群维度的虚机状态变化趋势
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "集群ID"
// @Param range query string false "时间范围 24h/7d/15d"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/virtualization/clusters/{id}/trend [get]
func (s *VirtualizationService) GetClusterTrend(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "集群ID")
	if !ok {
		return
	}

	data, err := s.useCase.GetClusterTrend(c.Request.Context(), id, c.DefaultQuery("range", "7d"))
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "获取趋势失败: "+err.Error())
		return
	}
	response.Success(c, data)
}

// ListGuests 获取虚机列表
// @Summary 获取虚机列表
// @Description 分页查询虚机列表
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(10)
// @Param keyword query string false "搜索关键字"
// @Param platformId query int false "平台ID"
// @Param clusterId query int false "集群ID"
// @Param powerState query string false "电源状态"
// @Param bound query string false "纳管状态 all/bound/unbound"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/virtualization/guests [get]
func (s *VirtualizationService) ListGuests(c *gin.Context) {
	var req assetbiz.VirtualizationGuestListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	list, total, err := s.useCase.ListGuests(c.Request.Context(), &req)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "查询虚机失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{
		"list":     list,
		"total":    total,
		"page":     req.Page,
		"pageSize": req.PageSize,
	})
}

// GetGuest 获取虚机详情
// @Summary 获取虚机详情
// @Description 获取指定虚机详情
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "虚机ID"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/virtualization/guests/{id} [get]
func (s *VirtualizationService) GetGuest(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "虚机ID")
	if !ok {
		return
	}

	data, err := s.useCase.GetGuestByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorCode(c, http.StatusNotFound, err.Error())
		return
	}
	response.Success(c, data)
}

// PrecheckGuestOnboard 纳管预检查
// @Summary 虚机纳管预检查
// @Description 纳管前检查IP/同名/历史绑定冲突，并给出建议主机
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "虚机ID"
// @Param assetHostId query int false "目标资产主机ID"
// @Success 200 {object} response.Response "检查完成"
// @Router /api/v1/virtualization/guests/{id}/precheck [get]
func (s *VirtualizationService) PrecheckGuestOnboard(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "虚机ID")
	if !ok {
		return
	}

	var assetHostID uint
	if raw := c.Query("assetHostId"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			response.ErrorCode(c, http.StatusBadRequest, "无效的目标主机ID")
			return
		}
		assetHostID = uint(parsed)
	}

	data, err := s.useCase.PrecheckGuestBinding(c.Request.Context(), id, assetHostID)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "预检查失败: "+err.Error())
		return
	}
	response.Success(c, data)
}

// BindGuest 纳管虚机
// @Summary 绑定虚机到资产主机
// @Description 将虚机纳管绑定到资产主机
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "虚机ID"
// @Param body body assetbiz.VirtualizationGuestBindingRequest true "绑定参数"
// @Success 200 {object} response.Response "绑定成功"
// @Router /api/v1/virtualization/guests/{id}/onboard [post]
func (s *VirtualizationService) BindGuest(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "虚机ID")
	if !ok {
		return
	}
	operatorID := rbacService.GetUserID(c)

	var req assetbiz.VirtualizationGuestBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	data, err := s.useCase.BindGuest(c.Request.Context(), id, &req, operatorID)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "纳管失败: "+err.Error())
		return
	}
	response.Success(c, data)
}

// UnbindGuest 解绑虚机纳管关系
// @Summary 解绑虚机纳管关系
// @Description 解除虚机与资产主机的绑定关系
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "虚机ID"
// @Param body body assetbiz.VirtualizationGuestUnbindRequest false "解绑参数"
// @Success 200 {object} response.Response "解绑成功"
// @Router /api/v1/virtualization/guests/{id}/unbind [post]
func (s *VirtualizationService) UnbindGuest(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "虚机ID")
	if !ok {
		return
	}

	var req assetbiz.VirtualizationGuestUnbindRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errorsIsEOF(err) {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	if err := s.useCase.UnbindGuest(c.Request.Context(), id, &req); err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "解绑失败: "+err.Error())
		return
	}
	response.SuccessWithMessage(c, "解绑成功", nil)
}

// PowerGuest 执行虚机开机/关机/重启
// @Summary 执行虚机电源操作
// @Description 对指定虚机执行开机、关机、重启，并记录操作审计
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "虚机ID"
// @Param body body assetbiz.VirtualizationGuestPowerRequest true "电源操作参数"
// @Success 200 {object} response.Response "执行成功"
// @Router /api/v1/virtualization/guests/{id}/power [post]
func (s *VirtualizationService) PowerGuest(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "虚机ID")
	if !ok {
		return
	}

	var req assetbiz.VirtualizationGuestPowerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	data, err := s.useCase.PowerGuest(c.Request.Context(), id, &req, rbacService.GetUserID(c), rbacService.GetUsername(c))
	if err != nil {
		writeVirtualizationGuestOperationError(c, "虚机电源操作失败: ", err)
		return
	}
	response.Success(c, data)
}

// ListGuestSnapshots 获取虚机快照列表
// @Summary 获取虚机快照列表
// @Description 查询指定虚机在虚拟化平台上的快照列表
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "虚机ID"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/virtualization/guests/{id}/snapshots [get]
func (s *VirtualizationService) ListGuestSnapshots(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "虚机ID")
	if !ok {
		return
	}

	data, err := s.useCase.ListGuestSnapshots(c.Request.Context(), id)
	if err != nil {
		writeVirtualizationGuestOperationError(c, "获取快照列表失败: ", err)
		return
	}
	response.Success(c, gin.H{"list": data})
}

// CreateGuestConsoleLink 创建虚机控制台跳转链接
// @Summary 创建虚机控制台跳转链接
// @Description 生成指定虚机的控制台跳转链接并写入审计日志
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "虚机ID"
// @Success 200 {object} response.Response "生成成功"
// @Router /api/v1/virtualization/guests/{id}/console-link [post]
func (s *VirtualizationService) CreateGuestConsoleLink(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "虚机ID")
	if !ok {
		return
	}

	data, err := s.useCase.CreateGuestConsoleLink(c.Request.Context(), id, rbacService.GetUserID(c), rbacService.GetUsername(c))
	if err != nil {
		writeVirtualizationGuestOperationError(c, "生成控制台链接失败: ", err)
		return
	}
	response.Success(c, data)
}

// CreateGuestSnapshot 创建虚机快照
// @Summary 创建虚机快照
// @Description 在虚拟化平台侧创建虚机快照并写入审计日志
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "虚机ID"
// @Param body body assetbiz.VirtualizationGuestSnapshotCreateRequest true "快照创建参数"
// @Success 200 {object} response.Response "创建成功"
// @Router /api/v1/virtualization/guests/{id}/snapshots [post]
func (s *VirtualizationService) CreateGuestSnapshot(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "虚机ID")
	if !ok {
		return
	}

	var req assetbiz.VirtualizationGuestSnapshotCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	if err := s.useCase.CreateGuestSnapshot(c.Request.Context(), id, &req, rbacService.GetUserID(c), rbacService.GetUsername(c)); err != nil {
		writeVirtualizationGuestOperationError(c, "创建快照失败: ", err)
		return
	}
	response.SuccessWithMessage(c, "创建快照成功", nil)
}

// RollbackGuestSnapshot 回滚虚机快照
// @Summary 回滚虚机快照
// @Description 回滚到指定快照并写入审计日志
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "虚机ID"
// @Param body body assetbiz.VirtualizationGuestSnapshotActionRequest true "快照回滚参数"
// @Success 200 {object} response.Response "回滚成功"
// @Router /api/v1/virtualization/guests/{id}/snapshots/rollback [post]
func (s *VirtualizationService) RollbackGuestSnapshot(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "虚机ID")
	if !ok {
		return
	}

	var req assetbiz.VirtualizationGuestSnapshotActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	if err := s.useCase.RollbackGuestSnapshot(c.Request.Context(), id, &req, rbacService.GetUserID(c), rbacService.GetUsername(c)); err != nil {
		writeVirtualizationGuestOperationError(c, "快照回滚失败: ", err)
		return
	}
	response.SuccessWithMessage(c, "快照回滚成功", nil)
}

// DeleteGuestSnapshot 删除虚机快照
// @Summary 删除虚机快照
// @Description 删除指定快照并写入审计日志
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "虚机ID"
// @Param body body assetbiz.VirtualizationGuestSnapshotActionRequest true "快照删除参数"
// @Success 200 {object} response.Response "删除成功"
// @Router /api/v1/virtualization/guests/{id}/snapshots/delete [post]
func (s *VirtualizationService) DeleteGuestSnapshot(c *gin.Context) {
	id, ok := parseUintParam(c, "id", "虚机ID")
	if !ok {
		return
	}

	var req assetbiz.VirtualizationGuestSnapshotActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	if err := s.useCase.DeleteGuestSnapshot(c.Request.Context(), id, &req, rbacService.GetUserID(c), rbacService.GetUsername(c)); err != nil {
		writeVirtualizationGuestOperationError(c, "删除快照失败: ", err)
		return
	}
	response.SuccessWithMessage(c, "删除快照成功", nil)
}

// ListActionLogs 获取虚拟化操作审计日志
// @Summary 获取虚拟化操作审计日志
// @Description 查询虚拟化模块的电源与快照操作审计
// @Tags 资产管理-虚拟化平台
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(10)
// @Param platformId query int false "平台ID"
// @Param guestId query int false "虚机ID"
// @Param action query string false "动作"
// @Param status query string false "状态"
// @Param keyword query string false "关键字"
// @Success 200 {object} response.Response "获取成功"
// @Router /api/v1/virtualization/action-logs [get]
func (s *VirtualizationService) ListActionLogs(c *gin.Context) {
	var req assetbiz.VirtualizationActionLogListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	list, total, err := s.useCase.ListActionLogs(c.Request.Context(), &req)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "查询操作审计失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{
		"list":     list,
		"total":    total,
		"page":     req.Page,
		"pageSize": req.PageSize,
	})
}

func errorsIsEOF(err error) bool {
	return err == io.EOF
}
