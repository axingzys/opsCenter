package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/model"
	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/provider/dns"
	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/repository"
	"gorm.io/gorm"
)

// DNSProviderService DNS服务商服务
type DNSProviderService struct {
	db         *gorm.DB
	repo       *repository.DNSProviderRepository
	dnsFactory *dns.Factory
}

// NewDNSProviderService 创建DNS服务商服务
func NewDNSProviderService(db *gorm.DB) *DNSProviderService {
	return &DNSProviderService{
		db:         db,
		repo:       repository.NewDNSProviderRepository(db),
		dnsFactory: dns.NewFactory(),
	}
}

// CreateDNSProviderRequest 创建DNS服务商请求
type CreateDNSProviderRequest struct {
	Name     string      `json:"name"`
	Provider string      `json:"provider"`
	Config   interface{} `json:"config"`
	Email    string      `json:"email"`
	Phone    string      `json:"phone"`
	Enabled  bool        `json:"enabled"`
}

// CreateDNSProvider 创建DNS服务商
func (s *DNSProviderService) CreateDNSProvider(ctx context.Context, req *CreateDNSProviderRequest) (*model.DNSProvider, error) {
	// 序列化配置
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return nil, fmt.Errorf("marshal config failed: %w", err)
	}

	provider := &model.DNSProvider{
		Name:     req.Name,
		Provider: req.Provider,
		Config:   string(configJSON),
		Email:    req.Email,
		Phone:    req.Phone,
		Enabled:  req.Enabled,
	}

	if err := s.repo.Create(ctx, provider); err != nil {
		return nil, fmt.Errorf("create dns provider failed: %w", err)
	}

	return provider, nil
}

// GetDNSProvider 获取DNS服务商
func (s *DNSProviderService) GetDNSProvider(ctx context.Context, id uint) (*model.DNSProvider, error) {
	return s.repo.GetByID(ctx, id)
}

// ListDNSProviders DNS服务商列表
func (s *DNSProviderService) ListDNSProviders(ctx context.Context, page, pageSize int, filters map[string]interface{}) ([]model.DNSProvider, int64, error) {
	return s.repo.List(ctx, page, pageSize, filters)
}

// ListAllDNSProviders 获取所有启用的DNS服务商
func (s *DNSProviderService) ListAllDNSProviders(ctx context.Context) ([]model.DNSProvider, error) {
	return s.repo.ListAll(ctx)
}

// UpdateDNSProvider 更新DNS服务商
func (s *DNSProviderService) UpdateDNSProvider(ctx context.Context, id uint, updates map[string]interface{}) error {
	provider, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if name, ok := updates["name"].(string); ok {
		provider.Name = name
	}
	if enabled, ok := updates["enabled"].(bool); ok {
		provider.Enabled = enabled
	}
	if email, ok := updates["email"].(string); ok {
		provider.Email = email
	}
	if phone, ok := updates["phone"].(string); ok {
		provider.Phone = phone
	}
	if config, ok := updates["config"]; ok && config != nil {
		// 只有当config不为空时才更新
		configJSON, err := json.Marshal(config)
		if err != nil {
			return fmt.Errorf("marshal config failed: %w", err)
		}
		if string(configJSON) != "{}" && string(configJSON) != "null" {
			provider.Config = string(configJSON)
		}
	}

	return s.repo.Update(ctx, provider)
}

// DeleteDNSProvider 删除DNS服务商
func (s *DNSProviderService) DeleteDNSProvider(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

// TestDNSProvider 测试DNS服务商连接
func (s *DNSProviderService) TestDNSProvider(ctx context.Context, id uint) error {
	provider, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get dns provider failed: %w", err)
	}

	dnsP, err := s.dnsFactory.Create(provider)
	if err != nil {
		return fmt.Errorf("create dns provider failed: %w", err)
	}

	err = dnsP.TestConnection(ctx)
	now := time.Now()
	if err != nil {
		s.repo.UpdateTestResult(ctx, id, false, &now)
		return fmt.Errorf("test connection failed: %w", err)
	}

	s.repo.UpdateTestResult(ctx, id, true, &now)
	return nil
}

// GetDNSProviderConfig 获取DNS服务商配置(脱敏)
func (s *DNSProviderService) GetDNSProviderConfig(ctx context.Context, id uint) (map[string]interface{}, error) {
	provider, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
		return nil, fmt.Errorf("unmarshal config failed: %w", err)
	}

	// 脱敏处理
	for key := range config {
		if isSecretKey(key) {
			if val, ok := config[key].(string); ok && len(val) > 4 {
				config[key] = val[:4] + "****"
			}
		}
	}

	return config, nil
}

// DNSProviderDetailResponse DNS服务商详情响应(包含配置)
type DNSProviderDetailResponse struct {
	ID         uint                   `json:"id"`
	Name       string                 `json:"name"`
	Provider   string                 `json:"provider"`
	Config     map[string]interface{} `json:"config"`
	Email      string                 `json:"email"`
	Phone      string                 `json:"phone"`
	Enabled    bool                   `json:"enabled"`
	LastTestAt *time.Time             `json:"last_test_at"`
	LastTestOK bool                   `json:"last_test_ok"`
}

// GetDNSProviderDetail 获取DNS服务商详情(包含完整配置,用于编辑)
func (s *DNSProviderService) GetDNSProviderDetail(ctx context.Context, id uint) (*DNSProviderDetailResponse, error) {
	provider, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
		return nil, fmt.Errorf("unmarshal config failed: %w", err)
	}

	return &DNSProviderDetailResponse{
		ID:         provider.ID,
		Name:       provider.Name,
		Provider:   provider.Provider,
		Config:     config,
		Email:      provider.Email,
		Phone:      provider.Phone,
		Enabled:    provider.Enabled,
		LastTestAt: provider.LastTestAt,
		LastTestOK: provider.LastTestOK,
	}, nil
}

// isSecretKey 判断是否是敏感字段
func isSecretKey(key string) bool {
	secretKeys := []string{
		"secret", "password", "key", "token", "credential",
		"access_key_secret", "secret_key", "api_key", "secret_access_key",
	}
	for _, sk := range secretKeys {
		if key == sk || containsSubstring(key, sk) {
			return true
		}
	}
	return false
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
