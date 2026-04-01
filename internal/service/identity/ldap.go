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

package identity

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ydcloud-dy/opshub/internal/biz/identity"
	"github.com/ydcloud-dy/opshub/pkg/response"
)

// LDAPService LDAP服务
type LDAPService struct {
	useCase *identity.LDAPUseCase
}

// NewLDAPService 创建LDAP服务
func NewLDAPService(useCase *identity.LDAPUseCase) *LDAPService {
	return &LDAPService{useCase: useCase}
}

// TestConnection 测试LDAP连接
func (s *LDAPService) TestConnection(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "invalid source id")
		return
	}

	if err := s.useCase.TestConnection(c.Request.Context(), uint(id)); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "connection successful"})
}

// SyncUsers 同步用户
func (s *LDAPService) SyncUsers(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "invalid source id")
		return
	}

	job, err := s.useCase.SyncUsers(c.Request.Context(), uint(id))
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, job)
}

// GetSyncStatus 获取同步状态
func (s *LDAPService) GetSyncStatus(c *gin.Context) {
	jobIDStr := c.Param("jobId")
	jobID, err := strconv.ParseUint(jobIDStr, 10, 64)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "invalid job id")
		return
	}

	job, err := s.useCase.GetSyncStatus(c.Request.Context(), uint(jobID))
	if err != nil {
		response.ErrorCode(c, http.StatusNotFound, "sync job not found")
		return
	}

	response.Success(c, job)
}

// ListSyncJobs 列出同步任务
func (s *LDAPService) ListSyncJobs(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "invalid source id")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	jobs, total, err := s.useCase.ListSyncJobs(c.Request.Context(), uint(id), page, pageSize)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, gin.H{
		"items": jobs,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
}
