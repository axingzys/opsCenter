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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// FeishuProvider 飞书OAuth提供商
type FeishuProvider struct{}

const (
	feishuAuthURL     = "https://passport.feishu.cn/suite/passport/oauth/authorize"
	feishuTokenURL    = "https://passport.feishu.cn/suite/passport/oauth/token"
	feishuUserInfoURL = "https://passport.feishu.cn/suite/passport/oauth/userinfo"
)

// GetProviderType 获取提供商类型
func (p *FeishuProvider) GetProviderType() string {
	return "feishu"
}

// GetAuthURL 获取飞书授权URL
func (p *FeishuProvider) GetAuthURL(state string, config *IdentitySourceConfig) string {
	authURL := config.AuthURL
	if authURL == "" {
		authURL = feishuAuthURL
	}

	params := url.Values{}
	params.Set("client_id", config.ClientID)
	params.Set("redirect_uri", config.RedirectURI)
	params.Set("response_type", "code")
	params.Set("state", state)

	return authURL + "?" + params.Encode()
}

// ExchangeToken 用授权码换取Token
func (p *FeishuProvider) ExchangeToken(ctx context.Context, code string, config *IdentitySourceConfig) (*OAuthToken, error) {
	tokenURL := config.TokenURL
	if tokenURL == "" {
		tokenURL = feishuTokenURL
	}

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", config.ClientID)
	data.Set("client_secret", config.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", config.RedirectURI)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request token failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("feishu oauth error: %s - %s", result.Error, result.ErrorDesc)
	}

	return &OAuthToken{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    result.TokenType,
		ExpiresIn:    result.ExpiresIn,
		ExpiresAt:    time.Now().Add(time.Duration(result.ExpiresIn) * time.Second),
	}, nil
}

// GetUserInfo 获取飞书用户信息
func (p *FeishuProvider) GetUserInfo(ctx context.Context, token *OAuthToken, config *IdentitySourceConfig) (*OAuthUserInfo, error) {
	userInfoURL := config.UserInfoURL
	if userInfoURL == "" {
		userInfoURL = feishuUserInfoURL
	}

	req, err := http.NewRequestWithContext(ctx, "GET", userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request user info failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	var user struct {
		Sub           string `json:"sub"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		OpenID        string `json:"open_id"`
		UnionID       string `json:"union_id"`
		EnName        string `json:"en_name"`
		TenantKey     string `json:"tenant_key"`
		AvatarURL     string `json:"avatar_url"`
		AvatarThumb   string `json:"avatar_thumb"`
		AvatarMiddle  string `json:"avatar_middle"`
		AvatarBig     string `json:"avatar_big"`
		Email         string `json:"email"`
		UserID        string `json:"user_id"`
		EmployeeNo    string `json:"employee_no"`
		Mobile        string `json:"mobile"`
	}

	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	openID := user.OpenID
	if openID == "" {
		openID = user.Sub
	}

	avatar := user.AvatarURL
	if avatar == "" {
		avatar = user.Picture
	}

	return &OAuthUserInfo{
		OpenID:   openID,
		UnionID:  user.UnionID,
		Nickname: user.Name,
		Avatar:   avatar,
		Email:    user.Email,
		Phone:    user.Mobile,
		ExtraInfo: map[string]interface{}{
			"tenantKey":  user.TenantKey,
			"userId":     user.UserID,
			"employeeNo": user.EmployeeNo,
		},
	}, nil
}
