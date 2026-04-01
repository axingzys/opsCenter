package server

import (
	"github.com/gin-gonic/gin"
	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/service"
)

// RegisterRoutes 注册路由
func RegisterRoutes(router *gin.RouterGroup, certSvc *service.CertificateService, dnsSvc *service.DNSProviderService, deploySvc *service.DeployService, taskSvc *service.TaskService) {
	// 证书管理
	certHandler := NewCertificateHandler(certSvc)
	certs := router.Group("/certificates")
	{
		certs.GET("", certHandler.List)
		certs.GET("/stats", certHandler.Stats)
		certs.GET("/cloud-accounts", certHandler.CloudAccounts)
		certs.GET("/:id", certHandler.Get)
		certs.POST("", certHandler.Create)
		certs.POST("/import", certHandler.Import)
		certs.PUT("/:id", certHandler.Update)
		certs.DELETE("/:id", certHandler.Delete)
		certs.POST("/:id/renew", certHandler.Renew)
		certs.POST("/:id/sync", certHandler.Sync)
		certs.GET("/:id/download", certHandler.Download)
	}

	// DNS Provider管理
	dnsHandler := NewDNSProviderHandler(dnsSvc)
	dnsProviders := router.Group("/dns-providers")
	{
		dnsProviders.GET("", dnsHandler.List)
		dnsProviders.GET("/all", dnsHandler.ListAll)
		dnsProviders.GET("/:id", dnsHandler.Get)
		dnsProviders.GET("/:id/detail", dnsHandler.GetDetail)
		dnsProviders.POST("", dnsHandler.Create)
		dnsProviders.PUT("/:id", dnsHandler.Update)
		dnsProviders.DELETE("/:id", dnsHandler.Delete)
		dnsProviders.POST("/:id/test", dnsHandler.Test)
	}

	// 部署配置管理
	deployHandler := NewDeployHandler(deploySvc)
	deployConfigs := router.Group("/deploy-configs")
	{
		deployConfigs.GET("", deployHandler.List)
		deployConfigs.GET("/:id", deployHandler.Get)
		deployConfigs.POST("", deployHandler.Create)
		deployConfigs.PUT("/:id", deployHandler.Update)
		deployConfigs.DELETE("/:id", deployHandler.Delete)
		deployConfigs.POST("/:id/deploy", deployHandler.Deploy)
		deployConfigs.POST("/:id/test", deployHandler.Test)
	}

	// 任务管理
	taskHandler := NewTaskHandler(taskSvc)
	tasks := router.Group("/tasks")
	{
		tasks.GET("", taskHandler.List)
		tasks.GET("/:id", taskHandler.Get)
	}
}
