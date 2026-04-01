package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/deployer"
	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/model"
	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/repository"
	"gorm.io/gorm"
)

// DeployService 部署服务
type DeployService struct {
	db              *gorm.DB
	certRepo        *repository.CertificateRepository
	deployRepo      *repository.DeployConfigRepository
	taskRepo        *repository.RenewTaskRepository
	deployerFactory *deployer.Factory
	deployerDeps    *deployer.Dependencies
}

// NewDeployService 创建部署服务
func NewDeployService(db *gorm.DB, deployerDeps *deployer.Dependencies) *DeployService {
	return &DeployService{
		db:              db,
		certRepo:        repository.NewCertificateRepository(db),
		deployRepo:      repository.NewDeployConfigRepository(db),
		taskRepo:        repository.NewRenewTaskRepository(db),
		deployerFactory: deployer.NewFactory(),
		deployerDeps:    deployerDeps,
	}
}

// CreateDeployConfigRequest 创建部署配置请求
type CreateDeployConfigRequest struct {
	CertificateID uint        `json:"certificate_id"`
	Name          string      `json:"name"`
	DeployType    string      `json:"deploy_type"`
	TargetConfig  interface{} `json:"target_config"`
	AutoDeploy    bool        `json:"auto_deploy"`
	Enabled       bool        `json:"enabled"`
}

// CreateDeployConfig 创建部署配置
func (s *DeployService) CreateDeployConfig(ctx context.Context, req *CreateDeployConfigRequest) (*model.DeployConfig, error) {
	// 验证证书是否存在
	_, err := s.certRepo.GetByID(ctx, req.CertificateID)
	if err != nil {
		return nil, fmt.Errorf("certificate not found: %w", err)
	}

	// 序列化配置
	configJSON, err := json.Marshal(req.TargetConfig)
	if err != nil {
		return nil, fmt.Errorf("marshal target config failed: %w", err)
	}

	config := &model.DeployConfig{
		CertificateID: req.CertificateID,
		Name:          req.Name,
		DeployType:    req.DeployType,
		TargetConfig:  string(configJSON),
		AutoDeploy:    req.AutoDeploy,
		Enabled:       req.Enabled,
	}

	if err := s.deployRepo.Create(ctx, config); err != nil {
		return nil, fmt.Errorf("create deploy config failed: %w", err)
	}

	return config, nil
}

// GetDeployConfig 获取部署配置
func (s *DeployService) GetDeployConfig(ctx context.Context, id uint) (*model.DeployConfig, error) {
	return s.deployRepo.GetByIDWithCert(ctx, id)
}

// ListDeployConfigs 部署配置列表
func (s *DeployService) ListDeployConfigs(ctx context.Context, page, pageSize int, filters map[string]interface{}) ([]model.DeployConfig, int64, error) {
	return s.deployRepo.List(ctx, page, pageSize, filters)
}

// UpdateDeployConfig 更新部署配置
func (s *DeployService) UpdateDeployConfig(ctx context.Context, id uint, updates map[string]interface{}) error {
	config, err := s.deployRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if name, ok := updates["name"].(string); ok {
		config.Name = name
	}
	if autoDeploy, ok := updates["auto_deploy"].(bool); ok {
		config.AutoDeploy = autoDeploy
	}
	if enabled, ok := updates["enabled"].(bool); ok {
		config.Enabled = enabled
	}
	if targetConfig, ok := updates["target_config"]; ok {
		configJSON, err := json.Marshal(targetConfig)
		if err != nil {
			return fmt.Errorf("marshal target config failed: %w", err)
		}
		config.TargetConfig = string(configJSON)
	}

	return s.deployRepo.Update(ctx, config)
}

// DeleteDeployConfig 删除部署配置
func (s *DeployService) DeleteDeployConfig(ctx context.Context, id uint) error {
	return s.deployRepo.Delete(ctx, id)
}

// ExecuteDeploy 执行部署
func (s *DeployService) ExecuteDeploy(ctx context.Context, configID uint) error {
	config, err := s.deployRepo.GetByIDWithCert(ctx, configID)
	if err != nil {
		return fmt.Errorf("get deploy config failed: %w", err)
	}

	cert, err := s.certRepo.GetByID(ctx, config.CertificateID)
	if err != nil {
		return fmt.Errorf("get certificate failed: %w", err)
	}

	if cert.Certificate == "" {
		return fmt.Errorf("certificate content is empty")
	}

	// 创建部署任务记录
	now := time.Now()
	task := &model.RenewTask{
		CertificateID: config.CertificateID,
		TaskType:      model.TaskTypeDeploy,
		Status:        model.TaskStatusRunning,
		TriggerType:   model.TriggerTypeManual,
		StartedAt:     &now,
	}
	if err := s.taskRepo.Create(ctx, task); err != nil {
		return fmt.Errorf("create deploy task failed: %w", err)
	}

	bundle := &model.CertBundle{
		Certificate: cert.Certificate,
		PrivateKey:  cert.PrivateKey,
		CertChain:   cert.CertChain,
	}

	d, err := s.deployerFactory.Create(config.DeployType, s.deployerDeps)
	if err != nil {
		s.taskRepo.UpdateStatus(ctx, task.ID, model.TaskStatusFailed, fmt.Sprintf("create deployer failed: %v", err), "")
		return fmt.Errorf("create deployer failed: %w", err)
	}

	err = d.Deploy(ctx, bundle, config)
	finishedAt := time.Now()
	if err != nil {
		s.deployRepo.UpdateDeployResult(ctx, configID, false, &finishedAt, err.Error())
		s.taskRepo.UpdateStatus(ctx, task.ID, model.TaskStatusFailed, err.Error(), fmt.Sprintf(`{"deploy_config_id":%d,"deploy_config_name":"%s"}`, configID, config.Name))
		return fmt.Errorf("deploy failed: %w", err)
	}

	s.deployRepo.UpdateDeployResult(ctx, configID, true, &finishedAt, "")
	s.taskRepo.UpdateStatus(ctx, task.ID, model.TaskStatusSuccess, "", fmt.Sprintf(`{"deploy_config_id":%d,"deploy_config_name":"%s"}`, configID, config.Name))
	return nil
}

// TestDeployConfig 测试部署配置
func (s *DeployService) TestDeployConfig(ctx context.Context, configID uint) error {
	config, err := s.deployRepo.GetByID(ctx, configID)
	if err != nil {
		return fmt.Errorf("get deploy config failed: %w", err)
	}

	d, err := s.deployerFactory.Create(config.DeployType, s.deployerDeps)
	if err != nil {
		return fmt.Errorf("create deployer failed: %w", err)
	}

	return d.Test(ctx, config)
}
