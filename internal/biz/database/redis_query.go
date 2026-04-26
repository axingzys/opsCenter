package database

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisReadCommand struct {
	Tokens    []string
	Name      string
	SubName   string
	Command   string
	SQLType   string
	Args      []string
	Formatted string
}

func analyzeReadOnlyRedisCommand(commandText string, limit int) SQLSafetyResult {
	command, err := parseRedisReadCommand(commandText, limit)
	if err != nil {
		return denySQL(err.Error())
	}
	return SQLSafetyResult{
		Allowed: true,
		SQLText: command.Formatted,
		SQLType: command.SQLType,
		Message: "通过 Redis 只读命令校验",
	}
}

func formatRedisCommand(commandText string, limit int) (*DatabaseQueryFormatVO, error) {
	command, err := parseRedisReadCommand(commandText, limit)
	if err != nil {
		return nil, err
	}
	return &DatabaseQueryFormatVO{
		SQLText: command.Formatted,
		SQLType: command.SQLType,
	}, nil
}

func parseRedisReadCommand(commandText string, limit int) (*redisReadCommand, error) {
	trimmed := strings.TrimSpace(commandText)
	if trimmed == "" {
		return nil, fmt.Errorf("Redis 命令不能为空")
	}
	if len([]rune(trimmed)) > 20000 {
		return nil, fmt.Errorf("Redis 命令长度不能超过 20000 个字符")
	}
	if strings.Contains(trimmed, ";") {
		return nil, fmt.Errorf("Redis 命令仅允许单条执行")
	}
	tokens, err := tokenizeRedisCommand(trimmed)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("无法识别 Redis 命令")
	}

	name := strings.ToUpper(tokens[0])
	subName := ""
	if len(tokens) > 1 {
		subName = strings.ToUpper(tokens[1])
	}
	command := &redisReadCommand{
		Tokens:  tokens,
		Name:    name,
		SubName: subName,
		Args:    tokens[1:],
		SQLType: name,
	}
	if subName != "" && (name == "MEMORY" || name == "CLUSTER") {
		command.Command = name + " " + subName
		command.SQLType = command.Command
	} else {
		command.Command = name
	}

	switch name {
	case "PING", "DBSIZE", "INFO":
	case "TYPE", "TTL", "PTTL", "GET", "STRLEN":
		if len(command.Args) != 1 {
			return nil, fmt.Errorf("%s 需要 1 个参数", name)
		}
	case "EXISTS", "MGET":
		if len(command.Args) == 0 {
			return nil, fmt.Errorf("%s 至少需要 1 个参数", name)
		}
	case "HLEN", "LLEN", "SCARD", "ZCARD", "XLEN":
		if len(command.Args) != 1 {
			return nil, fmt.Errorf("%s 需要 1 个参数", name)
		}
	case "HGET":
		if len(command.Args) != 2 {
			return nil, fmt.Errorf("HGET 需要 2 个参数")
		}
	case "HGETALL", "SMEMBERS":
		if len(command.Args) != 1 {
			return nil, fmt.Errorf("%s 需要 1 个参数", name)
		}
	case "LRANGE", "ZRANGE":
		if len(command.Args) < 3 {
			return nil, fmt.Errorf("%s 至少需要 3 个参数", name)
		}
	case "XRANGE":
		if len(command.Args) < 3 {
			return nil, fmt.Errorf("XRANGE 至少需要 3 个参数")
		}
	case "SCAN":
		command.Args = normalizeRedisScanArgs(command.Args, limit)
		command.Tokens = append([]string{name}, command.Args...)
	case "MEMORY":
		if subName != "USAGE" || len(command.Args) < 2 || len(command.Args) > 3 {
			return nil, fmt.Errorf("仅支持 MEMORY USAGE key")
		}
	case "CLUSTER":
		switch subName {
		case "INFO", "NODES", "SLOTS":
		default:
			return nil, fmt.Errorf("仅支持 CLUSTER INFO / NODES / SLOTS")
		}
	default:
		return nil, fmt.Errorf("Redis 查询控制台仅支持白名单只读命令")
	}

	command.Formatted = formatRedisCommandTokens(command.Tokens)
	return command, nil
}

func tokenizeRedisCommand(commandText string) ([]string, error) {
	tokens := make([]string, 0)
	var current strings.Builder
	var quote rune
	escaped := false
	for _, ch := range commandText {
		switch {
		case escaped:
			current.WriteRune(ch)
			escaped = false
		case ch == '\\':
			escaped = true
		case quote != 0:
			if ch == quote {
				quote = 0
			} else {
				current.WriteRune(ch)
			}
		case ch == '\'' || ch == '"':
			quote = ch
		case ch == '\n' || ch == '\r' || ch == '\t' || ch == ' ':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}
	if escaped || quote != 0 {
		return nil, fmt.Errorf("Redis 命令引号或转义未闭合")
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens, nil
}

func normalizeRedisScanArgs(args []string, limit int) []string {
	normalized := make([]string, 0, len(args)+2)
	if len(args) == 0 {
		normalized = append(normalized, "0")
	} else {
		normalized = append(normalized, args...)
	}
	foundCount := false
	for index := 1; index < len(normalized); index++ {
		if strings.EqualFold(normalized[index], "COUNT") && index+1 < len(normalized) {
			normalized[index+1] = strconv.Itoa(limit)
			foundCount = true
			break
		}
	}
	if !foundCount {
		normalized = append(normalized, "COUNT", strconv.Itoa(limit))
	}
	return normalized
}

func formatRedisCommandTokens(tokens []string) string {
	formatted := make([]string, 0, len(tokens))
	for index, token := range tokens {
		upperToken := strings.ToUpper(token)
		if index == 0 || (index == 1 && strings.EqualFold(tokens[0], "CLUSTER")) || (index == 1 && strings.EqualFold(tokens[0], "MEMORY")) {
			formatted = append(formatted, strings.ToUpper(token))
			continue
		}
		if upperToken == "MATCH" || upperToken == "COUNT" || upperToken == "WITHSCORES" {
			formatted = append(formatted, upperToken)
			continue
		}
		if strings.ContainsAny(token, " \t\r\n") {
			formatted = append(formatted, strconv.Quote(token))
			continue
		}
		formatted = append(formatted, token)
	}
	return strings.Join(formatted, " ")
}

func executeRedisCommand(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, schemaName, sqlText string, limit, timeoutSeconds int) (*DatabaseQueryResultVO, error) {
	access, err := openRedisAccess(ctx, item, credential)
	if err != nil {
		return nil, err
	}
	defer access.Close()

	command, err := parseRedisReadCommand(sqlText, limit)
	if err != nil {
		return nil, err
	}
	dbIndex, err := resolveRedisDBIndex(item, schemaName, access.clusterEnabled)
	if err != nil {
		return nil, err
	}
	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	if access.clusterEnabled {
		return executeRedisClusterCommand(queryCtx, access, command, limit)
	}
	client, err := openRedisClientWithDB(item, credential, dbIndex)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	return executeRedisStandaloneCommand(queryCtx, client, command, limit)
}

func executeRedisStandaloneCommand(ctx context.Context, client redis.Cmdable, command *redisReadCommand, limit int) (*DatabaseQueryResultVO, error) {
	switch command.Name {
	case "PING":
		value, err := client.Ping(ctx).Result()
		return buildRedisScalarResult(command, "value", value, err, limit)
	case "INFO":
		section := ""
		if len(command.Args) > 0 {
			section = command.Args[0]
		}
		value, err := client.Info(ctx, section).Result()
		if err != nil {
			return nil, fmt.Errorf("执行 Redis 命令失败: %w", err)
		}
		return buildRedisMapResult(command, parseRedisInfo(value), limit, "key", "value")
	case "DBSIZE":
		value, err := client.DBSize(ctx).Result()
		return buildRedisScalarResult(command, "value", value, err, limit)
	case "TYPE":
		value, err := client.Type(ctx, command.Args[0]).Result()
		return buildRedisScalarResult(command, "value", value, err, limit)
	case "TTL":
		value, err := client.TTL(ctx, command.Args[0]).Result()
		if err != nil {
			return nil, fmt.Errorf("执行 Redis 命令失败: %w", err)
		}
		return buildRedisScalarResult(command, "value", formatRedisTTL(int64(value/time.Millisecond)), nil, limit)
	case "PTTL":
		value, err := client.PTTL(ctx, command.Args[0]).Result()
		if err != nil {
			return nil, fmt.Errorf("执行 Redis 命令失败: %w", err)
		}
		return buildRedisScalarResult(command, "value", int64(value/time.Millisecond), nil, limit)
	case "EXISTS":
		value, err := client.Exists(ctx, command.Args...).Result()
		return buildRedisScalarResult(command, "value", value, err, limit)
	case "GET":
		value, err := client.Get(ctx, command.Args[0]).Result()
		if err == redis.Nil {
			value = ""
			err = nil
		}
		return buildRedisScalarResult(command, "value", sanitizeRedisPreview(value), err, limit)
	case "MGET":
		values, err := client.MGet(ctx, command.Args...).Result()
		if err != nil {
			return nil, fmt.Errorf("执行 Redis 命令失败: %w", err)
		}
		rows := make([]map[string]any, 0, minInt(len(values), limit))
		for index, value := range values {
			if len(rows) >= limit {
				break
			}
			rows = append(rows, map[string]any{
				"key":   command.Args[index],
				"value": normalizeRedisValue(value),
			})
		}
		return newRedisQueryResult(command.SQLType, command.Formatted, []string{"key", "value"}, rows, len(values) > limit, limit), nil
	case "STRLEN":
		value, err := client.StrLen(ctx, command.Args[0]).Result()
		return buildRedisScalarResult(command, "value", value, err, limit)
	case "MEMORY":
		value, err := client.MemoryUsage(ctx, command.Args[1]).Result()
		return buildRedisScalarResult(command, "value", value, err, limit)
	case "HGET":
		value, err := client.HGet(ctx, command.Args[0], command.Args[1]).Result()
		if err == redis.Nil {
			value = ""
			err = nil
		}
		rows := []map[string]any{{"field": command.Args[1], "value": sanitizeRedisPreview(value)}}
		return newRedisQueryResult(command.SQLType, command.Formatted, []string{"field", "value"}, rows, false, limit), nil
	case "HGETALL":
		value, err := client.HGetAll(ctx, command.Args[0]).Result()
		if err != nil {
			return nil, fmt.Errorf("执行 Redis 命令失败: %w", err)
		}
		return buildRedisMapResult(command, value, limit, "field", "value")
	case "HLEN":
		value, err := client.HLen(ctx, command.Args[0]).Result()
		return buildRedisScalarResult(command, "value", value, err, limit)
	case "LRANGE":
		start, _ := strconv.ParseInt(command.Args[1], 10, 64)
		stop, _ := strconv.ParseInt(command.Args[2], 10, 64)
		if stop < start || stop-start+1 > int64(limit) || stop == -1 {
			stop = start + int64(limit) - 1
		}
		values, err := client.LRange(ctx, command.Args[0], start, stop).Result()
		if err != nil {
			return nil, fmt.Errorf("执行 Redis 命令失败: %w", err)
		}
		rows := make([]map[string]any, 0, len(values))
		for index, value := range values {
			rows = append(rows, map[string]any{
				"index": start + int64(index),
				"value": sanitizeRedisPreview(value),
			})
		}
		return newRedisQueryResult(command.SQLType, command.Formatted, []string{"index", "value"}, rows, false, limit), nil
	case "LLEN":
		value, err := client.LLen(ctx, command.Args[0]).Result()
		return buildRedisScalarResult(command, "value", value, err, limit)
	case "SMEMBERS":
		values, err := client.SMembers(ctx, command.Args[0]).Result()
		if err != nil {
			return nil, fmt.Errorf("执行 Redis 命令失败: %w", err)
		}
		rows := make([]map[string]any, 0, minInt(len(values), limit))
		for _, value := range values {
			if len(rows) >= limit {
				break
			}
			rows = append(rows, map[string]any{"value": sanitizeRedisPreview(value)})
		}
		return newRedisQueryResult(command.SQLType, command.Formatted, []string{"value"}, rows, len(values) > limit, limit), nil
	case "SCARD":
		value, err := client.SCard(ctx, command.Args[0]).Result()
		return buildRedisScalarResult(command, "value", value, err, limit)
	case "ZRANGE":
		withScores := len(command.Args) > 3 && strings.EqualFold(command.Args[len(command.Args)-1], "WITHSCORES")
		start, _ := strconv.ParseInt(command.Args[1], 10, 64)
		stop, _ := strconv.ParseInt(command.Args[2], 10, 64)
		if stop < start || stop-start+1 > int64(limit) || stop == -1 {
			stop = start + int64(limit) - 1
		}
		if withScores {
			values, err := client.ZRangeWithScores(ctx, command.Args[0], start, stop).Result()
			if err != nil {
				return nil, fmt.Errorf("执行 Redis 命令失败: %w", err)
			}
			rows := make([]map[string]any, 0, len(values))
			for _, value := range values {
				rows = append(rows, map[string]any{
					"member": sanitizeRedisPreview(fmt.Sprint(value.Member)),
					"score":  value.Score,
				})
			}
			return newRedisQueryResult(command.SQLType, command.Formatted, []string{"member", "score"}, rows, false, limit), nil
		}
		values, err := client.ZRange(ctx, command.Args[0], start, stop).Result()
		if err != nil {
			return nil, fmt.Errorf("执行 Redis 命令失败: %w", err)
		}
		rows := make([]map[string]any, 0, len(values))
		for _, value := range values {
			rows = append(rows, map[string]any{"value": sanitizeRedisPreview(value)})
		}
		return newRedisQueryResult(command.SQLType, command.Formatted, []string{"value"}, rows, false, limit), nil
	case "ZCARD":
		value, err := client.ZCard(ctx, command.Args[0]).Result()
		return buildRedisScalarResult(command, "value", value, err, limit)
	case "XRANGE":
		values, err := client.XRangeN(ctx, command.Args[0], command.Args[1], command.Args[2], int64(limit)).Result()
		if err != nil {
			return nil, fmt.Errorf("执行 Redis 命令失败: %w", err)
		}
		rows := make([]map[string]any, 0, len(values))
		for _, value := range values {
			rows = append(rows, map[string]any{
				"id":     value.ID,
				"values": value.Values,
			})
		}
		return newRedisQueryResult(command.SQLType, command.Formatted, []string{"id", "values"}, rows, false, limit), nil
	case "XLEN":
		value, err := client.XLen(ctx, command.Args[0]).Result()
		return buildRedisScalarResult(command, "value", value, err, limit)
	case "SCAN":
		rows, nextCursor, err := executeRedisScan(ctx, client, command, limit)
		if err != nil {
			return nil, err
		}
		return newRedisQueryResult(command.SQLType, command.Formatted+" -- next_cursor="+strconv.FormatUint(nextCursor, 10), []string{"key"}, rows, false, limit), nil
	case "CLUSTER":
		return executeRedisClusterIntrospection(ctx, client, command, limit)
	default:
		return nil, fmt.Errorf("Redis 查询控制台仅支持白名单只读命令")
	}
}

func executeRedisClusterCommand(ctx context.Context, access *redisAccess, command *redisReadCommand, limit int) (*DatabaseQueryResultVO, error) {
	if access == nil || access.cluster == nil {
		return nil, fmt.Errorf("Redis Cluster 连接未初始化")
	}
	switch command.Command {
	case "DBSIZE":
		var total int64
		var mu sync.Mutex
		if err := access.cluster.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
			value, err := client.DBSize(ctx).Result()
			if err != nil {
				return err
			}
			mu.Lock()
			total += value
			mu.Unlock()
			return nil
		}); err != nil {
			return nil, fmt.Errorf("执行 Redis 命令失败: %w", err)
		}
		return buildRedisScalarResult(command, "value", total, nil, limit)
	case "SCAN":
		rows := make([]map[string]any, 0, limit)
		var mu sync.Mutex
		if err := access.cluster.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
			localRows, _, err := executeRedisScan(ctx, client, command, limit)
			if err != nil {
				return err
			}
			mu.Lock()
			for _, row := range localRows {
				if len(rows) >= limit {
					break
				}
				row["node"] = client.Options().Addr
				rows = append(rows, row)
			}
			mu.Unlock()
			return nil
		}); err != nil {
			return nil, fmt.Errorf("执行 Redis 命令失败: %w", err)
		}
		return newRedisQueryResult(command.SQLType, command.Formatted, []string{"node", "key"}, rows, false, limit), nil
	default:
		return executeRedisStandaloneCommand(ctx, access.cluster, command, limit)
	}
}

func executeRedisScan(ctx context.Context, client redis.Cmdable, command *redisReadCommand, limit int) ([]map[string]any, uint64, error) {
	args := command.Args
	cursor, _ := strconv.ParseUint(args[0], 10, 64)
	match := ""
	count := int64(limit)
	for index := 1; index < len(args); index++ {
		switch strings.ToUpper(args[index]) {
		case "MATCH":
			if index+1 < len(args) {
				match = args[index+1]
				index++
			}
		case "COUNT":
			if index+1 < len(args) {
				count, _ = strconv.ParseInt(args[index+1], 10, 64)
				index++
			}
		}
	}
	if count <= 0 || count > int64(limit) {
		count = int64(limit)
	}
	values, nextCursor, err := client.Scan(ctx, cursor, match, count).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("执行 Redis 命令失败: %w", err)
	}
	rows := make([]map[string]any, 0, len(values))
	for _, value := range values {
		rows = append(rows, map[string]any{"key": value})
	}
	return rows, nextCursor, nil
}

func executeRedisClusterIntrospection(ctx context.Context, client redis.Cmdable, command *redisReadCommand, limit int) (*DatabaseQueryResultVO, error) {
	switch command.SubName {
	case "INFO":
		value, err := client.ClusterInfo(ctx).Result()
		if err != nil {
			return nil, fmt.Errorf("执行 Redis 命令失败: %w", err)
		}
		return buildRedisMapResult(command, parseRedisInfo(value), limit, "key", "value")
	case "NODES":
		value, err := client.ClusterNodes(ctx).Result()
		if err != nil {
			return nil, fmt.Errorf("执行 Redis 命令失败: %w", err)
		}
		rows := make([]map[string]any, 0)
		for _, line := range strings.Split(strings.TrimSpace(value), "\n") {
			fields := strings.Fields(strings.TrimSpace(line))
			if len(fields) < 8 {
				continue
			}
			row := map[string]any{
				"nodeId":    fields[0],
				"address":   normalizeRedisClusterAddress(fields[1]),
				"flags":     fields[2],
				"masterId":  fields[3],
				"linkState": fields[7],
				"slots":     strings.Join(fields[8:], " "),
			}
			rows = append(rows, row)
			if len(rows) >= limit {
				break
			}
		}
		return newRedisQueryResult(command.SQLType, command.Formatted, []string{"nodeId", "address", "flags", "masterId", "linkState", "slots"}, rows, false, limit), nil
	case "SLOTS":
		values, err := client.ClusterSlots(ctx).Result()
		if err != nil {
			return nil, fmt.Errorf("执行 Redis 命令失败: %w", err)
		}
		rows := make([]map[string]any, 0, minInt(len(values), limit))
		for _, value := range values {
			replicas := make([]string, 0)
			for index, node := range value.Nodes {
				if index == 0 {
					continue
				}
				replicas = append(replicas, node.Addr)
			}
			master := ""
			if len(value.Nodes) > 0 {
				master = value.Nodes[0].Addr
			}
			rows = append(rows, map[string]any{
				"start":    value.Start,
				"end":      value.End,
				"master":   master,
				"replicas": strings.Join(replicas, ", "),
			})
			if len(rows) >= limit {
				break
			}
		}
		return newRedisQueryResult(command.SQLType, command.Formatted, []string{"start", "end", "master", "replicas"}, rows, false, limit), nil
	default:
		return nil, fmt.Errorf("仅支持 CLUSTER INFO / NODES / SLOTS")
	}
}

func buildRedisScalarResult(command *redisReadCommand, column string, value any, err error, limit int) (*DatabaseQueryResultVO, error) {
	if err != nil {
		return nil, fmt.Errorf("执行 Redis 命令失败: %w", err)
	}
	return newRedisQueryResult(command.SQLType, command.Formatted, []string{column}, []map[string]any{{column: normalizeRedisValue(value)}}, false, limit), nil
}

func buildRedisMapResult(command *redisReadCommand, values map[string]string, limit int, keyLabel, valueLabel string) (*DatabaseQueryResultVO, error) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]map[string]any, 0, minInt(len(keys), limit))
	for _, key := range keys {
		if len(rows) >= limit {
			break
		}
		rows = append(rows, map[string]any{
			keyLabel:   key,
			valueLabel: normalizeRedisValue(values[key]),
		})
	}
	return newRedisQueryResult(command.SQLType, command.Formatted, []string{keyLabel, valueLabel}, rows, len(keys) > limit, limit), nil
}

func newRedisQueryResult(sqlType, executedSQL string, columns []string, rows []map[string]any, truncated bool, limit int) *DatabaseQueryResultVO {
	return &DatabaseQueryResultVO{
		SQLType:      sqlType,
		ExecutedSQL:  executedSQL,
		Columns:      columns,
		ColumnTypes:  make([]string, len(columns)),
		Rows:         rows,
		RowsReturned: len(rows),
		Limit:        limit,
		Truncated:    truncated,
	}
}

func normalizeRedisValue(value any) any {
	switch current := value.(type) {
	case nil:
		return nil
	case string:
		return sanitizeRedisPreview(current)
	default:
		return current
	}
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
