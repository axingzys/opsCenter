package database

import "testing"

func TestReplicaApplyCommandSpecWhitelist(t *testing.T) {
	tests := []struct {
		name    string
		engine  string
		action  string
		command string
	}{
		{name: "mysql pause", engine: DBTypeMySQL, action: DatabaseReplicaApplyActionPause, command: "STOP REPLICA SQL_THREAD"},
		{name: "mysql resume", engine: DBTypeMySQL, action: DatabaseReplicaApplyActionResume, command: "START REPLICA SQL_THREAD"},
		{name: "postgres pause", engine: DBTypePostgreSQL, action: DatabaseReplicaApplyActionPause, command: "SELECT pg_wal_replay_pause()"},
		{name: "postgres resume", engine: DBTypePostgreSQL, action: DatabaseReplicaApplyActionResume, command: "SELECT pg_wal_replay_resume()"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, command, err := replicaApplyCommandSpec(tt.engine, tt.action)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if command != tt.command {
				t.Fatalf("command = %q, want %q", command, tt.command)
			}
		})
	}
	if _, _, err := replicaApplyCommandSpec(DBTypeMySQL, "DROP DATABASE"); err == nil {
		t.Fatalf("expected unsupported action to fail")
	}
	if _, _, err := replicaApplyCommandSpec(DBTypeRedis, DatabaseReplicaApplyActionPause); err == nil {
		t.Fatalf("expected unsupported engine to fail")
	}
}

func TestValidateReplicaApplyActionTargetRejectsPrimary(t *testing.T) {
	check := &DatabaseReplicationCheck{
		Engine:       DBTypeMySQL,
		RoleDetected: DatabaseReplicationRolePrimary,
	}
	if err := validateReplicaApplyActionTarget(check, DBTypeMySQL); err == nil {
		t.Fatalf("expected primary target to be rejected")
	}
	check.RoleDetected = DatabaseReplicationRoleReplica
	if err := validateReplicaApplyActionTarget(check, DBTypeMySQL); err != nil {
		t.Fatalf("expected mysql replica to pass: %v", err)
	}
	check.RoleDetected = DatabaseReplicationRoleStandby
	if err := validateReplicaApplyActionTarget(check, DBTypePostgreSQL); err != nil {
		t.Fatalf("expected postgres standby to pass: %v", err)
	}
}
