package database

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func parseConnectionParams(item *DatabaseInstance) (map[string]any, error) {
	if item == nil || strings.TrimSpace(item.ConnectionParams) == "" {
		return map[string]any{}, nil
	}
	result := make(map[string]any)
	if err := json.Unmarshal([]byte(item.ConnectionParams), &result); err != nil {
		return nil, fmt.Errorf("连接参数 JSON 格式错误: %w", err)
	}
	return result, nil
}

func connectionParamString(params map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := params[key]
		if !ok || value == nil {
			continue
		}
		switch current := value.(type) {
		case string:
			if strings.TrimSpace(current) != "" {
				return strings.TrimSpace(current)
			}
		case float64:
			return strconv.FormatInt(int64(current), 10)
		case bool:
			if current {
				return "true"
			}
			return "false"
		}
	}
	return ""
}

func connectionParamBool(params map[string]any, defaultValue bool, keys ...string) bool {
	for _, key := range keys {
		value, ok := params[key]
		if !ok || value == nil {
			continue
		}
		switch current := value.(type) {
		case bool:
			return current
		case string:
			if parsed, err := strconv.ParseBool(strings.TrimSpace(current)); err == nil {
				return parsed
			}
		case float64:
			return current != 0
		}
	}
	return defaultValue
}

func connectionParamInt(params map[string]any, defaultValue int, keys ...string) int {
	for _, key := range keys {
		value, ok := params[key]
		if !ok || value == nil {
			continue
		}
		switch current := value.(type) {
		case float64:
			return int(current)
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(current)); err == nil {
				return parsed
			}
		}
	}
	return defaultValue
}

func connectionParamStringList(params map[string]any, keys ...string) []string {
	result := make([]string, 0)
	seen := make(map[string]struct{})
	appendValue := func(value string) {
		text := strings.TrimSpace(value)
		if text == "" {
			return
		}
		for _, part := range strings.FieldsFunc(text, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n'
		}) {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if _, ok := seen[part]; ok {
				continue
			}
			seen[part] = struct{}{}
			result = append(result, part)
		}
	}
	for _, key := range keys {
		value, ok := params[key]
		if !ok || value == nil {
			continue
		}
		switch current := value.(type) {
		case string:
			appendValue(current)
		case []string:
			for _, item := range current {
				appendValue(item)
			}
		case []any:
			for _, item := range current {
				appendValue(fmt.Sprintf("%v", item))
			}
		}
	}
	return result
}
