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
	"github.com/gin-gonic/gin"
	bizIdentity "github.com/ydcloud-dy/opshub/internal/biz/identity"
	"github.com/ydcloud-dy/opshub/internal/biz/rbac"
	"github.com/ydcloud-dy/opshub/internal/conf"
	dataIdentity "github.com/ydcloud-dy/opshub/internal/data/identity"
	dataRbac "github.com/ydcloud-dy/opshub/internal/data/rbac"
	svcIdentity "github.com/ydcloud-dy/opshub/internal/service/identity"
	"gorm.io/gorm"
)

// HTTPServer 身份认证HTTP服务
type HTTPServer struct {
	sourceService      *svcIdentity.IdentitySourceService
	appService         *svcIdentity.SSOApplicationService
	portalService      *svcIdentity.PortalService
	credentialService  *svcIdentity.CredentialService
	permissionService  *svcIdentity.PermissionService
	authLogService     *svcIdentity.AuthLogService
	ldapService        *svcIdentity.LDAPService
	oauth2Service      *svcIdentity.OAuth2ServerService
	userRepo           rbac.UserRepo
}

// NewIdentityServices 创建身份认证相关服务
func NewIdentityServices(db *gorm.DB, cfg *conf.Config) (*HTTPServer, error) {
	// 自动迁移数据库表
	if err := db.AutoMigrate(
		&bizIdentity.IdentitySource{},
		&bizIdentity.SSOApplication{},
		&bizIdentity.UserCredential{},
		&bizIdentity.AppPermission{},
		&bizIdentity.UserOAuthBinding{},
		&bizIdentity.AuthLog{},
		&bizIdentity.UserFavoriteApp{},
		&bizIdentity.LDAPSyncJob{},
		&bizIdentity.OAuth2AuthorizationCode{},
		&bizIdentity.OAuth2AccessToken{},
		&bizIdentity.OAuth2RefreshToken{},
	); err != nil {
		return nil, err
	}

	// 创建仓库
	sourceRepo := dataIdentity.NewIdentitySourceRepo(db)
	appRepo := dataIdentity.NewSSOApplicationRepo(db)
	credentialRepo := dataIdentity.NewUserCredentialRepo(db)
	permissionRepo := dataIdentity.NewAppPermissionRepo(db)
	oauthBindingRepo := dataIdentity.NewUserOAuthBindingRepo(db)
	authLogRepo := dataIdentity.NewAuthLogRepo(db)
	favoriteRepo := dataIdentity.NewUserFavoriteAppRepo(db)
	authCodeRepo := dataIdentity.NewOAuth2AuthCodeRepo(db)
	tokenRepo := dataIdentity.NewOAuth2TokenRepo(db)
	userRepo := dataRbac.NewUserRepo(db)
	_ = dataIdentity.NewLDAPSyncJobRepo(db) // LDAP同步任务仓库，后续LDAP完整集成时使用

	// 创建用例
	sourceUseCase := bizIdentity.NewIdentitySourceUseCase(sourceRepo)
	appUseCase := bizIdentity.NewSSOApplicationUseCase(appRepo)
	credentialUseCase := bizIdentity.NewUserCredentialUseCase(credentialRepo)
	permissionUseCase := bizIdentity.NewAppPermissionUseCase(permissionRepo)
	_ = bizIdentity.NewUserOAuthBindingUseCase(oauthBindingRepo) // 后续OAuth功能使用
	authLogUseCase := bizIdentity.NewAuthLogUseCase(authLogRepo)
	favoriteUseCase := bizIdentity.NewUserFavoriteAppUseCase(favoriteRepo)

	// OAuth2 服务端用例
	oauth2UseCase := bizIdentity.NewOAuth2ServerUseCase(
		appRepo,
		authCodeRepo,
		tokenRepo,
		userRepo,
		permissionRepo,
		cfg.Server.GetOAuth2Issuer(), // 从配置读取 issuer
		cfg.Server.JWTSecret,         // 使用 JWT 密钥作为签名密钥
	)

	// 创建服务
	sourceService := svcIdentity.NewIdentitySourceService(sourceUseCase)
	appService := svcIdentity.NewSSOApplicationService(appUseCase)
	portalService := svcIdentity.NewPortalService(appUseCase, permissionUseCase, favoriteUseCase, authLogUseCase)
	credentialService := svcIdentity.NewCredentialService(credentialUseCase, appUseCase)
	permissionService := svcIdentity.NewPermissionService(permissionUseCase)
	authLogService := svcIdentity.NewAuthLogService(authLogUseCase)
	oauth2Service := svcIdentity.NewOAuth2ServerService(oauth2UseCase, cfg.Server.GetFrontendURL())

	// LDAP服务需要UserRepo，这里暂时传nil，实际使用时需要注入
	// 在完整集成时，需要从rbac模块获取UserRepo
	var ldapService *svcIdentity.LDAPService

	return &HTTPServer{
		sourceService:      sourceService,
		appService:         appService,
		portalService:      portalService,
		credentialService:  credentialService,
		permissionService:  permissionService,
		authLogService:     authLogService,
		ldapService:        ldapService,
		oauth2Service:      oauth2Service,
		userRepo:           userRepo,
	}, nil
}

// RegisterRoutes 注册路由
func (s *HTTPServer) RegisterRoutes(router *gin.RouterGroup) {
	identity := router.Group("/identity")
	{
		// 身份源管理
		sources := identity.Group("/sources")
		{
			sources.GET("", s.sourceService.ListSources)
			sources.GET("/enabled", s.sourceService.GetEnabledSources)
			sources.GET("/:id", s.sourceService.GetSource)
			sources.POST("", s.sourceService.CreateSource)
			sources.PUT("/:id", s.sourceService.UpdateSource)
			sources.DELETE("/:id", s.sourceService.DeleteSource)
		}

		// 应用管理
		apps := identity.Group("/apps")
		{
			apps.GET("", s.appService.ListApps)
			apps.GET("/templates", s.appService.GetTemplates)
			apps.GET("/categories", s.appService.GetCategories)
			apps.GET("/:id", s.appService.GetApp)
			apps.POST("", s.appService.CreateApp)
			apps.PUT("/:id", s.appService.UpdateApp)
			apps.DELETE("/:id", s.appService.DeleteApp)
		}

		// 应用门户
		portal := identity.Group("/portal")
		{
			portal.GET("/apps", s.portalService.GetPortalApps)
			portal.GET("/favorites", s.portalService.GetFavoriteApps)
			portal.POST("/access/:id", s.portalService.AccessApp)
			portal.POST("/favorite/:id", s.portalService.FavoriteApp)
		}

		// 凭证管理
		credentials := identity.Group("/credentials")
		{
			credentials.GET("", s.credentialService.ListCredentials)
			credentials.POST("", s.credentialService.CreateCredential)
			credentials.PUT("/:id", s.credentialService.UpdateCredential)
			credentials.DELETE("/:id", s.credentialService.DeleteCredential)
		}

		// 访问策略
		permissions := identity.Group("/permissions")
		{
			permissions.GET("", s.permissionService.ListPermissions)
			permissions.POST("", s.permissionService.CreatePermission)
			permissions.POST("/batch", s.permissionService.BatchCreatePermissions)
			permissions.DELETE("/:id", s.permissionService.DeletePermission)
			permissions.GET("/app/:id", s.permissionService.ListByApp)
		}

		// 认证日志
		logs := identity.Group("/logs")
		{
			logs.GET("", s.authLogService.ListLogs)
			logs.GET("/stats", s.authLogService.GetStats)
			logs.GET("/trend", s.authLogService.GetLoginTrend)
		}

		// LDAP管理
		if s.ldapService != nil {
			sources.POST("/:id/test", s.ldapService.TestConnection)
			sources.POST("/:id/sync", s.ldapService.SyncUsers)
			sources.GET("/:id/sync/jobs", s.ldapService.ListSyncJobs)
			sources.GET("/:id/sync/jobs/:jobId", s.ldapService.GetSyncStatus)
		}
	}
}

// RegisterOAuth2Routes 注册OAuth2服务端路由（需要在根路由注册）
func (s *HTTPServer) RegisterOAuth2Routes(router *gin.Engine, authMiddleware func() gin.HandlerFunc, optionalAuth func() gin.HandlerFunc) {
	// OAuth2 公开端点（不需要认证）
	oauth2 := router.Group("/oauth2")
	{
		// OIDC 发现端点
		oauth2.GET("/.well-known/openid-configuration", s.oauth2Service.Discovery)
		oauth2.GET("/jwks", s.oauth2Service.JWKS)

		// Token 端点（客户端认证，不需要用户认证）
		oauth2.POST("/token", s.oauth2Service.Token)

		// 用户信息端点（使用 OAuth2 access token 认证，在 handler 内部验证）
		oauth2.GET("/userinfo", s.oauth2Service.UserInfo)

		// Token 内省和撤销
		oauth2.POST("/introspect", s.oauth2Service.Introspect)
		oauth2.POST("/revoke", s.oauth2Service.Revoke)
	}

	// 授权端点 - 使用可选认证，未登录时重定向到登录页
	oauth2Optional := router.Group("/oauth2")
	oauth2Optional.Use(optionalAuth())
	{
		oauth2Optional.GET("/authorize", s.oauth2Service.Authorize)
	}

	// GitLab 兼容端点 - GitLab oauth2_generic 默认请求 /api/v4/user
	apiV4 := router.Group("/api/v4")
	{
		apiV4.GET("/user", s.oauth2Service.GitLabUserInfo)
	}
}

// GetOAuth2Service 获取OAuth2服务（供外部使用）
func (s *HTTPServer) GetOAuth2Service() *svcIdentity.OAuth2ServerService {
	return s.oauth2Service
}

// RegisterPublicRoutes 注册公开路由（不需要认证）
func (s *HTTPServer) RegisterPublicRoutes(router *gin.RouterGroup) {
	auth := router.Group("/auth")
	{
		// 获取启用的身份源提供商（用于登录页展示）
		auth.GET("/providers", s.sourceService.GetEnabledSources)
	}
}
