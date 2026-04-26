package asset

import (
	"regexp"
	"strings"
	"time"

	assetbiz "github.com/ydcloud-dy/opshub/internal/biz/asset"
)

type terminalRiskRule struct {
	Code        string
	Level       string
	Name        string
	Description string
	Match       func(commandMatchContext) bool
}

type terminalRiskMatch struct {
	Rule              terminalRiskRule
	NormalizedCommand string
}

type commandMatchContext struct {
	Raw        string
	Normalized string
	Fields     []string
	Base       string
}

type terminalCommandTracker struct {
	current   []byte
	escapeSeq []byte
	inEscape  bool
	lastCR    bool
}

var (
	terminalWhitespaceRe = regexp.MustCompile(`\s+`)
	terminalEtcPathRe    = regexp.MustCompile(`(^|\s)/etc/`)
	terminalRiskRules    = []terminalRiskRule{
		{
			Code:        "rm_recursive_force",
			Level:       "high",
			Name:        "递归强制删除",
			Description: "执行了 rm -rf 类命令，可能造成大规模数据删除",
			Match: func(ctx commandMatchContext) bool {
				if ctx.Base != "rm" {
					return false
				}
				hasRecursive := false
				hasForce := false
				for _, field := range ctx.Fields[1:] {
					if !strings.HasPrefix(field, "-") {
						continue
					}
					if field == "--recursive" || strings.ContainsRune(field, 'r') {
						hasRecursive = true
					}
					if field == "--force" || strings.ContainsRune(field, 'f') {
						hasForce = true
					}
				}
				return hasRecursive && hasForce
			},
		},
		{
			Code:        "disk_format_partition",
			Level:       "high",
			Name:        "磁盘格式化或分区",
			Description: "执行了磁盘格式化、分区或交换空间相关命令",
			Match: func(ctx commandMatchContext) bool {
				switch ctx.Base {
				case "mkfs", "fdisk", "parted", "gdisk", "cfdisk", "sfdisk", "mkswap", "wipefs":
					return true
				default:
					return false
				}
			},
		},
		{
			Code:        "dd_to_device",
			Level:       "high",
			Name:        "原始磁盘写入",
			Description: "执行了 dd 并向 /dev 设备写入数据",
			Match: func(ctx commandMatchContext) bool {
				return ctx.Base == "dd" && strings.Contains(ctx.Normalized, " of=/dev/")
			},
		},
		{
			Code:        "system_shutdown_reboot",
			Level:       "high",
			Name:        "关机或重启",
			Description: "执行了系统关机或重启命令",
			Match: func(ctx commandMatchContext) bool {
				switch ctx.Base {
				case "shutdown", "reboot", "halt", "poweroff":
					return true
				}
				return strings.HasPrefix(ctx.Normalized, "systemctl reboot") ||
					strings.HasPrefix(ctx.Normalized, "systemctl poweroff") ||
					strings.HasPrefix(ctx.Normalized, "systemctl halt") ||
					ctx.Normalized == "init 0" || ctx.Normalized == "init 6"
			},
		},
		{
			Code:        "firewall_flush_disable",
			Level:       "high",
			Name:        "清空或关闭防火墙",
			Description: "执行了清空防火墙规则或关闭防火墙命令",
			Match: func(ctx commandMatchContext) bool {
				return strings.HasPrefix(ctx.Normalized, "iptables -f") ||
					strings.HasPrefix(ctx.Normalized, "iptables -x") ||
					strings.HasPrefix(ctx.Normalized, "ip6tables -f") ||
					strings.HasPrefix(ctx.Normalized, "ufw disable") ||
					strings.HasPrefix(ctx.Normalized, "ufw reset")
			},
		},
		{
			Code:        "account_delete_escalate",
			Level:       "high",
			Name:        "账户删除或提权",
			Description: "执行了删除账户或将 UID 调整为 root 的命令",
			Match: func(ctx commandMatchContext) bool {
				return ctx.Base == "userdel" || ctx.Base == "groupdel" || strings.Contains(ctx.Normalized, "usermod -u 0")
			},
		},
		{
			Code:        "service_interrupt",
			Level:       "medium",
			Name:        "服务停止或重启",
			Description: "执行了 systemctl/service 的 stop、restart、disable、mask、kill 等操作",
			Match: func(ctx commandMatchContext) bool {
				return strings.HasPrefix(ctx.Normalized, "systemctl stop") ||
					strings.HasPrefix(ctx.Normalized, "systemctl restart") ||
					strings.HasPrefix(ctx.Normalized, "systemctl disable") ||
					strings.HasPrefix(ctx.Normalized, "systemctl mask") ||
					strings.HasPrefix(ctx.Normalized, "systemctl kill") ||
					strings.HasPrefix(ctx.Normalized, "service ") &&
						(strings.Contains(ctx.Normalized, " stop") || strings.Contains(ctx.Normalized, " restart"))
			},
		},
		{
			Code:        "dangerous_permission_change",
			Level:       "medium",
			Name:        "危险权限变更",
			Description: "执行了 chmod/chown/chgrp 等敏感权限调整命令",
			Match: func(ctx commandMatchContext) bool {
				if ctx.Base == "chmod" {
					return strings.Contains(ctx.Normalized, " 777") ||
						strings.Contains(ctx.Normalized, " 666") ||
						strings.Contains(ctx.Normalized, " 000") ||
						strings.Contains(ctx.Normalized, " a+w") ||
						strings.Contains(ctx.Normalized, " u+s") ||
						strings.Contains(ctx.Normalized, " g+s")
				}
				return strings.HasPrefix(ctx.Normalized, "chown -r") ||
					strings.HasPrefix(ctx.Normalized, "chgrp -r") ||
					ctx.Base == "setfacl" || ctx.Base == "chattr"
			},
		},
		{
			Code:        "task_schedule_change",
			Level:       "medium",
			Name:        "定时任务变更",
			Description: "执行了 crontab 或 at 的编辑、删除操作",
			Match: func(ctx commandMatchContext) bool {
				return strings.HasPrefix(ctx.Normalized, "crontab -r") ||
					strings.HasPrefix(ctx.Normalized, "crontab -e") ||
					strings.HasPrefix(ctx.Normalized, "at -r")
			},
		},
		{
			Code:        "container_resource_delete",
			Level:       "medium",
			Name:        "容器或集群资源删除",
			Description: "执行了 docker 或 kubectl 的删除类操作",
			Match: func(ctx commandMatchContext) bool {
				return strings.HasPrefix(ctx.Normalized, "docker rm") ||
					strings.HasPrefix(ctx.Normalized, "docker rmi") ||
					strings.HasPrefix(ctx.Normalized, "kubectl delete")
			},
		},
		{
			Code:        "account_lock_or_sudo_edit",
			Level:       "medium",
			Name:        "账户锁定或 sudo 配置修改",
			Description: "执行了 passwd 锁定、删密或 visudo 之类命令",
			Match: func(ctx commandMatchContext) bool {
				return strings.HasPrefix(ctx.Normalized, "passwd -l") ||
					strings.HasPrefix(ctx.Normalized, "passwd -d") ||
					ctx.Base == "visudo" || strings.HasPrefix(ctx.Normalized, "usermod -l")
			},
		},
		{
			Code:        "etc_config_change",
			Level:       "medium",
			Name:        "系统配置文件修改",
			Description: "命令直接操作了 /etc 下的配置文件",
			Match: func(ctx commandMatchContext) bool {
				if !terminalEtcPathRe.MatchString(ctx.Normalized) {
					return false
				}
				switch ctx.Base {
				case "vi", "vim", "nano", "sed", "cp", "mv", "tee", "cat":
					return true
				default:
					return false
				}
			},
		},
		{
			Code:        "package_remove",
			Level:       "medium",
			Name:        "软件包卸载",
			Description: "执行了 apt/yum/dnf/rpm 的卸载类操作",
			Match: func(ctx commandMatchContext) bool {
				return strings.HasPrefix(ctx.Normalized, "apt remove") ||
					strings.HasPrefix(ctx.Normalized, "apt purge") ||
					strings.HasPrefix(ctx.Normalized, "yum remove") ||
					strings.HasPrefix(ctx.Normalized, "dnf remove") ||
					strings.HasPrefix(ctx.Normalized, "rpm -e")
			},
		},
	}
)

func newTerminalCommandTracker() *terminalCommandTracker {
	return &terminalCommandTracker{current: make([]byte, 0, 128)}
}

func (t *terminalCommandTracker) Feed(data []byte) []string {
	commands := make([]string, 0, 2)
	for _, b := range data {
		switch {
		case t.inEscape:
			t.escapeSeq = append(t.escapeSeq, b)
			if isEscapeSequenceTerminator(b) {
				t.inEscape = false
				t.escapeSeq = t.escapeSeq[:0]
			}
		case b == 0x1b:
			t.inEscape = true
			t.escapeSeq = append(t.escapeSeq[:0], b)
		case b == '\r' || b == '\n':
			if t.lastCR && b == '\n' {
				t.lastCR = false
				continue
			}
			if cmd := strings.TrimSpace(string(t.current)); cmd != "" {
				commands = append(commands, cmd)
			}
			t.current = t.current[:0]
			t.lastCR = b == '\r'
		case b == 0x7f || b == 0x08:
			if len(t.current) > 0 {
				t.current = t.current[:len(t.current)-1]
			}
			t.lastCR = false
		case b == 0x03 || b == 0x15:
			t.current = t.current[:0]
			t.lastCR = false
		case b == 0x17:
			t.deleteLastWord()
			t.lastCR = false
		case b < 0x20 && b != '\t':
			t.lastCR = false
		default:
			t.current = append(t.current, b)
			t.lastCR = false
		}
	}
	return commands
}

func (t *terminalCommandTracker) deleteLastWord() {
	for len(t.current) > 0 && t.current[len(t.current)-1] == ' ' {
		t.current = t.current[:len(t.current)-1]
	}
	for len(t.current) > 0 && t.current[len(t.current)-1] != ' ' {
		t.current = t.current[:len(t.current)-1]
	}
}

func isEscapeSequenceTerminator(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '~'
}

func normalizeTerminalCommand(command string) string {
	cleaned := terminalWhitespaceRe.ReplaceAllString(strings.TrimSpace(command), " ")
	if cleaned == "" {
		return ""
	}

	fields := strings.Fields(strings.ToLower(cleaned))
	for len(fields) > 0 && strings.Contains(fields[0], "=") && !strings.HasPrefix(fields[0], "=") {
		fields = fields[1:]
	}
	for len(fields) > 0 {
		switch fields[0] {
		case "sudo", "command", "builtin", "env", "nohup":
			fields = fields[1:]
		default:
			goto normalized
		}
	}

normalized:
	if len(fields) >= 3 {
		switch fields[0] {
		case "bash", "sh", "zsh", "/bin/bash", "/bin/sh", "/bin/zsh":
			if fields[1] == "-c" {
				inner := strings.Trim(strings.Join(fields[2:], " "), "'\"")
				return normalizeTerminalCommand(inner)
			}
		}
	}

	return strings.Join(fields, " ")
}

func classifyTerminalCommand(command string) *terminalRiskMatch {
	normalized := normalizeTerminalCommand(command)
	if normalized == "" {
		return nil
	}

	ctx := commandMatchContext{
		Raw:        strings.TrimSpace(command),
		Normalized: normalized,
		Fields:     strings.Fields(normalized),
	}
	if len(ctx.Fields) > 0 {
		ctx.Base = ctx.Fields[0]
	}

	for _, rule := range terminalRiskRules {
		if rule.Match(ctx) {
			return &terminalRiskMatch{Rule: rule, NormalizedCommand: normalized}
		}
	}

	return nil
}

func buildTerminalRiskEvent(session *TerminalSession, command string, executedAt time.Time) *assetbiz.TerminalCommandEvent {
	match := classifyTerminalCommand(command)
	if match == nil {
		return nil
	}

	return &assetbiz.TerminalCommandEvent{
		SessionID:         session.AuditRecordID,
		HostID:            session.HostID,
		HostName:          session.HostName,
		HostIP:            session.HostIP,
		UserID:            session.UserID,
		Username:          session.Username,
		CommandText:       strings.TrimSpace(command),
		NormalizedCommand: match.NormalizedCommand,
		RiskLevel:         match.Rule.Level,
		RuleCode:          match.Rule.Code,
		RuleName:          match.Rule.Name,
		RuleDescription:   match.Rule.Description,
		Source:            "input",
		Confidence:        "medium",
		ExecutedAt:        executedAt,
	}
}
