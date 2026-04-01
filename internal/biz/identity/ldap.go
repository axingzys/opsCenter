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
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/ydcloud-dy/opshub/internal/biz/rbac"
)

// LDAPConfig LDAP配置
type LDAPConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	UseTLS       bool   `json:"use_tls"`
	StartTLS     bool   `json:"start_tls"`
	SkipVerify   bool   `json:"skip_verify"`
	BindDN       string `json:"bind_dn"`
	BindPassword string `json:"bind_password"`
	BaseDN       string `json:"base_dn"`
	UserFilter   string `json:"user_filter"`   // e.g., "(uid=%s)" or "(sAMAccountName=%s)"
	GroupFilter  string `json:"group_filter"`  // e.g., "(objectClass=groupOfNames)"
	UserAttrs    struct {
		Username string `json:"username"` // uid, sAMAccountName
		Email    string `json:"email"`    // mail
		RealName string `json:"real_name"` // cn, displayName
		Phone    string `json:"phone"`    // telephoneNumber, mobile
		Avatar   string `json:"avatar"`   // jpegPhoto
	} `json:"user_attrs"`
	GroupAttrs struct {
		Name    string `json:"name"`    // cn
		Members string `json:"members"` // member, uniqueMember
	} `json:"group_attrs"`
	SyncInterval int  `json:"sync_interval"` // 同步间隔（分钟）
	AutoSync     bool `json:"auto_sync"`     // 是否自动同步
}

// LDAPUser LDAP用户信息
type LDAPUser struct {
	DN       string
	Username string
	Email    string
	RealName string
	Phone    string
	Avatar   string
	Groups   []string
}

// LDAPSyncJob LDAP同步任务
type LDAPSyncJob struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SourceID     uint      `gorm:"index;not null" json:"source_id"`
	Status       string    `gorm:"type:varchar(20);not null" json:"status"` // pending, running, completed, failed
	TotalUsers   int       `json:"total_users"`
	SyncedUsers  int       `json:"synced_users"`
	FailedUsers  int       `json:"failed_users"`
	ErrorMessage string    `gorm:"type:text" json:"error_message"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// LDAPSyncJobRepo LDAP同步任务仓库接口
type LDAPSyncJobRepo interface {
	Create(ctx context.Context, job *LDAPSyncJob) error
	Update(ctx context.Context, job *LDAPSyncJob) error
	GetByID(ctx context.Context, id uint) (*LDAPSyncJob, error)
	GetLatestBySourceID(ctx context.Context, sourceID uint) (*LDAPSyncJob, error)
	List(ctx context.Context, sourceID uint, page, pageSize int) ([]*LDAPSyncJob, int64, error)
}

// LDAPUseCase LDAP用例
type LDAPUseCase struct {
	sourceRepo  IdentitySourceRepo
	userRepo    rbac.UserRepo
	syncJobRepo LDAPSyncJobRepo
}

// NewLDAPUseCase 创建LDAP用例
func NewLDAPUseCase(
	sourceRepo IdentitySourceRepo,
	userRepo rbac.UserRepo,
	syncJobRepo LDAPSyncJobRepo,
) *LDAPUseCase {
	return &LDAPUseCase{
		sourceRepo:  sourceRepo,
		userRepo:    userRepo,
		syncJobRepo: syncJobRepo,
	}
}

// TestConnection 测试LDAP连接
func (uc *LDAPUseCase) TestConnection(ctx context.Context, sourceID uint) error {
	source, err := uc.sourceRepo.GetByID(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("identity source not found: %w", err)
	}

	if source.Type != "ldap" {
		return errors.New("identity source is not LDAP type")
	}

	config, err := uc.parseLDAPConfig(source.Config)
	if err != nil {
		return fmt.Errorf("invalid LDAP config: %w", err)
	}

	conn, err := uc.connect(config)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()

	// 尝试绑定
	if err := conn.Bind(config.BindDN, config.BindPassword); err != nil {
		return fmt.Errorf("bind failed: %w", err)
	}

	return nil
}

// Authenticate 通过LDAP认证用户
func (uc *LDAPUseCase) Authenticate(ctx context.Context, sourceID uint, username, password string) (*LDAPUser, error) {
	source, err := uc.sourceRepo.GetByID(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("identity source not found: %w", err)
	}

	if source.Type != "ldap" {
		return nil, errors.New("identity source is not LDAP type")
	}

	config, err := uc.parseLDAPConfig(source.Config)
	if err != nil {
		return nil, fmt.Errorf("invalid LDAP config: %w", err)
	}

	conn, err := uc.connect(config)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()

	// 使用管理员绑定
	if err := conn.Bind(config.BindDN, config.BindPassword); err != nil {
		return nil, fmt.Errorf("admin bind failed: %w", err)
	}

	// 搜索用户
	userFilter := strings.Replace(config.UserFilter, "%s", ldap.EscapeFilter(username), -1)
	searchReq := ldap.NewSearchRequest(
		config.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1, 0, false,
		userFilter,
		uc.getUserAttributes(config),
		nil,
	)

	result, err := conn.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("user search failed: %w", err)
	}

	if len(result.Entries) == 0 {
		return nil, errors.New("user not found")
	}

	entry := result.Entries[0]
	userDN := entry.DN

	// 使用用户DN和密码重新绑定验证
	if err := conn.Bind(userDN, password); err != nil {
		return nil, errors.New("invalid credentials")
	}

	// 解析用户信息
	ldapUser := uc.parseUserEntry(entry, config)
	return ldapUser, nil
}

// SyncUsers 同步LDAP用户
func (uc *LDAPUseCase) SyncUsers(ctx context.Context, sourceID uint) (*LDAPSyncJob, error) {
	source, err := uc.sourceRepo.GetByID(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("identity source not found: %w", err)
	}

	if source.Type != "ldap" {
		return nil, errors.New("identity source is not LDAP type")
	}

	config, err := uc.parseLDAPConfig(source.Config)
	if err != nil {
		return nil, fmt.Errorf("invalid LDAP config: %w", err)
	}

	// 创建同步任务
	job := &LDAPSyncJob{
		SourceID:  sourceID,
		Status:    "running",
		StartedAt: time.Now(),
		CreatedAt: time.Now(),
	}

	if err := uc.syncJobRepo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to create sync job: %w", err)
	}

	// 异步执行同步
	go uc.runSync(context.Background(), job, source, config)

	return job, nil
}

// GetSyncStatus 获取同步状态
func (uc *LDAPUseCase) GetSyncStatus(ctx context.Context, jobID uint) (*LDAPSyncJob, error) {
	return uc.syncJobRepo.GetByID(ctx, jobID)
}

// ListSyncJobs 列出同步任务
func (uc *LDAPUseCase) ListSyncJobs(ctx context.Context, sourceID uint, page, pageSize int) ([]*LDAPSyncJob, int64, error) {
	return uc.syncJobRepo.List(ctx, sourceID, page, pageSize)
}

// runSync 执行同步
func (uc *LDAPUseCase) runSync(ctx context.Context, job *LDAPSyncJob, source *IdentitySource, config *LDAPConfig) {
	defer func() {
		job.CompletedAt = time.Now()
		if job.Status == "running" {
			job.Status = "completed"
		}
		uc.syncJobRepo.Update(ctx, job)
	}()

	conn, err := uc.connect(config)
	if err != nil {
		job.Status = "failed"
		job.ErrorMessage = fmt.Sprintf("connection failed: %v", err)
		return
	}
	defer conn.Close()

	if err := conn.Bind(config.BindDN, config.BindPassword); err != nil {
		job.Status = "failed"
		job.ErrorMessage = fmt.Sprintf("bind failed: %v", err)
		return
	}

	// 搜索所有用户
	userFilter := config.UserFilter
	if strings.Contains(userFilter, "%s") {
		userFilter = strings.Replace(userFilter, "%s", "*", -1)
	}

	searchReq := ldap.NewSearchRequest(
		config.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		userFilter,
		uc.getUserAttributes(config),
		nil,
	)

	result, err := conn.Search(searchReq)
	if err != nil {
		job.Status = "failed"
		job.ErrorMessage = fmt.Sprintf("search failed: %v", err)
		return
	}

	job.TotalUsers = len(result.Entries)

	for _, entry := range result.Entries {
		ldapUser := uc.parseUserEntry(entry, config)
		if ldapUser.Username == "" {
			job.FailedUsers++
			continue
		}

		// 检查用户是否存在
		existingUser, _ := uc.userRepo.GetByUsername(ctx, ldapUser.Username)
		if existingUser != nil {
			// 更新用户信息
			existingUser.Email = ldapUser.Email
			existingUser.RealName = ldapUser.RealName
			existingUser.Phone = ldapUser.Phone
			if ldapUser.Avatar != "" {
				existingUser.Avatar = ldapUser.Avatar
			}
			if err := uc.userRepo.Update(ctx, existingUser); err != nil {
				job.FailedUsers++
				continue
			}
		} else {
			// 创建新用户
			newUser := &rbac.SysUser{
				Username: ldapUser.Username,
				Email:    ldapUser.Email,
				RealName: ldapUser.RealName,
				Phone:    ldapUser.Phone,
				Avatar:   ldapUser.Avatar,
				Status:   1,
			}
			if err := uc.userRepo.Create(ctx, newUser); err != nil {
				job.FailedUsers++
				continue
			}
		}

		job.SyncedUsers++
	}
}

// connect 连接LDAP服务器
func (uc *LDAPUseCase) connect(config *LDAPConfig) (*ldap.Conn, error) {
	address := fmt.Sprintf("%s:%d", config.Host, config.Port)

	var conn *ldap.Conn
	var err error

	if config.UseTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: config.SkipVerify,
		}
		conn, err = ldap.DialTLS("tcp", address, tlsConfig)
	} else {
		conn, err = ldap.Dial("tcp", address)
		if err == nil && config.StartTLS {
			tlsConfig := &tls.Config{
				InsecureSkipVerify: config.SkipVerify,
			}
			err = conn.StartTLS(tlsConfig)
		}
	}

	if err != nil {
		return nil, err
	}

	return conn, nil
}

// parseLDAPConfig 解析LDAP配置
func (uc *LDAPUseCase) parseLDAPConfig(configStr string) (*LDAPConfig, error) {
	var config LDAPConfig
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		return nil, err
	}

	// 设置默认值
	if config.Port == 0 {
		if config.UseTLS {
			config.Port = 636
		} else {
			config.Port = 389
		}
	}

	if config.UserFilter == "" {
		config.UserFilter = "(uid=%s)"
	}

	if config.UserAttrs.Username == "" {
		config.UserAttrs.Username = "uid"
	}
	if config.UserAttrs.Email == "" {
		config.UserAttrs.Email = "mail"
	}
	if config.UserAttrs.RealName == "" {
		config.UserAttrs.RealName = "cn"
	}
	if config.UserAttrs.Phone == "" {
		config.UserAttrs.Phone = "telephoneNumber"
	}

	return &config, nil
}

// getUserAttributes 获取用户属性列表
func (uc *LDAPUseCase) getUserAttributes(config *LDAPConfig) []string {
	attrs := []string{
		config.UserAttrs.Username,
		config.UserAttrs.Email,
		config.UserAttrs.RealName,
		config.UserAttrs.Phone,
	}
	if config.UserAttrs.Avatar != "" {
		attrs = append(attrs, config.UserAttrs.Avatar)
	}
	return attrs
}

// parseUserEntry 解析LDAP用户条目
func (uc *LDAPUseCase) parseUserEntry(entry *ldap.Entry, config *LDAPConfig) *LDAPUser {
	return &LDAPUser{
		DN:       entry.DN,
		Username: entry.GetAttributeValue(config.UserAttrs.Username),
		Email:    entry.GetAttributeValue(config.UserAttrs.Email),
		RealName: entry.GetAttributeValue(config.UserAttrs.RealName),
		Phone:    entry.GetAttributeValue(config.UserAttrs.Phone),
		Avatar:   entry.GetAttributeValue(config.UserAttrs.Avatar),
	}
}

// AuthenticateWithEmail 通过邮箱认证（用于登录时自动匹配LDAP源）
func (uc *LDAPUseCase) AuthenticateWithEmail(ctx context.Context, email, password string) (*LDAPUser, *IdentitySource, error) {
	// 获取所有启用的LDAP源
	enabled := true
	sources, _, err := uc.sourceRepo.List(ctx, 1, 100, "", &enabled)
	if err != nil {
		return nil, nil, err
	}

	for _, source := range sources {
		if source.Type != "ldap" || !source.Enabled {
			continue
		}

		config, err := uc.parseLDAPConfig(source.Config)
		if err != nil {
			continue
		}

		// 尝试使用邮箱搜索用户
		conn, err := uc.connect(config)
		if err != nil {
			continue
		}

		if err := conn.Bind(config.BindDN, config.BindPassword); err != nil {
			conn.Close()
			continue
		}

		// 搜索用户（使用邮箱）
		emailFilter := fmt.Sprintf("(%s=%s)", config.UserAttrs.Email, ldap.EscapeFilter(email))
		searchReq := ldap.NewSearchRequest(
			config.BaseDN,
			ldap.ScopeWholeSubtree,
			ldap.NeverDerefAliases,
			1, 0, false,
			emailFilter,
			uc.getUserAttributes(config),
			nil,
		)

		result, err := conn.Search(searchReq)
		if err != nil || len(result.Entries) == 0 {
			conn.Close()
			continue
		}

		entry := result.Entries[0]
		userDN := entry.DN

		// 验证密码
		if err := conn.Bind(userDN, password); err != nil {
			conn.Close()
			continue
		}

		conn.Close()

		ldapUser := uc.parseUserEntry(entry, config)
		return ldapUser, source, nil
	}

	return nil, nil, errors.New("LDAP authentication failed")
}
