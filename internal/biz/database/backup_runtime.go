package database

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type backupCommandSpec struct {
	Commands     []string
	Args         []string
	Env          []string
	DatabaseName string
	FileExt      string
	OutputMode   string
	Runner       func(ctx context.Context, outputPath string) (int64, error)
}

const (
	backupOutputModeGzip = "gzip"
	backupOutputModeRaw  = "raw"
)

var (
	backupCommandLookPath = exec.LookPath
	backupCommandFactory  = exec.CommandContext
)

func (uc *UseCase) resolveBackupPolicy(ctx context.Context) (*DatabaseBackupPolicy, error) {
	defaultPolicy := &DatabaseBackupPolicy{
		DefaultRetentionDays: 7,
		StoragePath:          "./data/database-backups",
	}
	if uc.backupPolicyResolver == nil {
		return defaultPolicy, nil
	}
	policy, err := uc.backupPolicyResolver(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取备份配置失败: %w", err)
	}
	if policy == nil {
		return defaultPolicy, nil
	}
	if policy.DefaultRetentionDays <= 0 {
		policy.DefaultRetentionDays = defaultPolicy.DefaultRetentionDays
	}
	if strings.TrimSpace(policy.StoragePath) == "" {
		policy.StoragePath = defaultPolicy.StoragePath
	}
	return policy, nil
}

func buildBackupCommandSpec(item *DatabaseInstance, credential *ConnectionCredential, databaseName, backupType string) (*backupCommandSpec, error) {
	if item == nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	if strings.TrimSpace(databaseName) == "" {
		return nil, fmt.Errorf("备份目标数据库不能为空")
	}
	backupType = normalizeBackupType(backupType)
	if !supportsBackupType(item.DBType, backupType) {
		return nil, fmt.Errorf("%s 不支持备份类型 %s", DBTypeText(item.DBType), backupType)
	}

	host := strings.TrimSpace(item.Host)
	port := fmt.Sprintf("%d", item.Port)

	switch normalizeDBType(item.DBType) {
	case DBTypeMySQL, DBTypeMariaDB:
		if credential == nil || strings.TrimSpace(credential.Username) == "" {
			return nil, fmt.Errorf("凭据用户名不能为空")
		}
		if credential.Password == "" {
			return nil, fmt.Errorf("凭据密码不能为空")
		}
		username := strings.TrimSpace(credential.Username)
		args := []string{
			"--host=" + host,
			"--port=" + port,
			"--user=" + username,
			"--single-transaction",
			"--quick",
			"--skip-lock-tables",
			"--default-character-set=utf8mb4",
			databaseName,
		}
		if item.TLSEnabled {
			args = append(args[:len(args)-1], append([]string{"--ssl"}, args[len(args)-1])...)
		}
		return &backupCommandSpec{
			Commands:     []string{"mysqldump", "mariadb-dump"},
			Args:         args,
			Env:          []string{"MYSQL_PWD=" + credential.Password},
			DatabaseName: databaseName,
			FileExt:      ".sql.gz",
			OutputMode:   backupOutputModeGzip,
		}, nil
	case DBTypePostgreSQL:
		if credential == nil || strings.TrimSpace(credential.Username) == "" {
			return nil, fmt.Errorf("凭据用户名不能为空")
		}
		if credential.Password == "" {
			return nil, fmt.Errorf("凭据密码不能为空")
		}
		username := strings.TrimSpace(credential.Username)
		sslMode := "disable"
		if item.TLSEnabled {
			sslMode = "require"
		}
		format := "plain"
		fileExt := ".sql.gz"
		outputMode := backupOutputModeGzip
		if backupType == DatabaseBackupTypeLogicalCustom {
			format = "custom"
			fileExt = ".dump"
			outputMode = backupOutputModeRaw
		}
		return &backupCommandSpec{
			Commands: []string{"pg_dump"},
			Args: []string{
				"--host", host,
				"--port", port,
				"--username", username,
				"--dbname", databaseName,
				"--format=" + format,
				"--encoding=UTF8",
				"--no-owner",
				"--no-privileges",
			},
			Env: []string{
				"PGPASSWORD=" + credential.Password,
				"PGSSLMODE=" + sslMode,
			},
			DatabaseName: databaseName,
			FileExt:      fileExt,
			OutputMode:   outputMode,
		}, nil
	case DBTypeRedis:
		return buildRedisBackupCommandSpec(item, credential, databaseName)
	default:
		return nil, fmt.Errorf("%s 逻辑备份将在后续批次接入", DBTypeText(item.DBType))
	}
}

func resolveBackupDatabaseName(item *DatabaseInstance, credential *ConnectionCredential) (string, error) {
	if item == nil {
		return "", fmt.Errorf("数据库实例不存在")
	}

	switch normalizeDBType(item.DBType) {
	case DBTypeMySQL, DBTypeMariaDB:
		databaseName := strings.TrimSpace(item.DefaultDatabase)
		if databaseName == "" {
			return "", fmt.Errorf("MySQL / MariaDB 逻辑备份需要先配置默认库")
		}
		return databaseName, nil
	case DBTypePostgreSQL:
		if databaseName := strings.TrimSpace(item.DefaultDatabase); databaseName != "" {
			return databaseName, nil
		}
		if credential != nil && strings.TrimSpace(credential.Username) != "" {
			return strings.TrimSpace(credential.Username), nil
		}
		return "", fmt.Errorf("PostgreSQL 逻辑备份目标数据库不能为空")
	case DBTypeRedis:
		return "all-dbs", nil
	default:
		return "", fmt.Errorf("%s 逻辑备份将在后续批次接入", DBTypeText(item.DBType))
	}
}

func resolveBackupStorageRoot(policy *DatabaseBackupPolicy) (string, error) {
	root := "./data/database-backups"
	if policy != nil && strings.TrimSpace(policy.StoragePath) != "" {
		root = strings.TrimSpace(policy.StoragePath)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("解析备份目录失败: %w", err)
	}
	return absRoot, nil
}

func (uc *UseCase) secureBackupFilePath(ctx context.Context, filePath string) (string, error) {
	policy, err := uc.resolveBackupPolicy(ctx)
	if err != nil {
		return "", err
	}
	storageRoot, err := resolveBackupStorageRoot(policy)
	if err != nil {
		return "", err
	}
	return secureBackupPathUnderRoot(storageRoot, filePath)
}

func secureBackupPathUnderRoot(storageRoot, filePath string) (string, error) {
	storageRoot = strings.TrimSpace(storageRoot)
	filePath = strings.TrimSpace(filePath)
	if storageRoot == "" {
		return "", fmt.Errorf("备份目录不能为空")
	}
	if filePath == "" {
		return "", fmt.Errorf("备份文件路径不能为空")
	}

	absRoot, err := filepath.Abs(storageRoot)
	if err != nil {
		return "", fmt.Errorf("解析备份目录失败: %w", err)
	}
	absFile, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("解析备份文件路径失败: %w", err)
	}
	if !isPathUnderRoot(absRoot, absFile) {
		return "", fmt.Errorf("备份文件路径不在备份目录内")
	}

	realRoot := absRoot
	if evaluatedRoot, err := filepath.EvalSymlinks(absRoot); err == nil {
		realRoot = evaluatedRoot
	}
	if evaluatedFile, err := filepath.EvalSymlinks(absFile); err == nil && !isPathUnderRoot(realRoot, evaluatedFile) {
		return "", fmt.Errorf("备份文件路径不在备份目录内")
	}
	return absFile, nil
}

func isPathUnderRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

func buildBackupOutputPath(storageRoot string, item *DatabaseInstance, task *DatabaseBackupTask, databaseName string, startedAt time.Time, fileExt string) (string, string, error) {
	if strings.TrimSpace(storageRoot) == "" {
		return "", "", fmt.Errorf("备份目录不能为空")
	}
	instancePart := safeBackupFilenamePart("")
	taskPart := safeBackupFilenamePart("")
	databasePart := safeBackupFilenamePart(databaseName)
	if item != nil {
		instancePart = safeBackupFilenamePart(item.Name)
	}
	if task != nil {
		taskPart = safeBackupFilenamePart(task.Name)
	}
	fileExt = strings.TrimSpace(fileExt)
	if fileExt == "" {
		fileExt = ".sql.gz"
	}

	dir := filepath.Join(storageRoot, instancePart, startedAt.Format("20060102"))
	filename := fmt.Sprintf("%s-%s-%s%s", taskPart, databasePart, startedAt.Format("20060102150405"), fileExt)
	return filepath.Join(dir, filename), filename, nil
}

func runBackupCommand(ctx context.Context, spec *backupCommandSpec, outputPath string) (int64, error) {
	if spec == nil {
		return 0, fmt.Errorf("备份命令不能为空")
	}
	if spec.Runner != nil {
		return spec.Runner(ctx, outputPath)
	}
	commandPath, err := findBackupCommand(spec.Commands)
	if err != nil {
		return 0, err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return 0, fmt.Errorf("创建备份目录失败: %w", err)
	}

	tmpPath := outputPath + ".partial"
	file, err := os.Create(tmpPath)
	if err != nil {
		return 0, fmt.Errorf("创建备份文件失败: %w", err)
	}

	var stderr bytes.Buffer
	cmd := backupCommandFactory(ctx, commandPath, spec.Args...)
	cmd.Env = append(os.Environ(), spec.Env...)
	cmd.Stderr = &stderr

	outputMode := strings.ToLower(strings.TrimSpace(spec.OutputMode))
	if outputMode == "" {
		outputMode = backupOutputModeGzip
	}

	var gzipWriter *gzip.Writer
	if outputMode == backupOutputModeRaw {
		cmd.Stdout = file
	} else {
		gzipWriter = gzip.NewWriter(file)
		cmd.Stdout = gzipWriter
	}

	runErr := cmd.Run()
	var closeErr error
	if gzipWriter != nil {
		closeErr = gzipWriter.Close()
	}
	fileCloseErr := file.Close()
	if runErr != nil || closeErr != nil || fileCloseErr != nil {
		_ = os.Remove(tmpPath)
		if runErr != nil {
			return 0, fmt.Errorf("执行备份命令失败: %s", trimBackupCommandError(stderr.String(), runErr))
		}
		if closeErr != nil {
			if outputMode == backupOutputModeRaw {
				return 0, fmt.Errorf("写入备份文件失败: %w", closeErr)
			}
			return 0, fmt.Errorf("写入备份压缩文件失败: %w", closeErr)
		}
		return 0, fmt.Errorf("关闭备份文件失败: %w", fileCloseErr)
	}

	if err := os.Rename(tmpPath, outputPath); err != nil {
		_ = os.Remove(tmpPath)
		return 0, fmt.Errorf("写入备份文件失败: %w", err)
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		return 0, fmt.Errorf("读取备份文件失败: %w", err)
	}
	return info.Size(), nil
}

func findBackupCommand(commands []string) (string, error) {
	for _, command := range commands {
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}
		path, err := backupCommandLookPath(command)
		if err == nil {
			return path, nil
		}
	}
	display := strings.Join(commands, " / ")
	if strings.TrimSpace(display) == "" {
		display = "备份客户端命令"
	}
	return "", fmt.Errorf("未安装备份客户端命令: %s", display)
}

func trimBackupCommandError(output string, runErr error) string {
	output = strings.TrimSpace(output)
	if output != "" {
		output = strings.ReplaceAll(output, "\n", " ")
		if len(output) > 400 {
			output = output[:400]
		}
		return output
	}
	return runErr.Error()
}

func classifyBackupFailureMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "备份失败：未知错误"
	}
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "未安装备份客户端命令"):
		return "备份失败：备份客户端命令缺失，" + message
	case strings.Contains(lower, "凭据") || strings.Contains(lower, "password") || strings.Contains(lower, "authentication"):
		return "备份失败：凭据或认证异常，" + message
	case strings.Contains(lower, "permission denied") || strings.Contains(lower, "access denied"):
		return "备份失败：文件或数据库权限不足，" + message
	case strings.Contains(lower, "no space") || strings.Contains(lower, "disk") || strings.Contains(lower, "空间"):
		return "备份失败：磁盘空间或存储异常，" + message
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "timeout") || strings.Contains(lower, "network"):
		return "备份失败：网络或连接异常，" + message
	default:
		return "备份失败：" + message
	}
}

func safeBackupFilenamePart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "default"
	}
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	cleaned := replacer.Replace(value)
	cleaned = strings.Trim(cleaned, "._-")
	if cleaned == "" {
		return "default"
	}
	return trimText(cleaned, 80)
}

func buildBackupRecordMessage(item *DatabaseBackupRecord) string {
	if item == nil {
		return ""
	}
	if strings.TrimSpace(item.Status) == DatabaseBackupStatusSuccess &&
		strings.TrimSpace(item.FileName) != "" &&
		strings.TrimSpace(item.FilePath) == "" {
		return buildBackupPrunedMessage(item.FileName)
	}
	if message := strings.TrimSpace(item.ErrorMessage); message != "" {
		return message
	}
	switch strings.TrimSpace(item.Status) {
	case DatabaseBackupStatusSuccess:
		if strings.TrimSpace(item.FileName) != "" {
			return "逻辑备份完成，文件已生成"
		}
		return "逻辑备份完成"
	case DatabaseBackupStatusQueued:
		return "备份任务已进入执行队列"
	case DatabaseBackupStatusRunning:
		return "逻辑备份执行中"
	case DatabaseBackupStatusCleaning:
		return "备份保留策略清理中"
	case DatabaseBackupStatusPending:
		return "备份任务等待执行"
	case DatabaseBackupStatusFailed:
		return "逻辑备份失败"
	default:
		return ""
	}
}

func detectBackupContentType(fileName string) string {
	fileName = strings.ToLower(strings.TrimSpace(fileName))
	switch {
	case strings.HasSuffix(fileName, ".gz"):
		return "application/gzip"
	case strings.HasSuffix(fileName, ".dump"):
		return "application/octet-stream"
	case strings.HasSuffix(fileName, ".sql"):
		return "application/sql"
	default:
		return "application/octet-stream"
	}
}
