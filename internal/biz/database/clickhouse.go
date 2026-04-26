package database

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"net"
	"strings"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
)

func testClickHouseConnection(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (string, error) {
	db, err := openClickHouseDB(item, credential, "")
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
	if err := db.QueryRowContext(testCtx, "SELECT version()").Scan(&version); err != nil {
		return "", fmt.Errorf("读取数据库版本失败: %w", err)
	}
	return version, nil
}

func collectClickHouseMetadata(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (string, []*DatabaseSchema, []*DatabaseTable, []*DatabaseColumn, []*DatabaseIndex, error) {
	db, err := openClickHouseDB(item, credential, "")
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
	if err := db.QueryRowContext(queryCtx, "SELECT version()").Scan(&version); err != nil {
		return "", nil, nil, nil, nil, fmt.Errorf("读取数据库版本失败: %w", err)
	}

	schemas, err := collectClickHouseSchemas(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	tables, err := collectClickHouseTables(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	columns, err := collectClickHouseColumns(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	indexes, err := collectClickHouseIndexes(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	return version, schemas, tables, columns, indexes, nil
}

func executeClickHouseQuery(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, schemaName, sqlType, sqlText string, limit, timeoutSeconds int) (*DatabaseQueryResultVO, error) {
	db, err := openClickHouseDB(item, credential, schemaName)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	if err := db.PingContext(queryCtx); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	rows, err := db.QueryContext(queryCtx, sqlText)
	if err != nil {
		return nil, fmt.Errorf("执行查询失败: %w", err)
	}
	defer rows.Close()

	return readSQLQueryResult(rows, sqlType, sqlText, limit)
}

func openClickHouseDB(item *DatabaseInstance, credential *ConnectionCredential, databaseName string) (*sql.DB, error) {
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
		dbName = "default"
	}

	protocol := clickhouse.Native
	if strings.EqualFold(connectionParamString(params, "protocol"), "http") {
		protocol = clickhouse.HTTP
	}

	options := &clickhouse.Options{
		Protocol: protocol,
		Addr:     []string{net.JoinHostPort(strings.TrimSpace(item.Host), fmt.Sprintf("%d", item.Port))},
		Auth: clickhouse.Auth{
			Database: dbName,
			Username: strings.TrimSpace(credential.Username),
			Password: credential.Password,
		},
		DialTimeout:     5 * time.Second,
		ReadTimeout:     time.Duration(connectionParamInt(params, 10, "readTimeoutSeconds", "read_timeout_seconds")) * time.Second,
		MaxOpenConns:    1,
		MaxIdleConns:    1,
		ConnMaxLifetime: time.Minute,
	}
	if item.TLSEnabled {
		options.TLS = &tls.Config{InsecureSkipVerify: connectionParamBool(params, false, "insecureSkipVerify", "insecure_skip_verify")}
	}

	db := clickhouse.OpenDB(options)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Minute)
	return db, nil
}

func collectClickHouseSchemas(ctx context.Context, db *sql.DB) ([]*DatabaseSchema, error) {
	rows, err := db.QueryContext(ctx, `
WITH table_stats AS (
	SELECT
		database,
		count() AS table_count
	FROM system.tables
	WHERE database NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA')
	GROUP BY database
),
size_stats AS (
	SELECT
		database,
		sum(bytes_on_disk) AS size_bytes
	FROM system.parts
	WHERE active AND database NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA')
	GROUP BY database
)
SELECT
	d.name,
	'',
	'',
	COALESCE(s.size_bytes, 0),
	COALESCE(t.table_count, 0)
FROM system.databases d
LEFT JOIN table_stats t ON t.database = d.name
LEFT JOIN size_stats s ON s.database = d.name
WHERE d.name NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA')
ORDER BY d.name`)
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

func collectClickHouseTables(ctx context.Context, db *sql.DB) ([]*DatabaseTable, error) {
	rows, err := db.QueryContext(ctx, `
WITH part_stats AS (
	SELECT
		database,
		table,
		sum(rows) AS row_count,
		sum(bytes_on_disk) AS data_size_bytes
	FROM system.parts
	WHERE active
	GROUP BY database, table
)
SELECT
	t.database,
	t.name,
	multiIf(positionCaseInsensitive(t.engine, 'View') > 0, 'VIEW', 'BASE TABLE'),
	COALESCE(t.engine, ''),
	COALESCE(p.row_count, 0),
	COALESCE(p.data_size_bytes, 0),
	0,
	''
FROM system.tables t
LEFT JOIN part_stats p ON p.database = t.database AND p.table = t.name
WHERE t.database NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA')
ORDER BY t.database, t.name`)
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

func collectClickHouseColumns(ctx context.Context, db *sql.DB) ([]*DatabaseColumn, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
	database,
	table,
	name,
	position,
	type,
	CASE WHEN positionCaseInsensitive(type, 'Nullable(') = 1 THEN 1 ELSE 0 END,
	COALESCE(default_expression, ''),
	CASE WHEN is_in_primary_key = 1 THEN 'PRI' ELSE '' END,
	''
FROM system.columns
WHERE database NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA')
ORDER BY database, table, position`)
	if err != nil {
		return nil, fmt.Errorf("读取字段元数据失败: %w", err)
	}
	defer rows.Close()

	items := make([]*DatabaseColumn, 0)
	for rows.Next() {
		var (
			item       DatabaseColumn
			isNullable uint8
		)
		if err := rows.Scan(
			&item.SchemaName,
			&item.Table,
			&item.ColumnName,
			&item.OrdinalPosition,
			&item.DataType,
			&isNullable,
			&item.DefaultValue,
			&item.ColumnKey,
			&item.Comment,
		); err != nil {
			return nil, fmt.Errorf("解析字段元数据失败: %w", err)
		}
		item.IsNullable = isNullable == 1
		item.Comment = trimText(item.Comment, 500)
		item.IsSensitive = isSensitiveColumn(item.ColumnName)
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历字段元数据失败: %w", err)
	}
	return items, nil
}

func collectClickHouseIndexes(ctx context.Context, db *sql.DB) ([]*DatabaseIndex, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
	database,
	name,
	'PRIMARY',
	'PRIMARY',
	COALESCE(primary_key, ''),
	1,
	0,
	''
FROM system.tables
WHERE database NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA')
	AND primary_key != ''
UNION ALL
SELECT
	database,
	name,
	'ORDER_BY',
	'ORDER BY',
	COALESCE(sorting_key, ''),
	0,
	0,
	''
FROM system.tables
WHERE database NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA')
	AND sorting_key != ''
	AND sorting_key != primary_key
ORDER BY 1, 2, 3`)
	if err != nil {
		return nil, fmt.Errorf("读取索引元数据失败: %w", err)
	}
	defer rows.Close()

	items := make([]*DatabaseIndex, 0)
	for rows.Next() {
		var (
			item        DatabaseIndex
			isUnique    uint8
			cardinality int64
			comment     string
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
		item.Cardinality = cardinality
		item.Comment = trimText(comment, 500)
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历索引元数据失败: %w", err)
	}
	return items, nil
}
