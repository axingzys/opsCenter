package database

import (
	"context"
	"fmt"
	"strings"
)

const defaultMaxAffectedRows int64 = 1000

func (uc *UseCase) ValidateWriteQuery(ctx context.Context, instanceID uint, req *DatabaseWriteValidateRequest) (*DatabaseWriteValidateVO, error) {
	if req == nil {
		return nil, fmt.Errorf("请求不能为空")
	}

	item, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	if strings.TrimSpace(item.Status) != DatabaseInstanceStatusEnabled {
		return nil, fmt.Errorf("数据库实例已禁用")
	}

	policy, err := uc.resolveWritePolicy(ctx)
	if err != nil {
		return nil, err
	}

	schemaName := resolveQuerySchemaName(item, req.SchemaName)
	result := AnalyzeWriteSQLByDB(item.DBType, req.SQLText, policy)

	return &DatabaseWriteValidateVO{
		InstanceID:        item.ID,
		InstanceName:      item.Name,
		DBType:            item.DBType,
		DBTypeText:        DBTypeText(item.DBType),
		SchemaName:        schemaName,
		SQLType:           result.SQLType,
		RiskLevel:         result.RiskLevel,
		RiskLevelText:     QueryRiskLevelText(result.RiskLevel),
		Allowed:           result.Allowed,
		ConfirmRequired:   result.ConfirmRequired,
		ReasonRequired:    result.ReasonRequired,
		RowsAffectedLimit: result.RowsAffectedLimit,
		Message:           result.Message,
	}, nil
}

func (uc *UseCase) resolveWritePolicy(ctx context.Context) (*DatabaseWritePolicy, error) {
	policy := defaultDatabaseWritePolicy()
	if uc.writePolicyResolver == nil {
		return policy, nil
	}

	resolved, err := uc.writePolicyResolver(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取数据库写操作配置失败: %w", err)
	}
	if resolved == nil {
		return policy, nil
	}

	policy.WriteEnabled = resolved.WriteEnabled
	policy.HighRiskRequiresConfirm = resolved.HighRiskRequiresConfirm
	policy.OperationReasonRequired = resolved.OperationReasonRequired
	policy.MaxAffectedRows = normalizeMaxAffectedRows(resolved.MaxAffectedRows)
	return policy, nil
}

func defaultDatabaseWritePolicy() *DatabaseWritePolicy {
	return &DatabaseWritePolicy{
		WriteEnabled:            false,
		HighRiskRequiresConfirm: true,
		OperationReasonRequired: true,
		MaxAffectedRows:         defaultMaxAffectedRows,
	}
}

func normalizeMaxAffectedRows(limit int64) int64 {
	if limit <= 0 {
		return defaultMaxAffectedRows
	}
	return limit
}
