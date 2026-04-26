package database

import (
	"strings"
	"unicode"
)

type sqlFormatToken struct {
	text string
	kind string
}

var sqlFormatterKeywords = map[string]struct{}{
	"all": {}, "and": {}, "as": {}, "asc": {}, "between": {}, "by": {}, "case": {}, "desc": {}, "distinct": {},
	"else": {}, "end": {}, "exists": {}, "from": {}, "full": {}, "group": {}, "having": {}, "in": {}, "inner": {},
	"is": {}, "join": {}, "left": {}, "like": {}, "limit": {}, "not": {}, "null": {}, "offset": {}, "on": {},
	"or": {}, "order": {}, "outer": {}, "right": {}, "select": {}, "then": {}, "union": {}, "when": {}, "where": {},
	"with": {}, "cross": {}, "explain": {}, "show": {}, "describe": {},
}

var sqlFormatterPhrases = [][]string{
	{"group", "by"},
	{"order", "by"},
	{"union", "all"},
	{"left", "outer", "join"},
	{"right", "outer", "join"},
	{"full", "outer", "join"},
	{"left", "join"},
	{"right", "join"},
	{"inner", "join"},
	{"full", "join"},
	{"cross", "join"},
}

var sqlFormatterLineBreaks = map[string]struct{}{
	"SELECT":           {},
	"FROM":             {},
	"WHERE":            {},
	"GROUP BY":         {},
	"HAVING":           {},
	"ORDER BY":         {},
	"LIMIT":            {},
	"OFFSET":           {},
	"UNION":            {},
	"UNION ALL":        {},
	"LEFT JOIN":        {},
	"RIGHT JOIN":       {},
	"INNER JOIN":       {},
	"FULL JOIN":        {},
	"LEFT OUTER JOIN":  {},
	"RIGHT OUTER JOIN": {},
	"FULL OUTER JOIN":  {},
	"CROSS JOIN":       {},
	"JOIN":             {},
	"ON":               {},
}

func FormatSQL(sqlText string) string {
	tokens := tokenizeSQL(sqlText)
	if len(tokens) == 0 {
		return ""
	}

	var out strings.Builder
	lastToken := ""
	lastKind := ""

	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if token.kind == "comment" {
			ensureLineBreak(&out)
			out.WriteString(strings.TrimSpace(token.text))
			ensureLineBreak(&out)
			lastToken = ""
			lastKind = token.kind
			continue
		}

		if phrase, consumed := matchSQLPhrase(tokens, i); consumed > 0 {
			if _, ok := sqlFormatterLineBreaks[phrase]; ok {
				ensureLineBreak(&out)
			} else if needsSpace(lastToken, lastKind, phrase, "word") {
				out.WriteByte(' ')
			}
			out.WriteString(phrase)
			i += consumed - 1
			lastToken = phrase
			lastKind = "word"
			continue
		}

		text := token.text
		if token.kind == "word" {
			text = normalizeSQLKeyword(text)
			if _, ok := sqlFormatterLineBreaks[text]; ok {
				ensureLineBreak(&out)
			} else if needsSpace(lastToken, lastKind, text, token.kind) {
				out.WriteByte(' ')
			}
			out.WriteString(text)
			lastToken = text
			lastKind = token.kind
			continue
		}

		switch text {
		case ",":
			trimTrailingSpace(&out)
			out.WriteString(", ")
		case "(":
			if needsSpaceBeforeParen(lastToken) {
				out.WriteByte(' ')
			}
			out.WriteByte('(')
		case ")":
			trimTrailingSpace(&out)
			out.WriteByte(')')
		default:
			if needsSpace(lastToken, lastKind, text, token.kind) {
				out.WriteByte(' ')
			}
			out.WriteString(text)
		}
		lastToken = text
		lastKind = token.kind
	}

	return strings.TrimSpace(collapseBlankLines(out.String()))
}

func tokenizeSQL(sqlText string) []sqlFormatToken {
	runes := []rune(strings.TrimSpace(sqlText))
	tokens := make([]sqlFormatToken, 0)
	for i := 0; i < len(runes); {
		r := runes[i]
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		if unicode.IsSpace(r) {
			i++
			continue
		}
		if r == '-' && next == '-' {
			start := i
			i += 2
			for i < len(runes) && runes[i] != '\n' && runes[i] != '\r' {
				i++
			}
			tokens = append(tokens, sqlFormatToken{text: string(runes[start:i]), kind: "comment"})
			continue
		}
		if r == '#' {
			start := i
			i++
			for i < len(runes) && runes[i] != '\n' && runes[i] != '\r' {
				i++
			}
			tokens = append(tokens, sqlFormatToken{text: string(runes[start:i]), kind: "comment"})
			continue
		}
		if r == '/' && next == '*' {
			start := i
			i += 2
			for i < len(runes)-1 {
				if runes[i] == '*' && runes[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			tokens = append(tokens, sqlFormatToken{text: string(runes[start:i]), kind: "comment"})
			continue
		}
		if r == '\'' || r == '"' || r == '`' {
			start := i
			quote := r
			i++
			for i < len(runes) {
				if runes[i] == quote {
					if quote == '\'' && i+1 < len(runes) && runes[i+1] == quote {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			tokens = append(tokens, sqlFormatToken{text: string(runes[start:i]), kind: "literal"})
			continue
		}
		if strings.ContainsRune("(),=*+-/%<>", r) {
			tokens = append(tokens, sqlFormatToken{text: string(r), kind: "symbol"})
			i++
			continue
		}
		start := i
		for i < len(runes) {
			current := runes[i]
			if unicode.IsSpace(current) || strings.ContainsRune("(),=*+-/%<>", current) {
				break
			}
			if current == '\'' || current == '"' || current == '`' {
				break
			}
			if current == '-' && i+1 < len(runes) && runes[i+1] == '-' {
				break
			}
			if current == '#' || (current == '/' && i+1 < len(runes) && runes[i+1] == '*') {
				break
			}
			i++
		}
		tokens = append(tokens, sqlFormatToken{text: string(runes[start:i]), kind: "word"})
	}
	return tokens
}

func matchSQLPhrase(tokens []sqlFormatToken, index int) (string, int) {
	for _, phrase := range sqlFormatterPhrases {
		if len(tokens)-index < len(phrase) {
			continue
		}
		matched := true
		for i, part := range phrase {
			token := tokens[index+i]
			if token.kind != "word" || strings.ToLower(token.text) != part {
				matched = false
				break
			}
		}
		if matched {
			upper := make([]string, 0, len(phrase))
			for _, part := range phrase {
				upper = append(upper, strings.ToUpper(part))
			}
			return strings.Join(upper, " "), len(phrase)
		}
	}
	return "", 0
}

func normalizeSQLKeyword(token string) string {
	lower := strings.ToLower(token)
	if _, ok := sqlFormatterKeywords[lower]; ok {
		return strings.ToUpper(token)
	}
	return token
}

func needsSpace(previous, previousKind, current, currentKind string) bool {
	if previous == "" {
		return false
	}
	switch current {
	case ",", ")":
		return false
	}
	switch previous {
	case "(", ",", "":
		return false
	}
	if strings.HasSuffix(previous, "\n") {
		return false
	}
	if previousKind == "symbol" {
		return previous != "("
	}
	if currentKind == "symbol" && current != "(" {
		return true
	}
	return true
}

func needsSpaceBeforeParen(previous string) bool {
	switch previous {
	case "", "(", ",":
		return false
	case "IN", "NOT", "EXISTS":
		return true
	default:
		return false
	}
}

func ensureLineBreak(out *strings.Builder) {
	trimTrailingSpace(out)
	value := out.String()
	if value == "" {
		return
	}
	if strings.HasSuffix(value, "\n") {
		return
	}
	out.WriteByte('\n')
}

func trimTrailingSpace(out *strings.Builder) {
	value := out.String()
	trimmed := strings.TrimRight(value, " \t")
	if trimmed == value {
		return
	}
	out.Reset()
	out.WriteString(trimmed)
}

func collapseBlankLines(value string) string {
	lines := strings.Split(value, "\n")
	result := make([]string, 0, len(lines))
	lastBlank := false
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		if strings.TrimSpace(trimmed) == "" {
			if lastBlank {
				continue
			}
			lastBlank = true
			result = append(result, "")
			continue
		}
		lastBlank = false
		result = append(result, trimmed)
	}
	return strings.Join(result, "\n")
}
