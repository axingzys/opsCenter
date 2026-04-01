package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/service"
)

// DeployHandler 部署配置处理器
type DeployHandler struct {
	svc *service.DeployService
}

// NewDeployHandler 创建部署配置处理器
func NewDeployHandler(svc *service.DeployService) *DeployHandler {
	return &DeployHandler{svc: svc}
}

// List 部署配置列表
func (h *DeployHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	filters := make(map[string]interface{})
	if certID := c.Query("certificate_id"); certID != "" {
		if id, err := strconv.ParseUint(certID, 10, 64); err == nil {
			filters["certificate_id"] = uint(id)
		}
	}
	if deployType := c.Query("deploy_type"); deployType != "" {
		filters["deploy_type"] = deployType
	}

	configs, total, err := h.svc.ListDeployConfigs(c.Request.Context(), page, pageSize, filters)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"list":      configs,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// Get 获取部署配置详情
func (h *DeployHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid id"})
		return
	}

	config, err := h.svc.GetDeployConfig(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    config,
	})
}

// Create 创建部署配置
func (h *DeployHandler) Create(c *gin.Context) {
	var req service.CreateDeployConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": err.Error()})
		return
	}

	config, err := h.svc.CreateDeployConfig(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    config,
	})
}

// Update 更新部署配置
func (h *DeployHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid id"})
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.svc.UpdateDeployConfig(c.Request.Context(), uint(id), updates); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// Delete 删除部署配置
func (h *DeployHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid id"})
		return
	}

	if err := h.svc.DeleteDeployConfig(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// Deploy 执行部署
func (h *DeployHandler) Deploy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid id"})
		return
	}

	if err := h.svc.ExecuteDeploy(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// Test 测试部署配置
func (h *DeployHandler) Test(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid id"})
		return
	}

	if err := h.svc.TestDeployConfig(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}
