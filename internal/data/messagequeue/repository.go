package messagequeue

import (
	"context"
	"errors"
	"strings"

	mqbiz "github.com/ydcloud-dy/opshub/internal/biz/messagequeue"
	"gorm.io/gorm"
)

type instanceRepo struct {
	db *gorm.DB
}

func NewInstanceRepo(db *gorm.DB) mqbiz.InstanceRepo {
	return &instanceRepo{db: db}
}

func (r *instanceRepo) Create(ctx context.Context, item *mqbiz.MQInstance) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *instanceRepo) Update(ctx context.Context, item *mqbiz.MQInstance) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *instanceRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&mqbiz.MQInstance{}, id).Error
}

func (r *instanceRepo) GetByID(ctx context.Context, id uint) (*mqbiz.MQInstance, error) {
	var item mqbiz.MQInstance
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *instanceRepo) List(ctx context.Context, req *mqbiz.InstanceListRequest) ([]*mqbiz.MQInstance, int64, error) {
	var (
		items []*mqbiz.MQInstance
		total int64
	)
	query := r.db.WithContext(ctx).Model(&mqbiz.MQInstance{})
	if req != nil {
		query = applyAllowedInstanceFilter(query, "id", req.RestrictToAllowed, req.AllowedIDs)
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			query = query.Where("name LIKE ? OR endpoint LIKE ? OR management_url LIKE ? OR business_system LIKE ? OR owner LIKE ?", like, like, like, like, like)
		}
		if req.MQType != "" {
			query = query.Where("mq_type = ?", req.MQType)
		}
		if req.Status != "" {
			query = query.Where("status = ?", req.Status)
		}
		if req.HealthStatus != "" {
			query = query.Where("health_status = ?", req.HealthStatus)
		}
		if req.Environment != "" {
			query = query.Where("environment = ?", req.Environment)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := pageParams(req)
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

type brokerRepo struct {
	db *gorm.DB
}

func NewBrokerRepo(db *gorm.DB) mqbiz.BrokerRepo {
	return &brokerRepo{db: db}
}

func (r *brokerRepo) ListByInstanceID(ctx context.Context, instanceID uint) ([]*mqbiz.MQBroker, error) {
	var items []*mqbiz.MQBroker
	err := r.db.WithContext(ctx).Where("instance_id = ?", instanceID).Order("broker_name ASC").Find(&items).Error
	return items, err
}

func (r *brokerRepo) CountByInstanceID(ctx context.Context, instanceID uint) (int64, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&mqbiz.MQBroker{}).Where("instance_id = ?", instanceID).Count(&total).Error; err != nil {
		return 0, 0, err
	}
	var online int64
	if err := r.db.WithContext(ctx).Model(&mqbiz.MQBroker{}).
		Where("instance_id = ? AND status IN ?", instanceID, []string{"online", "running", "active"}).
		Count(&online).Error; err != nil {
		return 0, 0, err
	}
	return total, online, nil
}

type resourceRepo struct {
	db *gorm.DB
}

func NewResourceRepo(db *gorm.DB) mqbiz.ResourceRepo {
	return &resourceRepo{db: db}
}

func (r *resourceRepo) List(ctx context.Context, instanceID uint, req *mqbiz.ResourceListRequest) ([]*mqbiz.MQResource, int64, error) {
	var (
		items []*mqbiz.MQResource
		total int64
	)
	query := r.db.WithContext(ctx).Where("instance_id = ?", instanceID).Model(&mqbiz.MQResource{})
	if req != nil {
		if req.ResourceType != "" {
			query = query.Where("resource_type = ?", req.ResourceType)
		}
		if req.Namespace != "" {
			query = query.Where("namespace = ?", req.Namespace)
		}
		if req.HasBacklog == "true" {
			query = query.Where("backlog > 0")
		}
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			query = query.Where("name LIKE ? OR full_name LIKE ? OR namespace LIKE ?", like, like, like)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := pageParams(req)
	if err := query.Order("backlog DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *resourceRepo) GetByUnique(ctx context.Context, instanceID uint, resourceType, namespace, name string) (*mqbiz.MQResource, error) {
	var item mqbiz.MQResource
	err := r.db.WithContext(ctx).
		Where("instance_id = ? AND resource_type = ? AND namespace = ? AND name = ?", instanceID, resourceType, namespace, name).
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *resourceRepo) TopBacklog(ctx context.Context, instanceID uint, limit int) ([]*mqbiz.MQResource, error) {
	if limit <= 0 {
		limit = 10
	}
	var items []*mqbiz.MQResource
	err := r.db.WithContext(ctx).Where("instance_id = ? AND backlog > 0", instanceID).Order("backlog DESC").Limit(limit).Find(&items).Error
	return items, err
}

func (r *resourceRepo) Summary(ctx context.Context, instanceID uint) (*mqbiz.ResourceSummary, error) {
	var row mqbiz.ResourceSummary
	err := r.db.WithContext(ctx).Model(&mqbiz.MQResource{}).
		Select(`COUNT(*) AS count,
			COALESCE(SUM(message_count),0) AS message_count,
			COALESCE(SUM(backlog),0) AS backlog,
			COALESCE(SUM(produced_rate),0) AS produced_rate,
			COALESCE(SUM(consumed_rate),0) AS consumed_rate,
			COALESCE(SUM(CASE WHEN resource_type IN ('queue','topic') AND consumer_count = 0 THEN 1 ELSE 0 END),0) AS no_consumer_resource_count,
			COALESCE(SUM(CASE WHEN LOWER(name) LIKE '%dlq%' OR LOWER(name) LIKE '%dead%' OR LOWER(name) LIKE '%dead-letter%' OR LOWER(name) LIKE '%dead_letter%' THEN 1 ELSE 0 END),0) AS dlq_resource_count,
			COALESCE(SUM(CASE WHEN LOWER(name) LIKE '%retry%' OR LOWER(name) LIKE '%reconsume%' THEN 1 ELSE 0 END),0) AS retry_resource_count`).
		Where("instance_id = ?", instanceID).
		Scan(&row).Error
	return &row, err
}

type bindingRepo struct {
	db *gorm.DB
}

func NewBindingRepo(db *gorm.DB) mqbiz.BindingRepo {
	return &bindingRepo{db: db}
}

func (r *bindingRepo) ListByInstanceID(ctx context.Context, instanceID uint) ([]*mqbiz.MQBinding, error) {
	var items []*mqbiz.MQBinding
	err := r.db.WithContext(ctx).Where("instance_id = ?", instanceID).Order("vhost ASC, source ASC, destination ASC").Find(&items).Error
	return items, err
}

type consumerGroupRepo struct {
	db *gorm.DB
}

func NewConsumerGroupRepo(db *gorm.DB) mqbiz.ConsumerGroupRepo {
	return &consumerGroupRepo{db: db}
}

func (r *consumerGroupRepo) List(ctx context.Context, instanceID uint, req *mqbiz.ConsumerGroupListRequest) ([]*mqbiz.MQConsumerGroup, int64, error) {
	var (
		items []*mqbiz.MQConsumerGroup
		total int64
	)
	query := r.db.WithContext(ctx).Where("instance_id = ?", instanceID).Model(&mqbiz.MQConsumerGroup{})
	if req != nil {
		if req.Namespace != "" {
			query = query.Where("namespace = ?", req.Namespace)
		}
		if req.ResourceName != "" {
			query = query.Where("resource_name = ?", req.ResourceName)
		}
		if req.HasLag == "true" {
			query = query.Where("`lag` > 0 OR backlog > 0")
		}
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			query = query.Where("group_name LIKE ? OR resource_name LIKE ? OR namespace LIKE ?", like, like, like)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := pageParams(req)
	if err := query.Order("`lag` DESC, backlog DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *consumerGroupRepo) GetByUnique(ctx context.Context, instanceID uint, namespace, resourceName, groupName string) (*mqbiz.MQConsumerGroup, error) {
	var item mqbiz.MQConsumerGroup
	query := r.db.WithContext(ctx).Where("instance_id = ? AND group_name = ?", instanceID, groupName)
	if namespace != "" {
		query = query.Where("namespace = ?", namespace)
	}
	if resourceName != "" {
		query = query.Where("resource_name = ?", resourceName)
	}
	err := query.Order("id DESC").First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *consumerGroupRepo) Summary(ctx context.Context, instanceID uint) (*mqbiz.ConsumerGroupSummary, error) {
	var row mqbiz.ConsumerGroupSummary
	err := r.db.WithContext(ctx).Model(&mqbiz.MQConsumerGroup{}).
		Select("COUNT(*) AS count, COALESCE(SUM(`lag`),0) AS `lag`, COALESCE(SUM(backlog),0) AS backlog").
		Where("instance_id = ?", instanceID).
		Scan(&row).Error
	return &row, err
}

type partitionRepo struct {
	db *gorm.DB
}

func NewPartitionRepo(db *gorm.DB) mqbiz.PartitionRepo {
	return &partitionRepo{db: db}
}

func (r *partitionRepo) List(ctx context.Context, instanceID uint, req *mqbiz.PartitionListRequest) ([]*mqbiz.MQPartition, int64, error) {
	var (
		items []*mqbiz.MQPartition
		total int64
	)
	query := r.db.WithContext(ctx).Where("instance_id = ?", instanceID).Model(&mqbiz.MQPartition{})
	if req != nil && req.ResourceName != "" {
		query = query.Where("resource_name = ?", req.ResourceName)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := pageParams(req)
	if err := query.Order("resource_name ASC, partition_id ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *partitionRepo) CountByInstanceID(ctx context.Context, instanceID uint) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&mqbiz.MQPartition{}).Where("instance_id = ?", instanceID).Count(&total).Error
	return total, err
}

type metadataRepo struct {
	db *gorm.DB
}

func NewMetadataRepo(db *gorm.DB) mqbiz.MetadataRepo {
	return &metadataRepo{db: db}
}

func (r *metadataRepo) ReplaceAll(ctx context.Context, instanceID uint, snapshot *mqbiz.MQMetadataSnapshot) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("instance_id = ?", instanceID).Delete(&mqbiz.MQPartition{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("instance_id = ?", instanceID).Delete(&mqbiz.MQConsumerGroup{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("instance_id = ?", instanceID).Delete(&mqbiz.MQBinding{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("instance_id = ?", instanceID).Delete(&mqbiz.MQResource{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("instance_id = ?", instanceID).Delete(&mqbiz.MQBroker{}).Error; err != nil {
			return err
		}
		if snapshot == nil {
			return nil
		}
		if len(snapshot.Brokers) > 0 {
			if err := tx.CreateInBatches(snapshot.Brokers, 200).Error; err != nil {
				return err
			}
		}
		if len(snapshot.Resources) > 0 {
			if err := tx.CreateInBatches(snapshot.Resources, 500).Error; err != nil {
				return err
			}
		}
		if len(snapshot.Bindings) > 0 {
			if err := tx.CreateInBatches(snapshot.Bindings, 1000).Error; err != nil {
				return err
			}
		}
		if len(snapshot.ConsumerGroups) > 0 {
			if err := tx.CreateInBatches(snapshot.ConsumerGroups, 500).Error; err != nil {
				return err
			}
		}
		if len(snapshot.Partitions) > 0 {
			if err := tx.CreateInBatches(snapshot.Partitions, 1000).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

type syncJobRepo struct {
	db *gorm.DB
}

func NewSyncJobRepo(db *gorm.DB) mqbiz.SyncJobRepo {
	return &syncJobRepo{db: db}
}

func (r *syncJobRepo) Create(ctx context.Context, item *mqbiz.MQSyncJob) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *syncJobRepo) Update(ctx context.Context, item *mqbiz.MQSyncJob) error {
	return r.db.WithContext(ctx).Save(item).Error
}

type jobRepo struct {
	db *gorm.DB
}

func NewJobRepo(db *gorm.DB) mqbiz.JobRepo {
	return &jobRepo{db: db}
}

func (r *jobRepo) Create(ctx context.Context, item *mqbiz.MQJob) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *jobRepo) Update(ctx context.Context, item *mqbiz.MQJob) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *jobRepo) GetByID(ctx context.Context, id uint) (*mqbiz.MQJob, error) {
	var item mqbiz.MQJob
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *jobRepo) List(ctx context.Context, req *mqbiz.JobListRequest) ([]*mqbiz.MQJob, int64, error) {
	var (
		items []*mqbiz.MQJob
		total int64
	)
	query := r.db.WithContext(ctx).Model(&mqbiz.MQJob{})
	if req != nil {
		query = applyAllowedInstanceFilter(query, "instance_id", req.RestrictToAllowed, req.AllowedIDs)
		if req.InstanceID > 0 {
			query = query.Where("instance_id = ?", req.InstanceID)
		}
		if req.MQType != "" {
			query = query.Where("mq_type = ?", req.MQType)
		}
		if req.JobType != "" {
			query = query.Where("job_type = ?", req.JobType)
		}
		if req.Status != "" {
			query = query.Where("status = ?", req.Status)
		}
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			query = query.Where("instance_name LIKE ? OR operator_name LIKE ? OR current_stage LIKE ? OR message LIKE ? OR correlation_id LIKE ?", like, like, like, like, like)
		}
		if start, ok := parseTime(req.StartTime); ok {
			query = query.Where("created_at >= ?", start)
		}
		if end, ok := parseTime(req.EndTime); ok {
			query = query.Where("created_at <= ?", end)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := pageParams(req)
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

type metricSnapshotRepo struct {
	db *gorm.DB
}

func NewMetricSnapshotRepo(db *gorm.DB) mqbiz.MetricSnapshotRepo {
	return &metricSnapshotRepo{db: db}
}

func (r *metricSnapshotRepo) Create(ctx context.Context, item *mqbiz.MQMetricSnapshot) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *metricSnapshotRepo) List(ctx context.Context, instanceID uint, req *mqbiz.MetricSnapshotListRequest) ([]*mqbiz.MQMetricSnapshot, int64, error) {
	var (
		items []*mqbiz.MQMetricSnapshot
		total int64
	)
	query := r.db.WithContext(ctx).Where("instance_id = ?", instanceID).Model(&mqbiz.MQMetricSnapshot{})
	if req != nil {
		if req.ResourceType != "" {
			query = query.Where("resource_type = ?", req.ResourceType)
		}
		if req.ResourceName != "" {
			query = query.Where("resource_name = ?", req.ResourceName)
		}
		if start, ok := parseTime(req.StartTime); ok {
			query = query.Where("collected_at >= ?", start)
		}
		if end, ok := parseTime(req.EndTime); ok {
			query = query.Where("collected_at <= ?", end)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := pageParams(req)
	if err := query.Order("collected_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *metricSnapshotRepo) Latest(ctx context.Context, instanceID uint) (*mqbiz.MQMetricSnapshot, error) {
	var item mqbiz.MQMetricSnapshot
	err := r.db.WithContext(ctx).
		Where("instance_id = ? AND resource_type = ?", instanceID, "instance").
		Order("collected_at DESC, id DESC").
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func applyAllowedInstanceFilter(query *gorm.DB, column string, restrict bool, allowedIDs []uint) *gorm.DB {
	if !restrict {
		return query
	}
	if len(allowedIDs) == 0 {
		return query.Where("1 = 0")
	}
	return query.Where(column+" IN ?", allowedIDs)
}

func pageParams(req any) (int, int) {
	page := 1
	pageSize := 10
	switch item := req.(type) {
	case *mqbiz.InstanceListRequest:
		page, pageSize = item.Page, item.PageSize
	case *mqbiz.ResourceListRequest:
		page, pageSize = item.Page, item.PageSize
	case *mqbiz.ConsumerGroupListRequest:
		page, pageSize = item.Page, item.PageSize
	case *mqbiz.PartitionListRequest:
		page, pageSize = item.Page, item.PageSize
	case *mqbiz.AuditListRequest:
		page, pageSize = item.Page, item.PageSize
	case *mqbiz.MetricSnapshotListRequest:
		page, pageSize = item.Page, item.PageSize
	case *mqbiz.JobListRequest:
		page, pageSize = item.Page, item.PageSize
	case *mqbiz.InstancePermissionListRequest:
		page, pageSize = item.Page, item.PageSize
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	return page, pageSize
}
