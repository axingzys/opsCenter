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

// DingTalkProvider 钉钉OAuth提供商
type DingTalkProvider struct{}

const (
	dingtalkAuthURL     = "https://login.dingtalk.com/oauth2/auth"
	dingtalkTokenURL    = "https://api.dingtalk.com/v1.0/oauth2/userAccessToken"
	dingtalkUserInfoURL = "https://api.dingtalk.com/v1.0/contact/users/me"
)

// GetProviderType 获取提供商类型
func (p *DingTalkProvider) GetProviderType() string {
	return "dingtalk"
}

// GetAuthURL 获取钉钉授权URL
func (p *DingTalkProvider) GetAuthURL(state string, config *IdentitySourceConfig) string {
	authURL := config.AuthURL
	if authURL == "" {
		authURL = dingtalkAuthURL
	}

	scopes := config.Scopes
	if scopes == "" {
		scopes = "openid corpid"
	}

	params := url.Values{}
	params.Set("client_id", config.ClientID)
	params.Set("redirect_uri", config.RedirectURI)
	params.Set("scope", scopes)
	params.Set("state", state)
	params.Set("response_type", "code")
	params.Set("prompt", "consent")

	return authURL + "?" + params.Encode()
}

// ExchangeToken 用授权码换取Token
func (p *DingTalkProvider) ExchangeToken(ctx context.Context, code string, config *IdentitySourceConfig) (*OAuthToken, error) {
	tokenURL := config.TokenURL
	if tokenURL == "" {
		tokenURL = dingtalkTokenURL
	}

	reqBody := map[string]string{
		"clientId":     config.ClientID,
		"clientSecret": config.ClientSecret,
		"code":         code,
		"grantType":    "authorization_code",
	}

	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

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
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpireIn     int    `json:"expireIn"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	if result.AccessToken == "" {
		return nil, fmt.Errorf("dingtalk oauth error: %s", string(body))
	}

	return &OAuthToken{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    result.ExpireIn,
		ExpiresAt:    time.Now().Add(time.Duration(result.ExpireIn) * time.Second),
	}, nil
}

// GetUserInfo 获取钉钉用户信息
func (p *DingTalkProvider) GetUserInfo(ctx context.Context, token *OAuthToken, config *IdentitySourceConfig) (*OAuthUserInfo, error) {
	userInfoURL := config.UserInfoURL
	if userInfoURL == "" {
		userInfoURL = dingtalkUserInfoURL
	}

	req, err := http.NewRequestWithContext(ctx, "GET", userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("x-acs-dingtalk-access-token", token.AccessToken)

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
		OpenID    string `json:"openId"`
		UnionID   string `json:"unionId"`
		Nick      string `json:"nick"`
		AvatarURL string `json:"avatarUrl"`
		Email     string `json:"email"`
		Mobile    string `json:"mobile"`
	}

	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	return &OAuthUserInfo{
		OpenID:   user.OpenID,
		UnionID:  user.UnionID,
		Nickname: user.Nick,
		Avatar:   user.AvatarURL,
		Email:    user.Email,
		Phone:    user.Mobile,
	}, nil
}
