package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (uc *UseCase) ValidateDDLQuery(ctx context.Context, instanceID uint, req *DatabaseDDLValidateRequest) (*DatabaseDDLValidateVO, error) {
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
	result := AnalyzeDDLSQLByDB(item.DBType, req.SQLText, policy)
	return &DatabaseDDLValidateVO{
		InstanceID:      item.ID,
		InstanceName:    item.Name,
		DBType:          item.DBType,
		DBTypeText:      DBTypeText(item.DBType),
		SchemaName:      schemaName,
		SQLType:         result.SQLType,
		RiskLevel:       result.RiskLevel,
		RiskLevelText:   QueryRiskLevelText(result.RiskLevel),
		Allowed:         result.Allowed,
		ConfirmRequired: result.ConfirmRequired,
		ReasonRequired:  result.ReasonRequired,
		BackupRequired:  result.BackupRequired,
		Message:         result.Message,
	}, nil
}

func (uc *UseCase) ExecuteDDLQuery(ctx context.Context, instanceID uint, req *DatabaseWriteExecuteRequest, operator QueryOperator) (*DatabaseWriteExecuteVO, error) {
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
	safety := AnalyzeDDLSQLByDB(item.DBType, sqlText, policy)
	if !safety.Allowed {
		uc.recordDeniedDDLQuery(ctx, item, schemaName, sqlText, reason, req.Confirmed, safety, safety.Message, operator)
		return nil, errors.New(safety.Message)
	}
	if !supportsDDLExecution(item.DBType) {
		message := fmt.Sprintf("%s DDL 结构变更执行将在后续批次接入", DBTypeText(item.DBType))
		uc.recordDeniedDDLQuery(ctx, item, schemaName, sqlText, reason, req.Confirmed, safety, message, operator)
		return nil, errors.New(message)
	}
	if !isSupportedDDLExecuteType(safety.SQLType) {
		message := "当前 DDL 执行仅支持 CREATE TABLE / CREATE INDEX"
		uc.recordDeniedDDLQuery(ctx, item, schemaName, sqlText, reason, req.Confirmed, safety, message, operator)
		return nil, errors.New(message)
	}
	if safety.ReasonRequired && reason == "" {
		message := "DDL 结构变更原因不能为空"
		uc.recordDeniedDDLQuery(ctx, item, schemaName, sqlText, reason, req.Confirmed, safety, message, operator)
		return nil, errors.New(message)
	}
	if safety.ConfirmRequired && !req.Confirmed {
		message := "高风险 DDL 结构变更未确认"
		uc.recordDeniedDDLQuery(ctx, item, schemaName, sqlText, reason, req.Confirmed, safety, message, operator)
		return nil, errors.New(message)
	}

	audit := &DatabaseQueryAudit{
		InstanceID:      item.ID,
		SchemaName:      schemaName,
		OperatorID:      operator.ID,
		OperatorName:    trimText(operator.Username, 100),
		AuditAction:     DatabaseAuditActionDDLExecute,
		Reason:          trimText(reason, 500),
		ConfirmRequired: safety.ConfirmRequired,
		Confirmed:       req.Confirmed,
		SQLText:         trimText(sqlText, 20000),
		SQLFingerprint:  sqlFingerprint(sqlText),
		SQLType:         safety.SQLType,
		RiskLevel:       safety.RiskLevel,
		Status:          DatabaseQueryStatusPending,
		ClientIP:        trimText(operator.ClientIP, 64),
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
	rowsAffected, err := executeSQLDDL(ctx, item, credential, schemaName, safety.SQLText, timeout)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		uc.finishChangeAudit(ctx, audit, DatabaseQueryStatusFailed, normalizeRowsAffected(rowsAffected), duration, "", err.Error())
		return nil, err
	}

	rollbackSQL := buildDDLRollbackSQLHint(item.DBType, safety.SQLType)
	message := "DDL 结构变更执行成功，请同步元数据后查看最新结构"
	uc.finishChangeAudit(ctx, audit, DatabaseQueryStatusSuccess, rowsAffected, duration, rollbackSQL, "")

	return &DatabaseWriteExecuteVO{
		AuditID:           audit.ID,
		AuditAction:       DatabaseAuditActionDDLExecute,
		InstanceID:        item.ID,
		InstanceName:      item.Name,
		DBType:            item.DBType,
		DBTypeText:        DBTypeText(item.DBType),
		SchemaName:        schemaName,
		SQLType:           safety.SQLType,
		RiskLevel:         safety.RiskLevel,
		RiskLevelText:     QueryRiskLevelText(safety.RiskLevel),
		ExecutedSQL:       safety.SQLText,
		RowsAffected:      rowsAffected,
		RowsAffectedLimit: 0,
		Reason:            trimText(reason, 500),
		ConfirmRequired:   safety.ConfirmRequired,
		Confirmed:         req.Confirmed,
		RollbackSQL:       rollbackSQL,
		DurationMs:        duration,
		Message:           message,
		ExecutedAt:        time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

func (uc *UseCase) recordDeniedDDLQuery(ctx context.Context, item *DatabaseInstance, schemaName, sqlText, reason string, confirmed bool, safety SQLDDLSafetyResult, message string, operator QueryOperator) {
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
		InstanceID:      item.ID,
		SchemaName:      schemaName,
		OperatorID:      operator.ID,
		OperatorName:    trimText(operator.Username, 100),
		AuditAction:     DatabaseAuditActionDDLExecute,
		Reason:          trimText(reason, 500),
		ConfirmRequired: safety.ConfirmRequired,
		Confirmed:       confirmed,
		SQLText:         trimText(sqlText, 20000),
		SQLFingerprint:  sqlFingerprint(sqlText),
		SQLType:         sqlType,
		RiskLevel:       riskLevel,
		Status:          DatabaseQueryStatusDenied,
		ErrorMessage:    trimText(message, 500),
		ClientIP:        trimText(operator.ClientIP, 64),
	})
}

func executeSQLDDL(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, schemaName, sqlText string, timeoutSeconds int) (int64, error) {
	switch normalizeDBType(item.DBType) {
	case DBTypeMySQL, DBTypeMariaDB:
		return executeMySQLDDL(ctx, item, credential, schemaName, sqlText, timeoutSeconds)
	case DBTypePostgreSQL:
		return executePostgreSQLDDL(ctx, item, credential, schemaName, sqlText, timeoutSeconds)
	default:
		return 0, fmt.Errorf("%s DDL 结构变更执行将在后续批次接入", DBTypeText(item.DBType))
	}
}

func executeMySQLDDL(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, schemaName, sqlText string, timeoutSeconds int) (int64, error) {
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
		return 0, fmt.Errorf("执行 DDL 结构变更失败: %w", err)
	}
	return ddlRowsAffected(result), nil
}

func executePostgreSQLDDL(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, schemaName, sqlText string, timeoutSeconds int) (int64, error) {
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

	tx, err := db.BeginTx(queryCtx, &sql.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("开启 DDL 结构变更事务失败: %w", err)
	}
	if strings.TrimSpace(schemaName) != "" {
		if _, err := tx.ExecContext(queryCtx, "SET search_path TO "+quotePostgreSQLIdentifier(schemaName)+", public"); err != nil {
			rollbackSQLTx(tx)
			return 0, fmt.Errorf("设置 Schema 失败: %w", err)
		}
	}
	result, err := tx.ExecContext(queryCtx, sqlText)
	if err != nil {
		rollbackSQLTx(tx)
		return 0, fmt.Errorf("执行 DDL 结构变更失败: %w", err)
	}
	rowsAffected := ddlRowsAffected(result)
	if err := tx.Commit(); err != nil {
		return rowsAffected, fmt.Errorf("提交 DDL 结构变更失败: %w", err)
	}
	return rowsAffected, nil
}

func supportsDDLExecution(dbType string) bool {
	switch normalizeDBType(dbType) {
	case DBTypeMySQL, DBTypeMariaDB, DBTypePostgreSQL:
		return true
	default:
		return false
	}
}

func isSupportedDDLExecuteType(sqlType string) bool {
	return strings.EqualFold(strings.TrimSpace(sqlType), "CREATE")
}

func ddlRowsAffected(result sql.Result) int64 {
	if result == nil {
		return 0
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0
	}
	return normalizeRowsAffected(rowsAffected)
}

func buildDDLRollbackSQLHint(dbType, sqlType string) string {
	switch strings.ToUpper(strings.TrimSpace(sqlType)) {
	case "CREATE":
		return "DDL 未自动生成回滚 SQL。若需回滚，请确认对象未被业务依赖后，基于备份和结构版本手工编写反向 DDL。"
	default:
		return "DDL 未自动生成回滚 SQL，请基于备份、结构版本或变更记录手工恢复。"
	}
}
