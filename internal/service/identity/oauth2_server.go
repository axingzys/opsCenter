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
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ydcloud-dy/opshub/internal/biz/identity"
	appLogger "github.com/ydcloud-dy/opshub/pkg/logger"
	"github.com/ydcloud-dy/opshub/pkg/response"
	"go.uber.org/zap"
)

// OAuth2ServerService OAuth2服务端服务
type OAuth2ServerService struct {
	useCase     *identity.OAuth2ServerUseCase
	frontendURL string
}

// NewOAuth2ServerService 创建OAuth2服务端服务
func NewOAuth2ServerService(useCase *identity.OAuth2ServerUseCase, frontendURL string) *OAuth2ServerService {
	return &OAuth2ServerService{
		useCase:     useCase,
		frontendURL: frontendURL,
	}
}

// Authorize OAuth2授权端点
func (s *OAuth2ServerService) Authorize(c *gin.Context) {
	req := &identity.AuthorizeRequest{
		ClientID:            c.Query("client_id"),
		RedirectURI:         c.Query("redirect_uri"),
		ResponseType:        c.Query("response_type"),
		Scope:               c.Query("scope"),
		State:               c.Query("state"),
		Nonce:               c.Query("nonce"),
		CodeChallenge:       c.Query("code_challenge"),
		CodeChallengeMethod: c.Query("code_challenge_method"),
	}

	// 兼容性修复：如果 client_id 为空，尝试从 redirect_uri 中提取
	// 例如：http://example.com/users/auth/oauth2_generic/callback -> oauth2_generic
	if req.ClientID == "" && req.RedirectURI != "" {
		if u, err := url.Parse(req.RedirectURI); err == nil {
			// 从路径中提取 provider 名称
			// 路径格式通常是 /users/auth/{provider}/callback
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			for i, part := range parts {
				if part == "auth" && i+1 < len(parts) {
					req.ClientID = parts[i+1]
					appLogger.Info("从 redirect_uri 提取 client_id",
						zap.String("redirect_uri", req.RedirectURI),
						zap.String("extracted_client_id", req.ClientID),
					)
					break
				}
			}
		}
	}

	// Debug: 打印请求信息
	sessionCookie, _ := c.Cookie("opshub_session")
	appLogger.Info("OAuth2 Authorize 请求",
		zap.String("client_id", req.ClientID),
		zap.String("redirect_uri", req.RedirectURI),
		zap.String("host", c.Request.Host),
		zap.Bool("has_session_cookie", sessionCookie != ""),
	)

	// 验证请求
	_, err := s.useCase.ValidateAuthorizeRequest(c.Request.Context(), req)
	if err != nil {
		appLogger.Error("OAuth2 验证请求失败", zap.Error(err))
		s.redirectWithError(c, req.RedirectURI, req.State, "invalid_request", err.Error())
		return
	}

	// 检查用户是否已登录
	userID, exists := c.Get("userID")
	appLogger.Info("OAuth2 用户登录状态", zap.Bool("logged_in", exists), zap.Any("userID", userID))

	if !exists {
		// 重定向到前端登录页，登录后返回
		// 构建完整的当前 URL（包含 scheme 和 host）
		currentURL := c.Request.URL.String()
		if !strings.HasPrefix(currentURL, "http") {
			scheme := "http"
			if c.Request.TLS != nil {
				scheme = "https"
			}
			// 使用 X-Forwarded-Host 或 Host
			host := c.GetHeader("X-Forwarded-Host")
			if host == "" {
				host = c.Request.Host
			}
			currentURL = scheme + "://" + host + currentURL
		}
		loginURL := s.frontendURL + "/login?redirect=" + url.QueryEscape(currentURL)
		appLogger.Info("OAuth2 重定向到登录页", zap.String("loginURL", loginURL), zap.String("currentURL", currentURL))
		c.Redirect(http.StatusFound, loginURL)
		return
	}

	// 用户已登录，直接创建授权码并重定向（自动授权，无需用户同意）
	// 对于内部 SSO 应用，我们信任它们，自动授权
	authResp, err := s.useCase.CreateAuthorizationCode(c.Request.Context(), req, userID.(uint))
	if err != nil {
		appLogger.Error("OAuth2 创建授权码失败", zap.Error(err))
		s.redirectWithError(c, req.RedirectURI, req.State, "server_error", err.Error())
		return
	}

	// 重定向回客户端
	redirectURL := s.buildRedirectURL(req.RedirectURI, authResp.Code, authResp.State)
	appLogger.Info("OAuth2 授权成功，重定向到客户端", zap.String("redirectURL", redirectURL))
	c.Redirect(http.StatusFound, redirectURL)
}

// Token OAuth2令牌端点
func (s *OAuth2ServerService) Token(c *gin.Context) {
	var req identity.TokenRequest

	// 支持form和json两种格式
	contentType := c.GetHeader("Content-Type")
	if strings.Contains(contentType, "application/json") {
		if err := c.ShouldBindJSON(&req); err != nil {
			s.tokenError(c, "invalid_request", err.Error())
			return
		}
	} else {
		req.GrantType = c.PostForm("grant_type")
		req.Code = c.PostForm("code")
		req.RedirectURI = c.PostForm("redirect_uri")
		req.ClientID = c.PostForm("client_id")
		req.ClientSecret = c.PostForm("client_secret")
		req.CodeVerifier = c.PostForm("code_verifier")
		req.RefreshToken = c.PostForm("refresh_token")
	}

	// 支持Basic Auth传递client credentials
	if req.ClientID == "" {
		clientID, clientSecret, ok := c.Request.BasicAuth()
		if ok {
			req.ClientID = clientID
			req.ClientSecret = clientSecret
		}
	}

	// 兼容性修复：如果 client_id 为空，尝试从 redirect_uri 中提取
	// 例如：http://example.com/users/auth/gitlab/callback -> gitlab
	if req.ClientID == "" && req.RedirectURI != "" {
		if u, err := url.Parse(req.RedirectURI); err == nil {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			for i, part := range parts {
				if part == "auth" && i+1 < len(parts) {
					req.ClientID = parts[i+1]
					appLogger.Info("Token请求从 redirect_uri 提取 client_id",
						zap.String("redirect_uri", req.RedirectURI),
						zap.String("extracted_client_id", req.ClientID),
					)
					break
				}
			}
		}
	}

	// Debug: 打印 Token 请求信息
	appLogger.Info("OAuth2 Token 请求",
		zap.String("client_id", req.ClientID),
		zap.String("code", req.Code),
		zap.String("redirect_uri", req.RedirectURI),
		zap.String("grant_type", req.GrantType),
	)

	// 交换令牌
	tokenResp, err := s.useCase.ExchangeToken(c.Request.Context(), &req)
	if err != nil {
		s.tokenError(c, "invalid_grant", err.Error())
		return
	}

	c.JSON(http.StatusOK, tokenResp)
}

// UserInfo OAuth2用户信息端点
func (s *OAuth2ServerService) UserInfo(c *gin.Context) {
	// 从Authorization header获取access token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		response.ErrorCode(c, http.StatusUnauthorized, "missing authorization header")
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		response.ErrorCode(c, http.StatusUnauthorized, "invalid authorization header")
		return
	}

	accessToken := parts[1]

	userInfo, err := s.useCase.GetUserInfo(c.Request.Context(), accessToken)
	if err != nil {
		response.ErrorCode(c, http.StatusUnauthorized, err.Error())
		return
	}

	c.JSON(http.StatusOK, userInfo)
}

// Discovery OIDC发现端点
func (s *OAuth2ServerService) Discovery(c *gin.Context) {
	discovery := s.useCase.GetOIDCDiscovery()
	c.JSON(http.StatusOK, discovery)
}

// JWKS JWKS端点
func (s *OAuth2ServerService) JWKS(c *gin.Context) {
	jwks := identity.GetJWKS()
	c.JSON(http.StatusOK, jwks)
}

// GitLabUserInfo GitLab 兼容的用户信息端点
// GitLab 的 oauth2_generic 默认请求 /api/v4/user
func (s *OAuth2ServerService) GitLabUserInfo(c *gin.Context) {
	// 从Authorization header获取access token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
		return
	}

	accessToken := parts[1]

	userInfo, err := s.useCase.GetUserInfo(c.Request.Context(), accessToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// GitLab API v4 user 格式
	c.JSON(http.StatusOK, gin.H{
		"id":               userInfo.Sub,
		"username":         userInfo.Username,
		"name":             userInfo.Name,
		"email":            userInfo.Email,
		"avatar_url":       userInfo.Avatar,
		"state":            "active",
		"two_factor_enabled": false,
	})
}

// Revoke 令牌撤销端点
func (s *OAuth2ServerService) Revoke(c *gin.Context) {
	token := c.PostForm("token")
	if token == "" {
		response.ErrorCode(c, http.StatusBadRequest, "missing token")
		return
	}

	if err := s.useCase.RevokeToken(c.Request.Context(), token); err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Status(http.StatusOK)
}

// Introspect 令牌内省端点
func (s *OAuth2ServerService) Introspect(c *gin.Context) {
	token := c.PostForm("token")
	if token == "" {
		c.JSON(http.StatusOK, gin.H{"active": false})
		return
	}

	tokenRecord, err := s.useCase.ValidateAccessToken(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"active": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"active":    true,
		"client_id": tokenRecord.ClientID,
		"sub":       tokenRecord.UserID,
		"scope":     tokenRecord.Scope,
		"exp":       tokenRecord.ExpiresAt.Unix(),
		"iat":       tokenRecord.CreatedAt.Unix(),
	})
}

// 辅助函数

func (s *OAuth2ServerService) redirectWithError(c *gin.Context, redirectURI, state, errorCode, errorDesc string) {
	if redirectURI == "" {
		response.ErrorCode(c, http.StatusBadRequest, errorCode+": "+errorDesc)
		return
	}

	u, err := url.Parse(redirectURI)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "invalid redirect_uri")
		return
	}

	q := u.Query()
	q.Set("error", errorCode)
	q.Set("error_description", errorDesc)
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()

	c.Redirect(http.StatusFound, u.String())
}

func (s *OAuth2ServerService) tokenError(c *gin.Context, errorCode, errorDesc string) {
	c.JSON(http.StatusBadRequest, gin.H{
		"error":             errorCode,
		"error_description": errorDesc,
	})
}

func (s *OAuth2ServerService) buildRedirectURL(redirectURI, code, state string) string {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return redirectURI
	}

	q := u.Query()
	q.Set("code", code)
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()

	return u.String()
}
