package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type DatabaseQueryFormatRequest struct {
	SQLText string `json:"sqlText" binding:"required"`
}

type DatabaseQueryFormatVO struct {
	SQLText string `json:"sqlText"`
	SQLType string `json:"sqlType"`
}

type DatabaseQueryHistoryRequest struct {
	InstanceID uint   `form:"instanceId"`
	SchemaName string `form:"schemaName"`
	Keyword    string `form:"keyword"`
	Limit      int    `form:"limit"`
}

func (uc *UseCase) FormatQuerySQL(ctx context.Context, instanceID uint, req *DatabaseQueryFormatRequest) (*DatabaseQueryFormatVO, error) {
	if req == nil {
		return nil, fmt.Errorf("请求不能为空")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	if normalizeDBType(instance.DBType) == DBTypeRedis {
		return formatRedisCommand(req.SQLText, normalizeQueryLimit(50))
	}

	safety := AnalyzeReadOnlySQLRaw(req.SQLText)
	if !safety.Allowed {
		return nil, errors.New(safety.Message)
	}
	return &DatabaseQueryFormatVO{
		SQLText: FormatSQL(safety.SQLText),
		SQLType: safety.SQLType,
	}, nil
}

func (uc *UseCase) ExplainQuery(ctx context.Context, instanceID uint, req *DatabaseQueryRequest, operator QueryOperator) (*DatabaseQueryResultVO, error) {
	if req == nil {
		return nil, fmt.Errorf("请求不能为空")
	}
	item, err := uc.getEnabledQueryInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	timeout := normalizeQueryTimeout(req.TimeoutSeconds)
	schemaName := resolveQuerySchemaName(item, req.SchemaName)
	safety := AnalyzeExplainSQLByDB(item.DBType, req.SQLText)
	if !safety.Allowed {
		uc.recordDeniedQuery(ctx, item, schemaName, DatabaseAuditActionExplain, safety.SQLText, safety.SQLType, safety.Message, operator)
		return nil, errors.New(safety.Message)
	}

	return uc.runAuditedQuery(ctx, item, schemaName, DatabaseAuditActionExplain, safety.SQLText, safety.SQLType, safety.SQLText, 500, timeout, operator)
}

func (uc *UseCase) ExportQuery(ctx context.Context, instanceID uint, req *DatabaseQueryRequest, operator QueryOperator) (*DatabaseQueryResultVO, error) {
	if req == nil {
		return nil, fmt.Errorf("请求不能为空")
	}
	item, err := uc.getEnabledQueryInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	limit := normalizeQueryExportLimit(req.Limit)
	timeout := normalizeQueryTimeout(req.TimeoutSeconds)
	schemaName := resolveQuerySchemaName(item, req.SchemaName)
	sqlText := strings.TrimSpace(req.SQLText)
	if normalizeDBType(item.DBType) == DBTypeRedis {
		safety := analyzeReadOnlyRedisCommand(sqlText, minInt(limit, 500))
		if !safety.Allowed {
			uc.recordDeniedQuery(ctx, item, schemaName, DatabaseAuditActionQueryExport, sqlText, safety.SQLType, safety.Message, operator)
			return nil, errors.New(safety.Message)
		}
		return uc.runAuditedQuery(ctx, item, schemaName, DatabaseAuditActionQueryExport, sqlText, safety.SQLType, safety.SQLText, limit, timeout, operator)
	}
	safety := AnalyzeReadOnlySQLByDB(item.DBType, sqlText, limit)
	if !safety.Allowed {
		uc.recordDeniedQuery(ctx, item, schemaName, DatabaseAuditActionQueryExport, sqlText, safety.SQLType, safety.Message, operator)
		return nil, errors.New(safety.Message)
	}

	return uc.runAuditedQuery(ctx, item, schemaName, DatabaseAuditActionQueryExport, sqlText, safety.SQLType, safety.SQLText, limit, timeout, operator)
}

func (uc *UseCase) ListQueryHistory(ctx context.Context, req *DatabaseQueryHistoryRequest, operator QueryOperator) ([]*DatabaseQueryAuditVO, error) {
	if uc.auditRepo == nil {
		return nil, fmt.Errorf("查询审计未配置")
	}
	if operator.ID == 0 {
		return nil, fmt.Errorf("当前用户信息缺失")
	}

	normalizeQueryHistoryRequest(req)
	items, err := uc.auditRepo.ListHistory(ctx, operator.ID, req)
	if err != nil {
		return nil, err
	}

	instanceNames := make(map[uint]string)
	list := make([]*DatabaseQueryAuditVO, 0, len(items))
	for _, item := range items {
		if item.InstanceID > 0 {
			if _, ok := instanceNames[item.InstanceID]; !ok {
				instance, err := uc.instanceRepo.GetByID(ctx, item.InstanceID)
				if err == nil && instance != nil {
					instanceNames[item.InstanceID] = instance.Name
				} else {
					instanceNames[item.InstanceID] = ""
				}
			}
		}
		list = append(list, uc.toQueryAuditVO(item, instanceNames[item.InstanceID]))
	}
	return list, nil
}

func (uc *UseCase) getEnabledQueryInstance(ctx context.Context, instanceID uint) (*DatabaseInstance, error) {
	item, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	if strings.TrimSpace(item.Status) != DatabaseInstanceStatusEnabled {
		return nil, fmt.Errorf("数据库实例已禁用")
	}
	if uc.auditRepo == nil {
		return nil, fmt.Errorf("查询审计未配置")
	}
	if uc.credentialResolver == nil {
		return nil, fmt.Errorf("连接凭据解析器未配置")
	}
	return item, nil
}

func resolveQuerySchemaName(item *DatabaseInstance, schemaName string) string {
	schemaName = strings.TrimSpace(schemaName)
	if schemaName != "" {
		return schemaName
	}
	if item == nil {
		return ""
	}
	switch normalizeDBType(item.DBType) {
	case DBTypePostgreSQL, DBTypeSQLServer, DBTypeOracle, DBTypeOpenGauss, DBTypeKingbase:
		return ""
	}
	return strings.TrimSpace(item.DefaultDatabase)
}

func (uc *UseCase) recordDeniedQuery(ctx context.Context, item *DatabaseInstance, schemaName, auditAction, sqlText, sqlType, message string, operator QueryOperator) {
	if uc.auditRepo == nil || item == nil {
		return
	}
	sqlType = strings.ToUpper(strings.TrimSpace(sqlType))
	if sqlType == "" {
		sqlType = "UNKNOWN"
	}
	_ = uc.auditRepo.Create(ctx, &DatabaseQueryAudit{
		InstanceID:     item.ID,
		SchemaName:     schemaName,
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 100),
		AuditAction:    normalizeAuditAction(auditAction),
		SQLText:        trimText(sqlText, 20000),
		SQLFingerprint: sqlFingerprint(sqlText),
		SQLType:        sqlType,
		RiskLevel:      DatabaseQueryRiskHigh,
		Status:         DatabaseQueryStatusDenied,
		ErrorMessage:   trimText(message, 500),
		ClientIP:       trimText(operator.ClientIP, 64),
	})
}

func (uc *UseCase) runAuditedQuery(
	ctx context.Context,
	item *DatabaseInstance,
	schemaName,
	auditAction,
	auditSQLText,
	sqlType,
	executedSQLText string,
	limit,
	timeout int,
	operator QueryOperator,
) (*DatabaseQueryResultVO, error) {
	if item == nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	audit := &DatabaseQueryAudit{
		InstanceID:     item.ID,
		SchemaName:     schemaName,
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 100),
		AuditAction:    normalizeAuditAction(auditAction),
		SQLText:        trimText(auditSQLText, 20000),
		SQLFingerprint: sqlFingerprint(auditSQLText),
		SQLType:        strings.ToUpper(strings.TrimSpace(sqlType)),
		RiskLevel:      DatabaseQueryRiskLow,
		Status:         DatabaseQueryStatusPending,
		ClientIP:       trimText(operator.ClientIP, 64),
	}
	if audit.SQLType == "" {
		audit.SQLType = "UNKNOWN"
	}
	if err := uc.auditRepo.Create(ctx, audit); err != nil {
		return nil, fmt.Errorf("创建查询审计失败: %w", err)
	}

	credential, err := uc.credentialResolver(ctx, item.CredentialID)
	if err != nil {
		uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusFailed, 0, 0, "凭据不存在")
		return nil, fmt.Errorf("凭据不存在")
	}

	start := time.Now()
	result, err := executeSQLQuery(ctx, item, credential, schemaName, audit.SQLType, executedSQLText, limit, timeout)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusFailed, 0, duration, err.Error())
		return nil, err
	}

	audit.Status = DatabaseQueryStatusSuccess
	audit.RowsReturned = result.RowsReturned
	audit.DurationMs = duration
	if err := uc.auditRepo.Update(ctx, audit); err != nil {
		return nil, fmt.Errorf("更新查询审计失败: %w", err)
	}

	result.AuditID = audit.ID
	result.InstanceID = item.ID
	result.SchemaName = schemaName
	result.DurationMs = duration
	result.ExecutedAt = time.Now().Format("2006-01-02 15:04:05")
	return result, nil
}

func normalizeQueryHistoryRequest(req *DatabaseQueryHistoryRequest) {
	if req == nil {
		return
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 50 {
		req.Limit = 50
	}
	req.SchemaName = strings.TrimSpace(req.SchemaName)
	req.Keyword = strings.TrimSpace(req.Keyword)
}

func normalizeQueryExportLimit(limit int) int {
	if limit <= 0 {
		return 1000
	}
	if limit > 5000 {
		return 5000
	}
	return limit
}

func buildMetadataExportAuditSQL(schemaName, tableName string) string {
	qualified := strings.TrimSpace(tableName)
	schemaName = strings.TrimSpace(schemaName)
	if schemaName != "" && qualified != "" {
		qualified = schemaName + "." + qualified
	}
	if qualified == "" {
		qualified = "<unknown>"
	}
	return "EXPORT DICTIONARY " + qualified
}

func buildDiagnosisAuditSQL(action string, limit int) string {
	switch normalizeAuditAction(action) {
	case DatabaseAuditActionDiagnosisMetrics:
		return "SHOW METRICS"
	case DatabaseAuditActionDiagnosisSessions:
		return fmt.Sprintf("SHOW SESSIONS LIMIT %d", limit)
	case DatabaseAuditActionDiagnosisSlowQuery:
		return fmt.Sprintf("SHOW SLOW QUERIES LIMIT %d", limit)
	default:
		return "SHOW DIAGNOSIS"
	}
}
