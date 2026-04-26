package server

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ydcloud-dy/opshub/plugins/monitor/model"
	"github.com/ydcloud-dy/opshub/plugins/monitor/service"
	"gorm.io/gorm"
)

type HostHandler struct {
	hostService  *service.HostMonitorService
	alertService *service.HostAlertService
}

func NewHostHandler(db *gorm.DB) *HostHandler {
	return &HostHandler{
		hostService:  service.NewHostMonitorService(db),
		alertService: service.NewHostAlertService(db),
	}
}

func (h *HostHandler) ListHosts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	data, err := h.hostService.ListHosts(c.Request.Context(), c.Query("keyword"), c.Query("health"), c.Query("osType"), page, pageSize)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "获取主机监控列表失败", "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "success", "data": data})
}

func (h *HostHandler) GetHostOverview(c *gin.Context) {
	hostID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	data, err := h.hostService.GetOverview(c.Request.Context(), hostID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "获取主机监控详情失败", "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "success", "data": data})
}

func (h *HostHandler) GetHostHistory(c *gin.Context) {
	hostID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	data, err := h.hostService.GetHistory(c.Request.Context(), hostID, c.DefaultQuery("range", "1h"))
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "获取主机历史趋势失败", "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "success", "data": data})
}

func (h *HostHandler) ListHostProcesses(c *gin.Context) {
	hostID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	data, err := h.hostService.GetProcesses(c.Request.Context(), hostID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "获取主机进程失败", "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "success", "data": data})
}

func (h *HostHandler) ListHostPorts(c *gin.Context) {
	hostID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	data, err := h.hostService.GetPorts(c.Request.Context(), hostID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "获取监听端口失败", "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "success", "data": data})
}

func (h *HostHandler) ListHostAlertRules(c *gin.Context) {
	data, err := h.alertService.ListRules(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "获取主机告警规则失败", "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "success", "data": data})
}

func (h *HostHandler) GetHostAlertRule(c *gin.Context) {
	ruleID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	data, err := h.alertService.GetRule(c.Request.Context(), ruleID)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "message": "主机告警规则不存在", "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "success", "data": data})
}

func (h *HostHandler) CreateHostAlertRule(c *gin.Context) {
	var req model.HostAlertRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "请求参数错误", "error": err.Error()})
		return
	}
	if err := h.alertService.CreateRule(c.Request.Context(), &req); err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "创建主机告警规则失败", "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "创建成功", "data": req})
}

func (h *HostHandler) UpdateHostAlertRule(c *gin.Context) {
	ruleID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	var req model.HostAlertRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "请求参数错误", "error": err.Error()})
		return
	}
	data, err := h.alertService.UpdateRule(c.Request.Context(), ruleID, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "更新主机告警规则失败", "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "更新成功", "data": data})
}

func (h *HostHandler) DeleteHostAlertRule(c *gin.Context) {
	ruleID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.alertService.DeleteRule(c.Request.Context(), ruleID); err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "删除主机告警规则失败", "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "删除成功"})
}

func (h *HostHandler) GetHostAlertRuleStats(c *gin.Context) {
	data, err := h.alertService.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "获取规则统计失败", "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "success", "data": data})
}

func (h *HostHandler) ListAssignableHosts(c *gin.Context) {
	data, err := h.alertService.ListAssignableHosts(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "获取主机列表失败", "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "success", "data": data})
}

func parseUintParam(c *gin.Context, key string) (uint, bool) {
	var id uint
	if _, err := fmt.Sscanf(c.Param(key), "%d", &id); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "无效的ID"})
		return 0, false
	}
	return id, true
}
