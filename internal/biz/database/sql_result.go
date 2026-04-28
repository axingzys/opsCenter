package database

import (
	"database/sql"
	"fmt"
)

func readSQLQueryResult(rows *sql.Rows, sqlType, sqlText string, limit int) (*DatabaseQueryResultVO, error) {
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

	initialCapacity := limit
	if initialCapacity <= 0 || initialCapacity > 500 {
		initialCapacity = 500
	}
	resultRows := make([]map[string]any, 0, initialCapacity)
	truncated := false
	totalPreviewBytes := estimateQueryResultHeaderBytes(columns)
	for rows.Next() {
		if limit > 0 && len(resultRows) >= limit {
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
		if limit <= 0 {
			rowBytes := estimateQuerySafeRowBytes(columns, row, queryPreviewCellMaxRunes)
			if totalPreviewBytes+rowBytes > queryPreviewTotalMaxBytes {
				truncated = true
				break
			}
			totalPreviewBytes += rowBytes
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
