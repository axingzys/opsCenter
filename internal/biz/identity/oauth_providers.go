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
	"fmt"
)

// OAuthProvider OAuth提供商接口
type OAuthProvider interface {
	// GetAuthURL 获取授权URL
	GetAuthURL(state string, config *IdentitySourceConfig) string
	// ExchangeToken 用授权码换取Token
	ExchangeToken(ctx context.Context, code string, config *IdentitySourceConfig) (*OAuthToken, error)
	// GetUserInfo 获取用户信息
	GetUserInfo(ctx context.Context, token *OAuthToken, config *IdentitySourceConfig) (*OAuthUserInfo, error)
	// GetProviderType 获取提供商类型
	GetProviderType() string
}

// OAuthProviderRegistry OAuth提供商注册表
var OAuthProviderRegistry = make(map[string]OAuthProvider)

// RegisterProvider 注册OAuth提供商
func RegisterProvider(provider OAuthProvider) {
	OAuthProviderRegistry[provider.GetProviderType()] = provider
}

// GetProvider 获取OAuth提供商
func GetProvider(providerType string) (OAuthProvider, error) {
	provider, ok := OAuthProviderRegistry[providerType]
	if !ok {
		return nil, fmt.Errorf("unsupported OAuth provider: %s", providerType)
	}
	return provider, nil
}

// 初始化注册所有提供商
func init() {
	RegisterProvider(&GitHubProvider{})
	RegisterProvider(&DingTalkProvider{})
	RegisterProvider(&WeChatProvider{})
	RegisterProvider(&FeishuProvider{})
}
