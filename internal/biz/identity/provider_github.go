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
	"strconv"
	"strings"
	"time"
)

// GitHubProvider GitHub OAuth提供商
type GitHubProvider struct{}

const (
	githubAuthURL     = "https://github.com/login/oauth/authorize"
	githubTokenURL    = "https://github.com/login/oauth/access_token"
	githubUserInfoURL = "https://api.github.com/user"
)

// GetProviderType 获取提供商类型
func (p *GitHubProvider) GetProviderType() string {
	return "github"
}

// GetAuthURL 获取GitHub授权URL
func (p *GitHubProvider) GetAuthURL(state string, config *IdentitySourceConfig) string {
	authURL := config.AuthURL
	if authURL == "" {
		authURL = githubAuthURL
	}

	scopes := config.Scopes
	if scopes == "" {
		scopes = "user:email"
	}

	params := url.Values{}
	params.Set("client_id", config.ClientID)
	params.Set("redirect_uri", config.RedirectURI)
	params.Set("scope", scopes)
	params.Set("state", state)

	return authURL + "?" + params.Encode()
}

// ExchangeToken 用授权码换取Token
func (p *GitHubProvider) ExchangeToken(ctx context.Context, code string, config *IdentitySourceConfig) (*OAuthToken, error) {
	tokenURL := config.TokenURL
	if tokenURL == "" {
		tokenURL = githubTokenURL
	}

	data := url.Values{}
	data.Set("client_id", config.ClientID)
	data.Set("client_secret", config.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", config.RedirectURI)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

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
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("github oauth error: %s - %s", result.Error, result.ErrorDesc)
	}

	return &OAuthToken{
		AccessToken: result.AccessToken,
		TokenType:   result.TokenType,
	}, nil
}

// GetUserInfo 获取GitHub用户信息
func (p *GitHubProvider) GetUserInfo(ctx context.Context, token *OAuthToken, config *IdentitySourceConfig) (*OAuthUserInfo, error) {
	userInfoURL := config.UserInfoURL
	if userInfoURL == "" {
		userInfoURL = githubUserInfoURL
	}

	req, err := http.NewRequestWithContext(ctx, "GET", userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/json")

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
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}

	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	nickname := user.Name
	if nickname == "" {
		nickname = user.Login
	}

	return &OAuthUserInfo{
		OpenID:   strconv.Itoa(user.ID),
		Nickname: nickname,
		Avatar:   user.AvatarURL,
		Email:    user.Email,
		ExtraInfo: map[string]interface{}{
			"login": user.Login,
		},
	}, nil
}
