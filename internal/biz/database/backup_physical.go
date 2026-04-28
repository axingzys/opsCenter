package database

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	BackupEngineXtraBackup80 = "xtrabackup_8_0"
	BackupEngineXtraBackup84 = "xtrabackup_8_4"
	BackupEngineXtraBackup24 = "xtrabackup_2_4"
	BackupEngineMariaDB      = "mariadb_backup"
)

type physicalBackupScopeConfig struct {
	IncrementalBaseDir string   `json:"incrementalBaseDir"`
	ExtraArgs          []string `json:"extraArgs"`
}

type mysqlBackupMetadata struct {
	ServerUUID      string `json:"serverUuid,omitempty"`
	ServerID        string `json:"serverId,omitempty"`
	GTIDMode        string `json:"gtidMode,omitempty"`
	ExecutedGTIDSet string `json:"executedGtidSet,omitempty"`
	PurgedGTIDSet   string `json:"purgedGtidSet,omitempty"`
	BinlogFormat    string `json:"binlogFormat,omitempty"`
	BinlogRowImage  string `json:"binlogRowImage,omitempty"`
	BinlogFile      string `json:"binlogFile,omitempty"`
	BinlogPos       int64  `json:"binlogPos,omitempty"`
	ToolName        string `json:"toolName,omitempty"`
	ToolVersion     string `json:"toolVersion,omitempty"`
}

type physicalBackupBinlogInfo struct {
	File string
	Pos  int64
	GTID string
}

type mysqlBackupToolCompatibility struct {
	Engine      string `json:"engine"`
	ToolName    string `json:"toolName"`
	ToolVersion string `json:"toolVersion"`
	DBFlavor    string `json:"dbFlavor"`
	DBVersion   string `json:"dbVersion"`
	Compatible  bool   `json:"compatible"`
	Message     string `json:"message"`
}

func parsePhysicalBackupScopeConfig(value string) (*physicalBackupScopeConfig, error) {
	result := &physicalBackupScopeConfig{}
	value = strings.TrimSpace(value)
	if value == "" {
		return result, nil
	}
	if err := json.Unmarshal([]byte(value), result); err != nil {
		return nil, fmt.Errorf("物理备份范围配置 JSON 格式错误: %w", err)
	}
	cleaned := make([]string, 0, len(result.ExtraArgs))
	for _, arg := range result.ExtraArgs {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		if strings.Contains(strings.ToLower(arg), "password") || strings.Contains(arg, "=") && strings.Contains(strings.ToLower(arg), "secret") {
			return nil, fmt.Errorf("物理备份 extraArgs 不允许包含密码或密钥")
		}
		cleaned = append(cleaned, arg)
	}
	result.ExtraArgs = cleaned
	return result, nil
}

func normalizeMySQLPhysicalBackupEngine(value, dbType, dbVersion string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, ".", "_")
	switch value {
	case "xtrabackup80", "xtrabackup_80", "percona_xtrabackup_8_0", "xtrabackup_8_0":
		return BackupEngineXtraBackup80
	case "xtrabackup84", "xtrabackup_84", "percona_xtrabackup_8_4", "xtrabackup_8_4":
		return BackupEngineXtraBackup84
	case "xtrabackup24", "xtrabackup_24", "percona_xtrabackup_2_4", "xtrabackup_2_4":
		return BackupEngineXtraBackup24
	case "mariabackup", "mariadb_backup", "mariadb_backup_latest":
		return BackupEngineMariaDB
	}
	if normalizeDBType(dbType) == DBTypeMariaDB {
		return BackupEngineMariaDB
	}
	major, minor := parseMajorMinorVersion(dbVersion)
	if major == 5 {
		return BackupEngineXtraBackup24
	}
	if major == 8 && minor >= 4 {
		return BackupEngineXtraBackup84
	}
	return BackupEngineXtraBackup80
}

func mysqlPhysicalBackupCommand(engine, dbType string) string {
	engine = normalizeMySQLPhysicalBackupEngine(engine, dbType, "")
	switch engine {
	case BackupEngineMariaDB:
		return "mariadb-backup"
	case BackupEngineXtraBackup24, BackupEngineXtraBackup80, BackupEngineXtraBackup84:
		return "xtrabackup"
	default:
		return ""
	}
}

func mysqlPhysicalBackupToolName(engine, dbType string) string {
	return mysqlPhysicalBackupCommand(engine, dbType)
}

func validateMySQLPhysicalBackupCompatibility(ctx context.Context, instance *DatabaseInstance, credential *ConnectionCredential, backupEngine string) (*mysqlBackupToolCompatibility, error) {
	if instance == nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	dbType := normalizeDBType(instance.DBType)
	if dbType != DBTypeMySQL && dbType != DBTypeMariaDB {
		return nil, fmt.Errorf("%s 暂不支持物理备份任务", DBTypeText(instance.DBType))
	}
	version := strings.TrimSpace(instance.Version)
	if version == "" && credential != nil {
		if db, err := openMySQLDB(instance, credential); err == nil {
			queryCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
			version, _ = readMySQLCompatibleVersion(queryCtx, db, instance.DBType)
			cancel()
			_ = db.Close()
		}
	}
	engine := normalizeMySQLPhysicalBackupEngine(backupEngine, dbType, version)
	toolName := mysqlPhysicalBackupToolName(engine, dbType)
	result := &mysqlBackupToolCompatibility{
		Engine:    engine,
		ToolName:  toolName,
		DBFlavor:  DBTypeText(dbType),
		DBVersion: version,
	}
	if toolName == "" {
		result.Compatible = false
		result.Message = "未选择支持的物理备份工具"
		return result, errors.New(result.Message)
	}
	toolVersion := readBackupToolVersion(ctx, toolName)
	result.ToolVersion = toolVersion

	if dbType == DBTypeMariaDB {
		if engine != BackupEngineMariaDB {
			result.Message = "MariaDB 物理备份默认且仅推荐使用 mariadb-backup"
			return result, errors.New(result.Message)
		}
		result.Compatible = true
		result.Message = "MariaDB 使用 mariadb-backup"
		return result, nil
	}
	major, minor := parseMajorMinorVersion(version)
	switch {
	case major == 5:
		if engine != BackupEngineXtraBackup24 {
			result.Message = "MySQL 5.7 需要 XtraBackup 2.4 legacy"
			return result, errors.New(result.Message)
		}
	case major == 8 && minor < 4:
		if engine != BackupEngineXtraBackup80 {
			result.Message = "MySQL 8.0.x 需要 XtraBackup 8.0，不允许选择 XtraBackup 8.4"
			return result, errors.New(result.Message)
		}
	case major == 8 && minor >= 4:
		if engine != BackupEngineXtraBackup84 {
			result.Message = "MySQL 8.4.x 需要 XtraBackup 8.4，不允许混用 XtraBackup 8.0"
			return result, errors.New(result.Message)
		}
	case major >= 9:
		result.Message = "MySQL 9.x 暂不默认推荐 XtraBackup，请使用 external/enterprise 链路登记"
		return result, errors.New(result.Message)
	default:
		if version == "" {
			result.Message = "无法识别 MySQL 版本，请先测试连接或同步实例版本"
			return result, errors.New(result.Message)
		}
	}
	result.Compatible = true
	result.Message = "物理备份工具与数据库版本匹配"
	return result, nil
}

func readBackupToolVersion(ctx context.Context, command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	commandPath, err := backupCommandLookPath(command)
	if err != nil {
		return ""
	}
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(runCtx, commandPath, "--version").CombinedOutput()
	if err != nil {
		return ""
	}
	return trimText(strings.TrimSpace(strings.ReplaceAll(string(out), "\n", " ")), 120)
}

func collectMySQLBackupMetadata(ctx context.Context, instance *DatabaseInstance, credential *ConnectionCredential, toolName, toolVersion string) (*mysqlBackupMetadata, error) {
	db, err := openMySQLDB(instance, credential)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	queryCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	if err := db.PingContext(queryCtx); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	meta := &mysqlBackupMetadata{
		ServerUUID:      firstNonEmptyQueryValue(queryCtx, db, "SELECT @@server_uuid"),
		ServerID:        firstNonEmptyQueryValue(queryCtx, db, "SELECT @@server_id"),
		GTIDMode:        firstNonEmptyQueryValue(queryCtx, db, "SELECT @@gtid_mode"),
		ExecutedGTIDSet: firstNonEmptyQueryValue(queryCtx, db, "SELECT @@global.gtid_executed"),
		PurgedGTIDSet:   firstNonEmptyQueryValue(queryCtx, db, "SELECT @@global.gtid_purged"),
		BinlogFormat:    firstNonEmptyQueryValue(queryCtx, db, "SELECT @@binlog_format"),
		BinlogRowImage:  firstNonEmptyQueryValue(queryCtx, db, "SELECT @@binlog_row_image"),
		ToolName:        trimText(toolName, 60),
		ToolVersion:     trimText(toolVersion, 120),
	}
	if normalizeDBType(instance.DBType) == DBTypeMariaDB {
		meta.ServerUUID = firstNonEmpty(meta.ServerUUID, firstNonEmptyQueryValue(queryCtx, db, "SELECT @@server_uid"))
		meta.GTIDMode = firstNonEmpty(meta.GTIDMode, "mariadb")
		meta.ExecutedGTIDSet = firstNonEmpty(meta.ExecutedGTIDSet, firstNonEmptyQueryValue(queryCtx, db, "SELECT @@global.gtid_current_pos"), firstNonEmptyQueryValue(queryCtx, db, "SELECT @@global.gtid_binlog_pos"))
		meta.PurgedGTIDSet = firstNonEmpty(meta.PurgedGTIDSet, firstNonEmptyQueryValue(queryCtx, db, "SELECT @@global.gtid_binlog_pos"))
	}
	file, pos, gtid := readMySQLBinaryLogStatus(queryCtx, db)
	meta.BinlogFile = file
	meta.BinlogPos = pos
	meta.ExecutedGTIDSet = firstNonEmpty(meta.ExecutedGTIDSet, gtid)
	return meta, nil
}

func (uc *UseCase) enrichPhysicalBackupRecordMetadata(ctx context.Context, record *DatabaseBackupRecord, instance *DatabaseInstance, credential *ConnectionCredential, spec *backupCommandSpec, startedAt, finishedAt time.Time, outputPath, checksum string) {
	if record == nil || instance == nil || normalizeBackupMethod(record.BackupMethod) != DatabaseBackupMethodPhysical {
		return
	}
	dbType := normalizeDBType(instance.DBType)
	if dbType != DBTypeMySQL && dbType != DBTypeMariaDB {
		return
	}
	toolName := ""
	if spec != nil {
		toolName = firstNonEmpty(spec.ToolName, backupToolNameForSpec(spec))
	}
	if toolName == "" {
		toolName = mysqlPhysicalBackupToolName(record.BackupEngine, dbType)
	}
	toolVersion := readBackupToolVersion(ctx, toolName)
	meta, err := collectMySQLBackupMetadata(ctx, instance, credential, toolName, toolVersion)
	if err == nil && meta != nil {
		record.ServerUUID = meta.ServerUUID
		record.ServerID = meta.ServerID
		record.GTIDMode = meta.GTIDMode
		record.ExecutedGTIDSet = meta.ExecutedGTIDSet
		record.PurgedGTIDSet = meta.PurgedGTIDSet
		record.BinlogFormat = meta.BinlogFormat
		record.BinlogRowImage = meta.BinlogRowImage
		record.BackupBinlogFile = meta.BinlogFile
		record.BackupBinlogPos = meta.BinlogPos
		record.BackupGTIDSet = firstNonEmpty(meta.ExecutedGTIDSet, meta.PurgedGTIDSet)
		record.ToolName = firstNonEmpty(meta.ToolName, toolName)
		record.ToolVersion = firstNonEmpty(meta.ToolVersion, toolVersion)
	}
	if binlogInfo := readPhysicalBackupBinlogInfo(outputPath); binlogInfo != nil {
		record.BackupBinlogFile = firstNonEmpty(binlogInfo.File, record.BackupBinlogFile)
		if binlogInfo.Pos > 0 {
			record.BackupBinlogPos = binlogInfo.Pos
		}
		record.BackupGTIDSet = firstNonEmpty(binlogInfo.GTID, record.BackupGTIDSet)
		record.ExecutedGTIDSet = firstNonEmpty(record.ExecutedGTIDSet, binlogInfo.GTID)
	}
	record.StorageURI = "local://" + strings.TrimSpace(outputPath)
	record.ManifestJSON = buildMySQLPhysicalBackupManifest(record, instance, startedAt, finishedAt, checksum)
	record.PrepareStatus = "pending_prepare"
	if record.BackupLevel == DatabaseBackupLevelFull {
		record.BaseRecordID = record.ID
	}
	if uc.backupRecordRepo != nil {
		_ = uc.backupRecordRepo.Update(ctx, record)
	}
	uc.collectBinlogMetadataForPhysicalBackup(ctx, record, instance, startedAt, finishedAt)
}

func buildMySQLPhysicalBackupManifest(record *DatabaseBackupRecord, instance *DatabaseInstance, startedAt, finishedAt time.Time, checksum string) string {
	payload := map[string]any{
		"engine":            record.BackupEngine,
		"method":            record.BackupMethod,
		"level":             record.BackupLevel,
		"dbType":            instance.DBType,
		"serverUuid":        record.ServerUUID,
		"serverId":          record.ServerID,
		"backupBinlogFile":  record.BackupBinlogFile,
		"backupBinlogPos":   record.BackupBinlogPos,
		"backupGtidSet":     record.BackupGTIDSet,
		"executedGtidSet":   record.ExecutedGTIDSet,
		"purgedGtidSet":     record.PurgedGTIDSet,
		"binlogFormat":      record.BinlogFormat,
		"binlogRowImage":    record.BinlogRowImage,
		"checksumSha256":    checksum,
		"startedAt":         startedAt.Format("2006-01-02 15:04:05"),
		"finishedAt":        finishedAt.Format("2006-01-02 15:04:05"),
		"recoveryProofType": "physical_backup_metadata",
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func (uc *UseCase) collectBinlogMetadataForPhysicalBackup(ctx context.Context, record *DatabaseBackupRecord, instance *DatabaseInstance, startedAt, finishedAt time.Time) {
	if uc.logArchiveStreamRepo == nil || uc.logArchiveRepo == nil || record == nil || instance == nil {
		return
	}
	if strings.TrimSpace(record.BackupBinlogFile) == "" {
		return
	}
	stream := &DatabaseLogArchiveStream{
		InstanceID:       record.InstanceID,
		SourceInstanceID: record.SourceInstanceID,
		Engine:           normalizeDBType(instance.DBType),
		ArchiveType:      DatabaseArchiveTypeBinlog,
		ArchiveMode:      DatabaseArchiveModePolling,
		ArchiveEngine:    "mysqlbinlog",
		RPOTargetSeconds: 300,
		RetentionDays:    30,
		Enabled:          true,
		Status:           DatabaseLogArchiveStreamStatusRunning,
		DesiredState:     DatabaseLogArchiveDesiredStateRunning,
		DaemonStatus:     DatabaseLogArchiveDaemonStatusRunning,
		LastArchivedAt:   &finishedAt,
		LastArchiveName:  record.BackupBinlogFile,
		ConfigJSON:       `{"source":"physical_backup_metadata"}`,
	}
	if normalizeDBType(instance.DBType) == DBTypeMariaDB {
		stream.ArchiveEngine = "mariadb-binlog"
	}
	if err := uc.logArchiveStreamRepo.Create(ctx, stream); err != nil {
		return
	}
	item := &DatabaseLogArchive{
		StreamID:         stream.ID,
		InstanceID:       record.InstanceID,
		SourceInstanceID: record.SourceInstanceID,
		Engine:           normalizeDBType(instance.DBType),
		ArchiveType:      DatabaseArchiveTypeBinlog,
		FileName:         record.BackupBinlogFile,
		StorageURI:       "metadata://binlog/" + record.BackupBinlogFile,
		ChecksumSHA256:   "",
		FirstEventTime:   &startedAt,
		LastEventTime:    &finishedAt,
		Status:           DatabaseLogArchiveStatusArchived,
		ArchivedAt:       &finishedAt,
		ServerUUID:       record.ServerUUID,
		ServerID:         record.ServerID,
		StartPos:         record.BackupBinlogPos,
		EndPos:           record.BackupBinlogPos,
		StartGTIDSet:     record.BackupGTIDSet,
		EndGTIDSet:       record.ExecutedGTIDSet,
	}
	_ = uc.logArchiveRepo.Create(ctx, item)
}

func readPhysicalBackupBinlogInfo(archivePath string) *physicalBackupBinlogInfo {
	archivePath = strings.TrimSpace(archivePath)
	if archivePath == "" {
		return nil
	}
	file, err := os.Open(archivePath)
	if err != nil {
		return nil
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return nil
		}
		if header == nil || header.FileInfo().IsDir() {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(header.Name))
		if !strings.HasSuffix(name, "xtrabackup_binlog_info") && !strings.HasSuffix(name, "mariadb_backup_binlog_info") {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(tarReader, 64*1024))
		if err != nil {
			return nil
		}
		return parsePhysicalBackupBinlogInfo(string(data))
	}
}

func parsePhysicalBackupBinlogInfo(value string) *physicalBackupBinlogInfo {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) < 2 {
		return nil
	}
	pos, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		pos = 0
	}
	info := &physicalBackupBinlogInfo{
		File: fields[0],
		Pos:  pos,
	}
	if len(fields) > 2 {
		info.GTID = strings.Join(fields[2:], " ")
	}
	return info
}

func readMySQLBinaryLogStatus(ctx context.Context, db *sql.DB) (string, int64, string) {
	for _, query := range []string{"SHOW BINARY LOG STATUS", "SHOW MASTER STATUS"} {
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			continue
		}
		file, pos, gtid := scanBinaryLogStatusRows(rows)
		_ = rows.Close()
		if file != "" {
			return file, pos, gtid
		}
	}
	return "", 0, ""
}

func scanBinaryLogStatusRows(rows *sql.Rows) (string, int64, string) {
	if rows == nil || !rows.Next() {
		return "", 0, ""
	}
	columns, err := rows.Columns()
	if err != nil {
		return "", 0, ""
	}
	values := make([]sql.NullString, len(columns))
	dest := make([]any, len(columns))
	for i := range values {
		dest[i] = &values[i]
	}
	if err := rows.Scan(dest...); err != nil {
		return "", 0, ""
	}
	data := make(map[string]string)
	for i, column := range columns {
		data[strings.ToLower(strings.TrimSpace(column))] = strings.TrimSpace(values[i].String)
	}
	pos, _ := strconv.ParseInt(firstNonEmpty(data["position"], data["pos"]), 10, 64)
	return firstNonEmpty(data["file"], data["log_name"]), pos, firstNonEmpty(data["executed_gtid_set"], data["gtid_binlog_pos"])
}

func parseMajorMinorVersion(value string) (int, int) {
	value = strings.ToLower(strings.TrimSpace(value))
	re := regexp.MustCompile(`(\d+)\.(\d+)`)
	match := re.FindStringSubmatch(value)
	if len(match) < 3 {
		return 0, 0
	}
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	return major, minor
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
