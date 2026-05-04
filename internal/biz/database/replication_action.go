package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

const replicaApplyActionTimeout = 20 * time.Second

func (uc *UseCase) GetInstanceReplica(ctx context.Context, id uint) (*DatabaseInstanceReplicaVO, error) {
	if id == 0 {
		return nil, fmt.Errorf("请选择副本关系")
	}
	if uc.instanceReplicaRepo == nil {
		return nil, fmt.Errorf("副本关系仓储未配置")
	}
	item, err := uc.instanceReplicaRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("副本关系不存在")
	}
	return uc.toInstanceReplicaVO(ctx, item), nil
}

func (uc *UseCase) ListReplicaActions(ctx context.Context, req *DatabaseReplicaActionListRequest) ([]*DatabaseReplicaActionVO, int64, error) {
	if uc.replicaActionRepo == nil {
		return nil, 0, fmt.Errorf("副本 apply 操作仓储未配置")
	}
	items, total, err := uc.replicaActionRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*DatabaseReplicaActionVO, 0, len(items))
	for _, item := range items {
		result = append(result, uc.toReplicaActionVO(ctx, item))
	}
	return result, total, nil
}

func (uc *UseCase) GetReplicaAction(ctx context.Context, id uint) (*DatabaseReplicaActionVO, error) {
	if id == 0 {
		return nil, fmt.Errorf("请选择 Apply 操作记录")
	}
	if uc.replicaActionRepo == nil {
		return nil, fmt.Errorf("副本 apply 操作仓储未配置")
	}
	item, err := uc.replicaActionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("Apply 操作记录不存在")
	}
	return uc.toReplicaActionVO(ctx, item), nil
}

func (uc *UseCase) DeleteReplicaAction(ctx context.Context, id uint, operator QueryOperator) error {
	if id == 0 {
		return fmt.Errorf("请选择 Apply 操作记录")
	}
	if uc.replicaActionRepo == nil {
		return fmt.Errorf("副本 apply 操作仓储未配置")
	}
	item, err := uc.replicaActionRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("Apply 操作记录不存在")
	}
	if err := uc.replicaActionRepo.Delete(ctx, id); err != nil {
		return err
	}
	uc.recordReplicaActionDeleteAudit(ctx, item, operator)
	return nil
}

func (uc *UseCase) PauseReplicaApply(ctx context.Context, replicaID uint, req *DatabaseReplicaActionRequest, operator QueryOperator) (*DatabaseReplicaActionVO, error) {
	return uc.executeReplicaApplyAction(ctx, replicaID, DatabaseReplicaApplyActionPause, req, operator)
}

func (uc *UseCase) ResumeReplicaApply(ctx context.Context, replicaID uint, req *DatabaseReplicaActionRequest, operator QueryOperator) (*DatabaseReplicaActionVO, error) {
	return uc.executeReplicaApplyAction(ctx, replicaID, DatabaseReplicaApplyActionResume, req, operator)
}

func (uc *UseCase) executeReplicaApplyAction(ctx context.Context, replicaID uint, action string, req *DatabaseReplicaActionRequest, operator QueryOperator) (*DatabaseReplicaActionVO, error) {
	if replicaID == 0 {
		return nil, fmt.Errorf("请选择副本关系")
	}
	if uc.instanceReplicaRepo == nil || uc.replicationCheckRepo == nil || uc.replicaActionRepo == nil {
		return nil, fmt.Errorf("副本治理仓储未配置")
	}
	if req == nil {
		return nil, fmt.Errorf("请求不能为空")
	}
	if !req.Confirmed {
		return nil, fmt.Errorf("必须二次确认影响范围后才能执行")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return nil, fmt.Errorf("执行原因不能为空")
	}
	if strings.TrimSpace(req.ConfirmImpact) == "" {
		return nil, fmt.Errorf("确认影响范围不能为空")
	}
	incidentNo := strings.TrimSpace(req.IncidentNo)
	if incidentNo == "" && req.IncidentGuideID > 0 {
		incidentNo = fmt.Sprintf("GUIDE-%d", req.IncidentGuideID)
	}
	if incidentNo == "" {
		return nil, fmt.Errorf("事故编号不能为空")
	}
	normalizedAction, err := normalizeReplicaApplyAction(action)
	if err != nil {
		return nil, err
	}
	replica, err := uc.instanceReplicaRepo.GetByID(ctx, replicaID)
	if err != nil {
		return nil, fmt.Errorf("副本关系不存在")
	}
	target, err := uc.getEnabledQueryInstance(ctx, replica.ReplicaInstanceID)
	if err != nil {
		return nil, fmt.Errorf("副本实例不可用: %w", err)
	}
	if target.ID == replica.PrimaryInstanceID && replica.PrimaryInstanceID != 0 {
		return nil, fmt.Errorf("不能对主库执行 apply/replay 控制")
	}
	engine := normalizeDBType(target.DBType)
	if engine == "" {
		engine = normalizeDBType(replica.Engine)
	}
	allowedCommand, commandTemplate, err := replicaApplyCommandSpec(engine, normalizedAction)
	if err != nil {
		return nil, err
	}

	startedAt := time.Now()
	requestJSON := marshalReplicaJSON(map[string]any{
		"incidentGuideId": req.IncidentGuideID,
		"incidentNo":      incidentNo,
		"reason":          req.Reason,
		"confirmImpact":   req.ConfirmImpact,
		"confirmed":       req.Confirmed,
	})
	record := &DatabaseReplicaAction{
		ReplicaID:         replica.ID,
		PrimaryInstanceID: replica.PrimaryInstanceID,
		ReplicaInstanceID: replica.ReplicaInstanceID,
		IncidentGuideID:   req.IncidentGuideID,
		IncidentNo:        trimText(incidentNo, 120),
		Action:            normalizedAction,
		Engine:            engine,
		AllowedCommand:    allowedCommand,
		CommandTemplate:   commandTemplate,
		Reason:            trimText(req.Reason, 4000),
		ConfirmImpact:     trimText(req.ConfirmImpact, 4000),
		Confirmed:         req.Confirmed,
		Status:            DatabaseQueryStatusPending,
		OperatorID:        operator.ID,
		OperatorName:      trimText(operator.Username, 100),
		ClientIP:          trimText(operator.ClientIP, 64),
		StartedAt:         &startedAt,
		RequestJSON:       requestJSON,
	}
	if err := uc.replicaActionRepo.Create(ctx, record); err != nil {
		return nil, err
	}

	beforeCheck, runErr := uc.runReplicaActionPrecheck(ctx, target.ID, operator)
	if beforeCheck != nil {
		record.BeforeCheckID = beforeCheck.ID
		record.BeforeStatusJSON = trimText(beforeCheck.RawStatusJSON, 60000)
	}
	if runErr == nil {
		runErr = validateReplicaApplyActionTarget(beforeCheck, engine)
	}
	if runErr == nil {
		commandTemplate, stdout, stderr, exitCode, execErr := uc.executeReplicaApplyCommand(ctx, target, engine, normalizedAction)
		if strings.TrimSpace(commandTemplate) != "" {
			record.CommandTemplate = commandTemplate
		}
		record.Stdout = trimText(stdout, 4000)
		record.Stderr = trimText(stderr, 4000)
		record.ExitCode = exitCode
		runErr = execErr
	}

	afterCheck, afterErr := uc.runReplicaActionPrecheck(ctx, target.ID, operator)
	if afterCheck != nil {
		record.AfterCheckID = afterCheck.ID
		record.AfterStatusJSON = trimText(afterCheck.RawStatusJSON, 60000)
	}
	if runErr == nil && afterErr != nil {
		runErr = fmt.Errorf("命令已执行，但执行后复查失败: %w", afterErr)
	}

	finishedAt := time.Now()
	record.FinishedAt = &finishedAt
	record.DurationMs = finishedAt.Sub(startedAt).Milliseconds()
	if runErr != nil {
		record.Status = DatabaseQueryStatusFailed
		record.ErrorMessage = trimText(runErr.Error(), 1000)
		if record.ExitCode == 0 {
			record.ExitCode = 1
		}
	} else {
		record.Status = DatabaseQueryStatusSuccess
		record.ErrorMessage = ""
		record.ExitCode = 0
	}
	if err := uc.replicaActionRepo.Update(ctx, record); err != nil && runErr == nil {
		runErr = err
	}
	uc.recordReplicaApplyActionAudit(ctx, target, record)
	if runErr != nil {
		return uc.toReplicaActionVO(ctx, record), runErr
	}
	return uc.toReplicaActionVO(ctx, record), nil
}

func (uc *UseCase) runReplicaActionPrecheck(ctx context.Context, instanceID uint, operator QueryOperator) (*DatabaseReplicationCheck, error) {
	vo, err := uc.CheckInstanceReplication(ctx, instanceID, operator)
	if err != nil {
		if vo != nil && vo.ID > 0 {
			check, getErr := uc.replicationCheckRepo.GetByID(ctx, vo.ID)
			if getErr == nil {
				return check, err
			}
		}
		return nil, err
	}
	if vo == nil || vo.ID == 0 {
		return nil, fmt.Errorf("副本状态复查结果为空")
	}
	check, err := uc.replicationCheckRepo.GetByID(ctx, vo.ID)
	if err != nil {
		return nil, err
	}
	return check, nil
}

func validateReplicaApplyActionTarget(check *DatabaseReplicationCheck, engine string) error {
	if check == nil {
		return fmt.Errorf("缺少执行前副本状态检查")
	}
	switch check.RoleDetected {
	case DatabaseReplicationRoleReplica, DatabaseReplicationRoleStandby:
	default:
		return fmt.Errorf("只能对已确认的 replica/standby 执行，当前检测角色: %s", ReplicationRoleText(check.RoleDetected))
	}
	switch normalizeDBType(engine) {
	case DBTypeMySQL, DBTypeMariaDB:
		if check.RoleDetected != DatabaseReplicationRoleReplica {
			return fmt.Errorf("MySQL/MariaDB 目标必须是 replica，当前检测角色: %s", ReplicationRoleText(check.RoleDetected))
		}
	case DBTypePostgreSQL:
		if check.RoleDetected != DatabaseReplicationRoleStandby {
			return fmt.Errorf("PostgreSQL 目标必须是 standby，当前检测角色: %s", ReplicationRoleText(check.RoleDetected))
		}
	default:
		return fmt.Errorf("当前仅支持 MySQL / MariaDB / PostgreSQL 副本 apply/replay 控制")
	}
	return nil
}

func (uc *UseCase) executeReplicaApplyCommand(ctx context.Context, target *DatabaseInstance, engine, action string) (string, string, string, int, error) {
	credential, err := uc.credentialResolver(ctx, target.CredentialID)
	if err != nil {
		return "", "", "凭据不存在", 1, fmt.Errorf("凭据不存在")
	}
	execCtx, cancel := context.WithTimeout(ctx, replicaApplyActionTimeout)
	defer cancel()
	switch normalizeDBType(engine) {
	case DBTypeMySQL, DBTypeMariaDB:
		return executeMySQLReplicaApplyCommand(execCtx, target, credential, action)
	case DBTypePostgreSQL:
		return executePostgreSQLReplicaApplyCommand(execCtx, target, credential, action)
	default:
		return "", "", "unsupported engine", 1, fmt.Errorf("当前仅支持 MySQL / MariaDB / PostgreSQL 副本 apply/replay 控制")
	}
}

func executeMySQLReplicaApplyCommand(ctx context.Context, target *DatabaseInstance, credential *ConnectionCredential, action string) (string, string, string, int, error) {
	db, err := openMySQLDB(target, credential)
	if err != nil {
		return "", "", err.Error(), 1, err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return "", "", err.Error(), 1, fmt.Errorf("连接数据库失败: %w", err)
	}
	primaryCommand, legacyCommand := "STOP REPLICA SQL_THREAD", "STOP SLAVE SQL_THREAD"
	if action == DatabaseReplicaApplyActionResume {
		primaryCommand, legacyCommand = "START REPLICA SQL_THREAD", "START SLAVE SQL_THREAD"
	}
	if _, err := db.ExecContext(ctx, primaryCommand); err == nil {
		return primaryCommand, "ok", "", 0, nil
	} else {
		legacyErr := executeSQLNoRows(ctx, db, legacyCommand)
		if legacyErr == nil {
			return legacyCommand, "ok", trimText("首选命令失败，已使用旧版兼容命令: "+err.Error(), 1000), 0, nil
		}
		return primaryCommand, "", trimText(fmt.Sprintf("%s failed: %v; %s failed: %v", primaryCommand, err, legacyCommand, legacyErr), 4000), 1, err
	}
}

func executePostgreSQLReplicaApplyCommand(ctx context.Context, target *DatabaseInstance, credential *ConnectionCredential, action string) (string, string, string, int, error) {
	db, err := openPostgreSQLDB(target, credential)
	if err != nil {
		return "", "", err.Error(), 1, err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return "", "", err.Error(), 1, fmt.Errorf("连接数据库失败: %w", err)
	}
	command := "SELECT pg_wal_replay_pause()"
	if action == DatabaseReplicaApplyActionResume {
		command = "SELECT pg_wal_replay_resume()"
	}
	if err := executeSQLNoRows(ctx, db, command); err != nil {
		return command, "", err.Error(), 1, err
	}
	return command, "ok", "", 0, nil
}

func executeSQLNoRows(ctx context.Context, db *sql.DB, query string) error {
	if db == nil {
		return fmt.Errorf("数据库连接为空")
	}
	_, err := db.ExecContext(ctx, query)
	return err
}

func normalizeReplicaApplyAction(action string) (string, error) {
	switch strings.TrimSpace(action) {
	case DatabaseReplicaApplyActionPause:
		return DatabaseReplicaApplyActionPause, nil
	case DatabaseReplicaApplyActionResume:
		return DatabaseReplicaApplyActionResume, nil
	default:
		return "", fmt.Errorf("不支持的副本 apply 操作")
	}
}

func replicaApplyCommandSpec(engine, action string) (string, string, error) {
	normalizedEngine := normalizeDBType(engine)
	normalizedAction, err := normalizeReplicaApplyAction(action)
	if err != nil {
		return "", "", err
	}
	switch normalizedEngine {
	case DBTypeMySQL, DBTypeMariaDB:
		if normalizedAction == DatabaseReplicaApplyActionPause {
			return DatabaseRunnerAllowedCommandReplicaPauseApply, "STOP REPLICA SQL_THREAD", nil
		}
		return DatabaseRunnerAllowedCommandReplicaResumeApply, "START REPLICA SQL_THREAD", nil
	case DBTypePostgreSQL:
		if normalizedAction == DatabaseReplicaApplyActionPause {
			return DatabaseRunnerAllowedCommandReplicaPauseApply, "SELECT pg_wal_replay_pause()", nil
		}
		return DatabaseRunnerAllowedCommandReplicaResumeApply, "SELECT pg_wal_replay_resume()", nil
	default:
		return "", "", fmt.Errorf("当前仅支持 MySQL / MariaDB / PostgreSQL 副本 apply/replay 控制")
	}
}

func (uc *UseCase) toReplicaActionVO(ctx context.Context, item *DatabaseReplicaAction) *DatabaseReplicaActionVO {
	if item == nil {
		return nil
	}
	primaryName, primaryEndpoint := uc.instanceNameEndpoint(ctx, item.PrimaryInstanceID)
	replicaName, replicaEndpoint := uc.instanceNameEndpoint(ctx, item.ReplicaInstanceID)
	return &DatabaseReplicaActionVO{
		ID:                  item.ID,
		ReplicaID:           item.ReplicaID,
		PrimaryInstanceID:   item.PrimaryInstanceID,
		PrimaryInstanceName: primaryName,
		PrimaryEndpoint:     primaryEndpoint,
		ReplicaInstanceID:   item.ReplicaInstanceID,
		ReplicaInstanceName: replicaName,
		ReplicaEndpoint:     replicaEndpoint,
		IncidentGuideID:     item.IncidentGuideID,
		IncidentNo:          item.IncidentNo,
		Action:              item.Action,
		ActionText:          ReplicaApplyActionText(item.Action),
		Engine:              item.Engine,
		EngineText:          DBTypeText(item.Engine),
		AllowedCommand:      item.AllowedCommand,
		CommandTemplate:     item.CommandTemplate,
		Reason:              item.Reason,
		ConfirmImpact:       item.ConfirmImpact,
		Confirmed:           item.Confirmed,
		BeforeCheckID:       item.BeforeCheckID,
		AfterCheckID:        item.AfterCheckID,
		BeforeStatusJSON:    item.BeforeStatusJSON,
		AfterStatusJSON:     item.AfterStatusJSON,
		Stdout:              item.Stdout,
		Stderr:              item.Stderr,
		ExitCode:            item.ExitCode,
		Status:              item.Status,
		StatusText:          DatabaseActionStatusText(item.Status),
		ErrorMessage:        item.ErrorMessage,
		OperatorID:          item.OperatorID,
		OperatorName:        item.OperatorName,
		ClientIP:            item.ClientIP,
		StartedAt:           formatTime(item.StartedAt),
		FinishedAt:          formatTime(item.FinishedAt),
		DurationMs:          item.DurationMs,
		CreatedAt:           formatTime(&item.CreatedAt),
		UpdatedAt:           formatTime(&item.UpdatedAt),
	}
}

func (uc *UseCase) recordReplicaApplyActionAudit(ctx context.Context, instance *DatabaseInstance, action *DatabaseReplicaAction) {
	if uc == nil || uc.auditRepo == nil || instance == nil || action == nil {
		return
	}
	auditAction := DatabaseAuditActionReplicaPauseApply
	if action.Action == DatabaseReplicaApplyActionResume {
		auditAction = DatabaseAuditActionReplicaResumeApply
	}
	auditText := fmt.Sprintf("%s replica_id=%d incident=%s command=%s", ReplicaApplyActionText(action.Action), action.ReplicaID, action.IncidentNo, action.CommandTemplate)
	_ = uc.auditRepo.Create(ctx, &DatabaseQueryAudit{
		InstanceID:      instance.ID,
		SchemaName:      "",
		OperatorID:      action.OperatorID,
		OperatorName:    trimText(action.OperatorName, 100),
		AuditAction:     auditAction,
		Reason:          trimText(action.Reason, 500),
		ConfirmRequired: true,
		Confirmed:       action.Confirmed,
		SQLText:         trimText(auditText, 20000),
		SQLFingerprint:  sqlFingerprint(auditText),
		SQLType:         "REPLICA_APPLY_CONTROL",
		RiskLevel:       DatabaseQueryRiskHigh,
		Status:          action.Status,
		DurationMs:      action.DurationMs,
		ErrorMessage:    trimText(action.ErrorMessage, 500),
		ClientIP:        trimText(action.ClientIP, 64),
	})
}

func (uc *UseCase) recordReplicaActionDeleteAudit(ctx context.Context, action *DatabaseReplicaAction, operator QueryOperator) {
	if uc == nil || uc.auditRepo == nil || action == nil {
		return
	}
	instanceID := action.ReplicaInstanceID
	if instanceID == 0 {
		instanceID = action.PrimaryInstanceID
	}
	auditText := fmt.Sprintf("delete replica apply action #%d replica_id=%d action=%s incident=%s", action.ID, action.ReplicaID, action.Action, action.IncidentNo)
	_ = uc.auditRepo.Create(ctx, &DatabaseQueryAudit{
		InstanceID:     instanceID,
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 100),
		AuditAction:    DatabaseAuditActionReplicaActionDel,
		SQLText:        trimText(auditText, 20000),
		SQLFingerprint: sqlFingerprint(auditText),
		SQLType:        "REPLICA_APPLY_ACTION_DELETE",
		RiskLevel:      DatabaseQueryRiskMedium,
		Status:         DatabaseQueryStatusSuccess,
		RowsReturned:   1,
		ErrorMessage:   trimText(fmt.Sprintf("command=%s status=%s", action.CommandTemplate, action.Status), 500),
		ClientIP:       trimText(operator.ClientIP, 64),
	})
}

func isNotFoundError(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func marshalReplicaActionJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return trimText(string(data), 60000)
}
