package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/service"
)

// DNSProviderHandler DNS服务商处理器
type DNSProviderHandler struct {
	svc *service.DNSProviderService
}

// NewDNSProviderHandler 创建DNS服务商处理器
func NewDNSProviderHandler(svc *service.DNSProviderService) *DNSProviderHandler {
	return &DNSProviderHandler{svc: svc}
}

// List DNS服务商列表
func (h *DNSProviderHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	filters := make(map[string]interface{})
	if name := c.Query("name"); name != "" {
		filters["name"] = name
	}
	if provider := c.Query("provider"); provider != "" {
		filters["provider"] = provider
	}

	providers, total, err := h.svc.ListDNSProviders(c.Request.Context(), page, pageSize, filters)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"list":      providers,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// ListAll 获取所有启用的DNS服务商
func (h *DNSProviderHandler) ListAll(c *gin.Context) {
	providers, err := h.svc.ListAllDNSProviders(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    providers,
	})
}

// Get 获取DNS服务商详情
func (h *DNSProviderHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid id"})
		return
	}

	provider, err := h.svc.GetDNSProvider(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	// 获取脱敏配置
	config, _ := h.svc.GetDNSProviderConfig(c.Request.Context(), uint(id))

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id":           provider.ID,
			"name":         provider.Name,
			"provider":     provider.Provider,
			"config":       config,
			"email":        provider.Email,
			"phone":        provider.Phone,
			"enabled":      provider.Enabled,
			"last_test_at": provider.LastTestAt,
			"last_test_ok": provider.LastTestOK,
			"created_at":   provider.CreatedAt,
			"updated_at":   provider.UpdatedAt,
		},
	})
}

// GetDetail 获取DNS服务商完整详情(包含完整配置,用于编辑)
func (h *DNSProviderHandler) GetDetail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid id"})
		return
	}

	detail, err := h.svc.GetDNSProviderDetail(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    detail,
	})
}

// Create 创建DNS服务商
func (h *DNSProviderHandler) Create(c *gin.Context) {
	var req service.CreateDNSProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": err.Error()})
		return
	}

	provider, err := h.svc.CreateDNSProvider(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    provider,
	})
}

// Update 更新DNS服务商
func (h *DNSProviderHandler) Update(c *gin.Context) {
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

	if err := h.svc.UpdateDNSProvider(c.Request.Context(), uint(id), updates); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// Delete 删除DNS服务商
func (h *DNSProviderHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid id"})
		return
	}

	if err := h.svc.DeleteDNSProvider(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// Test 测试DNS服务商连接
func (h *DNSProviderHandler) Test(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid id"})
		return
	}

	if err := h.svc.TestDNSProvider(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}
