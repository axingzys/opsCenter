package database

import (
	"context"
	"fmt"
	"strings"
)

func cleanPostgreSQLDatabase(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, databaseName string) error {
	databaseName = strings.TrimSpace(databaseName)
	if databaseName == "" {
		return fmt.Errorf("PostgreSQL 清空目标库需要明确数据库名")
	}
	restoreItem := *item
	restoreItem.DefaultDatabase = databaseName
	db, err := openPostgreSQLDB(&restoreItem, credential)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("连接 PostgreSQL 恢复目标数据库失败: %w", err)
	}
	rows, err := db.QueryContext(ctx, `
SELECT nspname
FROM pg_namespace
WHERE nspname <> 'information_schema'
	AND nspname NOT LIKE 'pg_%'
ORDER BY nspname`)
	if err != nil {
		return fmt.Errorf("读取 PostgreSQL 目标库 Schema 失败: %w", err)
	}
	defer rows.Close()

	schemas := make([]string, 0)
	for rows.Next() {
		var schema string
		if err := rows.Scan(&schema); err != nil {
			return fmt.Errorf("解析 PostgreSQL 目标库 Schema 失败: %w", err)
		}
		schema = strings.TrimSpace(schema)
		if schema != "" {
			schemas = append(schemas, schema)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("遍历 PostgreSQL 目标库 Schema 失败: %w", err)
	}

	for _, schema := range schemas {
		if _, err := db.ExecContext(ctx, "DROP SCHEMA IF EXISTS "+quotePostgreSQLIdentifier(schema)+" CASCADE"); err != nil {
			return fmt.Errorf("删除 PostgreSQL Schema %s 失败: %w", schema, err)
		}
	}
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS public"); err != nil {
		return fmt.Errorf("重建 PostgreSQL public Schema 失败: %w", err)
	}
	return nil
}
