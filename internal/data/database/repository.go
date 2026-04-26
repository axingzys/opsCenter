package database

import (
	"context"
	"strings"

	dbbiz "github.com/ydcloud-dy/opshub/internal/biz/database"
	"gorm.io/gorm"
)

type instanceRepo struct {
	db *gorm.DB
}

func NewInstanceRepo(db *gorm.DB) dbbiz.InstanceRepo {
	return &instanceRepo{db: db}
}

func (r *instanceRepo) Create(ctx context.Context, item *dbbiz.DatabaseInstance) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *instanceRepo) Update(ctx context.Context, item *dbbiz.DatabaseInstance) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *instanceRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&dbbiz.DatabaseInstance{}, id).Error
}

func (r *instanceRepo) GetByID(ctx context.Context, id uint) (*dbbiz.DatabaseInstance, error) {
	var item dbbiz.DatabaseInstance
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *instanceRepo) List(ctx context.Context, req *dbbiz.DatabaseInstanceListRequest) ([]*dbbiz.DatabaseInstance, int64, error) {
	var (
		items []*dbbiz.DatabaseInstance
		total int64
	)

	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseInstance{})
	if req != nil {
		query = applyAllowedInstanceFilter(query, "id", req.RestrictToAllowed, req.AllowedIDs)
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			query = query.Where("name LIKE ? OR host LIKE ? OR default_database LIKE ? OR business_system LIKE ? OR owner LIKE ?", like, like, like, like, like)
		}
		if dbType := strings.TrimSpace(req.DBType); dbType != "" {
			query = query.Where("db_type = ?", dbType)
		}
		if status := strings.TrimSpace(req.Status); status != "" {
			query = query.Where("status = ?", status)
		}
		if environment := strings.TrimSpace(req.Environment); environment != "" {
			query = query.Where("environment = ?", environment)
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := 1
	pageSize := 10
	if req != nil {
		page = req.Page
		pageSize = req.PageSize
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *instanceRepo) ListEnabled(ctx context.Context) ([]*dbbiz.DatabaseInstance, error) {
	var items []*dbbiz.DatabaseInstance
	if err := r.db.WithContext(ctx).
		Where("status = ?", dbbiz.DatabaseInstanceStatusEnabled).
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

type schemaRepo struct {
	db *gorm.DB
}

func NewSchemaRepo(db *gorm.DB) dbbiz.SchemaRepo {
	return &schemaRepo{db: db}
}

func (r *schemaRepo) ListByInstanceID(ctx context.Context, instanceID uint) ([]*dbbiz.DatabaseSchema, error) {
	var items []*dbbiz.DatabaseSchema
	if err := r.db.WithContext(ctx).Where("instance_id = ?", instanceID).Order("schema_name ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

type tableRepo struct {
	db *gorm.DB
}

func NewTableRepo(db *gorm.DB) dbbiz.TableRepo {
	return &tableRepo{db: db}
}

func (r *tableRepo) List(ctx context.Context, instanceID uint, schemaName string) ([]*dbbiz.DatabaseTable, error) {
	var items []*dbbiz.DatabaseTable
	query := r.db.WithContext(ctx).Where("instance_id = ?", instanceID)
	if strings.TrimSpace(schemaName) != "" {
		query = query.Where("schema_name = ?", strings.TrimSpace(schemaName))
	}
	if err := query.Order("schema_name ASC, table_name ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *tableRepo) Get(ctx context.Context, instanceID uint, schemaName, tableName string) (*dbbiz.DatabaseTable, error) {
	var item dbbiz.DatabaseTable
	query := r.db.WithContext(ctx).Where("instance_id = ? AND table_name = ?", instanceID, strings.TrimSpace(tableName))
	if strings.TrimSpace(schemaName) != "" {
		query = query.Where("schema_name = ?", strings.TrimSpace(schemaName))
	}
	if err := query.First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

type columnRepo struct {
	db *gorm.DB
}

func NewColumnRepo(db *gorm.DB) dbbiz.ColumnRepo {
	return &columnRepo{db: db}
}

func (r *columnRepo) List(ctx context.Context, instanceID uint, schemaName, tableName string) ([]*dbbiz.DatabaseColumn, error) {
	var items []*dbbiz.DatabaseColumn
	query := r.db.WithContext(ctx).Where("instance_id = ?", instanceID)
	if strings.TrimSpace(schemaName) != "" {
		query = query.Where("schema_name = ?", strings.TrimSpace(schemaName))
	}
	if strings.TrimSpace(tableName) != "" {
		query = query.Where("table_name = ?", strings.TrimSpace(tableName))
	}
	if err := query.Order("schema_name ASC, table_name ASC, ordinal_position ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

type indexRepo struct {
	db *gorm.DB
}

func NewIndexRepo(db *gorm.DB) dbbiz.IndexRepo {
	return &indexRepo{db: db}
}

func (r *indexRepo) List(ctx context.Context, instanceID uint, schemaName, tableName string) ([]*dbbiz.DatabaseIndex, error) {
	var items []*dbbiz.DatabaseIndex
	query := r.db.WithContext(ctx).Where("instance_id = ?", instanceID)
	if strings.TrimSpace(schemaName) != "" {
		query = query.Where("schema_name = ?", strings.TrimSpace(schemaName))
	}
	if strings.TrimSpace(tableName) != "" {
		query = query.Where("table_name = ?", strings.TrimSpace(tableName))
	}
	if err := query.Order("schema_name ASC, table_name ASC, index_name ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

type metadataRepo struct {
	db *gorm.DB
}

func NewMetadataRepo(db *gorm.DB) dbbiz.MetadataRepo {
	return &metadataRepo{db: db}
}

func (r *metadataRepo) ReplaceAll(ctx context.Context, instanceID uint, schemas []*dbbiz.DatabaseSchema, tables []*dbbiz.DatabaseTable, columns []*dbbiz.DatabaseColumn, indexes []*dbbiz.DatabaseIndex) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("instance_id = ?", instanceID).Delete(&dbbiz.DatabaseIndex{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("instance_id = ?", instanceID).Delete(&dbbiz.DatabaseColumn{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("instance_id = ?", instanceID).Delete(&dbbiz.DatabaseTable{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("instance_id = ?", instanceID).Delete(&dbbiz.DatabaseSchema{}).Error; err != nil {
			return err
		}

		if len(schemas) > 0 {
			if err := tx.CreateInBatches(schemas, 200).Error; err != nil {
				return err
			}
		}
		if len(tables) > 0 {
			if err := tx.CreateInBatches(tables, 500).Error; err != nil {
				return err
			}
		}
		if len(columns) > 0 {
			if err := tx.CreateInBatches(columns, 1000).Error; err != nil {
				return err
			}
		}
		if len(indexes) > 0 {
			if err := tx.CreateInBatches(indexes, 1000).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

type redisMetadataRepo struct {
	db *gorm.DB
}

func NewRedisMetadataRepo(db *gorm.DB) dbbiz.RedisMetadataRepo {
	return &redisMetadataRepo{db: db}
}

func (r *redisMetadataRepo) ReplaceAll(ctx context.Context, instanceID uint, keyspaces []*dbbiz.DatabaseRedisKeyspace, keys []*dbbiz.DatabaseRedisKeySample) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("instance_id = ?", instanceID).Delete(&dbbiz.DatabaseRedisKeySample{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("instance_id = ?", instanceID).Delete(&dbbiz.DatabaseRedisKeyspace{}).Error; err != nil {
			return err
		}
		if len(keyspaces) > 0 {
			if err := tx.CreateInBatches(keyspaces, 200).Error; err != nil {
				return err
			}
		}
		if len(keys) > 0 {
			if err := tx.CreateInBatches(keys, 500).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *redisMetadataRepo) ListKeyspaces(ctx context.Context, instanceID uint) ([]*dbbiz.DatabaseRedisKeyspace, error) {
	var items []*dbbiz.DatabaseRedisKeyspace
	if err := r.db.WithContext(ctx).
		Where("instance_id = ?", instanceID).
		Order("db_index ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *redisMetadataRepo) ListKeys(ctx context.Context, instanceID uint, dbIndex int) ([]*dbbiz.DatabaseRedisKeySample, error) {
	var items []*dbbiz.DatabaseRedisKeySample
	if err := r.db.WithContext(ctx).
		Where("instance_id = ? AND db_index = ?", instanceID, dbIndex).
		Order("memory_usage_bytes DESC, key_name ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *redisMetadataRepo) GetKey(ctx context.Context, instanceID uint, dbIndex int, keyName string) (*dbbiz.DatabaseRedisKeySample, error) {
	var item dbbiz.DatabaseRedisKeySample
	if err := r.db.WithContext(ctx).
		Where("instance_id = ? AND db_index = ? AND key_name = ?", instanceID, dbIndex, strings.TrimSpace(keyName)).
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

type syncJobRepo struct {
	db *gorm.DB
}

func NewSyncJobRepo(db *gorm.DB) dbbiz.SyncJobRepo {
	return &syncJobRepo{db: db}
}

func (r *syncJobRepo) Create(ctx context.Context, item *dbbiz.DatabaseSyncJob) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *syncJobRepo) Update(ctx context.Context, item *dbbiz.DatabaseSyncJob) error {
	return r.db.WithContext(ctx).Save(item).Error
}

type queryAuditRepo struct {
	db *gorm.DB
}

func NewQueryAuditRepo(db *gorm.DB) dbbiz.QueryAuditRepo {
	return &queryAuditRepo{db: db}
}

func (r *queryAuditRepo) Create(ctx context.Context, item *dbbiz.DatabaseQueryAudit) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *queryAuditRepo) Update(ctx context.Context, item *dbbiz.DatabaseQueryAudit) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *queryAuditRepo) List(ctx context.Context, req *dbbiz.DatabaseQueryAuditListRequest) ([]*dbbiz.DatabaseQueryAudit, int64, error) {
	var (
		items []*dbbiz.DatabaseQueryAudit
		total int64
	)
	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseQueryAudit{})
	if req != nil {
		query = applyAllowedInstanceFilter(query, "instance_id", req.RestrictToAllowed, req.AllowedInstanceIDs)
		if req.InstanceID > 0 {
			query = query.Where("instance_id = ?", req.InstanceID)
		}
		if action := strings.TrimSpace(req.Action); action != "" {
			switch action {
			case dbbiz.DatabaseAuditActionQuery:
				query = query.Where("(audit_action = ? OR ((audit_action = '' OR audit_action IS NULL) AND sql_type <> ?))", action, "EXPLAIN")
			case dbbiz.DatabaseAuditActionExplain:
				query = query.Where("(audit_action = ? OR ((audit_action = '' OR audit_action IS NULL) AND sql_type = ?))", action, "EXPLAIN")
			default:
				query = query.Where("audit_action = ?", action)
			}
		}
		if status := strings.TrimSpace(req.Status); status != "" {
			query = query.Where("status = ?", status)
		}
		if riskLevel := strings.TrimSpace(req.RiskLevel); riskLevel != "" {
			query = query.Where("risk_level = ?", riskLevel)
		}
		if sqlType := strings.TrimSpace(req.SQLType); sqlType != "" {
			query = query.Where("sql_type = ?", sqlType)
		}
		if startTime := strings.TrimSpace(req.StartTime); startTime != "" {
			query = query.Where("created_at >= ?", startTime)
		}
		if endTime := strings.TrimSpace(req.EndTime); endTime != "" {
			query = query.Where("created_at <= ?", endTime)
		}
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			query = query.Where("operator_name LIKE ? OR sql_text LIKE ? OR schema_name LIKE ? OR client_ip LIKE ?", like, like, like, like)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page := 1
	pageSize := 10
	if req != nil {
		page = req.Page
		pageSize = req.PageSize
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *queryAuditRepo) ListHistory(ctx context.Context, operatorID uint, req *dbbiz.DatabaseQueryHistoryRequest) ([]*dbbiz.DatabaseQueryAudit, error) {
	var items []*dbbiz.DatabaseQueryAudit

	query := r.db.WithContext(ctx).
		Model(&dbbiz.DatabaseQueryAudit{}).
		Where("operator_id = ?", operatorID).
		Where("(audit_action IN ? OR audit_action = '' OR audit_action IS NULL)", []string{dbbiz.DatabaseAuditActionQuery, dbbiz.DatabaseAuditActionExplain})
	if req != nil {
		query = applyAllowedInstanceFilter(query, "instance_id", req.RestrictToAllowed, req.AllowedInstanceIDs)
		if req.InstanceID > 0 {
			query = query.Where("instance_id = ?", req.InstanceID)
		}
		if schemaName := strings.TrimSpace(req.SchemaName); schemaName != "" {
			query = query.Where("schema_name = ?", schemaName)
		}
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			query = query.Where("sql_text LIKE ? OR schema_name LIKE ?", like, like)
		}
	}

	limit := 20
	if req != nil && req.Limit > 0 {
		limit = req.Limit
	}
	if err := query.Order("id DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
