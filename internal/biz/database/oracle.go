package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	go_ora "github.com/sijms/go-ora/v2"
)

func testOracleConnection(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (string, error) {
	db, err := openOracleDB(item, credential)
	if err != nil {
		return "", err
	}
	defer db.Close()

	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(testCtx); err != nil {
		return "", fmt.Errorf("连接数据库失败: %w", err)
	}

	version, err := readOracleVersion(testCtx, db)
	if err != nil {
		return "", err
	}
	return version, nil
}

func collectOracleMetadata(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (string, []*DatabaseSchema, []*DatabaseTable, []*DatabaseColumn, []*DatabaseIndex, error) {
	db, err := openOracleDB(item, credential)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	defer db.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := db.PingContext(queryCtx); err != nil {
		return "", nil, nil, nil, nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	version, err := readOracleVersion(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}

	schemas, err := collectOracleSchemas(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	tables, err := collectOracleTables(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	columns, err := collectOracleColumns(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	indexes, err := collectOracleIndexes(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	return version, schemas, tables, columns, indexes, nil
}

func executeOracleQuery(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, schemaName, sqlType, sqlText string, limit, timeoutSeconds int) (*DatabaseQueryResultVO, error) {
	db, err := openOracleDB(item, credential)
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
		if _, err := db.ExecContext(queryCtx, "ALTER SESSION SET CURRENT_SCHEMA = "+quoteOracleIdentifier(schemaName)); err != nil {
			return nil, fmt.Errorf("设置 Schema 失败: %w", err)
		}
	}

	rows, err := db.QueryContext(queryCtx, sqlText)
	if err != nil {
		return nil, fmt.Errorf("执行查询失败: %w", err)
	}
	defer rows.Close()

	return readSQLQueryResult(rows, sqlType, sqlText, limit)
}

func openOracleDB(item *DatabaseInstance, credential *ConnectionCredential) (*sql.DB, error) {
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

	serviceName := connectionParamString(params, "serviceName", "service_name", "service", "service name")
	if serviceName == "" {
		serviceName = strings.TrimSpace(item.DefaultDatabase)
	}
	options := map[string]string{
		"TRACE FILE": "off",
	}
	if sid := connectionParamString(params, "sid", "SID"); sid != "" {
		options["SID"] = sid
	}
	if serverMode := connectionParamString(params, "server", "serverMode", "server_mode"); serverMode != "" {
		options["SERVER"] = serverMode
	}

	dsn := go_ora.BuildUrl(
		strings.TrimSpace(item.Host),
		item.Port,
		serviceName,
		strings.TrimSpace(credential.Username),
		credential.Password,
		options,
	)
	db, err := sql.Open("oracle", dsn)
	if err != nil {
		return nil, fmt.Errorf("创建数据库连接失败: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Minute)
	return db, nil
}

func readOracleVersion(ctx context.Context, db *sql.DB) (string, error) {
	candidates := []string{
		"SELECT product || ' ' || version FROM product_component_version WHERE product LIKE 'Oracle%' AND ROWNUM = 1",
		"SELECT banner FROM v$version WHERE ROWNUM = 1",
	}
	for _, statement := range candidates {
		var version string
		if err := db.QueryRowContext(ctx, statement).Scan(&version); err == nil && strings.TrimSpace(version) != "" {
			return version, nil
		}
	}
	return "", fmt.Errorf("读取数据库版本失败")
}

func collectOracleSchemas(ctx context.Context, db *sql.DB) ([]*DatabaseSchema, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
	owner,
	'',
	'',
	0,
	COUNT(*)
FROM (
	SELECT owner, table_name AS object_name FROM all_tables
	UNION ALL
	SELECT owner, view_name AS object_name FROM all_views
)
WHERE owner NOT IN ('SYS', 'SYSTEM', 'OUTLN', 'DBSNMP', 'APPQOSSYS', 'GSMADMIN_INTERNAL', 'XDB', 'CTXSYS', 'MDSYS', 'ORDSYS', 'WMSYS', 'OLAPSYS', 'LBACSYS', 'DVSYS', 'GGSYS', 'AUDSYS', 'ANONYMOUS')
GROUP BY owner
ORDER BY owner`)
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

func collectOracleTables(ctx context.Context, db *sql.DB) ([]*DatabaseTable, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
	t.owner,
	t.table_name,
	'BASE TABLE',
	'',
	COALESCE(t.num_rows, 0),
	COALESCE(t.blocks, 0) * 8192,
	0,
	COALESCE(c.comments, '')
FROM all_tables t
LEFT JOIN all_tab_comments c ON c.owner = t.owner AND c.table_name = t.table_name
WHERE t.owner NOT IN ('SYS', 'SYSTEM', 'OUTLN', 'DBSNMP', 'APPQOSSYS', 'GSMADMIN_INTERNAL', 'XDB', 'CTXSYS', 'MDSYS', 'ORDSYS', 'WMSYS', 'OLAPSYS', 'LBACSYS', 'DVSYS', 'GGSYS', 'AUDSYS', 'ANONYMOUS')
UNION ALL
SELECT
	v.owner,
	v.view_name,
	'VIEW',
	'',
	0,
	0,
	0,
	COALESCE(c.comments, '')
FROM all_views v
LEFT JOIN all_tab_comments c ON c.owner = v.owner AND c.table_name = v.view_name
WHERE v.owner NOT IN ('SYS', 'SYSTEM', 'OUTLN', 'DBSNMP', 'APPQOSSYS', 'GSMADMIN_INTERNAL', 'XDB', 'CTXSYS', 'MDSYS', 'ORDSYS', 'WMSYS', 'OLAPSYS', 'LBACSYS', 'DVSYS', 'GGSYS', 'AUDSYS', 'ANONYMOUS')
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

func collectOracleColumns(ctx context.Context, db *sql.DB) ([]*DatabaseColumn, error) {
	rows, err := db.QueryContext(ctx, `
WITH pk_columns AS (
	SELECT
		acc.owner,
		acc.table_name,
		acc.column_name
	FROM all_constraints ac
	JOIN all_cons_columns acc
		ON acc.owner = ac.owner
		AND acc.constraint_name = ac.constraint_name
		AND acc.table_name = ac.table_name
	WHERE ac.constraint_type = 'P'
)
SELECT
	c.owner,
	c.table_name,
	c.column_name,
	c.column_id,
	CASE
		WHEN c.data_type IN ('CHAR', 'NCHAR', 'VARCHAR2', 'NVARCHAR2', 'RAW') THEN c.data_type || '(' || c.data_length || ')'
		WHEN c.data_type = 'NUMBER' AND c.data_precision IS NOT NULL THEN c.data_type || '(' || c.data_precision || ',' || COALESCE(c.data_scale, 0) || ')'
		ELSE c.data_type
	END,
	c.nullable,
	c.data_default,
	CASE WHEN pk.column_name IS NULL THEN '' ELSE 'PRI' END,
	COALESCE(cc.comments, '')
FROM all_tab_columns c
LEFT JOIN pk_columns pk
	ON pk.owner = c.owner
	AND pk.table_name = c.table_name
	AND pk.column_name = c.column_name
LEFT JOIN all_col_comments cc
	ON cc.owner = c.owner
	AND cc.table_name = c.table_name
	AND cc.column_name = c.column_name
WHERE c.owner NOT IN ('SYS', 'SYSTEM', 'OUTLN', 'DBSNMP', 'APPQOSSYS', 'GSMADMIN_INTERNAL', 'XDB', 'CTXSYS', 'MDSYS', 'ORDSYS', 'WMSYS', 'OLAPSYS', 'LBACSYS', 'DVSYS', 'GGSYS', 'AUDSYS', 'ANONYMOUS')
ORDER BY c.owner, c.table_name, c.column_id`)
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
		item.IsNullable = strings.EqualFold(isNullable, "Y")
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

func collectOracleIndexes(ctx context.Context, db *sql.DB) ([]*DatabaseIndex, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
	i.table_owner,
	i.table_name,
	i.index_name,
	COALESCE(i.index_type, ''),
	COALESCE(LISTAGG(ic.column_name, ',') WITHIN GROUP (ORDER BY ic.column_position), ''),
	CASE WHEN i.uniqueness = 'UNIQUE' THEN 1 ELSE 0 END,
	COALESCE(i.distinct_keys, 0),
	''
FROM all_indexes i
JOIN all_ind_columns ic
	ON ic.index_owner = i.owner
	AND ic.index_name = i.index_name
	AND ic.table_owner = i.table_owner
	AND ic.table_name = i.table_name
WHERE i.table_owner NOT IN ('SYS', 'SYSTEM', 'OUTLN', 'DBSNMP', 'APPQOSSYS', 'GSMADMIN_INTERNAL', 'XDB', 'CTXSYS', 'MDSYS', 'ORDSYS', 'WMSYS', 'OLAPSYS', 'LBACSYS', 'DVSYS', 'GGSYS', 'AUDSYS', 'ANONYMOUS')
GROUP BY i.table_owner, i.table_name, i.index_name, i.index_type, i.uniqueness, i.distinct_keys
ORDER BY i.table_owner, i.table_name, i.index_name`)
	if err != nil {
		return nil, fmt.Errorf("读取索引元数据失败: %w", err)
	}
	defer rows.Close()

	items := make([]*DatabaseIndex, 0)
	for rows.Next() {
		var (
			item        DatabaseIndex
			isUnique    int
			cardinality sql.NullInt64
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
		item.Cardinality = nullInt64(cardinality)
		item.Comment = trimText(comment, 500)
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历索引元数据失败: %w", err)
	}
	return items, nil
}

func quoteOracleIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
