package asset

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ydcloud-dy/opshub/internal/biz/asset"
	rbacService "github.com/ydcloud-dy/opshub/internal/service/rbac"
	"github.com/ydcloud-dy/opshub/pkg/response"
)

type DesktopService struct {
	useCase *asset.DesktopSessionUseCase
}

func NewDesktopService(useCase *asset.DesktopSessionUseCase) *DesktopService {
	return &DesktopService{useCase: useCase}
}

func (s *DesktopService) CreateDesktopSession(c *gin.Context) {
	hostID64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "无效的主机ID")
		return
	}

	var req asset.DesktopLaunchRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	userID := rbacService.GetUserID(c)
	username := rbacService.GetUsername(c)

	resp, err := s.useCase.CreateLaunch(c.Request.Context(), uint(hostID64), userID, username, c.ClientIP(), &req)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, resp)
}

func (s *DesktopService) ListDesktopSessions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	keyword := strings.TrimSpace(c.Query("keyword"))
	status := strings.TrimSpace(c.Query("status"))
	scope := strings.TrimSpace(c.DefaultQuery("scope", "self"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	userID := rbacService.GetUserID(c)
	listUserID := userID
	if scope == "all" {
		listUserID = 0
	}

	list, total, err := s.useCase.List(c.Request.Context(), page, pageSize, keyword, status, listUserID)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "查询桌面会话失败: "+err.Error())
		return
	}

	response.Success(c, gin.H{
		"list":     list,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (s *DesktopService) DownloadDesktopSessionRecording(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "无效的会话ID")
		return
	}

	filePath, fileName, err := s.useCase.GetRecordingFile(c.Request.Context(), uint(id64))
	if err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist):
			response.ErrorCode(c, http.StatusNotFound, err.Error())
		default:
			response.ErrorCode(c, http.StatusBadRequest, err.Error())
		}
		return
	}

	c.FileAttachment(filePath, fileName)
}

func (s *DesktopService) PlayDesktopSessionRecording(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "无效的会话ID")
		return
	}

	filePath, _, err := s.useCase.GetRecordingFile(c.Request.Context(), uint(id64))
	if err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist):
			response.ErrorCode(c, http.StatusNotFound, err.Error())
		default:
			response.ErrorCode(c, http.StatusBadRequest, err.Error())
		}
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.File(filePath)
}

func (s *DesktopService) DeleteDesktopSession(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "无效的会话ID")
		return
	}

	if err := s.useCase.Delete(c.Request.Context(), uint(id64)); err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist):
			response.ErrorCode(c, http.StatusNotFound, err.Error())
		default:
			response.ErrorCode(c, http.StatusBadRequest, err.Error())
		}
		return
	}

	response.SuccessWithMessage(c, "桌面会话已删除", nil)
}

func (s *DesktopService) GetDesktopSession(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "无效的会话ID")
		return
	}

	userID := rbacService.GetUserID(c)
	info, err := s.useCase.GetByID(c.Request.Context(), uint(id64), userID)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, info)
}

func (s *DesktopService) CloseDesktopSession(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "无效的会话ID")
		return
	}

	userID := rbacService.GetUserID(c)
	if err := s.useCase.Close(c.Request.Context(), uint(id64), userID, "user_closed"); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, err.Error())
		return
	}

	response.SuccessWithMessage(c, "桌面会话已关闭", nil)
}

func (s *DesktopService) HeartbeatDesktopSession(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "无效的会话ID")
		return
	}

	userID := rbacService.GetUserID(c)
	if err := s.useCase.Heartbeat(c.Request.Context(), uint(id64), userID); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, nil)
}

func (s *DesktopService) UploadDesktopSessionFile(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "无效的会话ID")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "请选择要上传的文件")
		return
	}

	src, err := file.Open()
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "打开文件失败")
		return
	}
	defer src.Close()

	userID := rbacService.GetUserID(c)
	savedName, err := s.useCase.UploadFile(c.Request.Context(), uint(id64), userID, src, file.Filename)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, err.Error())
		return
	}

	response.SuccessWithMessage(c, "文件已上传到桌面映射盘根目录", gin.H{
		"name": savedName,
		"size": file.Size,
	})
}
