package database

import (
	"fmt"
	"strings"
	"unicode"
)

type SQLSafetyResult struct {
	Allowed bool
	SQLText string
	SQLType string
	Message string
}

type sqlAnalyzeOptions struct {
	appendLimit bool
	limit       int
}

type SQLWriteSafetyResult struct {
	Allowed           bool
	SQLText           string
	SQLType           string
	RiskLevel         string
	ConfirmRequired   bool
	ReasonRequired    bool
	RowsAffectedLimit int64
	Message           string
}

func AnalyzeReadOnlySQL(sqlText string, limit int) SQLSafetyResult {
	return AnalyzeReadOnlySQLByDB("", sqlText, limit)
}

func AnalyzeReadOnlySQLByDB(dbType, sqlText string, limit int) SQLSafetyResult {
	return analyzeReadOnlySQL(sqlText, sqlAnalyzeOptions{
		appendLimit: supportsAutoAppendLimit(dbType),
		limit:       limit,
	})
}

func AnalyzeReadOnlySQLRaw(sqlText string) SQLSafetyResult {
	return analyzeReadOnlySQL(sqlText, sqlAnalyzeOptions{})
}

func AnalyzeExplainSQL(sqlText string) SQLSafetyResult {
	return AnalyzeExplainSQLByDB("", sqlText)
}

func AnalyzeExplainSQLByDB(dbType, sqlText string) SQLSafetyResult {
	if !supportsExplain(dbType) {
		if strings.TrimSpace(dbType) == "" {
			return denySQL("当前数据库类型暂不支持执行计划")
		}
		return denySQL(DBTypeText(dbType) + " 暂不支持执行计划")
	}

	base := AnalyzeReadOnlySQLRaw(sqlText)
	if !base.Allowed {
		return base
	}

	tokens := sqlKeywords(maskSQLLiteralsAndComments(base.SQLText))
	if len(tokens) == 0 {
		return denySQL("无法识别 SQL 类型")
	}

	first := tokens[0]
	if first == "explain" {
		if len(tokens) < 2 {
			return denySQL("EXPLAIN 后必须跟只读查询")
		}
		if len(tokens) > 1 && tokens[1] == "analyze" {
			return denySQL("执行计划仅支持 EXPLAIN，不允许 EXPLAIN ANALYZE")
		}
		if tokens[1] != "select" && tokens[1] != "with" {
			return denySQL("执行计划仅支持 SELECT / WITH 查询")
		}
		return SQLSafetyResult{
			Allowed: true,
			SQLText: base.SQLText,
			SQLType: "EXPLAIN",
			Message: "通过执行计划 SQL 校验",
		}
	}

	if first != "select" && first != "with" {
		return denySQL("执行计划仅支持 SELECT / WITH 查询")
	}
	return SQLSafetyResult{
		Allowed: true,
		SQLText: "EXPLAIN " + base.SQLText,
		SQLType: "EXPLAIN",
		Message: "通过执行计划 SQL 校验",
	}
}

func AnalyzeWriteSQLByDB(dbType, sqlText string, policy *DatabaseWritePolicy) SQLWriteSafetyResult {
	if policy == nil {
		policy = defaultDatabaseWritePolicy()
	}

	result := SQLWriteSafetyResult{
		Allowed:           false,
		SQLType:           "UNKNOWN",
		RiskLevel:         DatabaseQueryRiskLow,
		ReasonRequired:    policy.OperationReasonRequired,
		RowsAffectedLimit: normalizeMaxAffectedRows(policy.MaxAffectedRows),
	}

	if !supportsWriteValidation(dbType) {
		if strings.TrimSpace(dbType) == "" {
			return denyWriteSQL(result, "当前数据库类型暂不支持写操作预检查")
		}
		return denyWriteSQL(result, DBTypeText(dbType)+" 暂不支持写操作预检查")
	}

	trimmed := strings.TrimSpace(sqlText)
	if trimmed == "" {
		return denyWriteSQL(result, "SQL 不能为空")
	}
	if len([]rune(trimmed)) > 20000 {
		return denyWriteSQL(result, "SQL 长度不能超过 20000 个字符")
	}
	if hasUnsafeSemicolon(trimmed) {
		return denyWriteSQL(result, "仅允许执行单条 SQL，禁止多语句")
	}

	withoutTrailingSemicolon := trimTrailingStatementSemicolon(trimmed)
	result.SQLText = withoutTrailingSemicolon

	normalized := stripSQLComments(withoutTrailingSemicolon)
	first := firstSQLKeyword(normalized)
	if first == "" {
		return denyWriteSQL(result, "无法识别 SQL 类型")
	}
	if strings.EqualFold(first, "with") {
		result.SQLType = "WITH"
		result.RiskLevel = DatabaseQueryRiskHigh
		result.ConfirmRequired = policy.HighRiskRequiresConfirm
		return denyWriteSQL(result, "暂不支持 WITH 写操作预检查")
	}

	sqlType, riskLevel, ok := classifyWriteSQLKeyword(first)
	result.SQLType = sqlType
	result.RiskLevel = riskLevel
	result.ConfirmRequired = policy.HighRiskRequiresConfirm && requiresWriteConfirmation(riskLevel)
	if !ok {
		return denyWriteSQL(result, "当前接口仅支持写操作预检查")
	}

	tokenText := maskSQLLiteralsAndComments(withoutTrailingSemicolon)
	switch sqlType {
	case "DROP", "TRUNCATE":
		return denyWriteSQL(result, "默认禁止 DROP / TRUNCATE")
	case "UPDATE", "DELETE":
		if !containsSQLToken(tokenText, "where") {
			result.RiskLevel = DatabaseQueryRiskCritical
			result.ConfirmRequired = policy.HighRiskRequiresConfirm && requiresWriteConfirmation(result.RiskLevel)
			return denyWriteSQL(result, "默认禁止无 WHERE 的 UPDATE / DELETE")
		}
	}

	if !policy.WriteEnabled {
		return denyWriteSQL(result, "数据库写操作总开关未开启")
	}

	result.Allowed = true
	result.Message = "通过写操作预检查"
	if result.ConfirmRequired {
		result.Message = "通过写操作预检查，执行前需二次确认"
	}
	return result
}

func supportsAutoAppendLimit(dbType string) bool {
	switch normalizeDBType(dbType) {
	case "", DBTypeMySQL, DBTypeMariaDB, DBTypePostgreSQL, DBTypeClickHouse, DBTypeTiDB, DBTypeOceanBase, DBTypeOpenGauss, DBTypeKingbase:
		return true
	default:
		return false
	}
}

func supportsWriteValidation(dbType string) bool {
	switch normalizeDBType(dbType) {
	case "", DBTypeMySQL, DBTypeMariaDB, DBTypePostgreSQL, DBTypeSQLServer, DBTypeClickHouse, DBTypeOracle:
		return true
	default:
		return false
	}
}

func supportsExplain(dbType string) bool {
	switch normalizeDBType(dbType) {
	case "", DBTypeMySQL, DBTypeMariaDB, DBTypePostgreSQL, DBTypeClickHouse, DBTypeTiDB, DBTypeOceanBase, DBTypeOpenGauss, DBTypeKingbase:
		return true
	default:
		return false
	}
}

func analyzeReadOnlySQL(sqlText string, opts sqlAnalyzeOptions) SQLSafetyResult {
	trimmed := strings.TrimSpace(sqlText)
	if trimmed == "" {
		return denySQL("SQL 不能为空")
	}
	if len([]rune(trimmed)) > 20000 {
		return denySQL("SQL 长度不能超过 20000 个字符")
	}
	if hasUnsafeSemicolon(trimmed) {
		return denySQL("仅允许执行单条 SQL，禁止多语句")
	}

	withoutTrailingSemicolon := trimTrailingStatementSemicolon(trimmed)
	normalized := stripSQLComments(withoutTrailingSemicolon)
	first := firstSQLKeyword(normalized)
	if first == "" {
		return denySQL("无法识别 SQL 类型")
	}
	if !isAllowedReadOnlyKeyword(first) {
		return denySQL("仅允许执行只读查询")
	}

	tokenText := maskSQLLiteralsAndComments(withoutTrailingSemicolon)
	if keyword, ok := findForbiddenSQLKeyword(tokenText); ok {
		return denySQL("仅允许执行只读查询，禁止关键字: " + keyword)
	}

	finalSQL := withoutTrailingSemicolon
	if opts.appendLimit && shouldAppendLimit(first, tokenText) {
		finalSQL = fmt.Sprintf("%s LIMIT %d", finalSQL, opts.limit)
	}
	return SQLSafetyResult{
		Allowed: true,
		SQLText: finalSQL,
		SQLType: strings.ToUpper(first),
		Message: "通过只读 SQL 校验",
	}
}

func denySQL(message string) SQLSafetyResult {
	return SQLSafetyResult{
		Allowed: false,
		SQLType: "UNKNOWN",
		Message: message,
	}
}

func denyWriteSQL(result SQLWriteSafetyResult, message string) SQLWriteSafetyResult {
	result.Allowed = false
	result.Message = message
	return result
}

func isAllowedReadOnlyKeyword(keyword string) bool {
	switch strings.ToLower(keyword) {
	case "select", "show", "desc", "describe", "explain", "with":
		return true
	default:
		return false
	}
}

func classifyWriteSQLKeyword(keyword string) (string, string, bool) {
	switch strings.ToLower(strings.TrimSpace(keyword)) {
	case "insert":
		return "INSERT", DatabaseQueryRiskMedium, true
	case "update":
		return "UPDATE", DatabaseQueryRiskHigh, true
	case "delete":
		return "DELETE", DatabaseQueryRiskHigh, true
	case "replace":
		return "REPLACE", DatabaseQueryRiskHigh, true
	case "create":
		return "CREATE", DatabaseQueryRiskHigh, true
	case "alter":
		return "ALTER", DatabaseQueryRiskHigh, true
	case "drop":
		return "DROP", DatabaseQueryRiskCritical, true
	case "truncate":
		return "TRUNCATE", DatabaseQueryRiskCritical, true
	case "rename":
		return "RENAME", DatabaseQueryRiskHigh, true
	case "grant":
		return "GRANT", DatabaseQueryRiskHigh, true
	case "revoke":
		return "REVOKE", DatabaseQueryRiskHigh, true
	case "merge":
		return "MERGE", DatabaseQueryRiskHigh, true
	default:
		return strings.ToUpper(strings.TrimSpace(keyword)), DatabaseQueryRiskLow, false
	}
}

func requiresWriteConfirmation(riskLevel string) bool {
	switch strings.TrimSpace(riskLevel) {
	case DatabaseQueryRiskHigh, DatabaseQueryRiskCritical:
		return true
	default:
		return false
	}
}

func findForbiddenSQLKeyword(sqlText string) (string, bool) {
	forbidden := map[string]struct{}{
		"insert":   {},
		"update":   {},
		"delete":   {},
		"drop":     {},
		"truncate": {},
		"alter":    {},
		"create":   {},
		"grant":    {},
		"revoke":   {},
		"replace":  {},
		"call":     {},
		"exec":     {},
		"execute":  {},
		"merge":    {},
		"load":     {},
		"lock":     {},
		"unlock":   {},
		"set":      {},
		"use":      {},
	}
	for _, token := range sqlKeywords(sqlText) {
		if _, ok := forbidden[token]; ok {
			return strings.ToUpper(token), true
		}
	}
	return "", false
}

func shouldAppendLimit(firstKeyword, tokenText string) bool {
	switch strings.ToLower(firstKeyword) {
	case "select", "with":
		for _, token := range sqlKeywords(tokenText) {
			if token == "limit" {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func containsSQLToken(sqlText, token string) bool {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		return false
	}
	for _, item := range sqlKeywords(sqlText) {
		if item == token {
			return true
		}
	}
	return false
}

func firstSQLKeyword(sqlText string) string {
	for _, token := range sqlKeywords(sqlText) {
		return token
	}
	return ""
}

func sqlKeywords(sqlText string) []string {
	tokens := make([]string, 0)
	var current strings.Builder
	flush := func() {
		if current.Len() == 0 {
			return
		}
		tokens = append(tokens, strings.ToLower(current.String()))
		current.Reset()
	}
	for _, r := range sqlText {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			current.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return tokens
}

func hasUnsafeSemicolon(sqlText string) bool {
	inSingle := false
	inDouble := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false
	semicolon := -1
	runes := []rune(sqlText)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		if inLineComment {
			if r == '\n' || r == '\r' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if r == '*' && next == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if inSingle {
			if r == '\'' {
				if next == '\'' {
					i++
					continue
				}
				inSingle = false
			}
			continue
		}
		if inDouble {
			if r == '"' {
				inDouble = false
			}
			continue
		}
		if inBacktick {
			if r == '`' {
				inBacktick = false
			}
			continue
		}
		if r == '-' && next == '-' {
			inLineComment = true
			i++
			continue
		}
		if r == '#' {
			inLineComment = true
			continue
		}
		if r == '/' && next == '*' {
			inBlockComment = true
			i++
			continue
		}
		switch r {
		case '\'':
			inSingle = true
		case '"':
			inDouble = true
		case '`':
			inBacktick = true
		case ';':
			if semicolon >= 0 {
				return true
			}
			semicolon = i
		}
	}
	if semicolon < 0 {
		return false
	}
	return strings.TrimSpace(string(runes[semicolon+1:])) != ""
}

func trimTrailingStatementSemicolon(sqlText string) string {
	trimmed := strings.TrimSpace(sqlText)
	if strings.HasSuffix(trimmed, ";") && !hasUnsafeSemicolon(trimmed) {
		return strings.TrimSpace(trimmed[:len(trimmed)-1])
	}
	return trimmed
}

func stripSQLComments(sqlText string) string {
	runes := []rune(sqlText)
	var out strings.Builder
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		if r == '-' && next == '-' {
			i += 2
			for i < len(runes) && runes[i] != '\n' && runes[i] != '\r' {
				i++
			}
			out.WriteRune(' ')
			continue
		}
		if r == '#' {
			for i < len(runes) && runes[i] != '\n' && runes[i] != '\r' {
				i++
			}
			out.WriteRune(' ')
			continue
		}
		if r == '/' && next == '*' {
			i += 2
			for i < len(runes)-1 {
				if runes[i] == '*' && runes[i+1] == '/' {
					i++
					break
				}
				i++
			}
			out.WriteRune(' ')
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

func maskSQLLiteralsAndComments(sqlText string) string {
	runes := []rune(sqlText)
	var out strings.Builder
	inSingle := false
	inDouble := false
	inBacktick := false
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		if inSingle {
			if r == '\'' {
				if next == '\'' {
					i++
					continue
				}
				inSingle = false
			}
			out.WriteRune(' ')
			continue
		}
		if inDouble {
			if r == '"' {
				inDouble = false
			}
			out.WriteRune(' ')
			continue
		}
		if inBacktick {
			if r == '`' {
				inBacktick = false
			}
			out.WriteRune(' ')
			continue
		}
		if r == '-' && next == '-' {
			i += 2
			for i < len(runes) && runes[i] != '\n' && runes[i] != '\r' {
				i++
			}
			out.WriteRune(' ')
			continue
		}
		if r == '#' {
			for i < len(runes) && runes[i] != '\n' && runes[i] != '\r' {
				i++
			}
			out.WriteRune(' ')
			continue
		}
		if r == '/' && next == '*' {
			i += 2
			for i < len(runes)-1 {
				if runes[i] == '*' && runes[i+1] == '/' {
					i++
					break
				}
				i++
			}
			out.WriteRune(' ')
			continue
		}
		switch r {
		case '\'':
			inSingle = true
			out.WriteRune(' ')
		case '"':
			inDouble = true
			out.WriteRune(' ')
		case '`':
			inBacktick = true
			out.WriteRune(' ')
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}
