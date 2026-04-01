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

package rbac

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"
	"github.com/ydcloud-dy/opshub/internal/biz/rbac"
	"github.com/ydcloud-dy/opshub/internal/biz/system"
	appLogger "github.com/ydcloud-dy/opshub/pkg/logger"
	"go.uber.org/zap"
	"math/rand"
)

// LDAPAuthResult LDAP认证结果
type LDAPAuthResult struct {
	Username string
	Email    string
	RealName string
	Phone    string
}

// LDAPAuthService LDAP认证服务
type LDAPAuthService struct {
	configUseCase *system.ConfigUseCase
	userUseCase   *rbac.UserUseCase
}

// NewLDAPAuthService 创建LDAP认证服务
func NewLDAPAuthService(configUseCase *system.ConfigUseCase, userUseCase *rbac.UserUseCase) *LDAPAuthService {
	return &LDAPAuthService{
		configUseCase: configUseCase,
		userUseCase:   userUseCase,
	}
}

// IsEnabled 检查LDAP是否启用
func (s *LDAPAuthService) IsEnabled(ctx context.Context) bool {
	return s.configUseCase.IsLDAPEnabled(ctx)
}

// Authenticate 通过LDAP认证用户并返回本地用户
// 如果用户不存在且配置了自动创建，会自动创建本地用户
func (s *LDAPAuthService) Authenticate(ctx context.Context, username, password string) (*rbac.SysUser, error) {
	config, err := s.configUseCase.GetLDAPConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取LDAP配置失败: %w", err)
	}

	if !config.Enabled {
		return nil, fmt.Errorf("LDAP未启用")
	}

	// 连接LDAP服务器
	conn, err := s.connect(config)
	if err != nil {
		appLogger.Error("LDAP连接失败", zap.Error(err))
		return nil, fmt.Errorf("LDAP服务器连接失败: %w", err)
	}
	defer conn.Close()

	// 使用管理员绑定
	if err := conn.Bind(config.BindDN, config.BindPassword); err != nil {
		appLogger.Error("LDAP管理员绑定失败", zap.Error(err))
		return nil, fmt.Errorf("LDAP管理员绑定失败: %w", err)
	}

	// 搜索用户
	userFilter := strings.Replace(config.UserFilter, "%s", ldap.EscapeFilter(username), -1)
	attrs := []string{
		"dn",
		config.AttrUsername,
		config.AttrEmail,
		config.AttrRealName,
		config.AttrPhone,
	}

	searchReq := ldap.NewSearchRequest(
		config.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1, 0, false,
		userFilter,
		attrs,
		nil,
	)

	result, err := conn.Search(searchReq)
	if err != nil {
		appLogger.Error("LDAP用户搜索失败", zap.String("filter", userFilter), zap.Error(err))
		return nil, fmt.Errorf("LDAP用户搜索失败: %w", err)
	}

	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("LDAP中未找到用户: %s", username)
	}

	entry := result.Entries[0]
	userDN := entry.DN

	// 使用用户DN和密码验证
	if err := conn.Bind(userDN, password); err != nil {
		appLogger.Info("LDAP用户密码验证失败", zap.String("username", username))
		return nil, fmt.Errorf("用户名或密码错误")
	}

	// 解析LDAP用户信息
	ldapResult := &LDAPAuthResult{
		Username: entry.GetAttributeValue(config.AttrUsername),
		Email:    entry.GetAttributeValue(config.AttrEmail),
		RealName: entry.GetAttributeValue(config.AttrRealName),
		Phone:    entry.GetAttributeValue(config.AttrPhone),
	}

	if ldapResult.Username == "" {
		ldapResult.Username = username
	}

	appLogger.Info("LDAP认证成功",
		zap.String("username", ldapResult.Username),
		zap.String("email", ldapResult.Email),
		zap.String("realName", ldapResult.RealName),
	)

	// 查找或创建本地用户
	return s.findOrCreateUser(ctx, ldapResult, config)
}

// findOrCreateUser 查找或创建本地用户
func (s *LDAPAuthService) findOrCreateUser(ctx context.Context, ldapResult *LDAPAuthResult, config *system.LDAPConfig) (*rbac.SysUser, error) {
	// 查找已存在的用户
	user, err := s.userUseCase.GetByUsername(ctx, ldapResult.Username)
	if err == nil && user != nil {
		// 用户已存在，更新信息
		updated := false
		if ldapResult.Email != "" && user.Email != ldapResult.Email {
			user.Email = ldapResult.Email
			updated = true
		}
		if ldapResult.RealName != "" && user.RealName != ldapResult.RealName {
			user.RealName = ldapResult.RealName
			updated = true
		}
		if ldapResult.Phone != "" && user.Phone != ldapResult.Phone {
			user.Phone = ldapResult.Phone
			updated = true
		}
		if user.Source != rbac.UserSourceLDAP {
			user.Source = rbac.UserSourceLDAP
			updated = true
		}

		if updated {
			if err := s.userUseCase.Update(ctx, user); err != nil {
				appLogger.Warn("更新LDAP用户信息失败", zap.String("username", user.Username), zap.Error(err))
			}
		}

		// 检查用户是否有角色，如果没有则分配默认角色
		if config.DefaultRoleID > 0 && len(user.Roles) == 0 {
			appLogger.Info("LDAP用户无角色，补充分配默认角色",
				zap.String("username", user.Username),
				zap.Uint("roleId", config.DefaultRoleID),
			)
			if err := s.userUseCase.AssignRoles(ctx, user.ID, []uint{config.DefaultRoleID}); err != nil {
				appLogger.Error("补充分配默认角色失败", zap.String("username", user.Username), zap.Error(err))
			}
			// 重新查询以加载角色
			user, _ = s.userUseCase.GetByUsername(ctx, user.Username)
		}

		return user, nil
	}

	// 用户不存在，检查是否需要自动创建
	if !config.AutoCreateUser {
		return nil, fmt.Errorf("LDAP认证成功，但系统中不存在该用户且未开启自动创建")
	}

	// 生成随机密码（LDAP用户不使用本地密码登录，由 UserUseCase.Create 内部bcrypt加密）
	randomPassword := generateRandomPassword(32)

	newUser := &rbac.SysUser{
		Username:     ldapResult.Username,
		Password:     randomPassword,
		RealName:     ldapResult.RealName,
		Email:        ldapResult.Email,
		Phone:        ldapResult.Phone,
		Status:       1,
		Source:       rbac.UserSourceLDAP,
		DepartmentID: config.DefaultDeptID,
	}

	if err := s.userUseCase.Create(ctx, newUser); err != nil {
		return nil, fmt.Errorf("创建LDAP用户失败: %w", err)
	}

	// 分配默认角色
	appLogger.Info("LDAP用户默认角色配置",
		zap.String("username", newUser.Username),
		zap.Uint("userId", newUser.ID),
		zap.Uint("defaultRoleId", config.DefaultRoleID),
	)
	if config.DefaultRoleID > 0 {
		if err := s.userUseCase.AssignRoles(ctx, newUser.ID, []uint{config.DefaultRoleID}); err != nil {
			appLogger.Error("为LDAP用户分配默认角色失败",
				zap.String("username", newUser.Username),
				zap.Uint("userId", newUser.ID),
				zap.Uint("roleId", config.DefaultRoleID),
				zap.Error(err),
			)
		} else {
			appLogger.Info("为LDAP用户分配默认角色成功",
				zap.String("username", newUser.Username),
				zap.Uint("roleId", config.DefaultRoleID),
			)
		}
	} else {
		appLogger.Warn("LDAP默认角色ID为0，跳过角色分配", zap.String("username", newUser.Username))
	}

	appLogger.Info("自动创建LDAP用户成功",
		zap.String("username", newUser.Username),
		zap.Uint("userId", newUser.ID),
	)

	// 重新查询以加载关联数据
	createdUser, err := s.userUseCase.GetByUsername(ctx, newUser.Username)
	if err != nil {
		return newUser, nil
	}
	return createdUser, nil
}

// connect 连接LDAP服务器
func (s *LDAPAuthService) connect(config *system.LDAPConfig) (*ldap.Conn, error) {
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

// generateRandomPassword 生成随机密码
func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
