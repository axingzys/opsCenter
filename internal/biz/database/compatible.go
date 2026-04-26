package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func readMySQLCompatibleVersion(ctx context.Context, db *sql.DB, dbType string) (string, error) {
	dbType = normalizeDBType(dbType)
	switch dbType {
	case DBTypeTiDB:
		if version := firstNonEmptyQueryValue(ctx, db, "SELECT tidb_version()"); version != "" {
			return trimVersionText(version), nil
		}
	case DBTypeOceanBase:
		version := firstNonEmptyQueryValue(ctx, db, "SELECT VERSION()")
		comment := firstNonEmptyQueryValue(ctx, db, "SELECT @@version_comment")
		return joinVersionParts(DBTypeText(dbType), version, comment), nil
	}

	version := firstNonEmptyQueryValue(ctx, db, "SELECT VERSION()")
	if version == "" {
		return "", fmt.Errorf("读取数据库版本失败")
	}
	return trimVersionText(version), nil
}

func readPostgreSQLCompatibleVersion(ctx context.Context, db *sql.DB, dbType string) (string, error) {
	version := firstNonEmptyQueryValue(ctx, db, "SELECT version()")
	if version == "" {
		return "", fmt.Errorf("读取数据库版本失败")
	}
	return trimVersionText(version), nil
}

func firstNonEmptyQueryValue(ctx context.Context, db *sql.DB, query string) string {
	var value sql.NullString
	if err := db.QueryRowContext(ctx, query).Scan(&value); err != nil {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func trimVersionText(value string) string {
	lines := strings.Split(strings.TrimSpace(value), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, " | ")
}

func joinVersionParts(prefix string, parts ...string) string {
	cleaned := make([]string, 0, len(parts)+1)
	if strings.TrimSpace(prefix) != "" {
		cleaned = append(cleaned, strings.TrimSpace(prefix))
	}
	for _, part := range parts {
		part = trimVersionText(part)
		if part == "" {
			continue
		}
		duplicate := false
		for _, existing := range cleaned {
			if strings.Contains(strings.ToLower(existing), strings.ToLower(part)) || strings.Contains(strings.ToLower(part), strings.ToLower(existing)) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			cleaned = append(cleaned, part)
		}
	}
	if len(cleaned) == 0 {
		return ""
	}
	return strings.Join(cleaned, " ")
}
