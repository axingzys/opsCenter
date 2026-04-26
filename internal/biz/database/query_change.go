package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (uc *UseCase) ExecuteWriteQuery(ctx context.Context, instanceID uint, req *DatabaseWriteExecuteRequest, operator QueryOperator) (*DatabaseWriteExecuteVO, error) {
	if req == nil {
		return nil, fmt.Errorf("请求不能为空")
	}

	item, err := uc.getEnabledQueryInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	policy, err := uc.resolveWritePolicy(ctx)
	if err != nil {
		return nil, err
	}

	timeout := normalizeQueryTimeout(req.TimeoutSeconds)
	schemaName := resolveQuerySchemaName(item, req.SchemaName)
	sqlText := strings.TrimSpace(req.SQLText)
	reason := strings.TrimSpace(req.Reason)
	safety := AnalyzeWriteSQLByDB(item.DBType, sqlText, policy)
	if !safety.Allowed {
		uc.recordDeniedWriteQuery(ctx, item, schemaName, sqlText, reason, req.Confirmed, safety, safety.Message, operator)
		return nil, errors.New(safety.Message)
	}
	if !supportsWriteExecution(item.DBType) {
		message := fmt.Sprintf("%s 写操作执行将在后续批次接入", DBTypeText(item.DBType))
		uc.recordDeniedWriteQuery(ctx, item, schemaName, sqlText, reason, req.Confirmed, safety, message, operator)
		return nil, errors.New(message)
	}
	if !isSupportedWriteExecuteType(safety.SQLType) {
		message := "当前仅支持 INSERT / UPDATE / DELETE 写操作执行"
		uc.recordDeniedWriteQuery(ctx, item, schemaName, sqlText, reason, req.Confirmed, safety, message, operator)
		return nil, errors.New(message)
	}
	if safety.ReasonRequired && reason == "" {
		message := "写操作原因不能为空"
		uc.recordDeniedWriteQuery(ctx, item, schemaName, sqlText, reason, req.Confirmed, safety, message, operator)
		return nil, errors.New(message)
	}
	if safety.ConfirmRequired && !req.Confirmed {
		message := "高风险写操作未确认"
		uc.recordDeniedWriteQuery(ctx, item, schemaName, sqlText, reason, req.Confirmed, safety, message, operator)
		return nil, errors.New(message)
	}

	audit := &DatabaseQueryAudit{
		InstanceID:        item.ID,
		SchemaName:        schemaName,
		OperatorID:        operator.ID,
		OperatorName:      trimText(operator.Username, 100),
		AuditAction:       DatabaseAuditActionChangeExecute,
		Reason:            trimText(reason, 500),
		ConfirmRequired:   safety.ConfirmRequired,
		Confirmed:         req.Confirmed,
		SQLText:           trimText(sqlText, 20000),
		SQLFingerprint:    sqlFingerprint(sqlText),
		SQLType:           safety.SQLType,
		RiskLevel:         safety.RiskLevel,
		Status:            DatabaseQueryStatusPending,
		RowsAffectedLimit: safety.RowsAffectedLimit,
		ClientIP:          trimText(operator.ClientIP, 64),
	}
	if err := uc.auditRepo.Create(ctx, audit); err != nil {
		return nil, fmt.Errorf("创建查询审计失败: %w", err)
	}

	credential, err := uc.credentialResolver(ctx, item.CredentialID)
	if err != nil {
		uc.finishChangeAudit(ctx, audit, DatabaseQueryStatusFailed, 0, 0, "", "凭据不存在")
		return nil, fmt.Errorf("凭据不存在")
	}

	start := time.Now()
	rowsAffected, err := executeSQLChange(ctx, item, credential, schemaName, sqlText, timeout)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		uc.finishChangeAudit(ctx, audit, DatabaseQueryStatusFailed, 0, duration, "", err.Error())
		return nil, err
	}

	rollbackSQL := buildWriteRollbackSQLHint(item.DBType, safety.SQLType)
	message := buildWriteExecuteMessage(rowsAffected, safety.RowsAffectedLimit)
	uc.finishChangeAudit(ctx, audit, DatabaseQueryStatusSuccess, rowsAffected, duration, rollbackSQL, "")

	return &DatabaseWriteExecuteVO{
		AuditID:           audit.ID,
		InstanceID:        item.ID,
		InstanceName:      item.Name,
		DBType:            item.DBType,
		DBTypeText:        DBTypeText(item.DBType),
		SchemaName:        schemaName,
		SQLType:           safety.SQLType,
		RiskLevel:         safety.RiskLevel,
		RiskLevelText:     QueryRiskLevelText(safety.RiskLevel),
		ExecutedSQL:       sqlText,
		RowsAffected:      rowsAffected,
		RowsAffectedLimit: safety.RowsAffectedLimit,
		Reason:            trimText(reason, 500),
		ConfirmRequired:   safety.ConfirmRequired,
		Confirmed:         req.Confirmed,
		RollbackSQL:       rollbackSQL,
		DurationMs:        duration,
		Message:           message,
		ExecutedAt:        time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

func (uc *UseCase) recordDeniedWriteQuery(ctx context.Context, item *DatabaseInstance, schemaName, sqlText, reason string, confirmed bool, safety SQLWriteSafetyResult, message string, operator QueryOperator) {
	if uc.auditRepo == nil || item == nil {
		return
	}

	sqlType := strings.ToUpper(strings.TrimSpace(safety.SQLType))
	if sqlType == "" {
		sqlType = "UNKNOWN"
	}
	riskLevel := strings.TrimSpace(safety.RiskLevel)
	if riskLevel == "" {
		riskLevel = DatabaseQueryRiskHigh
	}

	_ = uc.auditRepo.Create(ctx, &DatabaseQueryAudit{
		InstanceID:        item.ID,
		SchemaName:        schemaName,
		OperatorID:        operator.ID,
		OperatorName:      trimText(operator.Username, 100),
		AuditAction:       DatabaseAuditActionChangeExecute,
		Reason:            trimText(reason, 500),
		ConfirmRequired:   safety.ConfirmRequired,
		Confirmed:         confirmed,
		SQLText:           trimText(sqlText, 20000),
		SQLFingerprint:    sqlFingerprint(sqlText),
		SQLType:           sqlType,
		RiskLevel:         riskLevel,
		Status:            DatabaseQueryStatusDenied,
		RowsAffectedLimit: safety.RowsAffectedLimit,
		ErrorMessage:      trimText(message, 500),
		ClientIP:          trimText(operator.ClientIP, 64),
	})
}

func (uc *UseCase) finishChangeAudit(ctx context.Context, audit *DatabaseQueryAudit, status string, rowsAffected, durationMs int64, rollbackSQL, errorMessage string) {
	if uc.auditRepo == nil || audit == nil || audit.ID == 0 {
		return
	}
	audit.Status = status
	audit.RowsAffected = normalizeRowsAffected(rowsAffected)
	audit.DurationMs = durationMs
	audit.RollbackSQL = trimText(rollbackSQL, 20000)
	audit.ErrorMessage = trimText(errorMessage, 500)
	_ = uc.auditRepo.Update(ctx, audit)
}

func executeSQLChange(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, schemaName, sqlText string, timeoutSeconds int) (int64, error) {
	switch normalizeDBType(item.DBType) {
	case DBTypeMySQL, DBTypeMariaDB:
		return executeMySQLChange(ctx, item, credential, schemaName, sqlText, timeoutSeconds)
	case DBTypePostgreSQL:
		return executePostgreSQLChange(ctx, item, credential, schemaName, sqlText, timeoutSeconds)
	default:
		return 0, fmt.Errorf("%s 写操作执行将在后续批次接入", DBTypeText(item.DBType))
	}
}

func executeMySQLChange(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, schemaName, sqlText string, timeoutSeconds int) (int64, error) {
	queryItem := *item
	if strings.TrimSpace(schemaName) != "" {
		queryItem.DefaultDatabase = strings.TrimSpace(schemaName)
	}

	db, err := openMySQLDB(&queryItem, credential)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	if err := db.PingContext(queryCtx); err != nil {
		return 0, fmt.Errorf("连接数据库失败: %w", err)
	}

	result, err := db.ExecContext(queryCtx, sqlText)
	if err != nil {
		return 0, fmt.Errorf("执行写操作失败: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return normalizeRowsAffected(rowsAffected), nil
}

func executePostgreSQLChange(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, schemaName, sqlText string, timeoutSeconds int) (int64, error) {
	db, err := openPostgreSQLDB(item, credential)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	if err := db.PingContext(queryCtx); err != nil {
		return 0, fmt.Errorf("连接数据库失败: %w", err)
	}
	if strings.TrimSpace(schemaName) != "" {
		if _, err := db.ExecContext(queryCtx, "SET search_path TO "+quotePostgreSQLIdentifier(schemaName)+", public"); err != nil {
			return 0, fmt.Errorf("设置 Schema 失败: %w", err)
		}
	}

	result, err := db.ExecContext(queryCtx, sqlText)
	if err != nil {
		return 0, fmt.Errorf("执行写操作失败: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return normalizeRowsAffected(rowsAffected), nil
}

func supportsWriteExecution(dbType string) bool {
	switch normalizeDBType(dbType) {
	case DBTypeMySQL, DBTypeMariaDB, DBTypePostgreSQL:
		return true
	default:
		return false
	}
}

func isSupportedWriteExecuteType(sqlType string) bool {
	switch strings.ToUpper(strings.TrimSpace(sqlType)) {
	case "INSERT", "UPDATE", "DELETE":
		return true
	default:
		return false
	}
}

func buildWriteRollbackSQLHint(dbType, sqlType string) string {
	switch strings.ToUpper(strings.TrimSpace(sqlType)) {
	case "INSERT":
		return "暂不支持自动生成 INSERT 回滚 SQL，请基于主键或唯一键手工确认删除语句。"
	case "UPDATE", "DELETE":
		if normalizeDBType(dbType) == DBTypeMySQL || normalizeDBType(dbType) == DBTypeMariaDB || normalizeDBType(dbType) == DBTypePostgreSQL {
			return "暂不支持自动生成 UPDATE / DELETE 回滚 SQL，请优先结合逻辑备份、事务日志或变更前快照恢复。"
		}
	}
	return ""
}

func buildWriteExecuteMessage(rowsAffected, limit int64) string {
	rowsAffected = normalizeRowsAffected(rowsAffected)
	if limit > 0 && rowsAffected > limit {
		return fmt.Sprintf("写操作执行成功，影响 %d 行，已超过预设阈值 %d 行，请立即复核", rowsAffected, limit)
	}
	return fmt.Sprintf("写操作执行成功，影响 %d 行", rowsAffected)
}

func normalizeRowsAffected(rowsAffected int64) int64 {
	if rowsAffected < 0 {
		return 0
	}
	return rowsAffected
}
