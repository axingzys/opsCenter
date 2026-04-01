package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/service"
)

// TaskHandler 任务处理器
type TaskHandler struct {
	svc *service.TaskService
}

// NewTaskHandler 创建任务处理器
func NewTaskHandler(svc *service.TaskService) *TaskHandler {
	return &TaskHandler{svc: svc}
}

// List 任务列表
func (h *TaskHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	filters := make(map[string]interface{})
	if certID := c.Query("certificate_id"); certID != "" {
		if id, err := strconv.ParseUint(certID, 10, 64); err == nil {
			filters["certificate_id"] = uint(id)
		}
	}
	if taskType := c.Query("task_type"); taskType != "" {
		filters["task_type"] = taskType
	}
	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}
	if triggerType := c.Query("trigger_type"); triggerType != "" {
		filters["trigger_type"] = triggerType
	}

	tasks, total, err := h.svc.ListTasks(c.Request.Context(), page, pageSize, filters)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"list":      tasks,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// Get 获取任务详情
func (h *TaskHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid id"})
		return
	}

	task, err := h.svc.GetTask(c.Request.Context(), uint(id))
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
