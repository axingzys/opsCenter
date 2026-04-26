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

package asset

import (
	"github.com/gin-gonic/gin"
	assetbiz "github.com/ydcloud-dy/opshub/internal/biz/asset"
	rbacbiz "github.com/ydcloud-dy/opshub/internal/biz/rbac"
	"github.com/ydcloud-dy/opshub/internal/conf"
	assetdata "github.com/ydcloud-dy/opshub/internal/data/asset"
	rbacdata "github.com/ydcloud-dy/opshub/internal/data/rbac"
	assetService "github.com/ydcloud-dy/opshub/internal/service/asset"
	rbacService "github.com/ydcloud-dy/opshub/internal/service/rbac"
	"gorm.io/gorm"
)

type HTTPServer struct {
	assetGroupService     *assetService.AssetGroupService
	hostService           *assetService.HostService
	agentService          *assetService.AgentService
	desktopService        *assetService.DesktopService
	virtualizationService *assetService.VirtualizationService
	terminalManager       *TerminalManager
	terminalAuditHandler  *TerminalAuditHandler
	authMiddleware        *rbacService.AuthMiddleware
}

func NewHTTPServer(
	assetGroupService *assetService.AssetGroupService,
	hostService *assetService.HostService,
	agentService *assetService.AgentService,
	desktopService *assetService.DesktopService,
	virtualizationService *assetService.VirtualizationService,
	terminalManager *TerminalManager,
	terminalCfg conf.TerminalConfig,
	db *gorm.DB,
	authMiddleware *rbacService.AuthMiddleware,
) *HTTPServer {
	return &HTTPServer{
		assetGroupService:     assetGroupService,
		hostService:           hostService,
		agentService:          agentService,
		desktopService:        desktopService,
		virtualizationService: virtualizationService,
		terminalManager:       terminalManager,
		terminalAuditHandler:  NewTerminalAuditHandler(db, terminalCfg),
		authMiddleware:        authMiddleware,
	}
}

func (s *HTTPServer) RegisterRoutes(r *gin.RouterGroup) {
	// 资产分组管理
	groups := r.Group("/asset-groups")
	{
		groups.GET("/tree", s.assetGroupService.GetGroupTree)
		groups.GET("/parent-options", s.assetGroupService.GetParentOptions)
		groups.POST("", s.assetGroupService.CreateGroup)
		groups.GET("/:id", s.assetGroupService.GetGroup)
		groups.PUT("/:id", s.assetGroupService.UpdateGroup)
		groups.DELETE("/:id", s.assetGroupService.DeleteGroup)
	}

	// 主机管理
	hosts := r.Group("/hosts")
	{
		hosts.GET("", s.hostService.ListHosts)
		hosts.GET("/template/download", s.hostService.DownloadExcelTemplate)
		hosts.POST("/import", s.hostService.ImportFromExcel)
		hosts.POST("/batch-collect", s.hostService.BatchCollectHostInfo)
		hosts.POST("/batch-delete", s.hostService.BatchDeleteHosts)
		hosts.GET("/:id/trends",
			s.authMiddleware.RequireHostPermission(rbacbiz.PermissionView),
			s.hostService.GetMetricTrend)

		// 查看权限 - 查看主机详情
		hosts.GET("/:id",
			s.authMiddleware.RequireHostPermission(rbacbiz.PermissionView),
			s.hostService.GetHost)

		// 编辑权限 - 创建、修改主机配置
		hosts.POST("",
			s.authMiddleware.RequireHostPermission(rbacbiz.PermissionEdit),
			s.hostService.CreateHost)
		hosts.PUT("/:id",
			s.authMiddleware.RequireHostPermission(rbacbiz.PermissionEdit),
			s.hostService.UpdateHost)

		// 删除权限 - 删除主机
		hosts.DELETE("/:id",
			s.authMiddleware.RequireHostPermission(rbacbiz.PermissionDelete),
			s.hostService.DeleteHost)

		// 采集权限 - 采集主机信息
		hosts.POST("/:id/collect",
			s.authMiddleware.RequireHostPermission(rbacbiz.PermissionCollect),
			s.hostService.CollectHostInfo)
		hosts.POST("/:id/test", s.hostService.TestHostConnection)
		hosts.GET("/:id/agent/bootstrap",
			s.authMiddleware.RequireHostPermission(rbacbiz.PermissionEdit),
			s.agentService.GetHostBootstrap)
		hosts.POST("/:id/desktop/sessions",
			s.authMiddleware.RequireHostPermission(rbacbiz.PermissionDesktop),
			s.desktopService.CreateDesktopSession)

		// 文件管理权限 - 文件上传、下载、删除
		hosts.GET("/:id/files",
			s.authMiddleware.RequireHostPermission(rbacbiz.PermissionFile),
			s.hostService.ListHostFiles)
		hosts.POST("/:id/files/upload",
			s.authMiddleware.RequireHostPermission(rbacbiz.PermissionFile),
			s.hostService.UploadHostFile)
		hosts.GET("/:id/files/download",
			s.authMiddleware.RequireHostPermission(rbacbiz.PermissionFile),
			s.hostService.DownloadHostFile)
		hosts.DELETE("/:id/files",
			s.authMiddleware.RequireHostPermission(rbacbiz.PermissionFile),
			s.hostService.DeleteHostFile)
	}

	agents := r.Group("/agents")
	{
		agents.GET("", s.agentService.List)
		agents.POST("/deploy", s.agentService.Deploy)
		agents.POST("/uninstall", s.agentService.Uninstall)
		agents.GET("/jobs/:id", s.agentService.GetJob)
		agents.GET("/:id/inventory",
			s.authMiddleware.RequireHostPermission(rbacbiz.PermissionView),
			s.agentService.GetInventory)
	}

	// 凭证管理
	credentials := r.Group("/credentials")
	{
		credentials.GET("", s.hostService.ListCredentials)
		credentials.GET("/all", s.hostService.GetAllCredentials)
		credentials.GET("/:id", s.hostService.GetCredential)
		credentials.POST("", s.hostService.CreateCredential)
		credentials.PUT("/:id", s.hostService.UpdateCredential)
		credentials.DELETE("/:id", s.hostService.DeleteCredential)
	}

	// 云平台账号管理
	cloudAccounts := r.Group("/cloud-accounts")
	{
		cloudAccounts.GET("", s.hostService.ListCloudAccounts)
		cloudAccounts.GET("/all", s.hostService.GetAllCloudAccounts)
		cloudAccounts.GET("/:id", s.hostService.GetCloudAccount)
		cloudAccounts.GET("/:id/regions", s.hostService.GetCloudRegions)
		cloudAccounts.GET("/:id/instances", s.hostService.GetCloudInstances)
		cloudAccounts.POST("", s.hostService.CreateCloudAccount)
		cloudAccounts.PUT("/:id", s.hostService.UpdateCloudAccount)
		cloudAccounts.DELETE("/:id", s.hostService.DeleteCloudAccount)
		cloudAccounts.POST("/import", s.hostService.ImportFromCloud)
	}

	// 虚拟化平台管理（一期：平台/拓扑/纳管基础能力）
	virtualization := r.Group("/virtualization")
	virtualization.Use(s.authMiddleware.RequireAdmin())
	{
		virtualization.GET("/settings", s.virtualizationService.GetSettings)
		virtualization.PUT("/settings", s.virtualizationService.UpdateSettings)
		virtualization.GET("/action-logs", s.virtualizationService.ListActionLogs)

		platforms := virtualization.Group("/platforms")
		{
			platforms.GET("", s.virtualizationService.ListPlatforms)
			platforms.POST("", s.virtualizationService.CreatePlatform)
			platforms.GET("/:id", s.virtualizationService.GetPlatform)
			platforms.PUT("/:id", s.virtualizationService.UpdatePlatform)
			platforms.DELETE("/:id", s.virtualizationService.DeletePlatform)
			platforms.GET("/:id/trend", s.virtualizationService.GetPlatformTrend)
			platforms.POST("/:id/test", s.virtualizationService.TestPlatform)
			platforms.POST("/:id/sync", s.virtualizationService.SyncPlatform)
			platforms.GET("/:id/sync-jobs", s.virtualizationService.ListSyncJobs)
		}

		clusters := virtualization.Group("/clusters")
		{
			clusters.GET("/:id/trend", s.virtualizationService.GetClusterTrend)
		}

		virtualization.GET("/topology", s.virtualizationService.GetTopology)

		guests := virtualization.Group("/guests")
		{
			guests.GET("", s.virtualizationService.ListGuests)
			guests.GET("/:id", s.virtualizationService.GetGuest)
			guests.GET("/:id/precheck", s.virtualizationService.PrecheckGuestOnboard)
			guests.POST("/:id/console-link", s.virtualizationService.CreateGuestConsoleLink)
			guests.POST("/:id/power", s.virtualizationService.PowerGuest)
			guests.GET("/:id/snapshots", s.virtualizationService.ListGuestSnapshots)
			guests.POST("/:id/snapshots", s.virtualizationService.CreateGuestSnapshot)
			guests.POST("/:id/snapshots/rollback", s.virtualizationService.RollbackGuestSnapshot)
			guests.POST("/:id/snapshots/delete", s.virtualizationService.DeleteGuestSnapshot)
			guests.POST("/:id/onboard", s.virtualizationService.BindGuest)
			guests.POST("/:id/bind", s.virtualizationService.BindGuest)
			guests.POST("/:id/unbind", s.virtualizationService.UnbindGuest)
		}
	}

	// SSH终端 - 终端权限
	terminal := r.Group("/asset/terminal")
	{
		terminal.GET("/:id",
			s.authMiddleware.RequireHostPermission(rbacbiz.PermissionTerminal),
			s.HandleSSHConnection)
		terminal.POST("/:id/resize", s.ResizeTerminal)
	}

	// 终端审计
	terminalSessions := r.Group("/terminal-sessions")
	{
		terminalSessions.GET("",
			s.authMiddleware.RequireAdmin(),
			s.terminalAuditHandler.ListTerminalSessions)
		terminalSessions.GET("/:id/play",
			s.authMiddleware.RequireAdmin(),
			s.terminalAuditHandler.PlayTerminalSession)
		terminalSessions.GET("/:id/download",
			s.authMiddleware.RequireAdmin(),
			s.terminalAuditHandler.DownloadTerminalSession)
		terminalSessions.GET("/:id/events",
			s.authMiddleware.RequireAdmin(),
			s.terminalAuditHandler.ListTerminalSessionEvents)
		terminalSessions.DELETE("/:id",
			s.authMiddleware.RequireAdmin(),
			s.terminalAuditHandler.DeleteTerminalSession)
	}

	desktopSessions := r.Group("/desktop-sessions")
	{
		desktopSessions.GET("",
			s.authMiddleware.RequireAdmin(),
			s.desktopService.ListDesktopSessions)
		desktopSessions.GET("/:id/recording",
			s.authMiddleware.RequireAdmin(),
			s.desktopService.DownloadDesktopSessionRecording)
		desktopSessions.GET("/:id/recording/play",
			s.authMiddleware.RequireAdmin(),
			s.desktopService.PlayDesktopSessionRecording)
		desktopSessions.DELETE("/:id",
			s.authMiddleware.RequireAdmin(),
			s.desktopService.DeleteDesktopSession)
		desktopSessions.GET("/:id", s.desktopService.GetDesktopSession)
		desktopSessions.POST("/:id/files/upload", s.desktopService.UploadDesktopSessionFile)
		desktopSessions.POST("/:id/close", s.desktopService.CloseDesktopSession)
	}
}

func (s *HTTPServer) RegisterPublicRoutes(r *gin.RouterGroup) {
	agents := r.Group("/agents")
	{
		agents.GET("/install.ps1", s.agentService.InstallPowerShellScript)
		agents.GET("/install.sh", s.agentService.InstallShellScript)
		agents.GET("/download/:os/:arch", s.agentService.DownloadBinary)
		agents.GET("/echo-ip", s.agentService.EchoIP)
		agents.POST("/register", s.agentService.Register)
		agents.POST("/report", s.agentService.Report)
	}
}

// NewAssetServices 创建asset相关的服务
func NewAssetServices(db *gorm.DB, cfg *conf.Config) (
	*assetService.AssetGroupService,
	*assetService.HostService,
	*assetService.AgentService,
	*assetService.DesktopService,
	*assetService.VirtualizationService,
	*TerminalManager,
) {
	// 初始化Repository
	assetGroupRepo := assetdata.NewAssetGroupRepo(db)
	hostRepo := assetdata.NewHostRepo(db)
	credentialRepo := assetdata.NewCredentialRepo(db)
	cloudAccountRepo := assetdata.NewCloudAccountRepo(db)
	agentRepo := assetdata.NewAssetAgentRepo(db)
	hostInventoryRepo := assetdata.NewAssetHostInventoryRepo(db)
	publicIPHistoryRepo := assetdata.NewAssetHostPublicIPHistoryRepo(db)
	agentJobRepo := assetdata.NewAssetAgentJobRepo(db)
	desktopSessionRepo := assetdata.NewDesktopSessionRepo(db)
	virtualizationPlatformRepo := assetdata.NewVirtualizationPlatformRepo(db)
	virtualizationClusterRepo := assetdata.NewVirtualizationClusterRepo(db)
	virtualizationHostRepo := assetdata.NewVirtualizationHostRepo(db)
	virtualizationGuestRepo := assetdata.NewVirtualizationGuestRepo(db)
	virtualizationGuestBindingRepo := assetdata.NewVirtualizationGuestBindingRepo(db)
	virtualizationSyncJobRepo := assetdata.NewVirtualizationSyncJobRepo(db)
	virtualizationMetricRepo := assetdata.NewVirtualizationPlatformMetricRepo(db)
	virtualizationClusterMetricRepo := assetdata.NewVirtualizationClusterMetricRepo(db)
	virtualizationActionLogRepo := assetdata.NewVirtualizationActionLogRepo(db)
	virtualizationPolicyRepo := assetdata.NewVirtualizationPolicyRepo(db)
	assetPermissionRepo := rbacdata.NewAssetPermissionRepo(db)

	// 初始化UseCase
	assetGroupUseCase := assetbiz.NewAssetGroupUseCase(assetGroupRepo)
	credentialUseCase := assetbiz.NewCredentialUseCase(credentialRepo, hostRepo)
	cloudAccountUseCase := assetbiz.NewCloudAccountUseCase(cloudAccountRepo)
	hostUseCase := assetbiz.NewHostUseCase(hostRepo, credentialRepo, assetGroupRepo, cloudAccountRepo, agentRepo, hostInventoryRepo, cfg.Monitoring.Prometheus, []byte(cfg.Server.JWTSecret))
	agentUseCase := assetbiz.NewAgentUseCase(hostRepo, credentialRepo, agentRepo, hostInventoryRepo, publicIPHistoryRepo, agentJobRepo, cfg.Server.JWTSecret, cfg.Agent, cfg.Monitoring.Prometheus)
	desktopSessionUseCase := assetbiz.NewDesktopSessionUseCase(hostRepo, credentialRepo, desktopSessionRepo, cfg.Desktop)
	virtualizationUseCase := assetbiz.NewVirtualizationUseCase(
		virtualizationPlatformRepo,
		virtualizationClusterRepo,
		virtualizationHostRepo,
		virtualizationGuestRepo,
		virtualizationGuestBindingRepo,
		virtualizationSyncJobRepo,
		virtualizationMetricRepo,
		virtualizationClusterMetricRepo,
		virtualizationActionLogRepo,
		virtualizationPolicyRepo,
		hostRepo,
	)
	assetPermissionUseCase := rbacbiz.NewAssetPermissionUseCase(assetPermissionRepo)

	// 初始化Service
	assetGroupService := assetService.NewAssetGroupService(assetGroupUseCase)
	hostService := assetService.NewHostService(hostUseCase, credentialUseCase, cloudAccountUseCase, assetPermissionUseCase)
	agentService := assetService.NewAgentService(agentUseCase, assetPermissionUseCase, cfg)
	desktopService := assetService.NewDesktopService(desktopSessionUseCase)
	virtualizationService := assetService.NewVirtualizationService(virtualizationUseCase)

	// 初始化TerminalManager
	recordingStore := newTerminalRecordingStore(cfg.Terminal)
	terminalManager := NewTerminalManager(hostUseCase, db, recordingStore)

	return assetGroupService, hostService, agentService, desktopService, virtualizationService, terminalManager
}
