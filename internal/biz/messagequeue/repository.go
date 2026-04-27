package messagequeue

import "context"

type InstanceRepo interface {
	Create(ctx context.Context, item *MQInstance) error
	Update(ctx context.Context, item *MQInstance) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*MQInstance, error)
	List(ctx context.Context, req *InstanceListRequest) ([]*MQInstance, int64, error)
}

type PermissionRepo interface {
	HasAnyRules(ctx context.Context) (bool, error)
	IsAdmin(ctx context.Context, userID uint) (bool, error)
	GetUserInstancePermissions(ctx context.Context, userID, instanceID uint) (uint, error)
	GetUserAccessibleInstanceIDs(ctx context.Context, userID uint, required uint) ([]uint, error)
	List(ctx context.Context, req *InstancePermissionListRequest) ([]*InstancePermissionVO, int64, error)
	GetByID(ctx context.Context, id uint) (*InstancePermissionVO, error)
	GetByRoleInstance(ctx context.Context, roleID, instanceID uint) (*InstancePermissionVO, error)
	ValidateTarget(ctx context.Context, roleID, instanceID uint) error
	Upsert(ctx context.Context, item *MQInstancePermission) error
	Delete(ctx context.Context, id uint) error
}

type BrokerRepo interface {
	ListByInstanceID(ctx context.Context, instanceID uint) ([]*MQBroker, error)
	CountByInstanceID(ctx context.Context, instanceID uint) (int64, int64, error)
}

type ResourceRepo interface {
	List(ctx context.Context, instanceID uint, req *ResourceListRequest) ([]*MQResource, int64, error)
	TopBacklog(ctx context.Context, instanceID uint, limit int) ([]*MQResource, error)
	Summary(ctx context.Context, instanceID uint) (*ResourceSummary, error)
}

type BindingRepo interface {
	ListByInstanceID(ctx context.Context, instanceID uint) ([]*MQBinding, error)
}

type ConsumerGroupRepo interface {
	List(ctx context.Context, instanceID uint, req *ConsumerGroupListRequest) ([]*MQConsumerGroup, int64, error)
	Summary(ctx context.Context, instanceID uint) (*ConsumerGroupSummary, error)
}

type PartitionRepo interface {
	List(ctx context.Context, instanceID uint, req *PartitionListRequest) ([]*MQPartition, int64, error)
	CountByInstanceID(ctx context.Context, instanceID uint) (int64, error)
}

type MetadataRepo interface {
	ReplaceAll(ctx context.Context, instanceID uint, snapshot *MQMetadataSnapshot) error
}

type SyncJobRepo interface {
	Create(ctx context.Context, item *MQSyncJob) error
	Update(ctx context.Context, item *MQSyncJob) error
}

type OperationAuditRepo interface {
	Create(ctx context.Context, item *MQOperationAudit) error
	Update(ctx context.Context, item *MQOperationAudit) error
	List(ctx context.Context, req *AuditListRequest) ([]*MQOperationAudit, int64, error)
}

type MessageAuditRepo interface {
	Create(ctx context.Context, item *MQMessageAudit) error
	List(ctx context.Context, req *AuditListRequest) ([]*MQMessageAudit, int64, error)
}

type ResourceSummary struct {
	Count        int64
	MessageCount int64
	Backlog      int64
	ProducedRate float64
	ConsumedRate float64
}

type ConsumerGroupSummary struct {
	Count   int64
	Lag     int64
	Backlog int64
}
