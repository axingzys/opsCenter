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
	"time"
)

// WeChatProvider 微信OAuth提供商（企业微信）
type WeChatProvider struct{}

const (
	wechatAuthURL     = "https://open.weixin.qq.com/connect/oauth2/authorize"
	wechatTokenURL    = "https://api.weixin.qq.com/sns/oauth2/access_token"
	wechatUserInfoURL = "https://api.weixin.qq.com/sns/userinfo"
)

// GetProviderType 获取提供商类型
func (p *WeChatProvider) GetProviderType() string {
	return "wechat"
}

// GetAuthURL 获取微信授权URL
func (p *WeChatProvider) GetAuthURL(state string, config *IdentitySourceConfig) string {
	authURL := config.AuthURL
	if authURL == "" {
		authURL = wechatAuthURL
	}

	scopes := config.Scopes
	if scopes == "" {
		scopes = "snsapi_userinfo"
	}

	params := url.Values{}
	params.Set("appid", config.ClientID)
	params.Set("redirect_uri", config.RedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", scopes)
	params.Set("state", state)

	return authURL + "?" + params.Encode() + "#wechat_redirect"
}

// ExchangeToken 用授权码换取Token
func (p *WeChatProvider) ExchangeToken(ctx context.Context, code string, config *IdentitySourceConfig) (*OAuthToken, error) {
	tokenURL := config.TokenURL
	if tokenURL == "" {
		tokenURL = wechatTokenURL
	}

	params := url.Values{}
	params.Set("appid", config.ClientID)
	params.Set("secret", config.ClientSecret)
	params.Set("code", code)
	params.Set("grant_type", "authorization_code")

	reqURL := tokenURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

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
		ExpiresIn    int    `json:"expires_in"`
		OpenID       string `json:"openid"`
		UnionID      string `json:"unionid"`
		ErrCode      int    `json:"errcode"`
		ErrMsg       string `json:"errmsg"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	if result.ErrCode != 0 {
		return nil, fmt.Errorf("wechat oauth error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	return &OAuthToken{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    result.ExpiresIn,
		ExpiresAt:    time.Now().Add(time.Duration(result.ExpiresIn) * time.Second),
	}, nil
}

// GetUserInfo 获取微信用户信息
func (p *WeChatProvider) GetUserInfo(ctx context.Context, token *OAuthToken, config *IdentitySourceConfig) (*OAuthUserInfo, error) {
	userInfoURL := config.UserInfoURL
	if userInfoURL == "" {
		userInfoURL = wechatUserInfoURL
	}

	// 先从token响应中解析openid
	var tokenResp struct {
		OpenID  string `json:"openid"`
		UnionID string `json:"unionid"`
	}
	// 重新获取openid（从token交换响应中）
	tokenURL := config.TokenURL
	if tokenURL == "" {
		tokenURL = wechatTokenURL
	}

	params := url.Values{}
	params.Set("access_token", token.AccessToken)
	params.Set("openid", tokenResp.OpenID)
	params.Set("lang", "zh_CN")

	reqURL := userInfoURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

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
		OpenID     string `json:"openid"`
		UnionID    string `json:"unionid"`
		Nickname   string `json:"nickname"`
		HeadImgURL string `json:"headimgurl"`
		ErrCode    int    `json:"errcode"`
		ErrMsg     string `json:"errmsg"`
	}

	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	if user.ErrCode != 0 {
		return nil, fmt.Errorf("wechat userinfo error: %d - %s", user.ErrCode, user.ErrMsg)
	}

	return &OAuthUserInfo{
		OpenID:   user.OpenID,
		UnionID:  user.UnionID,
		Nickname: user.Nickname,
		Avatar:   user.HeadImgURL,
	}, nil
}
