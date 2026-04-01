package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/service"
)

// CertificateHandler 证书处理器
type CertificateHandler struct {
	svc *service.CertificateService
}

// NewCertificateHandler 创建证书处理器
func NewCertificateHandler(svc *service.CertificateService) *CertificateHandler {
	return &CertificateHandler{svc: svc}
}

// List 证书列表
func (h *CertificateHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	filters := make(map[string]interface{})
	if domain := c.Query("domain"); domain != "" {
		filters["domain"] = domain
	}
	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}
	if sourceType := c.Query("source_type"); sourceType != "" {
		filters["source_type"] = sourceType
	}

	certs, total, err := h.svc.ListCertificates(c.Request.Context(), page, pageSize, filters)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"list":      certs,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// Get 获取证书详情
func (h *CertificateHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid id"})
		return
	}

	cert, err := h.svc.GetCertificate(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    cert,
	})
}

// Create 创建证书(申请)
func (h *CertificateHandler) Create(c *gin.Context) {
	var req service.CreateCertificateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": err.Error()})
		return
	}

	cert, err := h.svc.CreateCertificate(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    cert,
	})
}

// Import 导入证书
func (h *CertificateHandler) Import(c *gin.Context) {
	var req service.ImportCertificateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": err.Error()})
		return
	}

	cert, err := h.svc.ImportCertificate(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    cert,
	})
}

// Update 更新证书配置
func (h *CertificateHandler) Update(c *gin.Context) {
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

	if err := h.svc.UpdateCertificate(c.Request.Context(), uint(id), updates); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// Delete 删除证书
func (h *CertificateHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid id"})
		return
	}

	if err := h.svc.DeleteCertificate(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// Renew 手动续期证书
func (h *CertificateHandler) Renew(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid id"})
		return
	}

	task, err := h.svc.RenewCertificate(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    task,
	})
}

// Download 下载证书
func (h *CertificateHandler) Download(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid id"})
		return
	}

	format := c.DefaultQuery("format", "pem") // pem, nginx, apache

	bundle, err := h.svc.GetCertificateContent(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	switch format {
	case "pem":
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"certificate": bundle.Certificate,
				"private_key": bundle.PrivateKey,
				"cert_chain":  bundle.CertChain,
			},
		})
	case "nginx":
		// Nginx格式：证书和链合并
		fullChain := bundle.Certificate
		if bundle.CertChain != "" {
			fullChain += "\n" + bundle.CertChain
		}
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"ssl_certificate":     fullChain,
				"ssl_certificate_key": bundle.PrivateKey,
			},
		})
	default:
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "unsupported format"})
	}
}

// Stats 证书统计
func (h *CertificateHandler) Stats(c *gin.Context) {
	stats, err := h.svc.GetCertificateStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// CloudAccounts 获取云账号列表(用于证书申请)
func (h *CertificateHandler) CloudAccounts(c *gin.Context) {
	provider := c.Query("provider") // 可选: aliyun

	accounts, err := h.svc.GetCloudAccounts(c.Request.Context(), provider)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    accounts,
	})
}

// Sync 同步云证书状态
func (h *CertificateHandler) Sync(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid id"})
		return
	}

	err = h.svc.SyncCloudCertificate(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "证书同步成功",
	})
}
