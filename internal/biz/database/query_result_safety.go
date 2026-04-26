package database

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	queryPreviewCellMaxRunes  = 2048
	queryPreviewTotalMaxBytes = 4 * 1024 * 1024
	queryExportCellMaxRunes   = 8192
	queryExportTotalMaxBytes  = 10 * 1024 * 1024
)

var sensitiveQueryColumnKeywords = []string{
	"password",
	"passwd",
	"pwd",
	"token",
	"secret",
	"phone",
	"mobile",
	"email",
}

type queryRawBytes []byte

type queryResultSafetyPolicy struct {
	maxCellRunes     int
	maxTotalBytes    int
	rejectTotalBytes bool
}

type queryCellSafety struct {
	truncated bool
	masked    bool
	binary    bool
}

func queryResultSafetyPolicyForAction(action string) queryResultSafetyPolicy {
	if normalizeAuditAction(action) == DatabaseAuditActionQueryExport {
		return queryResultSafetyPolicy{
			maxCellRunes:     queryExportCellMaxRunes,
			maxTotalBytes:    queryExportTotalMaxBytes,
			rejectTotalBytes: true,
		}
	}
	return queryResultSafetyPolicy{
		maxCellRunes:  queryPreviewCellMaxRunes,
		maxTotalBytes: queryPreviewTotalMaxBytes,
	}
}

func sanitizeDatabaseQueryResult(result *DatabaseQueryResultVO, policy queryResultSafetyPolicy) error {
	if result == nil {
		return nil
	}
	if policy.maxCellRunes <= 0 {
		policy.maxCellRunes = queryPreviewCellMaxRunes
	}

	totalBytes := estimateQueryResultHeaderBytes(result.Columns)
	safeRows := make([]map[string]any, 0, len(result.Rows))
	for _, row := range result.Rows {
		safeRow := make(map[string]any, len(row))
		rowBytes := 0
		for _, column := range result.Columns {
			value, safety := safeQueryCellPreview(column, row[column], policy.maxCellRunes)
			safeRow[column] = value
			rowBytes += estimateQueryCellBytes(column, value)
			result.CellTruncated = result.CellTruncated || safety.truncated
			result.CellsMasked = result.CellsMasked || safety.masked
			result.BinaryPreviewed = result.BinaryPreviewed || safety.binary
		}
		for column, value := range row {
			if _, ok := safeRow[column]; ok {
				continue
			}
			value, safety := safeQueryCellPreview(column, value, policy.maxCellRunes)
			safeRow[column] = value
			rowBytes += estimateQueryCellBytes(column, value)
			result.CellTruncated = result.CellTruncated || safety.truncated
			result.CellsMasked = result.CellsMasked || safety.masked
			result.BinaryPreviewed = result.BinaryPreviewed || safety.binary
		}

		if policy.maxTotalBytes > 0 && totalBytes+rowBytes > policy.maxTotalBytes {
			if policy.rejectTotalBytes {
				return fmt.Errorf("导出结果超过大小限制（最大 %s），请减少导出行数或列数后重试", humanizeBytes(int64(policy.maxTotalBytes)))
			}
			result.Truncated = true
			break
		}
		safeRows = append(safeRows, safeRow)
		totalBytes += rowBytes
	}

	if len(safeRows) < len(result.Rows) {
		result.Truncated = true
	}
	result.Rows = safeRows
	result.RowsReturned = len(safeRows)
	result.ResultBytes = totalBytes
	if result.CellTruncated && !strings.Contains(result.Message, "字段已安全预览") {
		if strings.TrimSpace(result.Message) == "" {
			result.Message = "字段已安全预览"
		} else {
			result.Message = strings.TrimSpace(result.Message + "，字段已安全预览")
		}
	}
	return nil
}

func safeQueryCellPreview(column string, value any, maxRunes int) (any, queryCellSafety) {
	if isSensitiveQueryColumn(column) {
		if value == nil {
			return nil, queryCellSafety{}
		}
		return "[已脱敏]", queryCellSafety{masked: true}
	}

	switch v := value.(type) {
	case nil:
		return nil, queryCellSafety{}
	case queryRawBytes:
		data := []byte(v)
		if !isLikelyTextBytes(data) {
			return fmt.Sprintf("[二进制内容, %d bytes]", len(data)), queryCellSafety{binary: true}
		}
		return safeQueryStringPreview(string(data), maxRunes)
	case []byte:
		if !isLikelyTextBytes(v) {
			return fmt.Sprintf("[二进制内容, %d bytes]", len(v)), queryCellSafety{binary: true}
		}
		return safeQueryStringPreview(string(v), maxRunes)
	case string:
		return safeQueryStringPreview(v, maxRunes)
	case time.Time:
		return v.Format("2006-01-02 15:04:05"), queryCellSafety{}
	default:
		return value, queryCellSafety{}
	}
}

func safeQueryStringPreview(value string, maxRunes int) (string, queryCellSafety) {
	if maxRunes <= 0 {
		maxRunes = queryPreviewCellMaxRunes
	}
	runeCount := utf8.RuneCountInString(value)
	if runeCount <= maxRunes {
		return value, queryCellSafety{}
	}
	marker := fmt.Sprintf("... [已截断, 原始长度=%d]", runeCount)
	markerRunes := []rune(marker)
	keep := maxRunes - len(markerRunes)
	if keep < 0 {
		keep = 0
	}
	runes := []rune(value)
	preview := string(append(runes[:keep], markerRunes...))
	return preview, queryCellSafety{truncated: true}
}

func isSensitiveQueryColumn(column string) bool {
	normalized := strings.ToLower(strings.TrimSpace(column))
	normalized = strings.NewReplacer(" ", "", "_", "", "-", "", ".", "").Replace(normalized)
	if normalized == "" {
		return false
	}
	for _, keyword := range sensitiveQueryColumnKeywords {
		if strings.Contains(normalized, keyword) {
			return true
		}
	}
	return false
}

func isLikelyTextBytes(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	if !utf8.Valid(data) {
		return false
	}
	for _, r := range string(data) {
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func estimateQueryResultHeaderBytes(columns []string) int {
	total := 2
	for _, column := range columns {
		total += len(column) + 4
	}
	return total
}

func estimateQueryCellBytes(column string, value any) int {
	return len(column) + len(fmt.Sprint(value))*2 + 16
}

func buildQueryExportAuditReason(result *DatabaseQueryResultVO) string {
	if result == nil {
		return ""
	}
	payload := map[string]any{
		"exportRows":      result.RowsReturned,
		"columns":         len(result.Columns),
		"truncated":       result.Truncated,
		"cellTruncated":   result.CellTruncated,
		"masked":          result.CellsMasked,
		"binaryPreviewed": result.BinaryPreviewed,
		"resultBytes":     result.ResultBytes,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return trimText(string(data), 500)
}
