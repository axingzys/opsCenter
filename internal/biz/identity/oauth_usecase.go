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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ydcloud-dy/opshub/internal/biz/rbac"
)

// OAuthUseCase OAuth登录用例
type OAuthUseCase struct {
	sourceRepo  IdentitySourceRepo
	bindingRepo UserOAuthBindingRepo
	stateRepo   OAuthStateRepo
	authLogRepo AuthLogRepo
	userRepo    rbac.UserRepo
	roleRepo    rbac.RoleRepo
}

// NewOAuthUseCase 创建OAuth登录用例
func NewOAuthUseCase(
	sourceRepo IdentitySourceRepo,
	bindingRepo UserOAuthBindingRepo,
	stateRepo OAuthStateRepo,
	authLogRepo AuthLogRepo,
	userRepo rbac.UserRepo,
	roleRepo rbac.RoleRepo,
) *OAuthUseCase {
	return &OAuthUseCase{
		sourceRepo:  sourceRepo,
		bindingRepo: bindingRepo,
		stateRepo:   stateRepo,
		authLogRepo: authLogRepo,
		userRepo:    userRepo,
		roleRepo:    roleRepo,
	}
}

// InitiateOAuth 发起OAuth认证，返回授权URL
func (uc *OAuthUseCase) InitiateOAuth(ctx context.Context, providerType, redirectURL, action string, userID uint) (string, error) {
	// 获取身份源配置
	source, err := uc.sourceRepo.GetByType(ctx, providerType)
	if err != nil {
		return "", fmt.Errorf("identity source not found: %w", err)
	}

	if !source.Enabled {
		return "", errors.New("identity source is disabled")
	}

	// 解析配置
	var config IdentitySourceConfig
	if err := json.Unmarshal([]byte(source.Config), &config); err != nil {
		return "", fmt.Errorf("invalid source config: %w", err)
	}

	// 生成state
	state, err := generateState()
	if err != nil {
		return "", fmt.Errorf("generate state failed: %w", err)
	}

	// 保存state
	oauthState := &OAuthState{
		State:       state,
		Provider:    providerType,
		RedirectURL: redirectURL,
		Action:      action,
		UserID:      userID,
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	}
	if err := uc.stateRepo.Create(ctx, oauthState); err != nil {
		return "", fmt.Errorf("save state failed: %w", err)
	}

	// 获取提供商
	provider, err := GetProvider(providerType)
	if err != nil {
		return "", err
	}

	// 生成授权URL
	authURL := provider.GetAuthURL(state, &config)
	return authURL, nil
}

// HandleCallback 处理OAuth回调
func (uc *OAuthUseCase) HandleCallback(ctx context.Context, providerType, code, state, ip, userAgent string) (*OAuthLoginResult, error) {
	// 验证state
	oauthState, err := uc.stateRepo.GetByState(ctx, state)
	if err != nil {
		return nil, errors.New("invalid or expired state")
	}

	// 检查是否过期
	if time.Now().After(oauthState.ExpiresAt) {
		uc.stateRepo.Delete(ctx, state)
		return nil, errors.New("state expired")
	}

	// 验证provider匹配
	if oauthState.Provider != providerType {
		return nil, errors.New("provider mismatch")
	}

	// 删除已使用的state
	defer uc.stateRepo.Delete(ctx, state)

	// 获取身份源配置
	source, err := uc.sourceRepo.GetByType(ctx, providerType)
	if err != nil {
		return nil, fmt.Errorf("identity source not found: %w", err)
	}

	var config IdentitySourceConfig
	if err := json.Unmarshal([]byte(source.Config), &config); err != nil {
		return nil, fmt.Errorf("invalid source config: %w", err)
	}

	// 获取提供商
	provider, err := GetProvider(providerType)
	if err != nil {
		return nil, err
	}

	// 交换token
	token, err := provider.ExchangeToken(ctx, code, &config)
	if err != nil {
		uc.logAuthEvent(ctx, 0, "", "login", providerType, ip, userAgent, "failed", err.Error())
		return nil, fmt.Errorf("exchange token failed: %w", err)
	}

	// 获取用户信息
	userInfo, err := provider.GetUserInfo(ctx, token, &config)
	if err != nil {
		uc.logAuthEvent(ctx, 0, "", "login", providerType, ip, userAgent, "failed", err.Error())
		return nil, fmt.Errorf("get user info failed: %w", err)
	}

	// 根据action处理
	if oauthState.Action == "bind" {
		// 绑定模式
		return uc.handleBind(ctx, oauthState.UserID, source, userInfo, ip, userAgent)
	}

	// 登录模式
	return uc.handleLogin(ctx, source, userInfo, ip, userAgent)
}

// handleLogin 处理OAuth登录
func (uc *OAuthUseCase) handleLogin(ctx context.Context, source *IdentitySource, userInfo *OAuthUserInfo, ip, userAgent string) (*OAuthLoginResult, error) {
	// 查找是否已绑定
	binding, err := uc.bindingRepo.GetByOpenID(ctx, source.ID, userInfo.OpenID)
	if err == nil && binding != nil {
		// 已绑定，直接登录
		user, err := uc.userRepo.GetByID(ctx, binding.UserID)
		if err != nil {
			return nil, fmt.Errorf("user not found: %w", err)
		}

		// 检查用户状态
		if user.Status != 1 {
			uc.logAuthEvent(ctx, user.ID, user.Username, "login", source.Type, ip, userAgent, "failed", "user disabled")
			return nil, errors.New("user is disabled")
		}

		// 更新绑定信息
		binding.Nickname = userInfo.Nickname
		binding.Avatar = userInfo.Avatar
		if userInfo.ExtraInfo != nil {
			extraBytes, _ := json.Marshal(userInfo.ExtraInfo)
			binding.ExtraInfo = string(extraBytes)
		}
		uc.bindingRepo.Update(ctx, binding)

		// 更新最后登录时间
		uc.userRepo.UpdateLastLogin(ctx, user.ID)

		// 记录登录日志
		uc.logAuthEvent(ctx, user.ID, user.Username, "login", source.Type, ip, userAgent, "success", "")

		return &OAuthLoginResult{
			IsNewUser: false,
			NeedBind:  false,
			UserID:    user.ID,
		}, nil
	}

	// 尝试通过UnionID查找（跨应用绑定）
	if userInfo.UnionID != "" {
		binding, err = uc.bindingRepo.GetByUnionID(ctx, source.Type, userInfo.UnionID)
		if err == nil && binding != nil {
			user, err := uc.userRepo.GetByID(ctx, binding.UserID)
			if err != nil {
				return nil, fmt.Errorf("user not found: %w", err)
			}

			if user.Status != 1 {
				uc.logAuthEvent(ctx, user.ID, user.Username, "login", source.Type, ip, userAgent, "failed", "user disabled")
				return nil, errors.New("user is disabled")
			}

			// 创建新的绑定记录（同一用户不同应用）
			newBinding := &UserOAuthBinding{
				UserID:     user.ID,
				SourceID:   source.ID,
				SourceType: source.Type,
				OpenID:     userInfo.OpenID,
				UnionID:    userInfo.UnionID,
				Nickname:   userInfo.Nickname,
				Avatar:     userInfo.Avatar,
			}
			if userInfo.ExtraInfo != nil {
				extraBytes, _ := json.Marshal(userInfo.ExtraInfo)
				newBinding.ExtraInfo = string(extraBytes)
			}
			uc.bindingRepo.Create(ctx, newBinding)

			uc.userRepo.UpdateLastLogin(ctx, user.ID)
			uc.logAuthEvent(ctx, user.ID, user.Username, "login", source.Type, ip, userAgent, "success", "")

			return &OAuthLoginResult{
				IsNewUser: false,
				NeedBind:  false,
				UserID:    user.ID,
			}, nil
		}
	}

	// 未绑定，检查是否自动创建用户
	if source.AutoCreateUser {
		user, err := uc.createUserFromOAuth(ctx, source, userInfo)
		if err != nil {
			uc.logAuthEvent(ctx, 0, "", "login", source.Type, ip, userAgent, "failed", err.Error())
			return nil, fmt.Errorf("create user failed: %w", err)
		}

		// 创建绑定
		binding := &UserOAuthBinding{
			UserID:     user.ID,
			SourceID:   source.ID,
			SourceType: source.Type,
			OpenID:     userInfo.OpenID,
			UnionID:    userInfo.UnionID,
			Nickname:   userInfo.Nickname,
			Avatar:     userInfo.Avatar,
		}
		if userInfo.ExtraInfo != nil {
			extraBytes, _ := json.Marshal(userInfo.ExtraInfo)
			binding.ExtraInfo = string(extraBytes)
		}
		if err := uc.bindingRepo.Create(ctx, binding); err != nil {
			return nil, fmt.Errorf("create binding failed: %w", err)
		}

		uc.userRepo.UpdateLastLogin(ctx, user.ID)
		uc.logAuthEvent(ctx, user.ID, user.Username, "login", source.Type, ip, userAgent, "success", "")

		return &OAuthLoginResult{
			IsNewUser: true,
			NeedBind:  false,
			UserID:    user.ID,
		}, nil
	}

	// 需要绑定现有账号
	bindToken, _ := generateState()
	return &OAuthLoginResult{
		IsNewUser: false,
		NeedBind:  true,
		BindToken: bindToken,
		OAuthInfo: userInfo,
	}, nil
}

// handleBind 处理账号绑定
func (uc *OAuthUseCase) handleBind(ctx context.Context, userID uint, source *IdentitySource, userInfo *OAuthUserInfo, ip, userAgent string) (*OAuthLoginResult, error) {
	// 检查是否已被其他用户绑定
	existingBinding, err := uc.bindingRepo.GetByOpenID(ctx, source.ID, userInfo.OpenID)
	if err == nil && existingBinding != nil && existingBinding.UserID != userID {
		return nil, errors.New("this account has been bindedto another user")
	}

	// 获取用户
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// 创建或更新绑定
	if existingBinding != nil && existingBinding.UserID == userID {
		// 更新
		existingBinding.Nickname = userInfo.Nickname
		existingBinding.Avatar = userInfo.Avatar
		if userInfo.ExtraInfo != nil {
			extraBytes, _ := json.Marshal(userInfo.ExtraInfo)
			existingBinding.ExtraInfo = string(extraBytes)
		}
		if err := uc.bindingRepo.Update(ctx, existingBinding); err != nil {
			return nil, fmt.Errorf("update binding failed: %w", err)
		}
	} else {
		// 创建
		binding := &UserOAuthBinding{
			UserID:     userID,
			SourceID:   source.ID,
			SourceType: source.Type,
			OpenID:     userInfo.OpenID,
			UnionID:    userInfo.UnionID,
			Nickname:   userInfo.Nickname,
			Avatar:     userInfo.Avatar,
		}
		if userInfo.ExtraInfo != nil {
			extraBytes, _ := json.Marshal(userInfo.ExtraInfo)
			binding.ExtraInfo = string(extraBytes)
		}
		if err := uc.bindingRepo.Create(ctx, binding); err != nil {
			return nil, fmt.Errorf("create binding failed: %w", err)
		}
	}

	uc.logAuthEvent(ctx, user.ID, user.Username, "bind", source.Type, ip, userAgent, "success", "")

	return &OAuthLoginResult{
		IsNewUser: false,
		NeedBind:  false,
		UserID:    userID,
	}, nil
}

// UnbindOAuth 解绑OAuth账号
func (uc *OAuthUseCase) UnbindOAuth(ctx context.Context, userID, sourceID uint) error {
	return uc.bindingRepo.DeleteByUserAndSource(ctx, userID, sourceID)
}

// GetUserBindings 获取用户的OAuth绑定列表
func (uc *OAuthUseCase) GetUserBindings(ctx context.Context, userID uint) ([]*UserOAuthBinding, error) {
	return uc.bindingRepo.ListByUser(ctx, userID)
}

// createUserFromOAuth 从OAuth信息创建用户
func (uc *OAuthUseCase) createUserFromOAuth(ctx context.Context, source *IdentitySource, userInfo *OAuthUserInfo) (*rbac.SysUser, error) {
	// 生成用户名
	username := fmt.Sprintf("%s_%s", source.Type, userInfo.OpenID)
	if len(username) > 50 {
		username = username[:50]
	}

	// 检查用户名是否已存在
	existingUser, _ := uc.userRepo.GetByUsername(ctx, username)
	if existingUser != nil {
		// 添加随机后缀
		suffix, _ := generateRandomString(4)
		username = fmt.Sprintf("%s_%s", username[:min(45, len(username))], suffix)
	}

	// 生成随机密码
	randomPassword, _ := generateRandomString(16)

	user := &rbac.SysUser{
		Username: username,
		Password: randomPassword,
		RealName: userInfo.Nickname,
		Email:    userInfo.Email,
		Phone:    userInfo.Phone,
		Avatar:   userInfo.Avatar,
		Status:   1,
	}

	if err := uc.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// 分配默认角色
	if source.DefaultRoleID > 0 {
		if err := uc.userRepo.AssignRoles(ctx, user.ID, []uint{source.DefaultRoleID}); err != nil {
			// 角色分配失败不影响用户创建
			fmt.Printf("assign default role failed: %v\n", err)
		}
	}

	return user, nil
}

// logAuthEvent 记录认证事件
func (uc *OAuthUseCase) logAuthEvent(ctx context.Context, userID uint, username, action, loginType, ip, userAgent, result, failReason string) {
	log := &AuthLog{
		UserID:     userID,
		Username:   username,
		Action:     action,
		LoginType:  loginType,
		IP:         ip,
		UserAgent:  userAgent,
		Result:     result,
		FailReason: failReason,
		CreatedAt:  time.Now(),
	}
	uc.authLogRepo.Create(ctx, log)
}

// generateState 生成随机state
func generateState() (string, error) {
	return generateRandomString(32)
}

// generateRandomString 生成随机字符串
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:length], nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
