package service

import (
	"context"

	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/model"
	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/repository"
	"gorm.io/gorm"
)

// TaskService 任务服务
type TaskService struct {
	db   *gorm.DB
	repo *repository.RenewTaskRepository
}

// NewTaskService 创建任务服务
func NewTaskService(db *gorm.DB) *TaskService {
	return &TaskService{
		db:   db,
		repo: repository.NewRenewTaskRepository(db),
	}
}

// GetTask 获取任务详情
func (s *TaskService) GetTask(ctx context.Context, id uint) (*model.RenewTask, error) {
	return s.repo.GetByIDWithCert(ctx, id)
}

// ListTasks 任务列表
func (s *TaskService) ListTasks(ctx context.Context, page, pageSize int, filters map[string]interface{}) ([]model.RenewTask, int64, error) {
	return s.repo.List(ctx, page, pageSize, filters)
}

// ListTasksByCertificate 根据证书获取任务列表
func (s *TaskService) ListTasksByCertificate(ctx context.Context, certID uint, limit int) ([]model.RenewTask, error) {
	return s.repo.ListByCertificateID(ctx, certID, limit)
}

// GetLatestTask 获取证书最新任务
func (s *TaskService) GetLatestTask(ctx context.Context, certID uint, taskType string) (*model.RenewTask, error) {
	return s.repo.GetLatestByCertificateID(ctx, certID, taskType)
}
