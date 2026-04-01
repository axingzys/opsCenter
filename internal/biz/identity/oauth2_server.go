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
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ydcloud-dy/opshub/internal/biz/rbac"
)

// RSA 密钥对（单例）
var (
	rsaPrivateKey *rsa.PrivateKey
	rsaPublicKey  *rsa.PublicKey
	rsaKeyOnce    sync.Once
	rsaKeyID      = "opshub-key-1"
)

// 初始化 RSA 密钥对
func initRSAKeys() {
	rsaKeyOnce.Do(func() {
		var err error
		rsaPrivateKey, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			panic("failed to generate RSA key: " + err.Error())
		}
		rsaPublicKey = &rsaPrivateKey.PublicKey
	})
}

// GetJWKS 获取 JWKS（JSON Web Key Set）
func GetJWKS() map[string]interface{} {
	initRSAKeys()
	return map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"use": "sig",
				"kid": rsaKeyID,
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(rsaPublicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(rsaPublicKey.E)).Bytes()),
			},
		},
	}
}

// OAuth2ServerUseCase OAuth2服务端用例
type OAuth2ServerUseCase struct {
	appRepo      SSOApplicationRepo
	authCodeRepo OAuth2AuthCodeRepo
	tokenRepo    OAuth2TokenRepo
	userRepo     rbac.UserRepo
	permRepo     AppPermissionRepo
	issuer       string
	signingKey   string
}

// NewOAuth2ServerUseCase 创建OAuth2服务端用例
func NewOAuth2ServerUseCase(
	appRepo SSOApplicationRepo,
	authCodeRepo OAuth2AuthCodeRepo,
	tokenRepo OAuth2TokenRepo,
	userRepo rbac.UserRepo,
	permRepo AppPermissionRepo,
	issuer string,
	signingKey string,
) *OAuth2ServerUseCase {
	return &OAuth2ServerUseCase{
		appRepo:      appRepo,
		authCodeRepo: authCodeRepo,
		tokenRepo:    tokenRepo,
		userRepo:     userRepo,
		permRepo:     permRepo,
		issuer:       issuer,
		signingKey:   signingKey,
	}
}

// AuthorizeRequest 授权请求
type AuthorizeRequest struct {
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	ResponseType        string `json:"response_type"`
	Scope               string `json:"scope"`
	State               string `json:"state"`
	Nonce               string `json:"nonce"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
}

// AuthorizeResponse 授权响应
type AuthorizeResponse struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

// TokenRequest 令牌请求
type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	CodeVerifier string `json:"code_verifier"`
	RefreshToken string `json:"refresh_token"`
}

// TokenResponse 令牌响应
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

// UserInfoResponse 用户信息响应
type UserInfoResponse struct {
	Sub      string `json:"sub"`
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Avatar   string `json:"picture,omitempty"`
	Username string `json:"preferred_username,omitempty"`
}

// ValidateAuthorizeRequest 验证授权请求
func (uc *OAuth2ServerUseCase) ValidateAuthorizeRequest(ctx context.Context, req *AuthorizeRequest) (*SSOApplication, error) {
	// 验证客户端
	app, err := uc.appRepo.GetByCode(ctx, req.ClientID)
	if err != nil {
		// Debug: 打印详细的错误信息
		fmt.Printf("ValidateAuthorizeRequest: client_id=%s, error=%v\n", req.ClientID, err)
		return nil, errors.New("invalid_client: client not found")
	}

	if !app.Enabled {
		return nil, errors.New("invalid_client: client is disabled")
	}

	// 验证response_type
	if req.ResponseType != "code" {
		return nil, errors.New("unsupported_response_type: only 'code' is supported")
	}

	// 验证redirect_uri
	var ssoConfig SSOConfig
	if err := json.Unmarshal([]byte(app.SSOConfig), &ssoConfig); err == nil {
		if ssoConfig.RedirectURI != "" && req.RedirectURI != "" && ssoConfig.RedirectURI != req.RedirectURI {
			return nil, errors.New("invalid_request: redirect_uri mismatch")
		}
	}

	return app, nil
}

// CreateAuthorizationCode 创建授权码
func (uc *OAuth2ServerUseCase) CreateAuthorizationCode(ctx context.Context, req *AuthorizeRequest, userID uint) (*AuthorizeResponse, error) {
	// Debug: 打印授权请求信息
	fmt.Printf("CreateAuthorizationCode: client_id=%s, redirect_uri=%s, scope=%s\n", req.ClientID, req.RedirectURI, req.Scope)

	// 生成授权码
	code, err := generateRandomCode(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate authorization code: %w", err)
	}

	authCode := &OAuth2AuthorizationCode{
		Code:                code,
		ClientID:            req.ClientID,
		UserID:              userID,
		Scope:               req.Scope,
		RedirectURI:         req.RedirectURI,
		Nonce:               req.Nonce,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
		ExpiresAt:           time.Now().Add(10 * time.Minute),
		Used:                false,
		CreatedAt:           time.Now(),
	}

	if err := uc.authCodeRepo.Create(ctx, authCode); err != nil {
		return nil, fmt.Errorf("failed to save authorization code: %w", err)
	}

	return &AuthorizeResponse{
		Code:  code,
		State: req.State,
	}, nil
}

// ExchangeToken 交换令牌
func (uc *OAuth2ServerUseCase) ExchangeToken(ctx context.Context, req *TokenRequest) (*TokenResponse, error) {
	switch req.GrantType {
	case "authorization_code":
		return uc.exchangeAuthorizationCode(ctx, req)
	case "refresh_token":
		return uc.refreshAccessToken(ctx, req)
	default:
		return nil, errors.New("unsupported_grant_type")
	}
}

// exchangeAuthorizationCode 使用授权码交换令牌
func (uc *OAuth2ServerUseCase) exchangeAuthorizationCode(ctx context.Context, req *TokenRequest) (*TokenResponse, error) {
	// 获取授权码
	authCode, err := uc.authCodeRepo.GetByCode(ctx, req.Code)
	if err != nil {
		return nil, errors.New("invalid_grant: authorization code not found")
	}

	// 检查是否已使用
	if authCode.Used {
		return nil, errors.New("invalid_grant: authorization code already used")
	}

	// 检查是否过期
	if time.Now().After(authCode.ExpiresAt) {
		return nil, errors.New("invalid_grant: authorization code expired")
	}

	// 验证客户端
	if authCode.ClientID != req.ClientID {
		fmt.Printf("client_id mismatch: authCode.ClientID=%s, req.ClientID=%s\n", authCode.ClientID, req.ClientID)
		return nil, errors.New("invalid_grant: client_id mismatch")
	}

	// 验证redirect_uri
	if authCode.RedirectURI != "" && authCode.RedirectURI != req.RedirectURI {
		return nil, errors.New("invalid_grant: redirect_uri mismatch")
	}

	// 验证PKCE（如果有）
	if authCode.CodeChallenge != "" {
		if req.CodeVerifier == "" {
			return nil, errors.New("invalid_grant: code_verifier required")
		}
		if !verifyPKCE(authCode.CodeChallenge, authCode.CodeChallengeMethod, req.CodeVerifier) {
			return nil, errors.New("invalid_grant: code_verifier invalid")
		}
	} else {
		// 如果没有PKCE，验证client_secret
		app, err := uc.appRepo.GetByCode(ctx, req.ClientID)
		if err != nil {
			return nil, errors.New("invalid_client")
		}
		var ssoConfig SSOConfig
		if err := json.Unmarshal([]byte(app.SSOConfig), &ssoConfig); err == nil {
			if ssoConfig.ClientSecret != "" && ssoConfig.ClientSecret != req.ClientSecret {
				return nil, errors.New("invalid_client: client_secret invalid")
			}
		}
	}

	// 标记授权码已使用
	if err := uc.authCodeRepo.MarkUsed(ctx, req.Code); err != nil {
		return nil, fmt.Errorf("failed to mark code as used: %w", err)
	}

	// 生成访问令牌
	accessToken, err := generateRandomCode(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	accessTokenHash := hashToken(accessToken)
	accessTokenRecord := &OAuth2AccessToken{
		TokenHash: accessTokenHash,
		ClientID:  authCode.ClientID,
		UserID:    authCode.UserID,
		Scope:     authCode.Scope,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		CreatedAt: time.Now(),
	}

	if err := uc.tokenRepo.CreateAccessToken(ctx, accessTokenRecord); err != nil {
		return nil, fmt.Errorf("failed to save access token: %w", err)
	}

	// 生成刷新令牌
	refreshToken, err := generateRandomCode(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	refreshTokenHash := hashToken(refreshToken)
	refreshTokenRecord := &OAuth2RefreshToken{
		TokenHash:     refreshTokenHash,
		AccessTokenID: accessTokenRecord.ID,
		ExpiresAt:     time.Now().Add(7 * 24 * time.Hour),
		Revoked:       false,
		CreatedAt:     time.Now(),
	}

	if err := uc.tokenRepo.CreateRefreshToken(ctx, refreshTokenRecord); err != nil {
		return nil, fmt.Errorf("failed to save refresh token: %w", err)
	}

	// 生成 id_token（OIDC 标准要求）
	idToken, err := uc.generateIDToken(ctx, authCode.UserID, authCode.ClientID, authCode.Scope, authCode.Nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to generate id_token: %w", err)
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		RefreshToken: refreshToken,
		Scope:        authCode.Scope,
		IDToken:      idToken,
	}, nil
}

// generateIDToken 生成 OIDC id_token
func (uc *OAuth2ServerUseCase) generateIDToken(ctx context.Context, userID uint, clientID, scope, nonce string) (string, error) {
	initRSAKeys()

	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":                uc.issuer,                     // Issuer
		"sub":                user.Username,                 // Subject (使用用户名，便于 Jenkins 等系统显示)
		"aud":                clientID,                      // Audience (client ID)
		"exp":                now.Add(1 * time.Hour).Unix(), // Expiration
		"iat":                now.Unix(),                    // Issued at
		"auth_time":          now.Unix(),                    // Authentication time
		"preferred_username": user.Username,                 // 始终包含用户名
		"name":               user.RealName,                 // 始终包含显示名称
	}

	// 添加 nonce（OIDC 要求）
	if nonce != "" {
		claims["nonce"] = nonce
	}

	// 添加头像（转换为绝对 URL）
	if user.Avatar != "" {
		avatarURL := uc.makeAbsoluteURL(user.Avatar)
		claims["picture"] = avatarURL
	}

	// 根据 scope 添加额外用户信息
	scopes := strings.Split(scope, " ")
	for _, s := range scopes {
		switch s {
		case "email":
			if user.Email != "" {
				claims["email"] = user.Email
			}
		case "phone":
			if user.Phone != "" {
				claims["phone_number"] = user.Phone
			}
		}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = rsaKeyID
	return token.SignedString(rsaPrivateKey)
}

// makeAbsoluteURL 将相对 URL 转换为绝对 URL
func (uc *OAuth2ServerUseCase) makeAbsoluteURL(path string) string {
	if path == "" {
		return ""
	}
	// 如果已经是绝对 URL，直接返回
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	// 将相对路径转换为绝对 URL
	return strings.TrimSuffix(uc.issuer, "/") + "/" + strings.TrimPrefix(path, "/")
}

// refreshAccessToken 刷新访问令牌
func (uc *OAuth2ServerUseCase) refreshAccessToken(ctx context.Context, req *TokenRequest) (*TokenResponse, error) {
	refreshTokenHash := hashToken(req.RefreshToken)
	refreshTokenRecord, err := uc.tokenRepo.GetRefreshTokenByHash(ctx, refreshTokenHash)
	if err != nil {
		return nil, errors.New("invalid_grant: refresh token not found")
	}

	if refreshTokenRecord.Revoked {
		return nil, errors.New("invalid_grant: refresh token revoked")
	}

	if time.Now().After(refreshTokenRecord.ExpiresAt) {
		return nil, errors.New("invalid_grant: refresh token expired")
	}

	// 获取原访问令牌信息
	// 这里需要从数据库获取原始访问令牌的信息（clientID, userID, scope）
	// 简化处理，假设刷新令牌关联的访问令牌信息

	// 生成新的访问令牌
	newAccessToken, err := generateRandomCode(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	return &TokenResponse{
		AccessToken: newAccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   3600,
	}, nil
}

// GetUserInfo 获取用户信息
func (uc *OAuth2ServerUseCase) GetUserInfo(ctx context.Context, accessToken string) (*UserInfoResponse, error) {
	tokenHash := hashToken(accessToken)
	tokenRecord, err := uc.tokenRepo.GetAccessTokenByHash(ctx, tokenHash)
	if err != nil {
		return nil, errors.New("invalid_token: token not found")
	}

	if time.Now().After(tokenRecord.ExpiresAt) {
		return nil, errors.New("invalid_token: token expired")
	}

	user, err := uc.userRepo.GetByID(ctx, tokenRecord.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// 使用用户名作为 sub，这样 Jenkins 等系统会显示正确的用户名
	// 头像转换为绝对 URL
	return &UserInfoResponse{
		Sub:      user.Username,
		Name:     user.RealName,
		Email:    user.Email,
		Phone:    user.Phone,
		Avatar:   uc.makeAbsoluteURL(user.Avatar),
		Username: user.Username,
	}, nil
}

// ValidateAccessToken 验证访问令牌
func (uc *OAuth2ServerUseCase) ValidateAccessToken(ctx context.Context, accessToken string) (*OAuth2AccessToken, error) {
	tokenHash := hashToken(accessToken)
	tokenRecord, err := uc.tokenRepo.GetAccessTokenByHash(ctx, tokenHash)
	if err != nil {
		return nil, errors.New("invalid_token")
	}

	if time.Now().After(tokenRecord.ExpiresAt) {
		return nil, errors.New("token_expired")
	}

	return tokenRecord, nil
}

// RevokeToken 撤销令牌
func (uc *OAuth2ServerUseCase) RevokeToken(ctx context.Context, token string) error {
	tokenHash := hashToken(token)
	return uc.tokenRepo.RevokeRefreshToken(ctx, tokenHash)
}

// 辅助函数

func generateRandomCode(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:length], nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func verifyPKCE(challenge, method, verifier string) bool {
	switch method {
	case "S256":
		hash := sha256.Sum256([]byte(verifier))
		computed := base64.RawURLEncoding.EncodeToString(hash[:])
		return computed == challenge
	case "plain", "":
		return verifier == challenge
	default:
		return false
	}
}

// GetOIDCDiscovery 获取OIDC发现文档
func (uc *OAuth2ServerUseCase) GetOIDCDiscovery() map[string]interface{} {
	return map[string]interface{}{
		"issuer":                                uc.issuer,
		"authorization_endpoint":               uc.issuer + "/oauth2/authorize",
		"token_endpoint":                        uc.issuer + "/oauth2/token",
		"userinfo_endpoint":                     uc.issuer + "/oauth2/userinfo",
		"jwks_uri":                              uc.issuer + "/oauth2/jwks",
		"response_types_supported":             []string{"code"},
		"grant_types_supported":                []string{"authorization_code", "refresh_token"},
		"subject_types_supported":              []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":                     []string{"openid", "profile", "email", "phone"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
		"code_challenge_methods_supported":     []string{"S256", "plain"},
	}
}

// CheckAppPermission 检查用户是否有权限访问应用
func (uc *OAuth2ServerUseCase) CheckAppPermission(ctx context.Context, appID, userID uint, roleIDs []uint, deptID uint) (bool, error) {
	// 检查用户直接权限
	hasPermission, err := uc.permRepo.CheckPermission(ctx, appID, "user", userID)
	if err == nil && hasPermission {
		return true, nil
	}

	// 检查角色权限
	for _, roleID := range roleIDs {
		hasPermission, err = uc.permRepo.CheckPermission(ctx, appID, "role", roleID)
		if err == nil && hasPermission {
			return true, nil
		}
	}

	// 检查部门权限
	if deptID > 0 {
		hasPermission, err = uc.permRepo.CheckPermission(ctx, appID, "dept", deptID)
		if err == nil && hasPermission {
			return true, nil
		}
	}

	return false, nil
}

// GetAppByClientID 根据ClientID获取应用
func (uc *OAuth2ServerUseCase) GetAppByClientID(ctx context.Context, clientID string) (*SSOApplication, error) {
	return uc.appRepo.GetByCode(ctx, clientID)
}

// ParseScope 解析scope
func ParseScope(scope string) []string {
	if scope == "" {
		return []string{}
	}
	return strings.Split(scope, " ")
}
