package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type DatabaseRunLogArchiveOnceRequest struct {
	RunnerHostID uint   `json:"runnerHostId" binding:"required"`
	FileName     string `json:"fileName" binding:"omitempty,max=255"`
}

type mysqlBinaryLogFile struct {
	Name string
	Size int64
}

type mysqlBinlogArchiveSelection struct {
	FileName string
	FileSize int64
	Previous string
	Next     string
}

type mysqlBinlogArchiveScriptInput struct {
	WorkDir          string
	StorageMountPath string
	StreamID         uint
	DBHost           string
	DBPort           int
	DBUser           string
	DBPassword       string
	BinlogFile       string
}

type mysqlBinlogArchiveOutput struct {
	FileName       string `json:"fileName"`
	StoragePath    string `json:"storagePath"`
	StorageURI     string `json:"storageUri"`
	FileSize       int64  `json:"fileSize"`
	ChecksumSHA256 string `json:"checksumSha256"`
	FirstEventLine string `json:"firstEventLine"`
	LastEventLine  string `json:"lastEventLine"`
	FirstEventTime string `json:"firstEventTime"`
	LastEventTime  string `json:"lastEventTime"`
}

type runnerBinlogArchiveResult struct {
	RunnerHostID   uint                     `json:"runnerHostId"`
	RunnerID       string                   `json:"runnerId"`
	StreamID       uint                     `json:"streamId"`
	SourceInstance uint                     `json:"sourceInstanceId"`
	Binlog         mysqlBinlogArchiveOutput `json:"binlog"`
	Stdout         string                   `json:"stdout"`
	Stderr         string                   `json:"stderr"`
	ExitCode       int                      `json:"exitCode"`
	StartedAt      string                   `json:"startedAt"`
	FinishedAt     string                   `json:"finishedAt"`
	DurationMs     int64                    `json:"durationMs"`
}

func (uc *UseCase) RunLogArchiveOnce(ctx context.Context, streamID uint, req *DatabaseRunLogArchiveOnceRequest, operator QueryOperator) (*DatabaseRunnerJobVO, error) {
	if uc.logArchiveStreamRepo == nil || uc.runnerHostRepo == nil || uc.runnerJobRepo == nil {
		return nil, fmt.Errorf("日志归档 Runner 仓库未配置")
	}
	if streamID == 0 {
		return nil, fmt.Errorf("日志归档流ID不能为空")
	}
	if req == nil || req.RunnerHostID == 0 {
		return nil, fmt.Errorf("请选择 Runner 主机")
	}
	stream, err := uc.logArchiveStreamRepo.GetByID(ctx, streamID)
	if err != nil {
		return nil, fmt.Errorf("日志归档流不存在")
	}
	if !stream.Enabled || stream.Status == DatabaseLogArchiveStreamStatusDisabled {
		return nil, fmt.Errorf("日志归档流已禁用")
	}
	if normalizeArchiveType(stream.ArchiveType) != DatabaseArchiveTypeBinlog {
		return nil, fmt.Errorf("当前仅支持 MySQL/MariaDB binlog 一次性归档")
	}
	sourceInstanceID := stream.SourceInstanceID
	if sourceInstanceID == 0 {
		sourceInstanceID = stream.InstanceID
	}
	instance, err := uc.instanceRepo.GetByID(ctx, sourceInstanceID)
	if err != nil {
		return nil, fmt.Errorf("日志来源实例不存在")
	}
	dbType := normalizeDBType(instance.DBType)
	if dbType != DBTypeMySQL && dbType != DBTypeMariaDB {
		return nil, fmt.Errorf("%s 暂不支持 binlog 归档 Runner", DBTypeText(instance.DBType))
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, req.RunnerHostID)
	if err != nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	if !host.Enabled || host.Status == DatabaseRunnerHostStatusDisabled {
		return nil, fmt.Errorf("Runner 主机已禁用")
	}
	if host.RunnerType != DatabaseRunnerTypeSSH {
		return nil, fmt.Errorf("当前仅支持 SSH Runner 执行 binlog 归档")
	}
	if uc.credentialResolver == nil {
		return nil, fmt.Errorf("连接凭据解析器未配置")
	}
	dbCredential, err := uc.credentialResolver(ctx, instance.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("解析数据库凭据失败: %w", err)
	}
	logs, err := listMySQLBinaryLogs(ctx, instance, dbCredential)
	if err != nil {
		return nil, err
	}
	selection, err := selectMySQLBinlogForArchive(logs, req.FileName, stream.LastArchiveName)
	if err != nil {
		return nil, err
	}
	job := &DatabaseRunnerJob{
		JobType:          DatabaseRunnerJobTypeBinlogArchive,
		RunnerHostID:     host.ID,
		RunnerID:         runnerIDForHost(host),
		SourceInstanceID: sourceInstanceID,
		Status:           DatabaseRunnerJobStatusQueued,
		AllowedCommand:   DatabaseRunnerAllowedCommandBinlogArchiveOnce,
		CommandSummary:   fmt.Sprintf("一次性归档 binlog: %s", selection.FileName),
		WorkDir:          host.WorkDir,
		OperatorID:       operator.ID,
		OperatorName:     trimText(operator.Username, 120),
		RequestJSON:      binlogArchiveRequestJSON(stream, host, selection, operator),
	}
	if err := uc.runnerJobRepo.Create(ctx, job); err != nil {
		return nil, err
	}
	go uc.executeBinlogArchiveOnce(context.Background(), stream.ID, host.ID, job.ID, selection.FileName, selection.FileSize, selection.Previous, selection.Next)
	return uc.toRunnerJobVO(ctx, job), nil
}

func (uc *UseCase) executeBinlogArchiveOnce(ctx context.Context, streamID, runnerHostID, jobID uint, fileName string, fileSize int64, previous, next string) {
	if uc.logArchiveStreamRepo == nil || uc.logArchiveRepo == nil || uc.runnerHostRepo == nil || uc.runnerJobRepo == nil {
		return
	}
	stream, streamErr := uc.logArchiveStreamRepo.GetByID(ctx, streamID)
	host, hostErr := uc.runnerHostRepo.GetByID(ctx, runnerHostID)
	job, jobErr := uc.runnerJobRepo.GetByID(ctx, jobID)
	if streamErr != nil || hostErr != nil || jobErr != nil || stream == nil || host == nil || job == nil {
		return
	}
	sourceInstanceID := stream.SourceInstanceID
	if sourceInstanceID == 0 {
		sourceInstanceID = stream.InstanceID
	}
	instance, err := uc.instanceRepo.GetByID(ctx, sourceInstanceID)
	if err != nil {
		uc.failBinlogArchiveJob(ctx, stream, job, fmt.Errorf("日志来源实例不存在"))
		return
	}
	dbCredential, err := uc.credentialResolver(ctx, instance.CredentialID)
	if err != nil {
		uc.failBinlogArchiveJob(ctx, stream, job, fmt.Errorf("解析数据库凭据失败: %w", err))
		return
	}
	runnerCredential, err := uc.credentialResolver(ctx, host.CredentialID)
	if err != nil {
		uc.failBinlogArchiveJob(ctx, stream, job, fmt.Errorf("解析 Runner 凭据失败: %w", err))
		return
	}
	started := time.Now()
	job.Status = DatabaseRunnerJobStatusRunning
	job.StartedAt = &started
	job.HeartbeatAt = &started
	_ = uc.runnerJobRepo.Update(ctx, job)

	script, err := buildMySQLBinlogArchiveOnceScript(mysqlBinlogArchiveScriptInput{
		WorkDir:          host.WorkDir,
		StorageMountPath: host.StorageMountPath,
		StreamID:         stream.ID,
		DBHost:           instance.Host,
		DBPort:           instance.Port,
		DBUser:           dbCredential.Username,
		DBPassword:       dbCredential.Password,
		BinlogFile:       fileName,
	})
	if err != nil {
		finished := time.Now()
		applyBinlogArchiveFailure(stream, job, started, finished, 1, err)
		_ = uc.runnerJobRepo.Update(ctx, job)
		_ = uc.logArchiveStreamRepo.Update(ctx, stream)
		return
	}
	stdout, stderr, exitCode, runErr := executeSSHRunnerScript(ctx, host.Host, host.Port, runnerCredential, script, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute)
	finished := time.Now()
	output := parseMySQLBinlogArchiveOutput(stdout, started, finished)
	if output.FileSize == 0 {
		output.FileSize = fileSize
	}
	if output.FileName == "" {
		output.FileName = fileName
	}
	if output.StoragePath != "" {
		output.StorageURI = fmt.Sprintf("runner://runner-host-%d%s", host.ID, output.StoragePath)
	}
	result := runnerBinlogArchiveResult{
		RunnerHostID:   host.ID,
		RunnerID:       runnerIDForHost(host),
		StreamID:       stream.ID,
		SourceInstance: sourceInstanceID,
		Binlog:         output,
		Stdout:         trimText(stdout, maxRunnerOutputLength),
		Stderr:         trimText(stderr, maxRunnerOutputLength),
		ExitCode:       exitCode,
		StartedAt:      started.Format("2006-01-02 15:04:05"),
		FinishedAt:     finished.Format("2006-01-02 15:04:05"),
		DurationMs:     finished.Sub(started).Milliseconds(),
	}
	resultJSON, _ := json.Marshal(result)
	job.ResultJSON = trimText(string(resultJSON), maxRunnerJSONLength)
	job.ExitCode = exitCode
	job.FinishedAt = &finished
	job.DurationMs = result.DurationMs
	job.HeartbeatAt = &finished
	if runErr != nil {
		applyBinlogArchiveFailure(stream, job, started, finished, exitCode, runErr)
		_ = uc.runnerJobRepo.Update(ctx, job)
		_ = uc.logArchiveStreamRepo.Update(ctx, stream)
		return
	}
	firstEvent := parseMySQLBinlogEventTime(output.FirstEventLine)
	lastEvent := parseMySQLBinlogEventTime(output.LastEventLine)
	if firstEvent == nil {
		firstEvent = &started
	}
	if lastEvent == nil {
		lastEvent = &finished
	}
	if lastEvent.Before(*firstEvent) {
		lastEvent = firstEvent
	}
	archive := &DatabaseLogArchive{
		StreamID:         stream.ID,
		InstanceID:       stream.InstanceID,
		SourceInstanceID: sourceInstanceID,
		Engine:           stream.Engine,
		ArchiveType:      DatabaseArchiveTypeBinlog,
		FileName:         fileName,
		StorageURI:       output.StorageURI,
		FileSize:         output.FileSize,
		ChecksumSHA256:   output.ChecksumSHA256,
		FirstEventTime:   firstEvent,
		LastEventTime:    lastEvent,
		Status:           DatabaseLogArchiveStatusArchived,
		ArchivedAt:       &finished,
		StartPos:         4,
		EndPos:           firstNonZeroInt64(fileSize, output.FileSize),
		PreviousFileName: previous,
		NextFileName:     next,
	}
	if archive.StorageURI == "" {
		archive.StorageURI = fmt.Sprintf("runner://runner-host-%d/binlog/stream-%d/%s", host.ID, stream.ID, fileName)
	}
	if err := uc.logArchiveRepo.Create(ctx, archive); err != nil {
		applyBinlogArchiveFailure(stream, job, started, finished, 1, fmt.Errorf("登记日志归档失败: %w", err))
		_ = uc.runnerJobRepo.Update(ctx, job)
		_ = uc.logArchiveStreamRepo.Update(ctx, stream)
		return
	}
	job.Status = DatabaseRunnerJobStatusSuccess
	job.ErrorMessage = ""
	stream.Status = DatabaseLogArchiveStreamStatusRunning
	stream.LastError = ""
	stream.LastArchivedAt = &finished
	stream.LastArchiveName = fileName
	_ = uc.runnerJobRepo.Update(ctx, job)
	_ = uc.logArchiveStreamRepo.Update(ctx, stream)
}

func (uc *UseCase) failBinlogArchiveJob(ctx context.Context, stream *DatabaseLogArchiveStream, job *DatabaseRunnerJob, err error) {
	now := time.Now()
	if job != nil {
		job.Status = DatabaseRunnerJobStatusFailed
		job.ExitCode = 1
		job.ErrorMessage = trimText(err.Error(), 1000)
		job.FinishedAt = &now
		job.HeartbeatAt = &now
		_ = uc.runnerJobRepo.Update(ctx, job)
	}
	if stream != nil {
		stream.Status = DatabaseLogArchiveStreamStatusDegraded
		stream.LastError = trimText(err.Error(), 1000)
		_ = uc.logArchiveStreamRepo.Update(ctx, stream)
	}
}

func applyBinlogArchiveFailure(stream *DatabaseLogArchiveStream, job *DatabaseRunnerJob, started, finished time.Time, exitCode int, runErr error) {
	if job != nil {
		job.Status = DatabaseRunnerJobStatusFailed
		job.ExitCode = exitCode
		job.ErrorMessage = trimText(runErr.Error(), 1000)
		job.FinishedAt = &finished
		job.DurationMs = finished.Sub(started).Milliseconds()
		job.HeartbeatAt = &finished
	}
	if stream != nil {
		stream.Status = DatabaseLogArchiveStreamStatusDegraded
		stream.LastError = trimText(runErr.Error(), 1000)
	}
}

func listMySQLBinaryLogs(ctx context.Context, instance *DatabaseInstance, credential *ConnectionCredential) ([]mysqlBinaryLogFile, error) {
	db, err := openMySQLDB(instance, credential)
	if err != nil {
		return nil, fmt.Errorf("连接日志来源数据库失败: %w", err)
	}
	defer db.Close()
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(queryCtx); err != nil {
		return nil, fmt.Errorf("连接日志来源数据库失败: %w", err)
	}
	for _, query := range []string{"SHOW BINARY LOGS", "SHOW MASTER LOGS"} {
		logs, err := scanMySQLBinaryLogs(queryCtx, db, query)
		if err == nil && len(logs) > 0 {
			return logs, nil
		}
	}
	return nil, fmt.Errorf("无法读取源库 binlog 列表，请确认已开启 log_bin 且账号具备复制/管理权限")
}

func scanMySQLBinaryLogs(ctx context.Context, db *sql.DB, query string) ([]mysqlBinaryLogFile, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := make([]mysqlBinaryLogFile, 0)
	for rows.Next() {
		values := make([]sql.NullString, len(columns))
		dest := make([]any, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		row := map[string]string{}
		for i, column := range columns {
			row[strings.ToLower(strings.TrimSpace(column))] = strings.TrimSpace(values[i].String)
		}
		name := firstNonEmpty(row["log_name"], row["file"])
		sizeRaw := firstNonEmpty(row["file_size"], row["size"])
		size, _ := strconv.ParseInt(sizeRaw, 10, 64)
		if name != "" {
			result = append(result, mysqlBinaryLogFile{Name: name, Size: size})
		}
	}
	return result, rows.Err()
}

func selectMySQLBinlogForArchive(logs []mysqlBinaryLogFile, requestedFile, lastArchived string) (*mysqlBinlogArchiveSelection, error) {
	if len(logs) == 0 {
		return nil, fmt.Errorf("源库没有可归档的 binlog 文件")
	}
	requestedFile = strings.TrimSpace(requestedFile)
	if requestedFile != "" && !isSafeRunnerFileName(requestedFile) {
		return nil, fmt.Errorf("binlog 文件名不合法")
	}
	index := len(logs) - 1
	if requestedFile != "" {
		index = -1
		for i, item := range logs {
			if item.Name == requestedFile {
				index = i
				break
			}
		}
		if index < 0 {
			return nil, fmt.Errorf("指定 binlog 文件不在源库当前列表中")
		}
	} else if lastArchived = strings.TrimSpace(lastArchived); lastArchived != "" {
		for i, item := range logs {
			if item.Name == lastArchived {
				if i+1 < len(logs) {
					index = i + 1
				} else {
					index = i
				}
				break
			}
		}
	}
	selection := &mysqlBinlogArchiveSelection{
		FileName: logs[index].Name,
		FileSize: logs[index].Size,
	}
	if index > 0 {
		selection.Previous = logs[index-1].Name
	}
	if index+1 < len(logs) {
		selection.Next = logs[index+1].Name
	}
	return selection, nil
}

func buildMySQLBinlogArchiveOnceScript(input mysqlBinlogArchiveScriptInput) (string, error) {
	if strings.TrimSpace(input.BinlogFile) == "" || !isSafeRunnerFileName(input.BinlogFile) {
		return "", fmt.Errorf("binlog 文件名不合法")
	}
	if strings.TrimSpace(input.DBHost) == "" || input.DBPort <= 0 {
		return "", fmt.Errorf("数据库连接地址不完整")
	}
	if strings.TrimSpace(input.DBUser) == "" {
		return "", fmt.Errorf("数据库用户名不能为空")
	}
	if input.DBPassword == "" {
		return "", fmt.Errorf("数据库密码不能为空")
	}
	workDir := strings.TrimSpace(input.WorkDir)
	if workDir == "" {
		workDir = defaultRunnerWorkDir
	}
	storageRoot := strings.TrimSpace(input.StorageMountPath)
	if storageRoot == "" {
		storageRoot = workDir
	}
	return strings.Join([]string{
		"set -eu",
		"WORK_DIR=" + shellSingleQuote(workDir),
		"DEST_ROOT=" + shellSingleQuote(storageRoot),
		fmt.Sprintf("STREAM_ID=%d", input.StreamID),
		"BINLOG_FILE=" + shellSingleQuote(input.BinlogFile),
		`DEST_DIR="$DEST_ROOT/binlog/stream-$STREAM_ID"`,
		`mkdir -p "$WORK_DIR" "$DEST_DIR"`,
		`DEFAULTS_FILE="$(mktemp "$WORK_DIR/.mysqlbinlog.XXXXXX.cnf")"`,
		`cleanup() { rm -f "$DEFAULTS_FILE"; }`,
		"trap cleanup EXIT",
		"{",
		"  printf '%s\\n' '[client]'",
		"  printf 'user=%s\\n' " + shellSingleQuote(input.DBUser),
		"  printf 'password=%s\\n' " + shellSingleQuote(input.DBPassword),
		"  printf 'host=%s\\n' " + shellSingleQuote(input.DBHost),
		fmt.Sprintf("  printf 'port=%%s\\n' %s", shellSingleQuote(strconv.Itoa(input.DBPort))),
		"} > \"$DEFAULTS_FILE\"",
		`chmod 600 "$DEFAULTS_FILE"`,
		`MYSQLBINLOG="$(command -v mysqlbinlog || command -v mariadb-binlog || true)"`,
		`if [ -z "$MYSQLBINLOG" ]; then echo "mysqlbinlog/mariadb-binlog not found" >&2; exit 127; fi`,
		`"$MYSQLBINLOG" --defaults-extra-file="$DEFAULTS_FILE" --read-from-remote-server --raw --result-file="$DEST_DIR/" "$BINLOG_FILE"`,
		`OUT_FILE="$DEST_DIR/$BINLOG_FILE"`,
		`test -s "$OUT_FILE"`,
		`SHA256="$(sha256sum "$OUT_FILE" | awk '{print $1}')"`,
		`SIZE="$(wc -c < "$OUT_FILE" | tr -d ' ')"`,
		`FIRST_LINE="$("$MYSQLBINLOG" --base64-output=decode-rows "$OUT_FILE" 2>/dev/null | grep -E '^#[0-9]{6}[[:space:]]+[0-9]{1,2}:[0-9]{2}:[0-9]{2}[[:space:]]+server id' | head -n 1 || true)"`,
		`LAST_LINE="$("$MYSQLBINLOG" --base64-output=decode-rows "$OUT_FILE" 2>/dev/null | grep -E '^#[0-9]{6}[[:space:]]+[0-9]{1,2}:[0-9]{2}:[0-9]{2}[[:space:]]+server id' | tail -n 1 || true)"`,
		`printf 'OPSHUB_BINLOG_FILE=%s\n' "$BINLOG_FILE"`,
		`printf 'OPSHUB_STORAGE_PATH=%s\n' "$OUT_FILE"`,
		`printf 'OPSHUB_FILE_SIZE=%s\n' "$SIZE"`,
		`printf 'OPSHUB_SHA256=%s\n' "$SHA256"`,
		`printf 'OPSHUB_FIRST_EVENT_LINE=%s\n' "$FIRST_LINE"`,
		`printf 'OPSHUB_LAST_EVENT_LINE=%s\n' "$LAST_LINE"`,
	}, "\n"), nil
}

func parseMySQLBinlogArchiveOutput(stdout string, started, finished time.Time) mysqlBinlogArchiveOutput {
	values := parseRunnerKeyValueOutput(stdout)
	output := mysqlBinlogArchiveOutput{
		FileName:       values["OPSHUB_BINLOG_FILE"],
		StoragePath:    values["OPSHUB_STORAGE_PATH"],
		ChecksumSHA256: values["OPSHUB_SHA256"],
		FirstEventLine: values["OPSHUB_FIRST_EVENT_LINE"],
		LastEventLine:  values["OPSHUB_LAST_EVENT_LINE"],
	}
	output.FileSize, _ = strconv.ParseInt(values["OPSHUB_FILE_SIZE"], 10, 64)
	if first := parseMySQLBinlogEventTime(output.FirstEventLine); first != nil {
		output.FirstEventTime = first.Format("2006-01-02 15:04:05")
	} else {
		output.FirstEventTime = started.Format("2006-01-02 15:04:05")
	}
	if last := parseMySQLBinlogEventTime(output.LastEventLine); last != nil {
		output.LastEventTime = last.Format("2006-01-02 15:04:05")
	} else {
		output.LastEventTime = finished.Format("2006-01-02 15:04:05")
	}
	return output
}

func parseRunnerKeyValueOutput(stdout string) map[string]string {
	result := map[string]string{}
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimRight(line, "\r")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if strings.HasPrefix(key, "OPSHUB_") {
			result[key] = strings.TrimSpace(value)
		}
	}
	return result
}

var mysqlBinlogEventLinePattern = regexp.MustCompile(`^#(\d{2})(\d{2})(\d{2})\s+(\d{1,2}):(\d{2}):(\d{2})\s+server id`)

func parseMySQLBinlogEventTime(line string) *time.Time {
	match := mysqlBinlogEventLinePattern.FindStringSubmatch(strings.TrimSpace(line))
	if len(match) != 7 {
		return nil
	}
	yy, _ := strconv.Atoi(match[1])
	month, _ := strconv.Atoi(match[2])
	day, _ := strconv.Atoi(match[3])
	hour, _ := strconv.Atoi(match[4])
	minute, _ := strconv.Atoi(match[5])
	second, _ := strconv.Atoi(match[6])
	year := 2000 + yy
	if yy >= 70 {
		year = 1900 + yy
	}
	value := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.Local)
	return &value
}

func binlogArchiveRequestJSON(stream *DatabaseLogArchiveStream, host *DatabaseRunnerHost, selection *mysqlBinlogArchiveSelection, operator QueryOperator) string {
	payload := map[string]any{
		"streamId":       stream.ID,
		"runnerHostId":   host.ID,
		"runnerType":     host.RunnerType,
		"fileName":       selection.FileName,
		"previousFile":   selection.Previous,
		"nextFile":       selection.Next,
		"allowedCommand": DatabaseRunnerAllowedCommandBinlogArchiveOnce,
		"operatorId":     operator.ID,
		"operatorName":   operator.Username,
	}
	data, _ := json.Marshal(payload)
	return trimText(string(data), maxRunnerJSONLength)
}

func isSafeRunnerFileName(value string) bool {
	if strings.TrimSpace(value) == "" || strings.Contains(value, "/") || strings.Contains(value, "\\") {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
