package messagequeue

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	mqbiz "github.com/ydcloud-dy/opshub/internal/biz/messagequeue"
	assetdata "github.com/ydcloud-dy/opshub/internal/data/asset"
	mqdata "github.com/ydcloud-dy/opshub/internal/data/messagequeue"
	systemdata "github.com/ydcloud-dy/opshub/internal/data/system"
	mqservice "github.com/ydcloud-dy/opshub/internal/service/messagequeue"
	rbacservice "github.com/ydcloud-dy/opshub/internal/service/rbac"
	"github.com/ydcloud-dy/opshub/pkg/response"
	"gorm.io/gorm"
)

const (
	permMQInstanceView     = "messagequeue:instance:view"
	permMQInstanceCreate   = "messagequeue:instance:create"
	permMQInstanceUpdate   = "messagequeue:instance:update"
	permMQInstanceDelete   = "messagequeue:instance:delete"
	permMQInstanceStatus   = "messagequeue:instance:status"
	permMQConnectionTest   = "messagequeue:connection:test"
	permMQMetadataView     = "messagequeue:metadata:view"
	permMQMetadataSync     = "messagequeue:metadata:sync"
	permMQDiagnosisView    = "messagequeue:diagnosis:view"
	permMQMessageRead      = "messagequeue:message:read"
	permMQMessageExport    = "messagequeue:message:export"
	permMQMessageWrite     = "messagequeue:message:write"
	permMQResourceManage   = "messagequeue:resource:manage"
	permMQHighRisk         = "messagequeue:operation:high-risk"
	permMQAuditView        = "messagequeue:audit:view"
	permMQAuditExport      = "messagequeue:audit:export"
	permMQPermissionManage = "messagequeue:permission:manage"
)

var uiPermissionCodes = map[string]string{
	"instanceCreate":   permMQInstanceCreate,
	"instanceUpdate":   permMQInstanceUpdate,
	"instanceDelete":   permMQInstanceDelete,
	"instanceStatus":   permMQInstanceStatus,
	"connectionTest":   permMQConnectionTest,
	"metadataSync":     permMQMetadataSync,
	"diagnosisView":    permMQDiagnosisView,
	"messageRead":      permMQMessageRead,
	"messageExport":    permMQMessageExport,
	"messageWrite":     permMQMessageWrite,
	"resourceManage":   permMQResourceManage,
	"highRisk":         permMQHighRisk,
	"auditView":        permMQAuditView,
	"auditExport":      permMQAuditExport,
	"permissionManage": permMQPermissionManage,
}

type HTTPServer struct {
	service        *mqservice.Service
	authMiddleware *rbacservice.AuthMiddleware
}

func NewHTTPServer(db *gorm.DB, authMiddleware *rbacservice.AuthMiddleware) *HTTPServer {
	_ = db.AutoMigrate(
		&mqbiz.MQInstance{},
		&mqbiz.MQInstancePermission{},
		&mqbiz.MQBroker{},
		&mqbiz.MQResource{},
		&mqbiz.MQBinding{},
		&mqbiz.MQConsumerGroup{},
		&mqbiz.MQPartition{},
		&mqbiz.MQSyncJob{},
		&mqbiz.MQJob{},
		&mqbiz.MQMetricSnapshot{},
		&mqbiz.MQOperationAudit{},
		&mqbiz.MQMessageAudit{},
		&mqbiz.MQDLQRecord{},
	)

	instanceRepo := mqdata.NewInstanceRepo(db)
	permissionRepo := mqdata.NewPermissionRepo(db)
	brokerRepo := mqdata.NewBrokerRepo(db)
	resourceRepo := mqdata.NewResourceRepo(db)
	bindingRepo := mqdata.NewBindingRepo(db)
	consumerGroupRepo := mqdata.NewConsumerGroupRepo(db)
	partitionRepo := mqdata.NewPartitionRepo(db)
	metadataRepo := mqdata.NewMetadataRepo(db)
	syncJobRepo := mqdata.NewSyncJobRepo(db)
	jobRepo := mqdata.NewJobRepo(db)
	metricSnapshotRepo := mqdata.NewMetricSnapshotRepo(db)
	operationAuditRepo := mqdata.NewOperationAuditRepo(db)
	messageAuditRepo := mqdata.NewMessageAuditRepo(db)
	dlqRecordRepo := mqdata.NewDLQRecordRepo(db)
	credentialRepo := assetdata.NewCredentialRepo(db)
	configRepo := systemdata.NewConfigRepo(db)

	useCase := mqbiz.NewUseCase(
		instanceRepo,
		permissionRepo,
		brokerRepo,
		resourceRepo,
		bindingRepo,
		consumerGroupRepo,
		partitionRepo,
		metadataRepo,
		syncJobRepo,
		metricSnapshotRepo,
		operationAuditRepo,
		messageAuditRepo,
		func(ctx context.Context, id uint) error {
			_, err := credentialRepo.GetByID(ctx, id)
			return err
		},
		func(ctx context.Context, id uint) (*mqbiz.ConnectionCredential, error) {
			credential, err := credentialRepo.GetByIDDecrypted(ctx, id)
			if err != nil {
				return nil, err
			}
			return &mqbiz.ConnectionCredential{
				Username:   credential.Username,
				Password:   credential.Password,
				PrivateKey: credential.PrivateKey,
				Passphrase: credential.Passphrase,
			}, nil
		},
		func(ctx context.Context) (*mqbiz.HighRiskOperationConfig, error) {
			enabled, err := readBoolConfig(ctx, configRepo, mqbiz.ConfigKeyMessageQueueHighRiskEnabled, false)
			if err != nil {
				return nil, err
			}
			reasonRequired, err := readBoolConfig(ctx, configRepo, mqbiz.ConfigKeyMessageQueueOperationReasonRequired, true)
			if err != nil {
				return nil, err
			}
			maxMetadataAgeMinutes, err := readIntConfig(ctx, configRepo, mqbiz.ConfigKeyMessageQueueOperationMaxMetadataAge, 30)
			if err != nil {
				return nil, err
			}
			maxMetricAgeMinutes, err := readIntConfig(ctx, configRepo, mqbiz.ConfigKeyMessageQueueOperationMaxMetricAge, 10)
			if err != nil {
				return nil, err
			}
			requireFreshMetric, err := readBoolConfig(ctx, configRepo, mqbiz.ConfigKeyMessageQueueRequireFreshMetric, true)
			if err != nil {
				return nil, err
			}
			return &mqbiz.HighRiskOperationConfig{
				Enabled:                    enabled,
				ReasonRequired:             reasonRequired,
				OperationMaxMetadataAge:    time.Duration(maxMetadataAgeMinutes) * time.Minute,
				OperationMaxMetricAge:      time.Duration(maxMetricAgeMinutes) * time.Minute,
				RequireFreshMetricHighRisk: requireFreshMetric,
			}, nil
		},
		mqbiz.NewDefaultAdapterRegistry(),
	).WithJobRepo(jobRepo).WithDLQRecordRepo(dlqRecordRepo)

	return &HTTPServer{
		service:        mqservice.NewService(useCase, permissionRepo, authMiddleware.HasMenuPermission),
		authMiddleware: authMiddleware,
	}
}

func readBoolConfig(ctx context.Context, repo *systemdata.ConfigRepo, key string, fallback bool) (bool, error) {
	if repo == nil {
		return fallback, nil
	}
	config, err := repo.GetByKey(ctx, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fallback, nil
		}
		return fallback, err
	}
	switch strings.ToLower(strings.TrimSpace(config.Value)) {
	case "true", "1", "yes", "on", "enabled":
		return true, nil
	case "false", "0", "no", "off", "disabled":
		return false, nil
	default:
		return fallback, nil
	}
}

func readIntConfig(ctx context.Context, repo *systemdata.ConfigRepo, key string, fallback int) (int, error) {
	if repo == nil {
		return fallback, nil
	}
	config, err := repo.GetByKey(ctx, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fallback, nil
		}
		return fallback, err
	}
	value, err := strconv.Atoi(strings.TrimSpace(config.Value))
	if err != nil || value <= 0 {
		return fallback, nil
	}
	return value, nil
}

func (s *HTTPServer) GetUIPermissions(c *gin.Context) {
	userID := rbacservice.GetUserID(c)
	if userID == 0 {
		response.ErrorCode(c, http.StatusUnauthorized, "未登录")
		return
	}
	result := gin.H{}
	for key, code := range uiPermissionCodes {
		ok, err := s.authMiddleware.HasMenuPermission(c.Request.Context(), userID, code)
		if err != nil {
			response.ErrorCode(c, http.StatusInternalServerError, "权限检查失败")
			return
		}
		result[key] = ok
	}
	response.Success(c, result)
}

func (s *HTTPServer) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/message-queues")
	{
		group.GET("/supported-types", s.authMiddleware.RequireMenuPermission(permMQInstanceView), s.service.GetSupportedTypes)
		group.GET("/ui-permissions", s.authMiddleware.RequireMenuPermission(permMQInstanceView), s.GetUIPermissions)
		group.GET("/instance-permissions", s.authMiddleware.RequireMenuPermission(permMQInstanceView), s.service.ListInstancePermissions)
		group.POST("/instance-permissions", s.authMiddleware.RequireMenuPermission(permMQPermissionManage), s.service.UpsertInstancePermission)
		group.DELETE("/instance-permissions/:id", s.authMiddleware.RequireMenuPermission(permMQPermissionManage), s.service.DeleteInstancePermission)
		group.GET("/operation-audits", s.authMiddleware.RequireMenuPermission(permMQAuditView), s.service.ListOperationAudits)
		group.GET("/message-audits", s.authMiddleware.RequireMenuPermission(permMQAuditView), s.service.ListMessageAudits)
		group.GET("/jobs", s.authMiddleware.RequireMenuPermission(permMQDiagnosisView), s.service.ListJobs)
		group.GET("/jobs/:id", s.authMiddleware.RequireMenuPermission(permMQDiagnosisView), s.service.GetJob)
		group.GET("/dashboard", s.authMiddleware.RequireMenuPermission(permMQDiagnosisView), s.service.GetProductionDashboard)
		group.GET("/governance-report", s.authMiddleware.RequireMenuPermission(permMQDiagnosisView), s.service.GetGovernanceReport)
		group.GET("/dlq-analysis", s.authMiddleware.RequireMenuPermission(permMQDiagnosisView), s.service.GetDLQAnalysis)
		group.POST("/dlq-records", s.authMiddleware.RequireMenuPermission(permMQDiagnosisView), s.service.UpsertDLQRecord)

		instances := group.Group("/instances")
		{
			instances.GET("", s.authMiddleware.RequireMenuPermission(permMQInstanceView), s.service.ListInstances)
			instances.POST("", s.authMiddleware.RequireMenuPermission(permMQInstanceCreate), s.service.CreateInstance)
			instances.GET("/:id", s.authMiddleware.RequireMenuPermission(permMQInstanceView), s.service.GetInstance)
			instances.PUT("/:id", s.authMiddleware.RequireMenuPermission(permMQInstanceUpdate), s.service.UpdateInstance)
			instances.DELETE("/:id", s.authMiddleware.RequireMenuPermission(permMQInstanceDelete), s.service.DeleteInstance)
			instances.POST("/:id/enable", s.authMiddleware.RequireMenuPermission(permMQInstanceStatus), s.service.EnableInstance)
			instances.POST("/:id/disable", s.authMiddleware.RequireMenuPermission(permMQInstanceStatus), s.service.DisableInstance)
			instances.POST("/:id/test", s.authMiddleware.RequireMenuPermission(permMQConnectionTest), s.service.TestInstance)
			instances.POST("/:id/sync-metadata", s.authMiddleware.RequireMenuPermission(permMQMetadataSync), s.service.SyncMetadata)
			instances.GET("/:id/overview", s.authMiddleware.RequireMenuPermission(permMQDiagnosisView), s.service.GetOverview)
			instances.GET("/:id/capabilities", s.authMiddleware.RequireMenuPermission(permMQInstanceView), s.service.GetCapabilities)
			instances.GET("/:id/topology", s.authMiddleware.RequireMenuPermission(permMQDiagnosisView), s.service.GetTopology)
			instances.POST("/:id/metric-snapshots", s.authMiddleware.RequireMenuPermission(permMQDiagnosisView), s.service.CollectMetricSnapshot)
			instances.GET("/:id/metric-snapshots", s.authMiddleware.RequireMenuPermission(permMQDiagnosisView), s.service.ListMetricSnapshots)
			instances.POST("/:id/inspection-reports", s.authMiddleware.RequireMenuPermission(permMQDiagnosisView), s.service.GenerateInspectionReport)
			instances.GET("/:id/brokers", s.authMiddleware.RequireMenuPermission(permMQDiagnosisView), s.service.ListBrokers)
			instances.GET("/:id/resources", s.authMiddleware.RequireMenuPermission(permMQMetadataView), s.service.ListResources)
			instances.GET("/:id/bindings", s.authMiddleware.RequireMenuPermission(permMQMetadataView), s.service.ListBindings)
			instances.GET("/:id/consumer-groups", s.authMiddleware.RequireMenuPermission(permMQDiagnosisView), s.service.ListConsumerGroups)
			instances.GET("/:id/partitions", s.authMiddleware.RequireMenuPermission(permMQDiagnosisView), s.service.ListPartitions)
			instances.POST("/:id/messages/sample", s.authMiddleware.RequireMenuPermission(permMQMessageRead), s.service.SampleMessages)
			instances.POST("/:id/messages/replay-requests", s.authMiddleware.RequireMenuPermission(permMQMessageWrite), s.service.PrepareMessageReplayApplication)
			instances.GET("/:id/operations/actions", s.authMiddleware.RequireMenuPermission(permMQResourceManage), s.service.ListOperationActions)
			instances.POST("/:id/operations/validate", s.authMiddleware.RequireMenuPermission(permMQResourceManage), s.service.ValidateResourceOperation)
			instances.POST("/:id/operations", s.authMiddleware.RequireMenuPermission(permMQResourceManage), s.service.ExecuteResourceOperation)
		}
	}
}
