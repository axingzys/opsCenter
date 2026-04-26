package database

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func testPostgreSQLConnection(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (string, error) {
	db, err := openPostgreSQLDB(item, credential)
	if err != nil {
		return "", err
	}
	defer db.Close()

	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(testCtx); err != nil {
		return "", fmt.Errorf("连接数据库失败: %w", err)
	}

	return readPostgreSQLCompatibleVersion(testCtx, db, item.DBType)
}

func collectPostgreSQLMetadata(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (string, []*DatabaseSchema, []*DatabaseTable, []*DatabaseColumn, []*DatabaseIndex, error) {
	db, err := openPostgreSQLDB(item, credential)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	defer db.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := db.PingContext(queryCtx); err != nil {
		return "", nil, nil, nil, nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	version, err := readPostgreSQLCompatibleVersion(queryCtx, db, item.DBType)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}

	schemas, err := collectPostgreSQLSchemas(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	tables, err := collectPostgreSQLTables(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	columns, err := collectPostgreSQLColumns(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	indexes, err := collectPostgreSQLIndexes(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	return version, schemas, tables, columns, indexes, nil
}

func executePostgreSQLQuery(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, schemaName, sqlType, sqlText string, limit, timeoutSeconds int) (*DatabaseQueryResultVO, error) {
	db, err := openPostgreSQLDB(item, credential)
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
		if _, err := db.ExecContext(queryCtx, "SET search_path TO "+quotePostgreSQLIdentifier(schemaName)+", public"); err != nil {
			return nil, fmt.Errorf("设置 Schema 失败: %w", err)
		}
	}

	rows, err := db.QueryContext(queryCtx, sqlText)
	if err != nil {
		return nil, fmt.Errorf("执行查询失败: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("读取结果列失败: %w", err)
	}
	columnTypes := make([]string, 0, len(columns))
	if types, err := rows.ColumnTypes(); err == nil {
		for _, item := range types {
			columnTypes = append(columnTypes, item.DatabaseTypeName())
		}
	}
	for len(columnTypes) < len(columns) {
		columnTypes = append(columnTypes, "")
	}

	resultRows := make([]map[string]any, 0, limit)
	truncated := false
	for rows.Next() {
		if len(resultRows) >= limit {
			truncated = true
			break
		}
		values := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("读取结果行失败: %w", err)
		}
		row := make(map[string]any, len(columns))
		for i, column := range columns {
			row[column] = normalizeSQLValue(values[i])
		}
		resultRows = append(resultRows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历查询结果失败: %w", err)
	}

	return &DatabaseQueryResultVO{
		SQLType:      sqlType,
		ExecutedSQL:  sqlText,
		Columns:      columns,
		ColumnTypes:  columnTypes,
		Rows:         resultRows,
		RowsReturned: len(resultRows),
		Limit:        limit,
		Truncated:    truncated,
		Message:      "查询成功",
	}, nil
}

func openPostgreSQLDB(item *DatabaseInstance, credential *ConnectionCredential) (*sql.DB, error) {
	if credential == nil || strings.TrimSpace(credential.Username) == "" {
		return nil, fmt.Errorf("凭据用户名不能为空")
	}
	if credential.Password == "" {
		return nil, fmt.Errorf("凭据密码不能为空")
	}

	dbName := strings.TrimSpace(item.DefaultDatabase)
	if dbName == "" {
		dbName = strings.TrimSpace(credential.Username)
	}
	sslMode := "disable"
	if item.TLSEnabled {
		sslMode = "require"
	}

	dsn := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(strings.TrimSpace(credential.Username), credential.Password),
		Host:   net.JoinHostPort(strings.TrimSpace(item.Host), fmt.Sprintf("%d", item.Port)),
		Path:   dbName,
	}
	query := dsn.Query()
	query.Set("sslmode", sslMode)
	query.Set("connect_timeout", "5")
	dsn.RawQuery = query.Encode()

	db, err := sql.Open("pgx", dsn.String())
	if err != nil {
		return nil, fmt.Errorf("创建数据库连接失败: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Minute)
	return db, nil
}

func collectPostgreSQLSchemas(ctx context.Context, db *sql.DB) ([]*DatabaseSchema, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
	n.nspname,
	COALESCE(pg_encoding_to_char(d.encoding), ''),
	COALESCE(d.datcollate, ''),
	COALESCE(SUM(CASE WHEN c.oid IS NULL THEN 0 ELSE pg_total_relation_size(c.oid) END), 0)::bigint,
	COUNT(c.oid)::bigint
FROM pg_namespace n
CROSS JOIN pg_database d
LEFT JOIN pg_class c ON c.relnamespace = n.oid AND c.relkind IN ('r', 'p', 'v', 'm', 'f')
WHERE d.datname = current_database()
	AND n.nspname NOT IN ('pg_catalog', 'information_schema')
	AND n.nspname NOT LIKE 'pg_toast%'
GROUP BY n.nspname, d.encoding, d.datcollate
ORDER BY n.nspname`)
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

func collectPostgreSQLTables(ctx context.Context, db *sql.DB) ([]*DatabaseTable, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
	n.nspname,
	c.relname,
	CASE c.relkind
		WHEN 'r' THEN 'BASE TABLE'
		WHEN 'p' THEN 'PARTITIONED TABLE'
		WHEN 'v' THEN 'VIEW'
		WHEN 'm' THEN 'MATERIALIZED VIEW'
		WHEN 'f' THEN 'FOREIGN TABLE'
		ELSE c.relkind::text
	END,
	'',
	COALESCE(GREATEST(c.reltuples, 0)::bigint, 0),
	COALESCE(pg_relation_size(c.oid), 0),
	COALESCE(pg_indexes_size(c.oid), 0),
	COALESCE(obj_description(c.oid), '')
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
	AND n.nspname NOT LIKE 'pg_toast%'
	AND c.relkind IN ('r', 'p', 'v', 'm', 'f')
ORDER BY n.nspname, c.relname`)
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

func collectPostgreSQLColumns(ctx context.Context, db *sql.DB) ([]*DatabaseColumn, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
	c.table_schema,
	c.table_name,
	c.column_name,
	c.ordinal_position,
	CASE
		WHEN c.character_maximum_length IS NOT NULL THEN c.data_type || '(' || c.character_maximum_length::text || ')'
		WHEN c.numeric_precision IS NOT NULL AND c.numeric_scale IS NOT NULL THEN c.data_type || '(' || c.numeric_precision::text || ',' || c.numeric_scale::text || ')'
		WHEN c.numeric_precision IS NOT NULL THEN c.data_type || '(' || c.numeric_precision::text || ')'
		ELSE c.udt_name
	END,
	COALESCE(c.is_nullable, ''),
	c.column_default,
	CASE
		WHEN EXISTS (
			SELECT 1
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu
				ON kcu.constraint_name = tc.constraint_name
				AND kcu.constraint_schema = tc.constraint_schema
				AND kcu.table_schema = tc.table_schema
				AND kcu.table_name = tc.table_name
			WHERE tc.constraint_type = 'PRIMARY KEY'
				AND kcu.table_schema = c.table_schema
				AND kcu.table_name = c.table_name
				AND kcu.column_name = c.column_name
		) THEN 'PRI'
		ELSE ''
	END,
	COALESCE(pg_catalog.col_description((quote_ident(c.table_schema) || '.' || quote_ident(c.table_name))::regclass::oid, c.ordinal_position), '')
FROM information_schema.columns c
WHERE c.table_schema NOT IN ('pg_catalog', 'information_schema')
	AND c.table_schema NOT LIKE 'pg_toast%'
ORDER BY c.table_schema, c.table_name, c.ordinal_position`)
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

func collectPostgreSQLIndexes(ctx context.Context, db *sql.DB) ([]*DatabaseIndex, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
	ns.nspname,
	tbl.relname,
	idx.relname,
	CASE
		WHEN ix.indisprimary THEN 'PRIMARY'
		WHEN ix.indisunique THEN 'UNIQUE'
		ELSE COALESCE(am.amname, 'INDEX')
	END,
	COALESCE(pg_get_indexdef(idx.oid), ''),
	ix.indisunique,
	COALESCE(GREATEST(idx.reltuples, 0)::bigint, 0),
	COALESCE(obj_description(idx.oid), '')
FROM pg_index ix
JOIN pg_class idx ON idx.oid = ix.indexrelid
JOIN pg_class tbl ON tbl.oid = ix.indrelid
JOIN pg_namespace ns ON ns.oid = tbl.relnamespace
LEFT JOIN pg_am am ON am.oid = idx.relam
WHERE ns.nspname NOT IN ('pg_catalog', 'information_schema')
	AND ns.nspname NOT LIKE 'pg_toast%'
ORDER BY ns.nspname, tbl.relname, idx.relname`)
	if err != nil {
		return nil, fmt.Errorf("读取索引元数据失败: %w", err)
	}
	defer rows.Close()

	items := make([]*DatabaseIndex, 0)
	for rows.Next() {
		var item DatabaseIndex
		if err := rows.Scan(
			&item.SchemaName,
			&item.Table,
			&item.IndexName,
			&item.IndexType,
			&item.Columns,
			&item.IsUnique,
			&item.Cardinality,
			&item.Comment,
		); err != nil {
			return nil, fmt.Errorf("解析索引元数据失败: %w", err)
		}
		item.Columns = trimText(normalizePostgreSQLIndexColumns(item.Columns), 500)
		item.Comment = trimText(item.Comment, 500)
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历索引元数据失败: %w", err)
	}
	return items, nil
}

func quotePostgreSQLIdentifier(value string) string {
	return `"` + strings.ReplaceAll(strings.TrimSpace(value), `"`, `""`) + `"`
}

func normalizePostgreSQLIndexColumns(indexDef string) string {
	indexDef = strings.TrimSpace(indexDef)
	if indexDef == "" {
		return ""
	}

	start := strings.Index(indexDef, "(")
	if start < 0 {
		return indexDef
	}

	depth := 0
	contentStart := -1
	for i := start; i < len(indexDef); i++ {
		switch indexDef[i] {
		case '(':
			depth++
			if depth == 1 {
				contentStart = i + 1
			}
		case ')':
			depth--
			if depth == 0 && contentStart >= 0 {
				return strings.TrimSpace(indexDef[contentStart:i])
			}
		}
	}

	return indexDef
}
