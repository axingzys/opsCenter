package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type DatabaseDiagnosisListRequest struct {
	Limit int `form:"limit"`
}

type DatabaseMetricCardVO struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

type DatabaseMetricsVO struct {
	InstanceID   uint                    `json:"instanceId"`
	InstanceName string                  `json:"instanceName"`
	DBType       string                  `json:"dbType"`
	DBTypeText   string                  `json:"dbTypeText"`
	CollectedAt  string                  `json:"collectedAt"`
	Cards        []*DatabaseMetricCardVO `json:"cards"`
}

type DatabaseSessionVO struct {
	SessionID       string `json:"sessionId"`
	DatabaseName    string `json:"databaseName"`
	User            string `json:"user"`
	Command         string `json:"command"`
	State           string `json:"state"`
	WaitEvent       string `json:"waitEvent"`
	ClientAddr      string `json:"clientAddr"`
	NodeAddress     string `json:"nodeAddress"`
	IdleSeconds     int64  `json:"idleSeconds"`
	DurationSeconds int64  `json:"durationSeconds"`
	SQLText         string `json:"sqlText"`
}

type DatabaseSessionListVO struct {
	InstanceID   uint                 `json:"instanceId"`
	InstanceName string               `json:"instanceName"`
	DBType       string               `json:"dbType"`
	DBTypeText   string               `json:"dbTypeText"`
	CollectedAt  string               `json:"collectedAt"`
	Items        []*DatabaseSessionVO `json:"items"`
}

type DatabaseSlowQueryVO struct {
	SchemaName    string  `json:"schemaName"`
	SQLText       string  `json:"sqlText"`
	SQLSummary    string  `json:"sqlSummary"`
	AvgDurationMs float64 `json:"avgDurationMs"`
	MaxDurationMs float64 `json:"maxDurationMs"`
	ExecCount     int64   `json:"execCount"`
	RowsMetric    int64   `json:"rowsMetric"`
	NodeAddress   string  `json:"nodeAddress"`
	LastSeen      string  `json:"lastSeen"`
}

type DatabaseSlowQueryListVO struct {
	InstanceID   uint                   `json:"instanceId"`
	InstanceName string                 `json:"instanceName"`
	DBType       string                 `json:"dbType"`
	DBTypeText   string                 `json:"dbTypeText"`
	CollectedAt  string                 `json:"collectedAt"`
	Message      string                 `json:"message"`
	Items        []*DatabaseSlowQueryVO `json:"items"`
}

func (uc *UseCase) GetDiagnosisMetrics(ctx context.Context, instanceID uint, operator QueryOperator) (*DatabaseMetricsVO, error) {
	item, err := uc.getDiagnosableInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	audit := uc.startDiagnosisAudit(ctx, item, DatabaseAuditActionDiagnosisMetrics, 0, operator)
	credential, err := uc.resolveDiagnosableCredential(ctx, item)
	if err != nil {
		uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusFailed, 0, 0, err.Error())
		return nil, err
	}

	start := time.Now()
	cards, err := collectDatabaseMetrics(ctx, item, credential)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusFailed, 0, duration, err.Error())
		return nil, err
	}
	uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusSuccess, len(cards), duration, "")
	return &DatabaseMetricsVO{
		InstanceID:   item.ID,
		InstanceName: item.Name,
		DBType:       item.DBType,
		DBTypeText:   DBTypeText(item.DBType),
		CollectedAt:  time.Now().Format("2006-01-02 15:04:05"),
		Cards:        cards,
	}, nil
}

func (uc *UseCase) ListDiagnosisSessions(ctx context.Context, instanceID uint, req *DatabaseDiagnosisListRequest, operator QueryOperator) (*DatabaseSessionListVO, error) {
	item, err := uc.getDiagnosableInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	limit := normalizeDiagnosisLimit(req, 20, 50)
	audit := uc.startDiagnosisAudit(ctx, item, DatabaseAuditActionDiagnosisSessions, limit, operator)
	credential, err := uc.resolveDiagnosableCredential(ctx, item)
	if err != nil {
		uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusFailed, 0, 0, err.Error())
		return nil, err
	}
	start := time.Now()
	items, err := collectDatabaseSessions(ctx, item, credential, limit)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusFailed, 0, duration, err.Error())
		return nil, err
	}
	uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusSuccess, len(items), duration, "")
	return &DatabaseSessionListVO{
		InstanceID:   item.ID,
		InstanceName: item.Name,
		DBType:       item.DBType,
		DBTypeText:   DBTypeText(item.DBType),
		CollectedAt:  time.Now().Format("2006-01-02 15:04:05"),
		Items:        items,
	}, nil
}

func (uc *UseCase) ListSlowQueries(ctx context.Context, instanceID uint, req *DatabaseDiagnosisListRequest, operator QueryOperator) (*DatabaseSlowQueryListVO, error) {
	item, err := uc.getDiagnosableInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	limit := normalizeDiagnosisLimit(req, 10, 50)
	audit := uc.startDiagnosisAudit(ctx, item, DatabaseAuditActionDiagnosisSlowQuery, limit, operator)
	credential, err := uc.resolveDiagnosableCredential(ctx, item)
	if err != nil {
		uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusFailed, 0, 0, err.Error())
		return nil, err
	}
	start := time.Now()
	items, message, err := collectDatabaseSlowQueries(ctx, item, credential, limit)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusFailed, 0, duration, err.Error())
		return nil, err
	}
	uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusSuccess, len(items), duration, "")
	return &DatabaseSlowQueryListVO{
		InstanceID:   item.ID,
		InstanceName: item.Name,
		DBType:       item.DBType,
		DBTypeText:   DBTypeText(item.DBType),
		CollectedAt:  time.Now().Format("2006-01-02 15:04:05"),
		Message:      message,
		Items:        items,
	}, nil
}

func (uc *UseCase) getDiagnosableInstance(ctx context.Context, instanceID uint) (*DatabaseInstance, error) {
	item, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	if strings.TrimSpace(item.Status) != DatabaseInstanceStatusEnabled {
		return nil, fmt.Errorf("数据库实例已禁用")
	}
	return item, nil
}

func (uc *UseCase) resolveDiagnosableCredential(ctx context.Context, item *DatabaseInstance) (*ConnectionCredential, error) {
	if uc.credentialResolver == nil {
		return nil, fmt.Errorf("连接凭据解析器未配置")
	}
	credential, err := uc.credentialResolver(ctx, item.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("凭据不存在")
	}
	return credential, nil
}

func (uc *UseCase) startDiagnosisAudit(ctx context.Context, item *DatabaseInstance, action string, limit int, operator QueryOperator) *DatabaseQueryAudit {
	if uc.auditRepo == nil || item == nil {
		return nil
	}
	auditSQLText := buildDiagnosisAuditSQL(action, limit)
	audit := &DatabaseQueryAudit{
		InstanceID:     item.ID,
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 100),
		AuditAction:    normalizeAuditAction(action),
		SQLText:        trimText(auditSQLText, 20000),
		SQLFingerprint: sqlFingerprint(auditSQLText),
		SQLType:        "DIAGNOSIS",
		RiskLevel:      DatabaseQueryRiskLow,
		Status:         DatabaseQueryStatusPending,
		ClientIP:       trimText(operator.ClientIP, 64),
	}
	if err := uc.auditRepo.Create(ctx, audit); err != nil {
		return nil
	}
	return audit
}

func collectDatabaseMetrics(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) ([]*DatabaseMetricCardVO, error) {
	switch normalizeDBType(item.DBType) {
	case DBTypeMySQL, DBTypeMariaDB, DBTypeTiDB, DBTypeOceanBase:
		return collectMySQLMetrics(ctx, item, credential)
	case DBTypePostgreSQL, DBTypeOpenGauss, DBTypeKingbase:
		return collectPostgreSQLMetrics(ctx, item, credential)
	case DBTypeRedis:
		return collectRedisMetrics(ctx, item, credential)
	default:
		return nil, fmt.Errorf("%s 诊断能力将在二期后续批次接入", DBTypeText(item.DBType))
	}
}

func collectDatabaseSessions(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, limit int) ([]*DatabaseSessionVO, error) {
	switch normalizeDBType(item.DBType) {
	case DBTypeMySQL, DBTypeMariaDB, DBTypeTiDB, DBTypeOceanBase:
		return collectMySQLSessions(ctx, item, credential, limit)
	case DBTypePostgreSQL, DBTypeOpenGauss, DBTypeKingbase:
		return collectPostgreSQLSessions(ctx, item, credential, limit)
	case DBTypeRedis:
		return collectRedisSessions(ctx, item, credential, limit)
	default:
		return nil, fmt.Errorf("%s 会话诊断将在二期后续批次接入", DBTypeText(item.DBType))
	}
}

func collectDatabaseSlowQueries(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, limit int) ([]*DatabaseSlowQueryVO, string, error) {
	switch normalizeDBType(item.DBType) {
	case DBTypeMySQL, DBTypeMariaDB, DBTypeTiDB, DBTypeOceanBase:
		return collectMySQLSlowQueries(ctx, item, credential, limit)
	case DBTypePostgreSQL, DBTypeOpenGauss, DBTypeKingbase:
		return collectPostgreSQLSlowQueries(ctx, item, credential, limit)
	case DBTypeRedis:
		return collectRedisSlowQueries(ctx, item, credential, limit)
	default:
		return nil, "", fmt.Errorf("%s 慢 SQL 诊断将在二期后续批次接入", DBTypeText(item.DBType))
	}
}

func collectMySQLMetrics(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) ([]*DatabaseMetricCardVO, error) {
	db, err := openMySQLDB(item, credential)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(queryCtx); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	var (
		currentConnections int64
		maxConnections     int64
		activeSessions     int64
		tableCount         int64
		totalSizeBytes     int64
	)
	if err := db.QueryRowContext(queryCtx, "SELECT COUNT(*) FROM information_schema.processlist").Scan(&currentConnections); err != nil {
		return nil, fmt.Errorf("读取连接数失败: %w", err)
	}
	if err := db.QueryRowContext(queryCtx, "SELECT @@max_connections").Scan(&maxConnections); err != nil {
		return nil, fmt.Errorf("读取最大连接数失败: %w", err)
	}
	if err := db.QueryRowContext(queryCtx, "SELECT COUNT(*) FROM information_schema.processlist WHERE COMMAND <> 'Sleep'").Scan(&activeSessions); err != nil {
		return nil, fmt.Errorf("读取活跃会话数失败: %w", err)
	}
	if err := db.QueryRowContext(queryCtx, `
SELECT COUNT(*)
FROM information_schema.tables
WHERE UPPER(TABLE_SCHEMA) NOT IN ('INFORMATION_SCHEMA', 'MYSQL', 'PERFORMANCE_SCHEMA', 'SYS', 'METRICS_SCHEMA', 'INSPECTION_SCHEMA', 'OCEANBASE', '__RECYCLEBIN')`).Scan(&tableCount); err != nil {
		return nil, fmt.Errorf("读取表数量失败: %w", err)
	}
	if err := db.QueryRowContext(queryCtx, `
SELECT COALESCE(SUM(DATA_LENGTH + INDEX_LENGTH), 0)
FROM information_schema.tables
WHERE UPPER(TABLE_SCHEMA) NOT IN ('INFORMATION_SCHEMA', 'MYSQL', 'PERFORMANCE_SCHEMA', 'SYS', 'METRICS_SCHEMA', 'INSPECTION_SCHEMA', 'OCEANBASE', '__RECYCLEBIN')`).Scan(&totalSizeBytes); err != nil {
		return nil, fmt.Errorf("读取容量失败: %w", err)
	}

	return []*DatabaseMetricCardVO{
		{Key: "connections", Label: "当前连接", Value: fmt.Sprintf("%d / %d", currentConnections, maxConnections), Description: "当前连接数 / 最大连接数"},
		{Key: "active_sessions", Label: "活跃会话", Value: fmt.Sprintf("%d", activeSessions), Description: "排除 Sleep 的实时会话数"},
		{Key: "objects", Label: "表 / 视图数", Value: fmt.Sprintf("%d", tableCount), Description: "非系统 Schema 的对象总数"},
		{Key: "size_bytes", Label: "库总容量", Value: humanizeBytes(totalSizeBytes), Description: "信息架构统计的数据与索引容量"},
	}, nil
}

func collectMySQLSessions(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, limit int) ([]*DatabaseSessionVO, error) {
	db, err := openMySQLDB(item, credential)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(queryCtx); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	rows, err := db.QueryContext(queryCtx, `
SELECT
	CAST(ID AS CHAR),
	COALESCE(DB, ''),
	COALESCE(USER, ''),
	COALESCE(COMMAND, ''),
	COALESCE(STATE, ''),
	COALESCE(HOST, ''),
	COALESCE(TIME, 0),
	COALESCE(INFO, '')
FROM information_schema.processlist
WHERE ID <> CONNECTION_ID()
ORDER BY TIME DESC, ID DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("读取活跃会话失败: %w", err)
	}
	defer rows.Close()

	items := make([]*DatabaseSessionVO, 0)
	for rows.Next() {
		var itemVO DatabaseSessionVO
		if err := rows.Scan(
			&itemVO.SessionID,
			&itemVO.DatabaseName,
			&itemVO.User,
			&itemVO.Command,
			&itemVO.State,
			&itemVO.ClientAddr,
			&itemVO.DurationSeconds,
			&itemVO.SQLText,
		); err != nil {
			return nil, fmt.Errorf("解析活跃会话失败: %w", err)
		}
		itemVO.SQLText = trimText(itemVO.SQLText, 2000)
		items = append(items, &itemVO)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历活跃会话失败: %w", err)
	}
	return items, nil
}

func collectMySQLSlowQueries(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, limit int) ([]*DatabaseSlowQueryVO, string, error) {
	db, err := openMySQLDB(item, credential)
	if err != nil {
		return nil, "", err
	}
	defer db.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(queryCtx); err != nil {
		return nil, "", fmt.Errorf("连接数据库失败: %w", err)
	}

	var performanceSchemaEnabled int64
	if err := db.QueryRowContext(queryCtx, "SELECT @@performance_schema").Scan(&performanceSchemaEnabled); err != nil {
		return nil, "", fmt.Errorf("读取 performance_schema 状态失败: %w", err)
	}
	if performanceSchemaEnabled == 0 {
		return []*DatabaseSlowQueryVO{}, "当前实例未开启 performance_schema，慢 SQL 暂不可用", nil
	}

	rows, err := db.QueryContext(queryCtx, `
SELECT
	COALESCE(SCHEMA_NAME, ''),
	COALESCE(DIGEST_TEXT, ''),
	COALESCE(COUNT_STAR, 0),
	COALESCE(ROUND(AVG_TIMER_WAIT / 1000000000, 2), 0),
	COALESCE(ROUND(MAX_TIMER_WAIT / 1000000000, 2), 0),
	COALESCE(SUM_ROWS_EXAMINED, 0),
	COALESCE(DATE_FORMAT(LAST_SEEN, '%Y-%m-%d %H:%i:%s'), '')
FROM performance_schema.events_statements_summary_by_digest
WHERE DIGEST_TEXT IS NOT NULL
	AND DIGEST_TEXT <> ''
	AND (SCHEMA_NAME IS NULL OR UPPER(SCHEMA_NAME) NOT IN ('INFORMATION_SCHEMA', 'MYSQL', 'PERFORMANCE_SCHEMA', 'SYS', 'METRICS_SCHEMA', 'INSPECTION_SCHEMA', 'OCEANBASE', '__RECYCLEBIN'))
ORDER BY AVG_TIMER_WAIT DESC, COUNT_STAR DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, "", fmt.Errorf("读取慢 SQL 失败: %w", err)
	}
	defer rows.Close()

	items := make([]*DatabaseSlowQueryVO, 0)
	for rows.Next() {
		var itemVO DatabaseSlowQueryVO
		if err := rows.Scan(
			&itemVO.SchemaName,
			&itemVO.SQLText,
			&itemVO.ExecCount,
			&itemVO.AvgDurationMs,
			&itemVO.MaxDurationMs,
			&itemVO.RowsMetric,
			&itemVO.LastSeen,
		); err != nil {
			return nil, "", fmt.Errorf("解析慢 SQL 失败: %w", err)
		}
		itemVO.SQLText = trimText(itemVO.SQLText, 2000)
		itemVO.SQLSummary = sqlSummary(itemVO.SQLText)
		items = append(items, &itemVO)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("遍历慢 SQL 失败: %w", err)
	}
	return items, "", nil
}

func collectPostgreSQLMetrics(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) ([]*DatabaseMetricCardVO, error) {
	db, err := openPostgreSQLDB(item, credential)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(queryCtx); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	var (
		currentConnections int64
		maxConnections     int64
		activeSessions     int64
		tableCount         int64
		totalSizeBytes     int64
	)
	if err := db.QueryRowContext(queryCtx, "SELECT COUNT(*) FROM pg_stat_activity WHERE datname = current_database()").Scan(&currentConnections); err != nil {
		return nil, fmt.Errorf("读取连接数失败: %w", err)
	}
	if err := db.QueryRowContext(queryCtx, "SELECT current_setting('max_connections')::bigint").Scan(&maxConnections); err != nil {
		return nil, fmt.Errorf("读取最大连接数失败: %w", err)
	}
	if err := db.QueryRowContext(queryCtx, "SELECT COUNT(*) FROM pg_stat_activity WHERE datname = current_database() AND state = 'active'").Scan(&activeSessions); err != nil {
		return nil, fmt.Errorf("读取活跃会话数失败: %w", err)
	}
	if err := db.QueryRowContext(queryCtx, `
SELECT COUNT(*)
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
	AND n.nspname NOT LIKE 'pg_toast%'
	AND c.relkind IN ('r', 'p', 'v', 'm', 'f')`).Scan(&tableCount); err != nil {
		return nil, fmt.Errorf("读取表数量失败: %w", err)
	}
	if err := db.QueryRowContext(queryCtx, "SELECT pg_database_size(current_database())").Scan(&totalSizeBytes); err != nil {
		return nil, fmt.Errorf("读取容量失败: %w", err)
	}

	return []*DatabaseMetricCardVO{
		{Key: "connections", Label: "当前连接", Value: fmt.Sprintf("%d / %d", currentConnections, maxConnections), Description: "当前连接数 / 最大连接数"},
		{Key: "active_sessions", Label: "活跃会话", Value: fmt.Sprintf("%d", activeSessions), Description: "pg_stat_activity 中 state=active 的会话"},
		{Key: "objects", Label: "表 / 视图数", Value: fmt.Sprintf("%d", tableCount), Description: "当前数据库中非系统 Schema 的对象总数"},
		{Key: "size_bytes", Label: "库总容量", Value: humanizeBytes(totalSizeBytes), Description: "pg_database_size(current_database())"},
	}, nil
}

func collectPostgreSQLSessions(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, limit int) ([]*DatabaseSessionVO, error) {
	db, err := openPostgreSQLDB(item, credential)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(queryCtx); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	rows, err := db.QueryContext(queryCtx, `
SELECT
	pid::text,
	COALESCE(datname, ''),
	COALESCE(usename, ''),
	COALESCE(state, ''),
	COALESCE(wait_event_type || ':' || wait_event, ''),
	COALESCE(client_addr::text, ''),
	COALESCE(EXTRACT(EPOCH FROM (now() - COALESCE(query_start, backend_start)))::bigint, 0),
	COALESCE(query, '')
FROM pg_stat_activity
WHERE pid <> pg_backend_pid()
	AND datname = current_database()
ORDER BY query_start ASC NULLS LAST, backend_start ASC
LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("读取活跃会话失败: %w", err)
	}
	defer rows.Close()

	items := make([]*DatabaseSessionVO, 0)
	for rows.Next() {
		var itemVO DatabaseSessionVO
		if err := rows.Scan(
			&itemVO.SessionID,
			&itemVO.DatabaseName,
			&itemVO.User,
			&itemVO.State,
			&itemVO.WaitEvent,
			&itemVO.ClientAddr,
			&itemVO.DurationSeconds,
			&itemVO.SQLText,
		); err != nil {
			return nil, fmt.Errorf("解析活跃会话失败: %w", err)
		}
		itemVO.Command = "QUERY"
		itemVO.SQLText = trimText(itemVO.SQLText, 2000)
		items = append(items, &itemVO)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历活跃会话失败: %w", err)
	}
	return items, nil
}

func collectPostgreSQLSlowQueries(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, limit int) ([]*DatabaseSlowQueryVO, string, error) {
	db, err := openPostgreSQLDB(item, credential)
	if err != nil {
		return nil, "", err
	}
	defer db.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(queryCtx); err != nil {
		return nil, "", fmt.Errorf("连接数据库失败: %w", err)
	}

	var extensionEnabled bool
	if err := db.QueryRowContext(queryCtx, "SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements')").Scan(&extensionEnabled); err != nil {
		return nil, "", fmt.Errorf("读取 pg_stat_statements 状态失败: %w", err)
	}
	if !extensionEnabled {
		return []*DatabaseSlowQueryVO{}, "当前数据库未安装 pg_stat_statements，慢 SQL 暂不可用", nil
	}

	rows, err := db.QueryContext(queryCtx, `
SELECT
	COALESCE(d.datname, current_database()),
	COALESCE(s.query, ''),
	COALESCE(s.calls, 0),
	COALESCE(ROUND(s.mean_exec_time::numeric, 2), 0),
	COALESCE(ROUND(s.max_exec_time::numeric, 2), 0),
	COALESCE(s.rows, 0)
FROM pg_stat_statements s
JOIN pg_database d ON d.oid = s.dbid
WHERE d.datname = current_database()
ORDER BY s.mean_exec_time DESC, s.calls DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, "", fmt.Errorf("读取慢 SQL 失败: %w", err)
	}
	defer rows.Close()

	items := make([]*DatabaseSlowQueryVO, 0)
	for rows.Next() {
		var itemVO DatabaseSlowQueryVO
		if err := rows.Scan(
			&itemVO.SchemaName,
			&itemVO.SQLText,
			&itemVO.ExecCount,
			&itemVO.AvgDurationMs,
			&itemVO.MaxDurationMs,
			&itemVO.RowsMetric,
		); err != nil {
			return nil, "", fmt.Errorf("解析慢 SQL 失败: %w", err)
		}
		itemVO.SQLText = trimText(itemVO.SQLText, 2000)
		itemVO.SQLSummary = sqlSummary(itemVO.SQLText)
		items = append(items, &itemVO)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("遍历慢 SQL 失败: %w", err)
	}
	return items, "", nil
}

func normalizeDiagnosisLimit(req *DatabaseDiagnosisListRequest, defaultLimit, maxLimit int) int {
	if req == nil || req.Limit <= 0 {
		return defaultLimit
	}
	if req.Limit > maxLimit {
		return maxLimit
	}
	return req.Limit
}

func humanizeBytes(size int64) string {
	if size <= 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	current := float64(size)
	index := 0
	for current >= 1024 && index < len(units)-1 {
		current /= 1024
		index++
	}
	if current >= 10 || index == 0 {
		return fmt.Sprintf("%.0f %s", current, units[index])
	}
	return fmt.Sprintf("%.1f %s", current, units[index])
}

func nullTimeText(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02 15:04:05")
}
