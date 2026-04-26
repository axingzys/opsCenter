package database

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

type restoreCommandSpec struct {
	Commands     []string
	Args         []string
	Env          []string
	DatabaseName string
	InputMode    string
	Runner       func(ctx context.Context, inputPath string) error
}

const (
	restoreInputModeStdin   = "stdin"
	restoreInputModeFileArg = "file_arg"
)

func buildRestoreCommandSpec(item *DatabaseInstance, credential *ConnectionCredential, databaseName, backupType string) (*restoreCommandSpec, error) {
	if item == nil {
		return nil, fmt.Errorf("目标数据库实例不存在")
	}
	if strings.TrimSpace(databaseName) == "" {
		return nil, fmt.Errorf("恢复目标数据库不能为空")
	}
	backupType = normalizeBackupType(backupType)
	if !supportsBackupType(item.DBType, backupType) {
		return nil, fmt.Errorf("%s 不支持恢复备份类型 %s", DBTypeText(item.DBType), backupType)
	}

	host := strings.TrimSpace(item.Host)
	port := fmt.Sprintf("%d", item.Port)

	switch normalizeDBType(item.DBType) {
	case DBTypeMySQL, DBTypeMariaDB:
		if credential == nil || strings.TrimSpace(credential.Username) == "" {
			return nil, fmt.Errorf("目标实例凭据用户名不能为空")
		}
		if credential.Password == "" {
			return nil, fmt.Errorf("目标实例凭据密码不能为空")
		}
		username := strings.TrimSpace(credential.Username)
		args := []string{
			"--host=" + host,
			"--port=" + port,
			"--user=" + username,
			"--default-character-set=utf8mb4",
			databaseName,
		}
		if item.TLSEnabled {
			args = append(args[:len(args)-1], append([]string{"--ssl"}, args[len(args)-1])...)
		}
		return &restoreCommandSpec{
			Commands:     []string{"mysql", "mariadb"},
			Args:         args,
			Env:          []string{"MYSQL_PWD=" + credential.Password},
			DatabaseName: databaseName,
			InputMode:    restoreInputModeStdin,
		}, nil
	case DBTypePostgreSQL:
		if credential == nil || strings.TrimSpace(credential.Username) == "" {
			return nil, fmt.Errorf("目标实例凭据用户名不能为空")
		}
		if credential.Password == "" {
			return nil, fmt.Errorf("目标实例凭据密码不能为空")
		}
		username := strings.TrimSpace(credential.Username)
		sslMode := "disable"
		if item.TLSEnabled {
			sslMode = "require"
		}
		args := []string{
			"--host", host,
			"--port", port,
			"--username", username,
			"--dbname", databaseName,
		}
		commands := []string{"psql"}
		inputMode := restoreInputModeStdin
		if backupType == DatabaseBackupTypeLogicalCustom {
			commands = []string{"pg_restore"}
			args = append(args,
				"--no-owner",
				"--no-privileges",
				"--single-transaction",
				"--exit-on-error",
			)
			inputMode = restoreInputModeFileArg
		} else {
			args = append(args,
				"--no-psqlrc",
				"--single-transaction",
				"--set", "ON_ERROR_STOP=on",
			)
		}
		return &restoreCommandSpec{
			Commands: commands,
			Args:     args,
			Env: []string{
				"PGPASSWORD=" + credential.Password,
				"PGSSLMODE=" + sslMode,
			},
			DatabaseName: databaseName,
			InputMode:    inputMode,
		}, nil
	case DBTypeRedis:
		return buildRedisRestoreCommandSpec(item, credential, databaseName)
	default:
		return nil, fmt.Errorf("%s 恢复演练将在后续批次接入", DBTypeText(item.DBType))
	}
}

func resolveRestoreDatabaseName(item *DatabaseInstance, credential *ConnectionCredential) (string, error) {
	if item == nil {
		return "", fmt.Errorf("目标数据库实例不存在")
	}
	switch normalizeDBType(item.DBType) {
	case DBTypeMySQL, DBTypeMariaDB:
		databaseName := strings.TrimSpace(item.DefaultDatabase)
		if databaseName == "" {
			return "", fmt.Errorf("MySQL / MariaDB 恢复演练需要先配置目标默认库")
		}
		return databaseName, nil
	case DBTypePostgreSQL:
		if databaseName := strings.TrimSpace(item.DefaultDatabase); databaseName != "" {
			return databaseName, nil
		}
		if credential != nil && strings.TrimSpace(credential.Username) != "" {
			return strings.TrimSpace(credential.Username), nil
		}
		return "", fmt.Errorf("PostgreSQL 恢复演练目标数据库不能为空")
	case DBTypeRedis:
		return "all-dbs", nil
	default:
		return "", fmt.Errorf("%s 恢复演练将在后续批次接入", DBTypeText(item.DBType))
	}
}

func runRestoreCommand(ctx context.Context, spec *restoreCommandSpec, inputPath string) error {
	if spec == nil {
		return fmt.Errorf("恢复命令不能为空")
	}
	if spec.Runner != nil {
		return spec.Runner(ctx, inputPath)
	}
	commandPath, err := findRestoreCommand(spec.Commands)
	if err != nil {
		return err
	}

	inputMode := strings.ToLower(strings.TrimSpace(spec.InputMode))
	if inputMode == "" {
		inputMode = restoreInputModeStdin
	}

	var stderr bytes.Buffer
	args := append([]string{}, spec.Args...)
	if inputMode == restoreInputModeFileArg {
		args = append(args, inputPath)
		cmd := backupCommandFactory(ctx, commandPath, args...)
		cmd.Env = append(os.Environ(), spec.Env...)
		cmd.Stdout = io.Discard
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("执行恢复演练命令失败: %s", trimBackupCommandError(stderr.String(), err))
		}
		return nil
	}

	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("打开备份文件失败: %w", err)
	}
	defer file.Close()

	var input io.Reader = file
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(inputPath)), ".gz") {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("读取备份压缩文件失败: %w", err)
		}
		defer gzipReader.Close()
		input = gzipReader
	}

	cmd := backupCommandFactory(ctx, commandPath, args...)
	cmd.Env = append(os.Environ(), spec.Env...)
	cmd.Stdin = input
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("执行恢复演练命令失败: %s", trimBackupCommandError(stderr.String(), err))
	}
	return nil
}

func findRestoreCommand(commands []string) (string, error) {
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
		display = "恢复客户端命令"
	}
	return "", fmt.Errorf("未安装恢复客户端命令: %s", display)
}
