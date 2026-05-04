package database

import (
	"encoding/json"
	"testing"
)

func TestBuildMySQLPITRWizardSyntheticRuleJSONDefaults(t *testing.T) {
	payload := buildMySQLPITRWizardSyntheticRuleJSON(&DatabaseMySQLPITRWizardRequest{}, &mysqlPITRWizardResolved{})
	var got map[string]any
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("unmarshal synthetic rule: %v", err)
	}
	if got["mode"] != "rolling_synthetic_full" {
		t.Fatalf("mode = %#v", got["mode"])
	}
	if got["autoRun"] != true || got["requireRestoreProof"] != true || got["neverDeleteWithoutProof"] != true {
		t.Fatalf("unexpected safety defaults: %#v", got)
	}
	if got["triggerAfterIncrementals"].(float64) != 5 || got["mergeOldestIncrementals"].(float64) != 5 {
		t.Fatalf("unexpected incremental defaults: %#v", got)
	}
}

func TestBuildMySQLPITRWizardSyntheticRuleJSONOverrides(t *testing.T) {
	falseValue := false
	req := &DatabaseMySQLPITRWizardRequest{
		SyntheticAutoRun:                  &falseValue,
		SyntheticTriggerAfterIncrementals: 7,
		SyntheticMergeOldestIncrementals:  3,
		SyntheticRequireRestoreProof:      &falseValue,
		SyntheticNeverDeleteWithoutProof:  &falseValue,
		SyntheticMarkSupersededAfterProof: &falseValue,
		SyntheticSupersededKeepDays:       11,
	}
	payload := buildMySQLPITRWizardSyntheticRuleJSON(req, &mysqlPITRWizardResolved{})
	var got map[string]any
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("unmarshal synthetic rule: %v", err)
	}
	if got["autoRun"] != false || got["requireRestoreProof"] != false || got["neverDeleteWithoutProof"] != false || got["markSupersededAfterProof"] != false {
		t.Fatalf("unexpected boolean overrides: %#v", got)
	}
	if got["triggerAfterIncrementals"].(float64) != 7 || got["mergeOldestIncrementals"].(float64) != 3 || got["supersededKeepDaysAfterProof"].(float64) != 11 {
		t.Fatalf("unexpected numeric overrides: %#v", got)
	}
}

func TestBuildMySQLPITRWizardRetentionJSONDefaultsAndOverrides(t *testing.T) {
	falseValue := false
	resolved := &mysqlPITRWizardResolved{BinlogRetentionDays: 33}
	req := &DatabaseMySQLPITRWizardRequest{
		RetentionFullKeepMonths:          12,
		RetentionIncrementalKeepDays:     90,
		RetentionNeverDeleteWithoutProof: &falseValue,
	}
	payload := buildMySQLPITRWizardRetentionJSON(req, resolved)
	var got map[string]any
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("unmarshal retention: %v", err)
	}
	if got["fullKeepMonths"].(float64) != 12 || got["incrementalKeepDays"].(float64) != 90 || got["binlogKeepDays"].(float64) != 33 {
		t.Fatalf("unexpected retention values: %#v", got)
	}
	if got["neverDeleteWithoutProof"] != false {
		t.Fatalf("unexpected proof retention default: %#v", got)
	}
}

func TestMySQLPITRWizardTemplateAndDefaults(t *testing.T) {
	if mysqlPITRWizardTemplateKey("") != "rolling_synthetic_full" {
		t.Fatalf("empty template key should use rolling_synthetic_full")
	}
	if mysqlPITRWizardTemplateName("conservative_pitr") == mysqlPITRWizardTemplateName("") {
		t.Fatalf("conservative template should have a distinct label")
	}
	trueValue := true
	if !boolDefault(&trueValue, false) || !boolDefault(nil, true) {
		t.Fatalf("boolDefault returned unexpected values")
	}
	if intDefault(0, 8) != 8 || intDefault(9, 8) != 9 {
		t.Fatalf("intDefault returned unexpected values")
	}
}
