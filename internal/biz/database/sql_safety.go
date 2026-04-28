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

type SQLDDLSafetyResult struct {
	Allowed         bool
	SQLText         string
	SQLType         string
	RiskLevel       string
	ConfirmRequired bool
	ReasonRequired  bool
	BackupRequired  bool
	Message         string
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

func AnalyzeWriteExplainSQLByDB(dbType, sqlText string, policy *DatabaseWritePolicy) SQLSafetyResult {
	if policy == nil {
		policy = defaultDatabaseWritePolicy()
	}
	if !supportsWriteExplain(dbType) {
		if strings.TrimSpace(dbType) == "" {
			return denySQL("当前数据库类型暂不支持写 SQL 执行计划")
		}
		return denySQL(DBTypeText(dbType) + " 暂不支持写 SQL 执行计划")
	}

	trimmed := strings.TrimSpace(sqlText)
	if trimmed == "" {
		return denySQL("SQL 不能为空")
	}
	if len([]rune(trimmed)) > 20000 {
		return denySQL("SQL 长度不能超过 20000 个字符")
	}
	if hasExecutableSQLComment(trimmed) {
		return denySQL("禁止使用数据库可执行注释")
	}
	if hasUnsafeSemicolon(trimmed) {
		return denySQL("仅允许执行单条 SQL，禁止多语句")
	}

	withoutTrailingSemicolon := trimTrailingStatementSemicolon(trimmed)
	tokenText := maskSQLLiteralsAndComments(withoutTrailingSemicolon)
	if hasUnsafeNumericKeywordToken(tokenText) {
		return denySQL("SQL 存在数字和关键字粘连，无法安全识别")
	}
	tokens := sqlKeywords(tokenText)
	if len(tokens) == 0 {
		return denySQL("无法识别 SQL 类型")
	}

	statementKeyword := tokens[0]
	explicitExplain := statementKeyword == "explain"
	if explicitExplain {
		if len(tokens) < 2 {
			return denySQL("EXPLAIN 后必须跟写 SQL")
		}
		if tokens[1] == "analyze" {
			return denySQL("写 SQL 执行计划仅支持 EXPLAIN，不允许 EXPLAIN ANALYZE")
		}
		statementKeyword = tokens[1]
	}
	if statementKeyword == "with" {
		return denySQL("写 SQL 执行计划暂不支持 WITH 语句")
	}

	sqlType, _, knownWriteType := classifyWriteSQLKeyword(statementKeyword)
	if !knownWriteType {
		return denySQL("写 SQL 执行计划仅支持 INSERT / UPDATE / DELETE / REPLACE / MERGE")
	}
	if isDDLOrPrivilegeSQLType(sqlType) {
		return denySQL("DDL 不支持执行计划，请使用 DDL 检查")
	}
	if !isSupportedWriteExplainSQLType(dbType, sqlType) {
		return denySQL(DBTypeText(dbType) + " 暂不支持 " + sqlType + " 执行计划")
	}
	if keyword, ok := findForbiddenWriteExplainKeyword(tokenText); ok {
		return denySQL("写 SQL 执行计划禁止关键字: " + keyword)
	}
	if !policy.WriteExplainEnabled {
		return denySQL("数据库写 SQL 执行计划开关未开启")
	}

	explainSQL := withoutTrailingSemicolon
	if !explicitExplain {
		explainSQL = "EXPLAIN " + withoutTrailingSemicolon
	}
	return SQLSafetyResult{
		Allowed: true,
		SQLText: explainSQL,
		SQLType: "EXPLAIN",
		Message: "通过写 SQL 执行计划校验",
	}
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
	if hasExecutableSQLComment(trimmed) {
		return denyWriteSQL(result, "禁止使用数据库可执行注释")
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
	if !isSupportedWriteExecuteType(sqlType) {
		if isDDLOrPrivilegeSQLType(sqlType) {
			return denyWriteSQL(result, "当前受控写入仅支持 INSERT / UPDATE / DELETE，DDL 请使用 DDL 检查 / DDL 执行")
		}
		return denyWriteSQL(result, "当前仅支持 INSERT / UPDATE / DELETE 写操作执行")
	}
	if !supportsWriteExecution(dbType) {
		return denyWriteSQL(result, DBTypeText(dbType)+" 写操作执行将在后续批次接入")
	}

	tokenText := maskSQLLiteralsAndComments(withoutTrailingSemicolon)
	if hasUnsafeNumericKeywordToken(tokenText) {
		return denyWriteSQL(result, "SQL 存在数字和关键字粘连，无法安全识别")
	}
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

func AnalyzeDDLSQLByDB(dbType, sqlText string, policy *DatabaseWritePolicy) SQLDDLSafetyResult {
	if policy == nil {
		policy = defaultDatabaseWritePolicy()
	}
	result := SQLDDLSafetyResult{
		Allowed:         false,
		SQLType:         "UNKNOWN",
		RiskLevel:       DatabaseQueryRiskHigh,
		ReasonRequired:  policy.DDLReasonRequired,
		BackupRequired:  policy.DDLRequireBackupHint,
		ConfirmRequired: policy.DDLHighRiskConfirm,
	}

	if !supportsDDLValidation(dbType) {
		if strings.TrimSpace(dbType) == "" {
			return denyDDLSQL(result, "当前数据库类型暂不支持 DDL 结构变更检查")
		}
		return denyDDLSQL(result, DBTypeText(dbType)+" 暂不支持 DDL 结构变更检查")
	}

	trimmed := strings.TrimSpace(sqlText)
	if trimmed == "" {
		return denyDDLSQL(result, "SQL 不能为空")
	}
	if len([]rune(trimmed)) > 20000 {
		return denyDDLSQL(result, "SQL 长度不能超过 20000 个字符")
	}
	if hasExecutableSQLComment(trimmed) {
		return denyDDLSQL(result, "禁止使用数据库可执行注释")
	}
	if hasUnsafeSemicolon(trimmed) {
		return denyDDLSQL(result, "仅允许执行单条 SQL，禁止多语句")
	}

	withoutTrailingSemicolon := trimTrailingStatementSemicolon(trimmed)
	result.SQLText = withoutTrailingSemicolon
	tokenText := maskSQLLiteralsAndComments(withoutTrailingSemicolon)
	if hasUnsafeNumericKeywordToken(tokenText) {
		return denyDDLSQL(result, "SQL 存在数字和关键字粘连，无法安全识别")
	}
	tokens := sqlKeywords(tokenText)
	if len(tokens) == 0 {
		return denyDDLSQL(result, "无法识别 SQL 类型")
	}

	sqlType, riskLevel, knownWriteType := classifyWriteSQLKeyword(tokens[0])
	result.SQLType = sqlType
	result.RiskLevel = riskLevel
	result.ConfirmRequired = policy.DDLHighRiskConfirm && requiresWriteConfirmation(riskLevel)
	if !knownWriteType || (!isDDLSQLType(sqlType) && !isPrivilegeSQLType(sqlType)) {
		return denyDDLSQL(result, "当前接口仅支持 DDL 结构变更检查")
	}
	if isPrivilegeSQLType(sqlType) {
		return denyDDLSQL(result, "GRANT / REVOKE 属于账号权限变更，请走账号权限管理通道")
	}
	switch sqlType {
	case "DROP", "TRUNCATE":
		result.RiskLevel = DatabaseQueryRiskCritical
		result.ConfirmRequired = policy.DDLHighRiskConfirm && requiresWriteConfirmation(result.RiskLevel)
		return denyDDLSQL(result, "默认禁止 DROP / TRUNCATE")
	case "ALTER", "RENAME":
		return denyDDLSQL(result, "当前 DDL 执行仅支持 CREATE TABLE / CREATE INDEX")
	case "CREATE":
		if !isSupportedCreateDDL(tokens) {
			return denyDDLSQL(result, "当前 DDL 执行仅支持 CREATE TABLE / CREATE INDEX")
		}
	}
	if !policy.DDLEnabled {
		return denyDDLSQL(result, "数据库 DDL 结构变更开关未开启")
	}

	result.Allowed = true
	result.Message = "通过 DDL 结构变更检查"
	if result.ConfirmRequired {
		result.Message = "通过 DDL 结构变更检查，执行前需二次确认"
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

func supportsWriteExplain(dbType string) bool {
	switch normalizeDBType(dbType) {
	case "", DBTypeMySQL, DBTypeMariaDB, DBTypePostgreSQL, DBTypeTiDB, DBTypeOceanBase, DBTypeOpenGauss, DBTypeKingbase:
		return true
	default:
		return false
	}
}

func supportsDDLValidation(dbType string) bool {
	switch normalizeDBType(dbType) {
	case "", DBTypeMySQL, DBTypeMariaDB, DBTypePostgreSQL:
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
	if hasExecutableSQLComment(trimmed) {
		return denySQL("禁止使用数据库可执行注释")
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
	if hasUnsafeNumericKeywordToken(tokenText) {
		return denySQL("SQL 存在数字和关键字粘连，无法安全识别")
	}
	if keyword, ok := findForbiddenSQLKeyword(tokenText); ok {
		return denySQL("仅允许执行只读查询，禁止关键字: " + keyword)
	}

	finalSQL := withoutTrailingSemicolon
	if opts.appendLimit && opts.limit > 0 && shouldAppendLimit(first, tokenText) {
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

func denyDDLSQL(result SQLDDLSafetyResult, message string) SQLDDLSafetyResult {
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

func isDDLOrPrivilegeSQLType(sqlType string) bool {
	switch strings.ToUpper(strings.TrimSpace(sqlType)) {
	case "CREATE", "ALTER", "DROP", "TRUNCATE", "RENAME", "GRANT", "REVOKE":
		return true
	default:
		return false
	}
}

func isDDLSQLType(sqlType string) bool {
	switch strings.ToUpper(strings.TrimSpace(sqlType)) {
	case "CREATE", "ALTER", "DROP", "TRUNCATE", "RENAME":
		return true
	default:
		return false
	}
}

func isPrivilegeSQLType(sqlType string) bool {
	switch strings.ToUpper(strings.TrimSpace(sqlType)) {
	case "GRANT", "REVOKE":
		return true
	default:
		return false
	}
}

func isSupportedCreateDDL(tokens []string) bool {
	if len(tokens) < 2 || tokens[0] != "create" {
		return false
	}
	switch tokens[1] {
	case "table", "index":
		return true
	case "temporary":
		return len(tokens) > 2 && tokens[2] == "table"
	case "unique", "fulltext", "spatial":
		return len(tokens) > 2 && tokens[2] == "index"
	default:
		return false
	}
}

func isSupportedWriteExplainSQLType(dbType, sqlType string) bool {
	sqlType = strings.ToUpper(strings.TrimSpace(sqlType))
	switch normalizeDBType(dbType) {
	case "", DBTypeMySQL, DBTypeMariaDB, DBTypeTiDB, DBTypeOceanBase:
		switch sqlType {
		case "INSERT", "UPDATE", "DELETE", "REPLACE":
			return true
		default:
			return false
		}
	case DBTypePostgreSQL, DBTypeOpenGauss, DBTypeKingbase:
		switch sqlType {
		case "INSERT", "UPDATE", "DELETE", "MERGE":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func findForbiddenWriteExplainKeyword(sqlText string) (string, bool) {
	forbidden := map[string]struct{}{
		"call":      {},
		"exec":      {},
		"execute":   {},
		"load":      {},
		"lock":      {},
		"unlock":    {},
		"outfile":   {},
		"dumpfile":  {},
		"load_file": {},
		"sleep":     {},
		"benchmark": {},
	}
	for _, token := range sqlKeywords(sqlText) {
		if _, ok := forbidden[token]; ok {
			return strings.ToUpper(token), true
		}
	}
	return "", false
}

func findForbiddenSQLKeyword(sqlText string) (string, bool) {
	forbidden := map[string]struct{}{
		"insert":    {},
		"update":    {},
		"delete":    {},
		"drop":      {},
		"truncate":  {},
		"alter":     {},
		"create":    {},
		"grant":     {},
		"revoke":    {},
		"replace":   {},
		"call":      {},
		"exec":      {},
		"execute":   {},
		"merge":     {},
		"load":      {},
		"lock":      {},
		"unlock":    {},
		"set":       {},
		"use":       {},
		"into":      {},
		"outfile":   {},
		"dumpfile":  {},
		"load_file": {},
		"sleep":     {},
		"benchmark": {},
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

func hasUnsafeNumericKeywordToken(sqlText string) bool {
	for _, token := range sqlKeywords(sqlText) {
		if token == "" {
			continue
		}
		runes := []rune(token)
		if len(runes) == 0 || !unicode.IsDigit(runes[0]) {
			continue
		}
		for _, r := range runes[1:] {
			if unicode.IsLetter(r) || r == '_' {
				return true
			}
		}
	}
	return false
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

func hasExecutableSQLComment(sqlText string) bool {
	inSingle := false
	inDouble := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false
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
			if i+2 < len(runes) && runes[i+2] == '!' {
				return true
			}
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
		}
	}
	return false
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
