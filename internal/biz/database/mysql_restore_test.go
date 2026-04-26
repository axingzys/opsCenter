package database

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

type recordingRestoreExecer struct {
	statements []string
}

func (r *recordingRestoreExecer) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	r.statements = append(r.statements, query)
	return nil, nil
}

func TestExecMySQLRestoreScriptSplitsDumpStatements(t *testing.T) {
	input := strings.NewReader(`
-- dump header;
/*!40101 SET NAMES utf8mb4 */;
CREATE TABLE ` + "`demo`" + ` (
  ` + "`id`" + ` bigint NOT NULL,
  ` + "`text`" + ` text,
  PRIMARY KEY (` + "`id`" + `)
);
INSERT INTO ` + "`demo`" + ` VALUES
(1,'semi;colon'),
(2,'escaped \' quote');
UNLOCK TABLES;
`)
	recorder := &recordingRestoreExecer{}
	if err := execMySQLRestoreScript(context.Background(), recorder, input); err != nil {
		t.Fatalf("execMySQLRestoreScript() error = %v", err)
	}
	if len(recorder.statements) != 4 {
		t.Fatalf("statement count = %d, want 4: %#v", len(recorder.statements), recorder.statements)
	}
	if !strings.Contains(recorder.statements[2], "semi;colon") {
		t.Fatalf("expected semicolon inside string to be preserved, got %q", recorder.statements[2])
	}
}

func TestRewriteGeneratedColumnInsert(t *testing.T) {
	state := &mysqlRestoreScriptState{tables: make(map[string][]mysqlRestoreTableColumn)}
	state.captureCreateTable("CREATE TABLE `sys_user` (`id` bigint, `deleted_at` datetime(3), `is_deleted` tinyint(1) GENERATED ALWAYS AS ((case when (`deleted_at` is null) then 0 else 1 end)) STORED, `name` varchar(20), PRIMARY KEY (`id`)) ENGINE=InnoDB")

	rewritten, ok, err := state.rewriteGeneratedColumnInsert("INSERT INTO `sys_user` VALUES\n(1,NULL,0,'a,b'),\n(2,'2026-01-01 00:00:00.000',1,'semi;colon')")
	if err != nil {
		t.Fatalf("rewriteGeneratedColumnInsert() error = %v", err)
	}
	if !ok {
		t.Fatalf("expected insert to be rewritten")
	}
	if strings.Contains(rewritten, "`is_deleted`") {
		t.Fatalf("generated column should be omitted: %s", rewritten)
	}
	if !strings.Contains(rewritten, "(`id`,`deleted_at`,`name`)") {
		t.Fatalf("expected explicit non-generated columns: %s", rewritten)
	}
	if strings.Contains(rewritten, "NULL,0,'a,b'") || !strings.Contains(rewritten, "NULL,'a,b'") {
		t.Fatalf("expected generated values to be removed: %s", rewritten)
	}
}
