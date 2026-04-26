package database

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

func testSQLServerConnection(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (string, error) {
	db, err := openSQLServerDB(item, credential, "")
	if err != nil {
		return "", err
	}
	defer db.Close()

	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(testCtx); err != nil {
		return "", fmt.Errorf("连接数据库失败: %w", err)
	}

	var version string
	if err := db.QueryRowContext(testCtx, "SELECT @@VERSION").Scan(&version); err != nil {
		return "", fmt.Errorf("读取数据库版本失败: %w", err)
	}
	return version, nil
}

func collectSQLServerMetadata(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (string, []*DatabaseSchema, []*DatabaseTable, []*DatabaseColumn, []*DatabaseIndex, error) {
	db, err := openSQLServerDB(item, credential, "")
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	defer db.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := db.PingContext(queryCtx); err != nil {
		return "", nil, nil, nil, nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	var version string
	if err := db.QueryRowContext(queryCtx, "SELECT @@VERSION").Scan(&version); err != nil {
		return "", nil, nil, nil, nil, fmt.Errorf("读取数据库版本失败: %w", err)
	}

	schemas, err := collectSQLServerSchemas(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	tables, err := collectSQLServerTables(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	columns, err := collectSQLServerColumns(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	indexes, err := collectSQLServerIndexes(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	return version, schemas, tables, columns, indexes, nil
}

func executeSQLServerQuery(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, schemaName, sqlType, sqlText string, limit, timeoutSeconds int) (*DatabaseQueryResultVO, error) {
	db, err := openSQLServerDB(item, credential, "")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	if err := db.PingContext(queryCtx); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	if strings.TrimSpace(schemaName) != "" {
		// SQL Server 没有通用的 session schema 切换语法，这里仅保留 schemaName 供审计与前端状态展示。
	}

	rows, err := db.QueryContext(queryCtx, sqlText)
	if err != nil {
		return nil, fmt.Errorf("执行查询失败: %w", err)
	}
	defer rows.Close()

	return readSQLQueryResult(rows, sqlType, sqlText, limit)
}

func openSQLServerDB(item *DatabaseInstance, credential *ConnectionCredential, databaseName string) (*sql.DB, error) {
	if credential == nil || strings.TrimSpace(credential.Username) == "" {
		return nil, fmt.Errorf("凭据用户名不能为空")
	}
	if credential.Password == "" {
		return nil, fmt.Errorf("凭据密码不能为空")
	}

	params, err := parseConnectionParams(item)
	if err != nil {
		return nil, err
	}

	dbName := strings.TrimSpace(databaseName)
	if dbName == "" {
		dbName = strings.TrimSpace(item.DefaultDatabase)
	}
	if dbName == "" {
		dbName = "master"
	}

	dsn := url.URL{
		Scheme: "sqlserver",
		User:   url.UserPassword(strings.TrimSpace(credential.Username), credential.Password),
		Host:   net.JoinHostPort(strings.TrimSpace(item.Host), fmt.Sprintf("%d", item.Port)),
	}
	query := dsn.Query()
	query.Set("database", dbName)
	query.Set("connection timeout", "5")

	encrypt := connectionParamString(params, "encrypt")
	if encrypt == "" {
		if item.TLSEnabled {
			encrypt = "true"
		} else {
			encrypt = "disable"
		}
	}
	query.Set("encrypt", encrypt)

	if trustServerCert := connectionParamString(params, "trustServerCertificate", "trust_server_certificate"); trustServerCert != "" {
		query.Set("TrustServerCertificate", trustServerCert)
	}
	if applicationIntent := connectionParamString(params, "applicationIntent", "application_intent"); applicationIntent != "" {
		query.Set("ApplicationIntent", applicationIntent)
	}
	if appName := connectionParamString(params, "appName", "app_name"); appName != "" {
		query.Set("app name", appName)
	}
	dsn.RawQuery = query.Encode()

	db, err := sql.Open("sqlserver", dsn.String())
	if err != nil {
		return nil, fmt.Errorf("创建数据库连接失败: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Minute)
	return db, nil
}

func collectSQLServerSchemas(ctx context.Context, db *sql.DB) ([]*DatabaseSchema, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
	s.name,
	'',
	'',
	0,
	SUM(CASE WHEN o.object_id IS NULL THEN 0 ELSE 1 END)
FROM sys.schemas s
LEFT JOIN sys.objects o ON o.schema_id = s.schema_id AND o.type IN ('U', 'V')
WHERE s.name NOT IN ('sys', 'INFORMATION_SCHEMA')
GROUP BY s.name
ORDER BY s.name`)
	if err != nil {
		return nil, fmt.Errorf("读取 Schema 元数据失败: %w", err)
	}
	defer rows.Close()

	items := make([]*DatabaseSchema, 0)
	for rows.Next() {
		var (
			item       DatabaseSchema
			sizeBytes  int64
			tableCount int64
		)
		if err := rows.Scan(&item.SchemaName, &item.Charset, &item.Collation, &sizeBytes, &tableCount); err != nil {
			return nil, fmt.Errorf("解析 Schema 元数据失败: %w", err)
		}
		item.SizeBytes = sizeBytes
		item.TableCount = int(tableCount)
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 Schema 元数据失败: %w", err)
	}
	return items, nil
}

func collectSQLServerTables(ctx context.Context, db *sql.DB) ([]*DatabaseTable, error) {
	rows, err := db.QueryContext(ctx, `
WITH table_stats AS (
	SELECT
		t.object_id,
		SUM(CASE WHEN i.index_id IN (0, 1) THEN COALESCE(p.rows, 0) ELSE 0 END) AS row_count,
		SUM(CASE WHEN i.index_id IN (0, 1) THEN COALESCE(a.used_pages, 0) ELSE 0 END) * 8192 AS data_size_bytes,
		SUM(CASE WHEN i.index_id > 1 THEN COALESCE(a.used_pages, 0) ELSE 0 END) * 8192 AS index_size_bytes
	FROM sys.tables t
	LEFT JOIN sys.indexes i ON i.object_id = t.object_id
	LEFT JOIN sys.partitions p ON p.object_id = i.object_id AND p.index_id = i.index_id
	LEFT JOIN sys.allocation_units a ON a.container_id = p.partition_id
	GROUP BY t.object_id
)
SELECT
	s.name,
	t.name,
	'BASE TABLE',
	COALESCE(base_index.type_desc, ''),
	COALESCE(ts.row_count, 0),
	COALESCE(ts.data_size_bytes, 0),
	COALESCE(ts.index_size_bytes, 0),
	COALESCE(CAST(ep.value AS NVARCHAR(500)), '')
FROM sys.tables t
JOIN sys.schemas s ON s.schema_id = t.schema_id
LEFT JOIN table_stats ts ON ts.object_id = t.object_id
LEFT JOIN sys.indexes base_index ON base_index.object_id = t.object_id AND base_index.index_id IN (0, 1)
LEFT JOIN sys.extended_properties ep ON ep.major_id = t.object_id AND ep.minor_id = 0 AND ep.class = 1 AND ep.name = 'MS_Description'
WHERE s.name NOT IN ('sys', 'INFORMATION_SCHEMA')
UNION ALL
SELECT
	s.name,
	v.name,
	'VIEW',
	'',
	0,
	0,
	0,
	COALESCE(CAST(ep.value AS NVARCHAR(500)), '')
FROM sys.views v
JOIN sys.schemas s ON s.schema_id = v.schema_id
LEFT JOIN sys.extended_properties ep ON ep.major_id = v.object_id AND ep.minor_id = 0 AND ep.class = 1 AND ep.name = 'MS_Description'
WHERE s.name NOT IN ('sys', 'INFORMATION_SCHEMA')
ORDER BY 1, 2`)
	if err != nil {
		return nil, fmt.Errorf("读取表元数据失败: %w", err)
	}
	defer rows.Close()

	items := make([]*DatabaseTable, 0)
	for rows.Next() {
		var item DatabaseTable
		if err := rows.Scan(
			&item.SchemaName,
			&item.Name,
			&item.TableType,
			&item.Engine,
			&item.RowCount,
			&item.DataSizeBytes,
			&item.IndexSizeBytes,
			&item.Comment,
		); err != nil {
			return nil, fmt.Errorf("解析表元数据失败: %w", err)
		}
		item.Comment = trimText(item.Comment, 500)
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历表元数据失败: %w", err)
	}
	return items, nil
}

func collectSQLServerColumns(ctx context.Context, db *sql.DB) ([]*DatabaseColumn, error) {
	rows, err := db.QueryContext(ctx, `
WITH pk_columns AS (
	SELECT
		ku.TABLE_SCHEMA,
		ku.TABLE_NAME,
		ku.COLUMN_NAME
	FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
	JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE ku
		ON ku.CONSTRAINT_NAME = tc.CONSTRAINT_NAME
		AND ku.TABLE_SCHEMA = tc.TABLE_SCHEMA
		AND ku.TABLE_NAME = tc.TABLE_NAME
	WHERE tc.CONSTRAINT_TYPE = 'PRIMARY KEY'
)
SELECT
	c.TABLE_SCHEMA,
	c.TABLE_NAME,
	c.COLUMN_NAME,
	c.ORDINAL_POSITION,
	CASE
		WHEN c.DATA_TYPE IN ('char', 'varchar', 'nchar', 'nvarchar', 'binary', 'varbinary') THEN
			c.DATA_TYPE + '(' + CASE WHEN c.CHARACTER_MAXIMUM_LENGTH = -1 THEN 'max' ELSE CAST(c.CHARACTER_MAXIMUM_LENGTH AS VARCHAR(20)) END + ')'
		WHEN c.DATA_TYPE IN ('decimal', 'numeric') THEN
			c.DATA_TYPE + '(' + CAST(COALESCE(c.NUMERIC_PRECISION, 0) AS VARCHAR(20)) + ',' + CAST(COALESCE(c.NUMERIC_SCALE, 0) AS VARCHAR(20)) + ')'
		WHEN c.DATA_TYPE IN ('datetime2', 'datetimeoffset', 'time') THEN
			c.DATA_TYPE + '(' + CAST(COALESCE(c.DATETIME_PRECISION, 7) AS VARCHAR(20)) + ')'
		ELSE c.DATA_TYPE
	END,
	COALESCE(c.IS_NULLABLE, ''),
	c.COLUMN_DEFAULT,
	CASE WHEN pk.COLUMN_NAME IS NULL THEN '' ELSE 'PRI' END,
	COALESCE(CAST(ep.value AS NVARCHAR(500)), '')
FROM INFORMATION_SCHEMA.COLUMNS c
LEFT JOIN pk_columns pk
	ON pk.TABLE_SCHEMA = c.TABLE_SCHEMA
	AND pk.TABLE_NAME = c.TABLE_NAME
	AND pk.COLUMN_NAME = c.COLUMN_NAME
LEFT JOIN sys.schemas ss ON ss.name = c.TABLE_SCHEMA
LEFT JOIN sys.objects so ON so.schema_id = ss.schema_id AND so.name = c.TABLE_NAME AND so.type IN ('U', 'V')
LEFT JOIN sys.columns sc ON sc.object_id = so.object_id AND sc.name = c.COLUMN_NAME
LEFT JOIN sys.extended_properties ep ON ep.major_id = sc.object_id AND ep.minor_id = sc.column_id AND ep.class = 1 AND ep.name = 'MS_Description'
WHERE c.TABLE_SCHEMA NOT IN ('sys', 'INFORMATION_SCHEMA')
ORDER BY c.TABLE_SCHEMA, c.TABLE_NAME, c.ORDINAL_POSITION`)
	if err != nil {
		return nil, fmt.Errorf("读取字段元数据失败: %w", err)
	}
	defer rows.Close()

	items := make([]*DatabaseColumn, 0)
	for rows.Next() {
		var (
			item         DatabaseColumn
			isNullable   string
			defaultValue sql.NullString
		)
		if err := rows.Scan(
			&item.SchemaName,
			&item.Table,
			&item.ColumnName,
			&item.OrdinalPosition,
			&item.DataType,
			&isNullable,
			&defaultValue,
			&item.ColumnKey,
			&item.Comment,
		); err != nil {
			return nil, fmt.Errorf("解析字段元数据失败: %w", err)
		}
		item.IsNullable = strings.EqualFold(isNullable, "YES")
		item.DefaultValue = nullString(defaultValue)
		item.Comment = trimText(item.Comment, 500)
		item.IsSensitive = isSensitiveColumn(item.ColumnName)
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历字段元数据失败: %w", err)
	}
	return items, nil
}

func collectSQLServerIndexes(ctx context.Context, db *sql.DB) ([]*DatabaseIndex, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
	s.name,
	t.name,
	i.name,
	CASE
		WHEN i.is_primary_key = 1 THEN 'PRIMARY'
		ELSE COALESCE(i.type_desc, '')
	END,
	COALESCE(STRING_AGG(c.name, ','), ''),
	CASE WHEN i.is_unique = 1 THEN 1 ELSE 0 END,
	COALESCE(MAX(ps.row_count), 0),
	''
FROM sys.tables t
JOIN sys.schemas s ON s.schema_id = t.schema_id
JOIN sys.indexes i ON i.object_id = t.object_id
JOIN sys.index_columns ic ON ic.object_id = i.object_id AND ic.index_id = i.index_id
JOIN sys.columns c ON c.object_id = ic.object_id AND c.column_id = ic.column_id
LEFT JOIN sys.dm_db_partition_stats ps ON ps.object_id = i.object_id AND ps.index_id = i.index_id
WHERE s.name NOT IN ('sys', 'INFORMATION_SCHEMA')
	AND i.name IS NOT NULL
GROUP BY s.name, t.name, i.name, i.type_desc, i.is_unique, i.is_primary_key
ORDER BY s.name, t.name, i.name`)
	if err != nil {
		return nil, fmt.Errorf("读取索引元数据失败: %w", err)
	}
	defer rows.Close()

	items := make([]*DatabaseIndex, 0)
	for rows.Next() {
		var (
			item        DatabaseIndex
			isUnique    int
			comment     string
			cardinality sql.NullInt64
		)
		if err := rows.Scan(
			&item.SchemaName,
			&item.Table,
			&item.IndexName,
			&item.IndexType,
			&item.Columns,
			&isUnique,
			&cardinality,
			&comment,
		); err != nil {
			return nil, fmt.Errorf("解析索引元数据失败: %w", err)
		}
		item.Columns = trimText(item.Columns, 500)
		item.IsUnique = isUnique == 1
		item.Cardinality = nullInt64(cardinality)
		item.Comment = trimText(comment, 500)
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历索引元数据失败: %w", err)
	}
	return items, nil
}
