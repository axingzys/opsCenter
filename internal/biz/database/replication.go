package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

const replicationCheckTimeout = 15 * time.Second

func (uc *UseCase) ListInstanceReplicas(ctx context.Context, req *DatabaseInstanceReplicaListRequest) ([]*DatabaseInstanceReplicaVO, int64, error) {
	if uc.instanceReplicaRepo == nil {
		return nil, 0, fmt.Errorf("副本关系仓储未配置")
	}
	list, total, err := uc.instanceReplicaRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	return uc.toInstanceReplicaVOList(ctx, list), total, nil
}

func (uc *UseCase) ListReplicationChecks(ctx context.Context, req *DatabaseReplicationCheckListRequest) ([]*DatabaseReplicationCheckVO, int64, error) {
	if uc.replicationCheckRepo == nil {
		return nil, 0, fmt.Errorf("副本状态仓储未配置")
	}
	list, total, err := uc.replicationCheckRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	return uc.toReplicationCheckVOList(ctx, list), total, nil
}

func (uc *UseCase) GetInstanceReplicationStatus(ctx context.Context, instanceID uint) (*DatabaseReplicationStatusVO, error) {
	if instanceID == 0 {
		return nil, fmt.Errorf("请选择数据库实例")
	}
	if uc.instanceReplicaRepo == nil || uc.replicationCheckRepo == nil {
		return nil, fmt.Errorf("副本治理仓储未配置")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	replicas, _, err := uc.instanceReplicaRepo.List(ctx, &DatabaseInstanceReplicaListRequest{
		Page:       1,
		PageSize:   200,
		InstanceID: instanceID,
	})
	if err != nil {
		return nil, err
	}
	checks, _, err := uc.replicationCheckRepo.List(ctx, &DatabaseReplicationCheckListRequest{
		Page:       1,
		PageSize:   20,
		InstanceID: instanceID,
	})
	if err != nil {
		return nil, err
	}
	last, err := uc.replicationCheckRepo.LatestByInstanceID(ctx, instanceID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &DatabaseReplicationStatusVO{
		Instance: uc.toInstanceVO(instance),
		Replicas: uc.toInstanceReplicaVOList(ctx, replicas),
		Checks:   uc.toReplicationCheckVOList(ctx, checks),
		LastCheck: func() *DatabaseReplicationCheckVO {
			if last == nil {
				return nil
			}
			return uc.toReplicationCheckVO(ctx, last)
		}(),
		Message: "副本治理状态只读展示，未执行 pause/resume/promote/failover 动作",
	}, nil
}

func (uc *UseCase) CheckInstanceReplication(ctx context.Context, instanceID uint, operator QueryOperator) (*DatabaseReplicationCheckVO, error) {
	if instanceID == 0 {
		return nil, fmt.Errorf("请选择数据库实例")
	}
	if uc.instanceReplicaRepo == nil || uc.replicationCheckRepo == nil {
		return nil, fmt.Errorf("副本治理仓储未配置")
	}
	item, err := uc.getEnabledQueryInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	credential, err := uc.credentialResolver(ctx, item.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("凭据不存在")
	}
	checkCtx, cancel := context.WithTimeout(ctx, replicationCheckTimeout)
	defer cancel()

	now := time.Now()
	var check *DatabaseReplicationCheck
	switch normalizeDBType(item.DBType) {
	case DBTypeMySQL, DBTypeMariaDB:
		check, err = uc.collectMySQLReplicationStatus(checkCtx, item, credential, now)
	case DBTypePostgreSQL:
		check, err = uc.collectPostgreSQLReplicationStatus(checkCtx, item, credential, now)
	default:
		err = fmt.Errorf("当前仅支持 MySQL / MariaDB / PostgreSQL 副本状态采集")
		check = &DatabaseReplicationCheck{
			InstanceID:    item.ID,
			Engine:        normalizeDBType(item.DBType),
			RoleDetected:  DatabaseReplicationRoleUnknown,
			HealthStatus:  DatabaseReplicaHealthUnknown,
			CheckedAt:     &now,
			ErrorMessage:  trimText(err.Error(), 1000),
			RawStatusJSON: marshalReplicaJSON(map[string]any{"error": err.Error()}),
		}
	}
	if err != nil && check == nil {
		check = &DatabaseReplicationCheck{
			InstanceID:    item.ID,
			Engine:        normalizeDBType(item.DBType),
			RoleDetected:  DatabaseReplicationRoleUnknown,
			HealthStatus:  DatabaseReplicaHealthUnknown,
			CheckedAt:     &now,
			ErrorMessage:  trimText(err.Error(), 1000),
			RawStatusJSON: marshalReplicaJSON(map[string]any{"error": err.Error()}),
		}
	}
	if check == nil {
		return nil, fmt.Errorf("副本状态采集结果为空")
	}
	if saveErr := uc.replicationCheckRepo.Create(ctx, check); saveErr != nil {
		return nil, saveErr
	}
	if err == nil && isReplicaDetected(check.RoleDetected) {
		replica := uc.buildReplicaRelationFromCheck(ctx, item, check)
		if replica != nil {
			replica.LastCheckID = check.ID
			savedReplica, upsertErr := uc.instanceReplicaRepo.UpsertByReplicaInstance(ctx, replica)
			if upsertErr != nil {
				check.HealthStatus = DatabaseReplicaHealthWarning
				check.ErrorMessage = trimText("副本关系保存失败: "+upsertErr.Error(), 1000)
				_ = uc.replicationCheckRepo.Update(ctx, check)
			} else if savedReplica != nil {
				check.ReplicaID = savedReplica.ID
				_ = uc.replicationCheckRepo.Update(ctx, check)
			}
		}
	}
	uc.recordReplicationAudit(ctx, item, check, operator, err)
	if err != nil {
		return uc.toReplicationCheckVO(ctx, check), err
	}
	return uc.toReplicationCheckVO(ctx, check), nil
}

func (uc *UseCase) collectMySQLReplicationStatus(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, now time.Time) (*DatabaseReplicationCheck, error) {
	db, err := openMySQLDB(item, credential)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	variables := collectMySQLTopologyVariables(ctx, db)

	row, ok, queryErr := querySingleRowMap(ctx, db, "SHOW REPLICA STATUS")
	if queryErr != nil {
		row, ok, queryErr = querySingleRowMap(ctx, db, "SHOW SLAVE STATUS")
	}
	check := &DatabaseReplicationCheck{
		InstanceID:            item.ID,
		Engine:                normalizeDBType(item.DBType),
		RoleDetected:          DatabaseReplicationRolePrimary,
		SecondsBehindSource:   -1,
		RemainingDelaySeconds: -1,
		HealthStatus:          DatabaseReplicaHealthHealthy,
		CheckedAt:             &now,
	}
	if queryErr != nil {
		check.RoleDetected = DatabaseReplicationRoleUnknown
		check.HealthStatus = DatabaseReplicaHealthUnknown
		check.ErrorMessage = trimText(queryErr.Error(), 1000)
		check.RawStatusJSON = marshalReplicaJSON(mysqlTopologyRawMap(map[string]any{"error": queryErr.Error()}, variables))
		return check, queryErr
	}
	if !ok {
		raw := map[string]any{
			"role":    "primary_or_not_configured_as_replica",
			"message": "SHOW REPLICA/SLAVE STATUS returned no rows",
		}
		if reported, reportedErr := collectMySQLReportedReplicas(ctx, db); reportedErr != nil {
			raw["reported_replicas_error"] = reportedErr.Error()
		} else if len(reported) > 0 {
			raw["reported_replicas"] = redactReplicaStatusRows(reported)
		}
		riskFlags := mysqlTopologyVariableRiskFlags(DatabaseReplicationRolePrimary, variables)
		check.HealthStatus = replicaHealthFromRiskFlags(riskFlags)
		check.RiskFlagsJSON = marshalReplicaJSON(riskFlags)
		check.RawStatusJSON = marshalReplicaJSON(mysqlTopologyRawMap(raw, variables))
		if len(riskFlags) > 0 {
			check.ErrorMessage = trimText(strings.Join(riskFlags, "；"), 1000)
		}
		return check, nil
	}

	sourceHost := firstNonEmpty(row["Source_Host"], row["Master_Host"])
	sourcePort := parseInt(firstNonEmpty(row["Source_Port"], row["Master_Port"]), 0)
	sourceInstanceID := uc.matchRegisteredInstance(ctx, sourceHost, sourcePort, item.ID)
	secondsBehind := parseNullableInt(firstNonEmpty(row["Seconds_Behind_Source"], row["Seconds_Behind_Master"]), -1)
	configuredDelay := parseInt(row["SQL_Delay"], 0)
	remainingDelay := parseNullableInt(row["SQL_Remaining_Delay"], -1)
	ioRunning := firstNonEmpty(row["Replica_IO_Running"], row["Slave_IO_Running"])
	sqlRunning := firstNonEmpty(row["Replica_SQL_Running"], row["Slave_SQL_Running"])
	riskFlags := mysqlReplicaRiskFlags(ioRunning, sqlRunning, secondsBehind, configuredDelay, remainingDelay, sourceInstanceID)
	riskFlags = append(riskFlags, mysqlTopologyVariableRiskFlags(DatabaseReplicationRoleReplica, variables)...)

	check.RoleDetected = DatabaseReplicationRoleReplica
	check.SourceInstanceID = sourceInstanceID
	check.ReplicaIORunning = trimText(ioRunning, 30)
	check.ReplicaSQLRunning = trimText(sqlRunning, 30)
	check.SecondsBehindSource = secondsBehind
	check.ConfiguredDelaySeconds = configuredDelay
	check.RemainingDelaySeconds = remainingDelay
	check.HealthStatus = replicaHealthFromRiskFlags(riskFlags)
	check.RiskFlagsJSON = marshalReplicaJSON(riskFlags)
	check.RawStatusJSON = marshalReplicaJSON(mysqlTopologyRawStringMap(redactReplicaStatusMap(row), variables))
	if len(riskFlags) > 0 {
		check.ErrorMessage = trimText(strings.Join(riskFlags, "；"), 1000)
	}
	return check, nil
}

func collectMySQLTopologyVariables(ctx context.Context, db *sql.DB) map[string]string {
	result := make(map[string]string)
	queries := []struct {
		key   string
		query string
	}{
		{key: "server_id", query: "SELECT @@server_id"},
		{key: "server_uuid", query: "SELECT @@server_uuid"},
		{key: "version", query: "SELECT @@version"},
		{key: "read_only", query: "SELECT @@read_only"},
		{key: "super_read_only", query: "SELECT @@super_read_only"},
		{key: "log_bin", query: "SELECT @@log_bin"},
		{key: "gtid_mode", query: "SELECT @@gtid_mode"},
		{key: "binlog_format", query: "SELECT @@binlog_format"},
		{key: "binlog_row_image", query: "SELECT @@binlog_row_image"},
	}
	for _, item := range queries {
		if value := firstNonEmptyQueryValue(ctx, db, item.query); value != "" {
			result[item.key] = value
		}
	}
	if result["server_uuid"] == "" {
		if value := firstNonEmptyQueryValue(ctx, db, "SELECT @@server_uid"); value != "" {
			result["server_uuid"] = value
			result["server_uid"] = value
		}
	}
	return result
}

func collectMySQLReportedReplicas(ctx context.Context, db *sql.DB) ([]map[string]string, error) {
	rows, err := queryRowsMap(ctx, db, "SHOW REPLICAS")
	if err == nil {
		return rows, nil
	}
	return queryRowsMap(ctx, db, "SHOW SLAVE HOSTS")
}

func mysqlTopologyRawMap(raw map[string]any, variables map[string]string) map[string]any {
	if raw == nil {
		raw = map[string]any{}
	}
	for key, value := range variables {
		if strings.TrimSpace(value) != "" {
			raw[key] = value
		}
	}
	return raw
}

func mysqlTopologyRawStringMap(raw map[string]string, variables map[string]string) map[string]string {
	if raw == nil {
		raw = map[string]string{}
	}
	for key, value := range variables {
		if strings.TrimSpace(value) != "" {
			raw[key] = value
		}
	}
	return raw
}

func (uc *UseCase) collectPostgreSQLReplicationStatus(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, now time.Time) (*DatabaseReplicationCheck, error) {
	db, err := openPostgreSQLDB(item, credential)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	check := &DatabaseReplicationCheck{
		InstanceID:            item.ID,
		Engine:                normalizeDBType(item.DBType),
		RoleDetected:          DatabaseReplicationRoleUnknown,
		SecondsBehindSource:   -1,
		RemainingDelaySeconds: -1,
		HealthStatus:          DatabaseReplicaHealthUnknown,
		CheckedAt:             &now,
	}

	inRecovery := false
	if err := db.QueryRowContext(ctx, "SELECT pg_is_in_recovery()").Scan(&inRecovery); err != nil {
		check.ErrorMessage = trimText(err.Error(), 1000)
		check.RawStatusJSON = marshalReplicaJSON(map[string]any{"error": err.Error()})
		return check, err
	}
	if !inRecovery {
		rows, err := queryRowsMap(ctx, db, `
SELECT
	application_name,
	COALESCE(client_addr::text, '') AS client_addr,
	COALESCE(state, '') AS state,
	COALESCE(sync_state, '') AS sync_state,
	COALESCE(write_lag::text, '') AS write_lag,
	COALESCE(flush_lag::text, '') AS flush_lag,
	COALESCE(replay_lag::text, '') AS replay_lag,
	sent_lsn::text AS sent_lsn,
	write_lsn::text AS write_lsn,
	flush_lsn::text AS flush_lsn,
	replay_lsn::text AS replay_lsn
FROM pg_stat_replication`)
		if err != nil {
			check.RoleDetected = DatabaseReplicationRolePrimary
			check.HealthStatus = DatabaseReplicaHealthWarning
			check.ErrorMessage = trimText(err.Error(), 1000)
			check.RawStatusJSON = marshalReplicaJSON(map[string]any{"role": "primary", "error": err.Error()})
			return check, nil
		}
		check.RoleDetected = DatabaseReplicationRolePrimary
		check.HealthStatus = DatabaseReplicaHealthHealthy
		check.RawStatusJSON = marshalReplicaJSON(map[string]any{"role": "primary", "pg_stat_replication": redactReplicaStatusRows(rows)})
		if len(rows) > 0 {
			check.PGWriteLagMs = pgDurationTextMillis(rows[0]["write_lag"])
			check.PGFlushLagMs = pgDurationTextMillis(rows[0]["flush_lag"])
			check.PGReplayLagMs = pgDurationTextMillis(rows[0]["replay_lag"])
		}
		return check, nil
	}

	raw := map[string]any{"role": "standby"}
	replayPaused := false
	if err := db.QueryRowContext(ctx, "SELECT pg_is_wal_replay_paused()").Scan(&replayPaused); err != nil {
		raw["pg_is_wal_replay_paused_error"] = err.Error()
	} else {
		raw["pg_is_wal_replay_paused"] = replayPaused
	}
	walReceiver, _, receiverErr := querySingleRowMap(ctx, db, postgreSQLWALReceiverStatusQuery())
	if receiverErr != nil {
		raw["pg_stat_wal_receiver_error"] = receiverErr.Error()
	} else {
		raw["pg_stat_wal_receiver"] = redactReplicaStatusMap(walReceiver)
	}

	var receiveLSN, replayLSN sql.NullString
	var replayTs sql.NullTime
	if err := db.QueryRowContext(ctx, "SELECT pg_last_wal_receive_lsn()::text, pg_last_wal_replay_lsn()::text, pg_last_xact_replay_timestamp()").Scan(&receiveLSN, &replayLSN, &replayTs); err != nil {
		raw["standby_replay_error"] = err.Error()
	} else {
		raw["pg_last_wal_receive_lsn"] = receiveLSN.String
		raw["pg_last_wal_replay_lsn"] = replayLSN.String
		if replayTs.Valid {
			raw["pg_last_xact_replay_timestamp"] = replayTs.Time.Format(time.RFC3339)
			check.PGLastXactReplayTimestamp = &replayTs.Time
		}
		check.PGLastWALReplayLSN = replayLSN.String
	}
	delaySeconds, delayRaw, delayErr := readPostgreSQLApplyDelay(ctx, db)
	if delayErr != nil {
		raw["recovery_min_apply_delay_error"] = delayErr.Error()
	} else {
		raw["recovery_min_apply_delay"] = delayRaw
	}
	systemID := firstNonEmptyQueryValue(ctx, db, "SELECT system_identifier::text FROM pg_control_system()")
	if systemID != "" {
		raw["pg_system_identifier"] = systemID
	}
	sourceHost, sourcePort, applicationName := parsePostgreSQLConnInfo(firstNonEmpty(walReceiver["conninfo"], firstNonEmptyQueryValue(ctx, db, "SHOW primary_conninfo")))
	sourceInstanceID := uc.matchRegisteredInstance(ctx, sourceHost, sourcePort, item.ID)
	riskFlags := postgresStandbyRiskFlags(walReceiver["status"], sourceInstanceID, receiverErr, replayPaused)

	check.RoleDetected = DatabaseReplicationRoleStandby
	check.SourceInstanceID = sourceInstanceID
	check.ConfiguredDelaySeconds = delaySeconds
	check.HealthStatus = replicaHealthFromRiskFlags(riskFlags)
	check.RiskFlagsJSON = marshalReplicaJSON(riskFlags)
	check.RawStatusJSON = marshalReplicaJSON(redactReplicaRawMap(raw))
	if len(riskFlags) > 0 {
		check.ErrorMessage = trimText(strings.Join(riskFlags, "；"), 1000)
	}
	_ = applicationName
	return check, nil
}

func postgreSQLWALReceiverStatusQuery() string {
	return `
SELECT
	COALESCE(status, '') AS status,
	COALESCE(receive_start_lsn::text, '') AS receive_start_lsn,
	COALESCE(written_lsn::text, '') AS received_lsn,
	COALESCE(flushed_lsn::text, '') AS flushed_lsn,
	COALESCE(latest_end_lsn::text, '') AS latest_end_lsn,
	COALESCE(latest_end_time::text, '') AS latest_end_time,
	COALESCE(conninfo, '') AS conninfo
FROM pg_stat_wal_receiver
LIMIT 1`
}

func (uc *UseCase) buildReplicaRelationFromCheck(ctx context.Context, item *DatabaseInstance, check *DatabaseReplicationCheck) *DatabaseInstanceReplica {
	if item == nil || check == nil {
		return nil
	}
	replica := &DatabaseInstanceReplica{
		PrimaryInstanceID:      check.SourceInstanceID,
		ReplicaInstanceID:      item.ID,
		Engine:                 check.Engine,
		ConfiguredDelaySeconds: check.ConfiguredDelaySeconds,
		Status:                 check.HealthStatus,
		LastCheckedAt:          check.CheckedAt,
		LastError:              trimText(check.ErrorMessage, 1000),
	}
	switch check.RoleDetected {
	case DatabaseReplicationRoleReplica:
		replica.DiscoverySource = DatabaseReplicaDiscoveryReplicaStatus
		if check.ConfiguredDelaySeconds > 0 {
			replica.ReplicaRole = DatabaseReplicaRoleDelayed
		} else {
			replica.ReplicaRole = DatabaseReplicaRoleRealtime
		}
		raw := decodeReplicaRawMap(check.RawStatusJSON)
		replica.SourceHost = trimText(firstNonEmpty(toString(raw["Source_Host"]), toString(raw["Master_Host"])), 255)
		replica.SourcePort = parseInt(firstNonEmpty(toString(raw["Source_Port"]), toString(raw["Master_Port"])), 0)
		replica.SourceServerUUID = trimText(firstNonEmpty(toString(raw["Source_UUID"]), toString(raw["Master_UUID"])), 120)
	case DatabaseReplicationRoleStandby:
		replica.DiscoverySource = DatabaseReplicaDiscoveryReplicaStatus
		if check.ConfiguredDelaySeconds > 0 {
			replica.ReplicaRole = DatabaseReplicaRoleDelayed
		} else {
			replica.ReplicaRole = DatabaseReplicaRoleStandby
		}
		raw := decodeReplicaRawMap(check.RawStatusJSON)
		if receiver, ok := raw["pg_stat_wal_receiver"].(map[string]any); ok {
			sourceHost, sourcePort, applicationName := parsePostgreSQLConnInfo(toString(receiver["conninfo"]))
			replica.SourceHost = trimText(sourceHost, 255)
			replica.SourcePort = sourcePort
			replica.ApplicationName = trimText(applicationName, 120)
		}
		replica.PGSystemIdentifier = trimText(toString(raw["pg_system_identifier"]), 120)
	default:
		return nil
	}
	return replica
}

func (uc *UseCase) matchRegisteredInstance(ctx context.Context, host string, port int, excludeID uint) uint {
	host = normalizeEndpointHost(host)
	if host == "" || uc.instanceRepo == nil {
		return 0
	}
	instances, err := uc.instanceRepo.ListEnabled(ctx)
	if err != nil {
		return 0
	}
	for _, item := range instances {
		if item == nil || item.ID == excludeID {
			continue
		}
		if normalizeEndpointHost(item.Host) != host {
			continue
		}
		if port > 0 && item.Port != port {
			continue
		}
		return item.ID
	}
	return 0
}

func (uc *UseCase) toInstanceReplicaVOList(ctx context.Context, list []*DatabaseInstanceReplica) []*DatabaseInstanceReplicaVO {
	result := make([]*DatabaseInstanceReplicaVO, 0, len(list))
	for _, item := range list {
		result = append(result, uc.toInstanceReplicaVO(ctx, item))
	}
	return result
}

func (uc *UseCase) toInstanceReplicaVO(ctx context.Context, item *DatabaseInstanceReplica) *DatabaseInstanceReplicaVO {
	if item == nil {
		return nil
	}
	primaryName, primaryEndpoint := uc.instanceNameEndpoint(ctx, item.PrimaryInstanceID)
	replicaName, replicaEndpoint := uc.instanceNameEndpoint(ctx, item.ReplicaInstanceID)
	applyState := uc.latestReplicaApplyState(ctx, item.ReplicaInstanceID)
	return &DatabaseInstanceReplicaVO{
		ID:                     item.ID,
		PrimaryInstanceID:      item.PrimaryInstanceID,
		PrimaryInstanceName:    primaryName,
		PrimaryEndpoint:        primaryEndpoint,
		ReplicaInstanceID:      item.ReplicaInstanceID,
		ReplicaInstanceName:    replicaName,
		ReplicaEndpoint:        replicaEndpoint,
		Engine:                 item.Engine,
		EngineText:             DBTypeText(item.Engine),
		ReplicaRole:            item.ReplicaRole,
		ReplicaRoleText:        ReplicaRoleText(item.ReplicaRole),
		SourceHost:             item.SourceHost,
		SourcePort:             item.SourcePort,
		SourceServerUUID:       item.SourceServerUUID,
		PGSystemIdentifier:     item.PGSystemIdentifier,
		ApplicationName:        item.ApplicationName,
		ConfiguredDelaySeconds: item.ConfiguredDelaySeconds,
		DiscoverySource:        item.DiscoverySource,
		DiscoverySourceText:    ReplicaDiscoverySourceText(item.DiscoverySource),
		Status:                 item.Status,
		StatusText:             ReplicaHealthText(item.Status),
		ApplyState:             applyState,
		ApplyStateText:         ReplicaApplyStateText(applyState),
		ApplyPaused:            applyState == DatabaseReplicaApplyStatePaused,
		LastCheckID:            item.LastCheckID,
		LastCheckedAt:          formatTime(item.LastCheckedAt),
		LastError:              item.LastError,
		CreatedAt:              formatTime(&item.CreatedAt),
		UpdatedAt:              formatTime(&item.UpdatedAt),
	}
}

func (uc *UseCase) latestReplicaApplyState(ctx context.Context, replicaInstanceID uint) string {
	if uc == nil || uc.replicationCheckRepo == nil || replicaInstanceID == 0 {
		return DatabaseReplicaApplyStateUnknown
	}
	check, err := uc.replicationCheckRepo.LatestByInstanceID(ctx, replicaInstanceID)
	if err != nil || check == nil {
		return DatabaseReplicaApplyStateUnknown
	}
	switch check.RoleDetected {
	case DatabaseReplicationRoleReplica:
		status := strings.TrimSpace(check.ReplicaSQLRunning)
		if strings.EqualFold(status, "No") || strings.EqualFold(status, "Stopped") {
			return DatabaseReplicaApplyStatePaused
		}
		if strings.EqualFold(status, "Yes") || strings.EqualFold(status, "Running") {
			return DatabaseReplicaApplyStateRunning
		}
	case DatabaseReplicationRoleStandby:
		raw := decodeReplicaRawMap(check.RawStatusJSON)
		if toBool(raw["pg_is_wal_replay_paused"]) {
			return DatabaseReplicaApplyStatePaused
		}
		return DatabaseReplicaApplyStateRunning
	}
	return DatabaseReplicaApplyStateUnknown
}

func (uc *UseCase) toReplicationCheckVOList(ctx context.Context, list []*DatabaseReplicationCheck) []*DatabaseReplicationCheckVO {
	result := make([]*DatabaseReplicationCheckVO, 0, len(list))
	for _, item := range list {
		result = append(result, uc.toReplicationCheckVO(ctx, item))
	}
	return result
}

func (uc *UseCase) toReplicationCheckVO(ctx context.Context, item *DatabaseReplicationCheck) *DatabaseReplicationCheckVO {
	if item == nil {
		return nil
	}
	instanceName, instanceEndpoint := uc.instanceNameEndpoint(ctx, item.InstanceID)
	sourceName, _ := uc.instanceNameEndpoint(ctx, item.SourceInstanceID)
	return &DatabaseReplicationCheckVO{
		ID:                        item.ID,
		InstanceID:                item.InstanceID,
		InstanceName:              instanceName,
		InstanceEndpoint:          instanceEndpoint,
		ReplicaID:                 item.ReplicaID,
		Engine:                    item.Engine,
		EngineText:                DBTypeText(item.Engine),
		RoleDetected:              item.RoleDetected,
		RoleDetectedText:          ReplicationRoleText(item.RoleDetected),
		SourceInstanceID:          item.SourceInstanceID,
		SourceInstanceName:        sourceName,
		ReplicaIORunning:          item.ReplicaIORunning,
		ReplicaSQLRunning:         item.ReplicaSQLRunning,
		SecondsBehindSource:       item.SecondsBehindSource,
		ConfiguredDelaySeconds:    item.ConfiguredDelaySeconds,
		RemainingDelaySeconds:     item.RemainingDelaySeconds,
		RelayLogBytes:             item.RelayLogBytes,
		PGWriteLagMs:              item.PGWriteLagMs,
		PGFlushLagMs:              item.PGFlushLagMs,
		PGReplayLagMs:             item.PGReplayLagMs,
		PGLastWALReplayLSN:        item.PGLastWALReplayLSN,
		PGLastXactReplayTimestamp: formatTime(item.PGLastXactReplayTimestamp),
		WALBacklogBytes:           item.WALBacklogBytes,
		HealthStatus:              item.HealthStatus,
		HealthStatusText:          ReplicaHealthText(item.HealthStatus),
		RiskFlagsJSON:             item.RiskFlagsJSON,
		RawStatusJSON:             item.RawStatusJSON,
		CheckedAt:                 formatTime(item.CheckedAt),
		ErrorMessage:              item.ErrorMessage,
		CreatedAt:                 formatTime(&item.CreatedAt),
	}
}

func (uc *UseCase) instanceNameEndpoint(ctx context.Context, id uint) (string, string) {
	if id == 0 || uc.instanceRepo == nil {
		return "", ""
	}
	item, err := uc.instanceRepo.GetByID(ctx, id)
	if err != nil || item == nil {
		return "", ""
	}
	return item.Name, net.JoinHostPort(strings.TrimSpace(item.Host), fmt.Sprintf("%d", item.Port))
}

func (uc *UseCase) recordReplicationAudit(ctx context.Context, item *DatabaseInstance, check *DatabaseReplicationCheck, operator QueryOperator, runErr error) {
	if uc == nil || uc.auditRepo == nil || item == nil || check == nil {
		return
	}
	status := DatabaseQueryStatusSuccess
	message := check.ErrorMessage
	if runErr != nil {
		status = DatabaseQueryStatusFailed
		message = runErr.Error()
	}
	audit := &DatabaseQueryAudit{
		InstanceID:     item.ID,
		SchemaName:     "",
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 100),
		AuditAction:    DatabaseAuditActionReplicaCheckRun,
		SQLText:        trimText("replication status check", 20000),
		SQLFingerprint: sqlFingerprint("replication status check"),
		SQLType:        "REPLICATION_CHECK",
		RiskLevel:      DatabaseQueryRiskLow,
		Status:         status,
		RowsReturned:   1,
		DurationMs:     0,
		ErrorMessage:   trimText(message, 500),
		ClientIP:       trimText(operator.ClientIP, 64),
	}
	_ = uc.auditRepo.Create(ctx, audit)
}

func querySingleRowMap(ctx context.Context, db *sql.DB, query string) (map[string]string, bool, error) {
	rows, err := queryRowsMap(ctx, db, query)
	if err != nil {
		return nil, false, err
	}
	if len(rows) == 0 {
		return map[string]string{}, false, nil
	}
	return rows[0], true, nil
}

func queryRowsMap(ctx context.Context, db *sql.DB, query string, args ...any) ([]map[string]string, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var result []map[string]string
	for rows.Next() {
		raw := make([]sql.RawBytes, len(columns))
		dest := make([]any, len(columns))
		for i := range raw {
			dest[i] = &raw[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		item := make(map[string]string, len(columns))
		for i, col := range columns {
			if raw[i] == nil {
				item[col] = ""
				continue
			}
			item[col] = string(raw[i])
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func mysqlReplicaRiskFlags(ioRunning, sqlRunning string, secondsBehind, configuredDelay, remainingDelay int, sourceInstanceID uint) []string {
	flags := make([]string, 0, 4)
	if strings.TrimSpace(ioRunning) != "" && !strings.EqualFold(strings.TrimSpace(ioRunning), "Yes") {
		flags = append(flags, "IO 线程异常")
	}
	if strings.TrimSpace(sqlRunning) != "" && !strings.EqualFold(strings.TrimSpace(sqlRunning), "Yes") {
		flags = append(flags, "SQL apply 线程异常")
	}
	if configuredDelay <= 0 && secondsBehind > 300 {
		flags = append(flags, "复制延迟过大")
	}
	if configuredDelay > 0 && remainingDelay < 0 {
		flags = append(flags, "剩余保护窗口未知")
	}
	if sourceInstanceID == 0 {
		flags = append(flags, "来源主库未匹配")
	}
	return flags
}

func mysqlTopologyVariableRiskFlags(role string, variables map[string]string) []string {
	flags := make([]string, 0, 4)
	serverID := strings.TrimSpace(variables["server_id"])
	if serverID == "" || serverID == "0" {
		flags = append(flags, "server_id 配置无效")
	}
	switch role {
	case DatabaseReplicationRoleReplica:
		if mysqlVariableOff(variables["read_only"]) {
			flags = append(flags, "从库未开启只读保护")
		}
		if mysqlVariableOff(variables["super_read_only"]) {
			flags = append(flags, "从库未开启 super_read_only")
		}
	case DatabaseReplicationRolePrimary:
		if mysqlVariableOff(variables["log_bin"]) {
			flags = append(flags, "主库未开启 binlog")
		}
	}
	return flags
}

func mysqlVariableOff(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "0", "off", "false", "no", "disabled":
		return true
	default:
		return false
	}
}

func postgresStandbyRiskFlags(receiverStatus string, sourceInstanceID uint, receiverErr error, replayPaused bool) []string {
	flags := make([]string, 0, 3)
	if receiverErr != nil {
		flags = append(flags, "WAL receiver 状态读取失败")
	}
	if replayPaused {
		flags = append(flags, "WAL replay 已暂停")
	}
	if status := strings.TrimSpace(receiverStatus); status != "" && !strings.EqualFold(status, "streaming") {
		flags = append(flags, "WAL receiver 非 streaming")
	}
	if sourceInstanceID == 0 {
		flags = append(flags, "来源主库未匹配")
	}
	return flags
}

func replicaHealthFromRiskFlags(flags []string) string {
	if len(flags) == 0 {
		return DatabaseReplicaHealthHealthy
	}
	for _, flag := range flags {
		if strings.Contains(flag, "异常") || strings.Contains(flag, "失败") {
			return DatabaseReplicaHealthCritical
		}
	}
	return DatabaseReplicaHealthWarning
}

func isReplicaDetected(role string) bool {
	switch strings.TrimSpace(role) {
	case DatabaseReplicationRoleReplica, DatabaseReplicationRoleStandby:
		return true
	default:
		return false
	}
}

func readPostgreSQLApplyDelay(ctx context.Context, db *sql.DB) (int, string, error) {
	var raw string
	if err := db.QueryRowContext(ctx, "SHOW recovery_min_apply_delay").Scan(&raw); err != nil {
		return 0, "", err
	}
	return parsePostgreSQLDelaySeconds(raw), raw, nil
}

func parsePostgreSQLDelaySeconds(raw string) int {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" || value == "0" || value == "0s" {
		return 0
	}
	if strings.Contains(value, ":") {
		parts := strings.Split(value, ":")
		if len(parts) == 3 {
			hour := parseInt(parts[0], 0)
			minute := parseInt(parts[1], 0)
			second := parseInt(parts[2], 0)
			return hour*3600 + minute*60 + second
		}
	}
	re := regexp.MustCompile(`(?i)(\d+)\s*(ms|milliseconds?|s|sec|secs|seconds?|min|mins|minutes?|h|hr|hour|hours|d|day|days)?`)
	matches := re.FindAllStringSubmatch(value, -1)
	if len(matches) == 0 {
		return 0
	}
	total := 0
	for _, match := range matches {
		num := parseInt(match[1], 0)
		switch strings.ToLower(match[2]) {
		case "ms", "millisecond", "milliseconds":
			if num > 0 {
				total += 1
			}
		case "min", "mins", "minute", "minutes":
			total += num * 60
		case "h", "hr", "hour", "hours":
			total += num * 3600
		case "d", "day", "days":
			total += num * 86400
		default:
			total += num
		}
	}
	return total
}

func pgDurationTextMillis(value string) int64 {
	seconds := parsePostgreSQLDelaySeconds(value)
	return int64(seconds) * 1000
}

func parsePostgreSQLConnInfo(conninfo string) (string, int, string) {
	fields := strings.Fields(strings.TrimSpace(conninfo))
	values := make(map[string]string)
	for _, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		values[strings.ToLower(strings.TrimSpace(key))] = strings.Trim(strings.TrimSpace(value), "'")
	}
	return values["host"], parseInt(values["port"], 5432), values["application_name"]
}

func parseInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return value
}

func parseNullableInt(raw string, fallback int) int {
	value := strings.TrimSpace(raw)
	if value == "" || strings.EqualFold(value, "NULL") {
		return fallback
	}
	return parseInt(value, fallback)
}

func normalizeEndpointHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if strings.HasPrefix(host, "[") && strings.Contains(host, "]") {
		if unwrapped, _, err := net.SplitHostPort(host + ":0"); err == nil {
			return unwrapped
		}
	}
	return host
}

func marshalReplicaJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return trimText(string(data), 60000)
}

func decodeReplicaRawMap(raw string) map[string]any {
	var result map[string]any
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return map[string]any{}
	}
	return result
}

func redactReplicaStatusRows(rows []map[string]string) []map[string]string {
	result := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		result = append(result, redactReplicaStatusMap(row))
	}
	return result
}

func redactReplicaStatusMap(row map[string]string) map[string]string {
	result := make(map[string]string, len(row))
	for key, value := range row {
		result[key] = redactReplicaValue(key, value)
	}
	return result
}

func redactReplicaRawMap(row map[string]any) map[string]any {
	result := make(map[string]any, len(row))
	for key, value := range row {
		switch typed := value.(type) {
		case string:
			result[key] = redactReplicaValue(key, typed)
		case map[string]string:
			result[key] = redactReplicaStatusMap(typed)
		default:
			result[key] = value
		}
	}
	return result
}

func redactReplicaValue(key, value string) string {
	lowerKey := strings.ToLower(strings.TrimSpace(key))
	if strings.Contains(lowerKey, "password") || strings.Contains(lowerKey, "passwd") {
		return "******"
	}
	if strings.Contains(strings.ToLower(value), "password=") {
		re := regexp.MustCompile(`(?i)(password=)[^\s]+`)
		return re.ReplaceAllString(value, "${1}******")
	}
	return value
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case fmt.Stringer:
		return typed.String()
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func toBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "on":
			return true
		}
	case float64:
		return typed != 0
	case int:
		return typed != 0
	}
	return false
}

func ReplicaRoleText(role string) string {
	switch strings.TrimSpace(role) {
	case DatabaseReplicaRoleRealtime:
		return "实时副本"
	case DatabaseReplicaRoleDelayed:
		return "延迟副本"
	case DatabaseReplicaRoleStandby:
		return "Standby"
	default:
		return "未知"
	}
}

func ReplicationRoleText(role string) string {
	switch strings.TrimSpace(role) {
	case DatabaseReplicationRolePrimary:
		return "主库"
	case DatabaseReplicationRoleReplica:
		return "从库"
	case DatabaseReplicationRoleStandby:
		return "Standby"
	default:
		return "未知"
	}
}

func ReplicaApplyStateText(state string) string {
	switch strings.TrimSpace(state) {
	case DatabaseReplicaApplyStateRunning:
		return "运行中"
	case DatabaseReplicaApplyStatePaused:
		return "已暂停"
	default:
		return "未知"
	}
}

func ReplicaApplyActionText(action string) string {
	switch strings.TrimSpace(action) {
	case DatabaseReplicaApplyActionPause:
		return "暂停 apply"
	case DatabaseReplicaApplyActionResume:
		return "恢复 apply"
	default:
		return "未知"
	}
}

func DatabaseActionStatusText(status string) string {
	switch strings.TrimSpace(status) {
	case DatabaseQueryStatusPending:
		return "待执行"
	case DatabaseQueryStatusSuccess:
		return "成功"
	case DatabaseQueryStatusFailed:
		return "失败"
	default:
		return "未知"
	}
}

func ReplicaHealthText(status string) string {
	switch strings.TrimSpace(status) {
	case DatabaseReplicaHealthHealthy:
		return "健康"
	case DatabaseReplicaHealthWarning:
		return "警告"
	case DatabaseReplicaHealthCritical:
		return "异常"
	default:
		return "未知"
	}
}

func ReplicaDiscoverySourceText(source string) string {
	switch strings.TrimSpace(source) {
	case DatabaseReplicaDiscoveryReplicaStatus:
		return "副本状态"
	case DatabaseReplicaDiscoveryPrimaryStat:
		return "主库状态"
	case DatabaseReplicaDiscoveryManual:
		return "人工登记"
	case DatabaseReplicaDiscoveryInferred:
		return "推断"
	default:
		return "未知"
	}
}
