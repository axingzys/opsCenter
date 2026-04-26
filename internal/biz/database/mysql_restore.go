package database

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

type mysqlRestoreExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type mysqlRestoreTableColumn struct {
	Name      string
	Generated bool
}

type mysqlRestoreScriptState struct {
	tables map[string][]mysqlRestoreTableColumn
}

func runMySQLLogicalRestore(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, databaseName, inputPath, restoreStrategy string) error {
	restoreItem := *item
	restoreItem.DefaultDatabase = strings.TrimSpace(databaseName)
	db, err := openMySQLRestoreDB(&restoreItem, credential)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("连接恢复目标数据库失败: %w", err)
	}
	if normalizeRestoreStrategy(restoreStrategy) == DatabaseRestoreStrategyDatabaseClean {
		if err := cleanMySQLDatabase(ctx, db, databaseName); err != nil {
			return err
		}
	}

	input, closeInput, err := openRestoreSQLInput(inputPath)
	if err != nil {
		return err
	}
	defer closeInput()

	if err := execMySQLRestoreScript(ctx, db, input); err != nil {
		return err
	}
	return nil
}

func cleanMySQLDatabase(ctx context.Context, db *sql.DB, databaseName string) error {
	databaseName = strings.TrimSpace(databaseName)
	if databaseName == "" {
		return fmt.Errorf("MySQL / MariaDB 清空目标库需要明确数据库名")
	}
	if _, err := db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		return fmt.Errorf("关闭外键检查失败: %w", err)
	}
	defer db.ExecContext(context.Background(), "SET FOREIGN_KEY_CHECKS=1")

	views, err := listMySQLObjects(ctx, db, `
SELECT TABLE_NAME
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'VIEW'
ORDER BY TABLE_NAME`, databaseName)
	if err != nil {
		return err
	}
	for _, name := range views {
		if _, err := db.ExecContext(ctx, "DROP VIEW IF EXISTS "+quoteMySQLIdentifier(name)); err != nil {
			return fmt.Errorf("删除 MySQL / MariaDB 视图 %s 失败: %w", name, err)
		}
	}

	tables, err := listMySQLObjects(ctx, db, `
SELECT TABLE_NAME
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = ? AND TABLE_TYPE <> 'VIEW'
ORDER BY TABLE_NAME`, databaseName)
	if err != nil {
		return err
	}
	for _, name := range tables {
		if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS "+quoteMySQLIdentifier(name)); err != nil {
			return fmt.Errorf("删除 MySQL / MariaDB 表 %s 失败: %w", name, err)
		}
	}

	routines, err := listMySQLRoutines(ctx, db, databaseName)
	if err != nil {
		return err
	}
	for _, routine := range routines {
		if _, err := db.ExecContext(ctx, "DROP "+routine.kind+" IF EXISTS "+quoteMySQLIdentifier(routine.name)); err != nil {
			return fmt.Errorf("删除 MySQL / MariaDB %s %s 失败: %w", routine.kind, routine.name, err)
		}
	}

	events, err := listMySQLObjects(ctx, db, `
SELECT EVENT_NAME
FROM information_schema.EVENTS
WHERE EVENT_SCHEMA = ?
ORDER BY EVENT_NAME`, databaseName)
	if err != nil {
		return err
	}
	for _, name := range events {
		if _, err := db.ExecContext(ctx, "DROP EVENT IF EXISTS "+quoteMySQLIdentifier(name)); err != nil {
			return fmt.Errorf("删除 MySQL / MariaDB EVENT %s 失败: %w", name, err)
		}
	}
	return nil
}

type mysqlRoutine struct {
	name string
	kind string
}

func listMySQLObjects(ctx context.Context, db *sql.DB, query, databaseName string) ([]string, error) {
	rows, err := db.QueryContext(ctx, query, databaseName)
	if err != nil {
		return nil, fmt.Errorf("读取 MySQL / MariaDB 目标库对象失败: %w", err)
	}
	defer rows.Close()
	items := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("解析 MySQL / MariaDB 目标库对象失败: %w", err)
		}
		name = strings.TrimSpace(name)
		if name != "" {
			items = append(items, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 MySQL / MariaDB 目标库对象失败: %w", err)
	}
	return items, nil
}

func listMySQLRoutines(ctx context.Context, db *sql.DB, databaseName string) ([]mysqlRoutine, error) {
	rows, err := db.QueryContext(ctx, `
SELECT ROUTINE_NAME, ROUTINE_TYPE
FROM information_schema.ROUTINES
WHERE ROUTINE_SCHEMA = ?
ORDER BY ROUTINE_NAME`, databaseName)
	if err != nil {
		return nil, fmt.Errorf("读取 MySQL / MariaDB 目标库存储程序失败: %w", err)
	}
	defer rows.Close()
	items := make([]mysqlRoutine, 0)
	for rows.Next() {
		var item mysqlRoutine
		if err := rows.Scan(&item.name, &item.kind); err != nil {
			return nil, fmt.Errorf("解析 MySQL / MariaDB 目标库存储程序失败: %w", err)
		}
		item.name = strings.TrimSpace(item.name)
		item.kind = strings.ToUpper(strings.TrimSpace(item.kind))
		if item.name == "" || (item.kind != "PROCEDURE" && item.kind != "FUNCTION") {
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 MySQL / MariaDB 目标库存储程序失败: %w", err)
	}
	return items, nil
}

func quoteMySQLIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func openMySQLRestoreDB(item *DatabaseInstance, credential *ConnectionCredential) (*sql.DB, error) {
	if credential == nil || strings.TrimSpace(credential.Username) == "" {
		return nil, fmt.Errorf("目标实例凭据用户名不能为空")
	}
	if credential.Password == "" {
		return nil, fmt.Errorf("目标实例凭据密码不能为空")
	}

	cfg := mysqlDriver.NewConfig()
	cfg.User = strings.TrimSpace(credential.Username)
	cfg.Passwd = credential.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(strings.TrimSpace(item.Host), fmt.Sprintf("%d", item.Port))
	cfg.DBName = strings.TrimSpace(item.DefaultDatabase)
	cfg.ParseTime = true
	cfg.Loc = time.Local
	cfg.Timeout = 10 * time.Second
	cfg.ReadTimeout = restoreCommandTimeout
	cfg.WriteTimeout = restoreCommandTimeout
	cfg.MaxAllowedPacket = 0
	cfg.Params = map[string]string{"charset": "utf8mb4"}
	if item.TLSEnabled {
		cfg.TLSConfig = "true"
	}

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("创建恢复目标数据库连接失败: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(restoreCommandTimeout)
	return db, nil
}

func openRestoreSQLInput(inputPath string) (io.Reader, func(), error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, func() {}, fmt.Errorf("打开备份文件失败: %w", err)
	}

	if !strings.HasSuffix(strings.ToLower(strings.TrimSpace(inputPath)), ".gz") {
		return file, func() { _ = file.Close() }, nil
	}

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		_ = file.Close()
		return nil, func() {}, fmt.Errorf("读取备份压缩文件失败: %w", err)
	}
	return gzipReader, func() {
		_ = gzipReader.Close()
		_ = file.Close()
	}, nil
}

func execMySQLRestoreScript(ctx context.Context, db mysqlRestoreExecer, input io.Reader) error {
	reader := bufio.NewReaderSize(input, 1024*1024)
	state := &mysqlRestoreScriptState{tables: make(map[string][]mysqlRestoreTableColumn)}
	var stmt bytes.Buffer
	var (
		inSingleQuote  bool
		inDoubleQuote  bool
		inBacktick     bool
		inLineComment  bool
		inBlockComment bool
		escaped        bool
		blockStar      bool
		statementNo    int
	)

	for {
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("读取备份文件失败: %w", err)
		}
		stmt.WriteByte(b)

		if inLineComment {
			if b == '\n' || b == '\r' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if blockStar && b == '/' {
				inBlockComment = false
				blockStar = false
				continue
			}
			blockStar = b == '*'
			continue
		}

		if inSingleQuote || inDoubleQuote || inBacktick {
			if escaped {
				escaped = false
				continue
			}
			if b == '\\' && !inBacktick {
				escaped = true
				continue
			}
			if b == '\'' && inSingleQuote {
				inSingleQuote = false
			} else if b == '"' && inDoubleQuote {
				inDoubleQuote = false
			} else if b == '`' && inBacktick {
				inBacktick = false
			}
			continue
		}

		switch b {
		case '\'':
			inSingleQuote = true
			continue
		case '"':
			inDoubleQuote = true
			continue
		case '`':
			inBacktick = true
			continue
		case '#':
			inLineComment = true
			continue
		case '-':
			next, err := reader.Peek(1)
			if err == nil && len(next) == 1 && next[0] == '-' {
				nextByte, _ := reader.ReadByte()
				stmt.WriteByte(nextByte)
				inLineComment = true
			}
			continue
		case '/':
			next, err := reader.Peek(1)
			if err == nil && len(next) == 1 && next[0] == '*' {
				nextByte, _ := reader.ReadByte()
				stmt.WriteByte(nextByte)
				inBlockComment = true
				blockStar = false
			}
			continue
		case ';':
			statementNo++
			if err := execMySQLRestoreStatement(ctx, db, state, stmt.String(), statementNo); err != nil {
				return err
			}
			stmt.Reset()
		}
	}

	if strings.TrimSpace(stmt.String()) != "" {
		statementNo++
		if err := execMySQLRestoreStatement(ctx, db, state, stmt.String(), statementNo); err != nil {
			return err
		}
	}
	return nil
}

func execMySQLRestoreStatement(ctx context.Context, db mysqlRestoreExecer, state *mysqlRestoreScriptState, raw string, statementNo int) error {
	statement := strings.TrimSpace(raw)
	statement = strings.TrimSuffix(statement, ";")
	statement = strings.TrimSpace(statement)
	if statement == "" {
		return nil
	}
	if state != nil {
		state.captureCreateTable(statement)
		rewritten, ok, err := state.rewriteGeneratedColumnInsert(statement)
		if err != nil {
			return fmt.Errorf("重写 MySQL 恢复语句 #%d 失败: %w", statementNo, err)
		}
		if ok {
			statement = rewritten
		}
	}
	if _, err := db.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("执行 MySQL 恢复语句 #%d 失败: %w", statementNo, err)
	}
	return nil
}

func (s *mysqlRestoreScriptState) captureCreateTable(statement string) {
	if s == nil {
		return
	}
	upper := strings.ToUpper(statement)
	idx := strings.Index(upper, "CREATE TABLE `")
	if idx < 0 {
		return
	}
	tableStart := idx + len("CREATE TABLE `")
	tableEnd := strings.Index(statement[tableStart:], "`")
	if tableEnd < 0 {
		return
	}
	tableName := statement[tableStart : tableStart+tableEnd]
	open := strings.Index(statement[tableStart+tableEnd:], "(")
	if open < 0 {
		return
	}
	open += tableStart + tableEnd
	close := findMatchingParen(statement, open)
	if close <= open {
		return
	}

	parts := splitTopLevelComma(statement[open+1 : close])
	columns := make([]mysqlRestoreTableColumn, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "`") {
			continue
		}
		nameEnd := strings.Index(part[1:], "`")
		if nameEnd < 0 {
			continue
		}
		columnName := part[1 : 1+nameEnd]
		partUpper := strings.ToUpper(part)
		columns = append(columns, mysqlRestoreTableColumn{
			Name:      columnName,
			Generated: strings.Contains(partUpper, " GENERATED ") || strings.Contains(partUpper, " GENERATED ALWAYS "),
		})
	}
	if len(columns) > 0 {
		s.tables[tableName] = columns
	}
}

func (s *mysqlRestoreScriptState) rewriteGeneratedColumnInsert(statement string) (string, bool, error) {
	if s == nil || len(s.tables) == 0 {
		return statement, false, nil
	}
	trimmed := strings.TrimSpace(statement)
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "INSERT INTO `") {
		return statement, false, nil
	}
	tableStart := len("INSERT INTO `")
	tableEnd := strings.Index(trimmed[tableStart:], "`")
	if tableEnd < 0 {
		return statement, false, nil
	}
	tableName := trimmed[tableStart : tableStart+tableEnd]
	columns := s.tables[tableName]
	if !hasGeneratedColumns(columns) {
		return statement, false, nil
	}
	rest := strings.TrimSpace(trimmed[tableStart+tableEnd+1:])
	if !strings.HasPrefix(strings.ToUpper(rest), "VALUES") {
		return statement, false, nil
	}
	values := strings.TrimSpace(rest[len("VALUES"):])
	rewrittenValues, err := removeGeneratedColumnValues(values, columns)
	if err != nil {
		return statement, false, err
	}
	columnNames := make([]string, 0, len(columns))
	for _, column := range columns {
		if !column.Generated {
			columnNames = append(columnNames, "`"+column.Name+"`")
		}
	}
	return "INSERT INTO `" + tableName + "` (" + strings.Join(columnNames, ",") + ") VALUES\n" + rewrittenValues, true, nil
}

func hasGeneratedColumns(columns []mysqlRestoreTableColumn) bool {
	for _, column := range columns {
		if column.Generated {
			return true
		}
	}
	return false
}

func removeGeneratedColumnValues(values string, columns []mysqlRestoreTableColumn) (string, error) {
	var output strings.Builder
	i := 0
	for i < len(values) {
		ch := values[i]
		if ch != '(' {
			output.WriteByte(ch)
			i++
			continue
		}
		end := findMatchingParen(values, i)
		if end < 0 {
			return "", fmt.Errorf("INSERT VALUES 元组括号不完整")
		}
		items := splitTopLevelComma(values[i+1 : end])
		if len(items) != len(columns) {
			return "", fmt.Errorf("INSERT VALUES 字段数量 %d 与表字段数量 %d 不一致", len(items), len(columns))
		}
		output.WriteByte('(')
		written := 0
		for idx, item := range items {
			if columns[idx].Generated {
				continue
			}
			if written > 0 {
				output.WriteByte(',')
			}
			output.WriteString(strings.TrimSpace(item))
			written++
		}
		output.WriteByte(')')
		i = end + 1
	}
	return output.String(), nil
}

func splitTopLevelComma(input string) []string {
	parts := make([]string, 0)
	start := 0
	depth := 0
	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false
	escaped := false
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if inSingleQuote || inDoubleQuote || inBacktick {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' && !inBacktick {
				escaped = true
				continue
			}
			if ch == '\'' && inSingleQuote {
				inSingleQuote = false
			} else if ch == '"' && inDoubleQuote {
				inDoubleQuote = false
			} else if ch == '`' && inBacktick {
				inBacktick = false
			}
			continue
		}
		switch ch {
		case '\'':
			inSingleQuote = true
		case '"':
			inDoubleQuote = true
		case '`':
			inBacktick = true
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, input[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, input[start:])
	return parts
}

func findMatchingParen(input string, open int) int {
	depth := 0
	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false
	escaped := false
	for i := open; i < len(input); i++ {
		ch := input[i]
		if inSingleQuote || inDoubleQuote || inBacktick {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' && !inBacktick {
				escaped = true
				continue
			}
			if ch == '\'' && inSingleQuote {
				inSingleQuote = false
			} else if ch == '"' && inDoubleQuote {
				inDoubleQuote = false
			} else if ch == '`' && inBacktick {
				inBacktick = false
			}
			continue
		}
		switch ch {
		case '\'':
			inSingleQuote = true
		case '"':
			inDoubleQuote = true
		case '`':
			inBacktick = true
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
