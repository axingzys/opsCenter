package database

import (
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestBuildRunnerToolProfileFromOutput(t *testing.T) {
	stdout := strings.Join([]string{
		"noise",
		"OPSHUB_PROFILE_FIELD os_family=ubuntu",
		"OPSHUB_PROFILE_FIELD os_version=22.04",
		"OPSHUB_PROFILE_FIELD os_pretty_name=Ubuntu 22.04.5 LTS",
		"OPSHUB_PROFILE_FIELD arch=x86_64",
		"OPSHUB_PROFILE_FIELD package_manager=apt",
		"OPSHUB_PROFILE_FIELD is_root=false",
		"OPSHUB_PROFILE_FIELD has_sudo=true",
		"OPSHUB_PROFILE_FIELD has_systemd=true",
		"OPSHUB_PROFILE_FIELD has_docker=true",
		"OPSHUB_PROFILE_FIELD tool.xtrabackup.path=/usr/bin/xtrabackup",
		"OPSHUB_PROFILE_FIELD tool.xtrabackup.version=xtrabackup version 8.0.35",
		"OPSHUB_PROFILE_FIELD tool.mysqlbinlog.path=/usr/bin/mysqlbinlog",
		"OPSHUB_PROFILE_FIELD tool.mysqlbinlog.version=mysqlbinlog Ver 8.0.44",
	}, "\n")
	profile, err := buildRunnerToolProfileFromOutput(9, stdout, time.Date(2026, 5, 3, 12, 0, 0, 0, time.Local), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.RunnerHostID != 9 || profile.OSFamily != "ubuntu" || profile.PackageManager != "apt" {
		t.Fatalf("unexpected profile: %+v", profile)
	}
	if !profile.HasSudo || !profile.HasDocker || profile.IsRoot {
		t.Fatalf("unexpected privilege flags: root=%v sudo=%v docker=%v", profile.IsRoot, profile.HasSudo, profile.HasDocker)
	}
	if !strings.Contains(profile.ToolManifestJSON, "xtrabackup") || !strings.Contains(profile.CapabilityJSON, "mysql_physical_backup") {
		t.Fatalf("tool/capability json not populated: %s / %s", profile.ToolManifestJSON, profile.CapabilityJSON)
	}
}

func TestBuildRunnerToolInstallScriptForAptMySQL80(t *testing.T) {
	host := &DatabaseRunnerHost{Model: gorm.Model{ID: 7}, Name: "runner-15", Host: "192.168.1.15", Port: 22}
	profile := &DatabaseRunnerToolProfile{
		RunnerHostID:   7,
		OSFamily:       "ubuntu",
		OSVersion:      "22.04",
		OSPrettyName:   "Ubuntu 22.04.5 LTS",
		Arch:           "x86_64",
		PackageManager: "apt",
		HasSudo:        true,
	}
	result := buildRunnerToolInstallScript(host, profile, &DatabaseRunnerToolInstallScriptRequest{
		Profiles:    []string{"mysql_80_physical", "mysql_binlog_archiver", "restore_runner"},
		InstallMode: "online",
		DryRun:      true,
	})
	if result.PackageManager != "apt" {
		t.Fatalf("unexpected package manager: %s", result.PackageManager)
	}
	for _, want := range []string{"percona-release setup pxb-80", "percona-xtrabackup-80", "mysql-client", "docker.io"} {
		if !strings.Contains(result.Script, want) {
			t.Fatalf("script missing %q:\n%s", want, result.Script)
		}
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("unexpected unsupported profiles: %+v", result.Unsupported)
	}
}

func TestBuildRunnerToolInstallScriptUnsupportedPackageManager(t *testing.T) {
	host := &DatabaseRunnerHost{Model: gorm.Model{ID: 7}, Name: "runner"}
	profile := &DatabaseRunnerToolProfile{RunnerHostID: 7, OSFamily: "suse", PackageManager: "zypper"}
	result := buildRunnerToolInstallScript(host, profile, &DatabaseRunnerToolInstallScriptRequest{Profiles: []string{"postgres_barman"}})
	if len(result.Unsupported) == 0 {
		t.Fatalf("expected unsupported warning")
	}
	if !strings.Contains(result.Script, "Unsupported package manager") {
		t.Fatalf("expected unsupported script, got:\n%s", result.Script)
	}
}
