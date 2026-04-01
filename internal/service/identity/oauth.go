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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ydcloud-dy/opshub/internal/biz/identity"
	"github.com/ydcloud-dy/opshub/pkg/response"
)

// OAuthService OAuth认证服务
type OAuthService struct {
	useCase      *identity.OAuthUseCase
	sourceUC     *identity.IdentitySourceUseCase
	jwtSecret    string
	jwtExpiresIn int
}

// NewOAuthService 创建OAuth服务
func NewOAuthService(useCase *identity.OAuthUseCase, sourceUC *identity.IdentitySourceUseCase, jwtSecret string, jwtExpiresIn int) *OAuthService {
	return &OAuthService{
		useCase:      useCase,
		sourceUC:     sourceUC,
		jwtSecret:    jwtSecret,
		jwtExpiresIn: jwtExpiresIn,
	}
}

// GetEnabledProviders 获取启用的身份源列表（用于登录页显示）
func (s *OAuthService) GetEnabledProviders(c *gin.Context) {
	sources, err := s.sourceUC.GetEnabled(c.Request.Context())
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "获取身份源失败")
		return
	}

	// 过滤敏感信息
	result := make([]map[string]interface{}, 0, len(sources))
	for _, source := range sources {
		result = append(result, map[string]interface{}{
			"id":   source.ID,
			"name": source.Name,
			"type": source.Type,
			"icon": source.Icon,
		})
	}

	response.Success(c, result)
}

// Authorize 获取OAuth授权URL
func (s *OAuthService) Authorize(c *gin.Context) {
	provider := c.Param("provider")
	redirectURL := c.Query("redirect_url")
	action := c.DefaultQuery("action", "login")

	var userID uint
	if action == "bind" {
		// 绑定模式需要用户已登录
		uid, exists := c.Get("userID")
		if !exists {
			response.ErrorCode(c, http.StatusUnauthorized, "请先登录")
			return
		}
		userID = uid.(uint)
	}

	authURL, err := s.useCase.InitiateOAuth(c.Request.Context(), provider, redirectURL, action, userID)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{
		"authUrl": authURL,
	})
}

// Callback 处理OAuth回调
func (s *OAuthService) Callback(c *gin.Context) {
	provider := c.Param("provider")
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		// 检查是否有错误
		errorCode := c.Query("error")
		errorDesc := c.Query("error_description")
		if errorCode != "" {
			response.ErrorCode(c, http.StatusBadRequest, "OAuth error: "+errorCode+" - "+errorDesc)
			return
		}
		response.ErrorCode(c, http.StatusBadRequest, "缺少code或state参数")
		return
	}

	ip := c.ClientIP()
	userAgent := c.Request.UserAgent()

	result, err := s.useCase.HandleCallback(c.Request.Context(), provider, code, state, ip, userAgent)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, err.Error())
		return
	}

	if result.NeedBind {
		// 需要绑定账号
		response.Success(c, gin.H{
			"needBind":  true,
			"bindToken": result.BindToken,
			"oauthInfo": result.OAuthInfo,
		})
		return
	}

	// 生成JWT token
	token, err := s.generateToken(result.UserID)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "生成token失败")
		return
	}

	response.Success(c, gin.H{
		"isNewUser": result.IsNewUser,
		"token":     token,
		"userId":    result.UserID,
	})
}

// BindWithCredentials 使用账号密码绑定OAuth
func (s *OAuthService) BindWithCredentials(c *gin.Context) {
	var req struct {
		Provider  string `json:"provider" binding:"required"`
		BindToken string `json:"bindToken" binding:"required"`
		Username  string `json:"username" binding:"required"`
		Password  string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	// TODO: 验证用户名密码，获取userID
	// 这里需要集成现有的用户认证逻辑
	// 暂时返回未实现

	response.ErrorCode(c, http.StatusNotImplemented, "功能开发中")
}

// Unbind 解绑OAuth账号
func (s *OAuthService) Unbind(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		response.ErrorCode(c, http.StatusUnauthorized, "请先登录")
		return
	}

	var sourceID uint
	if err := BindUintParam(c, "sourceId", &sourceID); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误")
		return
	}

	if err := s.useCase.UnbindOAuth(c.Request.Context(), userID.(uint), sourceID); err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil)
}

// GetBindings 获取用户的OAuth绑定列表
func (s *OAuthService) GetBindings(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		response.ErrorCode(c, http.StatusUnauthorized, "请先登录")
		return
	}

	bindings, err := s.useCase.GetUserBindings(c.Request.Context(), userID.(uint))
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, err.Error())
		return
	}

	// 过滤敏感信息
	result := make([]map[string]interface{}, 0, len(bindings))
	for _, binding := range bindings {
		result = append(result, map[string]interface{}{
			"id":         binding.ID,
			"sourceId":   binding.SourceID,
			"sourceType": binding.SourceType,
			"nickname":   binding.Nickname,
			"avatar":     binding.Avatar,
			"createdAt":  binding.CreatedAt,
		})
	}

	response.Success(c, result)
}

// generateToken 生成JWT token
func (s *OAuthService) generateToken(userID uint) (string, error) {
	// 使用现有的JWT生成逻辑
	// 这里简化处理，实际应该复用rbac模块的token生成
	return GenerateJWT(userID, s.jwtSecret, s.jwtExpiresIn)
}

// BindUintParam 绑定uint参数
func BindUintParam(c *gin.Context, key string, target *uint) error {
	val := c.Param(key)
	if val == "" {
		return nil
	}
	var id uint64
	_, err := parseUint(val, &id)
	if err != nil {
		return err
	}
	*target = uint(id)
	return nil
}

// parseUint 解析uint
func parseUint(s string, result *uint64) (int, error) {
	var n uint64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return i, nil
		}
		n = n*10 + uint64(c-'0')
	}
	*result = n
	return len(s), nil
}

// JwtClaims JWT声明
type JwtClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// GenerateJWT 生成JWT token
func GenerateJWT(userID uint, secretKey string, expiresIn int) (string, error) {
	if expiresIn <= 0 {
		expiresIn = 24 // 默认24小时
	}

	claims := JwtClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expiresIn) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}
