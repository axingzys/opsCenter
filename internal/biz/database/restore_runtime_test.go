package database

import "testing"

func TestBuildRestoreCommandSpec(t *testing.T) {
	mysqlSpec, err := buildRestoreCommandSpec(&DatabaseInstance{
		DBType: DBTypeMariaDB,
		Host:   "127.0.0.1",
		Port:   3306,
	}, &ConnectionCredential{
		Username: "root",
		Password: "secret",
	}, "restore_db", DatabaseBackupTypeLogical)
	if err != nil {
		t.Fatalf("buildRestoreCommandSpec(mysql) error = %v", err)
	}
	if len(mysqlSpec.Commands) == 0 || mysqlSpec.Commands[0] != "mysql" {
		t.Fatalf("unexpected mysql commands: %#v", mysqlSpec.Commands)
	}
	if !containsString(mysqlSpec.Env, "MYSQL_PWD=secret") {
		t.Fatalf("expected MYSQL_PWD env, got %#v", mysqlSpec.Env)
	}
	if !containsString(mysqlSpec.Args, "restore_db") {
		t.Fatalf("expected database name in args, got %#v", mysqlSpec.Args)
	}

	pgSpec, err := buildRestoreCommandSpec(&DatabaseInstance{
		DBType:     DBTypePostgreSQL,
		Host:       "127.0.0.1",
		Port:       5432,
		TLSEnabled: true,
	}, &ConnectionCredential{
		Username: "postgres",
		Password: "secret",
	}, "restore_db", DatabaseBackupTypeLogical)
	if err != nil {
		t.Fatalf("buildRestoreCommandSpec(postgresql) error = %v", err)
	}
	if len(pgSpec.Commands) != 1 || pgSpec.Commands[0] != "psql" {
		t.Fatalf("unexpected postgresql commands: %#v", pgSpec.Commands)
	}
	if !containsString(pgSpec.Env, "PGPASSWORD=secret") {
		t.Fatalf("expected PGPASSWORD env, got %#v", pgSpec.Env)
	}
	if !containsString(pgSpec.Env, "PGSSLMODE=require") {
		t.Fatalf("expected PGSSLMODE=require, got %#v", pgSpec.Env)
	}
	if !containsString(pgSpec.Args, "--single-transaction") {
		t.Fatalf("expected single transaction psql arg, got %#v", pgSpec.Args)
	}
	if pgSpec.InputMode != restoreInputModeStdin {
		t.Fatalf("unexpected postgresql plain input mode: %s", pgSpec.InputMode)
	}

	pgCustomSpec, err := buildRestoreCommandSpec(&DatabaseInstance{
		DBType: DBTypePostgreSQL,
		Host:   "127.0.0.1",
		Port:   5432,
	}, &ConnectionCredential{
		Username: "postgres",
		Password: "secret",
	}, "restore_db", DatabaseBackupTypeLogicalCustom)
	if err != nil {
		t.Fatalf("buildRestoreCommandSpec(postgresql custom) error = %v", err)
	}
	if len(pgCustomSpec.Commands) != 1 || pgCustomSpec.Commands[0] != "pg_restore" {
		t.Fatalf("unexpected postgresql custom commands: %#v", pgCustomSpec.Commands)
	}
	if pgCustomSpec.InputMode != restoreInputModeFileArg {
		t.Fatalf("unexpected postgresql custom input mode: %s", pgCustomSpec.InputMode)
	}
	if !containsString(pgCustomSpec.Args, "--exit-on-error") {
		t.Fatalf("expected pg_restore exit-on-error arg, got %#v", pgCustomSpec.Args)
	}

	redisSpec, err := buildRestoreCommandSpec(&DatabaseInstance{
		DBType: DBTypeRedis,
		Host:   "127.0.0.1",
		Port:   6379,
	}, &ConnectionCredential{}, "all-dbs", DatabaseBackupTypeLogical)
	if err != nil {
		t.Fatalf("buildRestoreCommandSpec(redis) error = %v", err)
	}
	if redisSpec.Runner == nil {
		t.Fatalf("expected redis restore runner")
	}
}

func TestResolveRestoreDatabaseName(t *testing.T) {
	tests := []struct {
		name       string
		item       *DatabaseInstance
		credential *ConnectionCredential
		want       string
		wantErr    bool
	}{
		{
			name: "mysql uses default database",
			item: &DatabaseInstance{
				DBType:          DBTypeMySQL,
				DefaultDatabase: "restore_db",
			},
			credential: &ConnectionCredential{Username: "root"},
			want:       "restore_db",
		},
		{
			name: "postgres falls back to username",
			item: &DatabaseInstance{
				DBType: DBTypePostgreSQL,
			},
			credential: &ConnectionCredential{Username: "postgres"},
			want:       "postgres",
		},
		{
			name: "mysql requires default database",
			item: &DatabaseInstance{
				DBType: DBTypeMySQL,
			},
			credential: &ConnectionCredential{Username: "root"},
			wantErr:    true,
		},
		{
			name: "redis uses all dbs label",
			item: &DatabaseInstance{
				DBType: DBTypeRedis,
			},
			credential: &ConnectionCredential{},
			want:       "all-dbs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveRestoreDatabaseName(tt.item, tt.credential)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveRestoreDatabaseName() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveRestoreDatabaseName() = %q, want %q", got, tt.want)
			}
		})
	}
}
