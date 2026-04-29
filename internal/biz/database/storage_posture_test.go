package database

import (
	"context"
	"strings"
	"testing"
)

func TestEvaluateStoragePostureReportPassed(t *testing.T) {
	profile := &DatabaseStorageProfile{
		StorageType:         "s3",
		VersioningEnabled:   true,
		ImmutabilityEnabled: true,
		KMSKeyID:            "kms-key-1",
		RetentionLockDays:   30,
	}
	report := storagePostureReport{
		Actual: storagePostureActual{
			BucketReachable:             true,
			VersioningStatus:            "Enabled",
			VersioningEnabled:           true,
			ObjectLockEnabled:           true,
			ObjectLockMode:              "GOVERNANCE",
			ObjectLockRetentionDays:     30,
			EncryptionEnabled:           true,
			EncryptionAlgorithm:         "aws:kms",
			EncryptionKMSKeyID:          "kms-key-1",
			BucketPolicyPublic:          "private",
			PublicAccessBlockConfigured: true,
			BlockPublicACLs:             true,
			IgnorePublicACLs:            true,
			BlockPublicPolicy:           true,
			RestrictPublicBuckets:       true,
		},
		Checks: []storagePostureCheck{{Key: "connectivity", Label: "Bucket 访问", Status: "passed"}},
	}
	got := evaluateStoragePostureReport(profile, report, false)
	if got.Status != "passed" {
		t.Fatalf("expected passed, got %s: %s", got.Status, got.Summary)
	}
	if len(got.Checks) != 6 {
		t.Fatalf("expected connectivity + 5 posture checks, got %d", len(got.Checks))
	}
}

func TestEvaluateStoragePostureReportWarnsOnRegisteredMismatch(t *testing.T) {
	profile := &DatabaseStorageProfile{
		StorageType:         "minio",
		VersioningEnabled:   true,
		ImmutabilityEnabled: true,
		KMSKeyID:            "kms-key-1",
		RetentionLockDays:   90,
	}
	report := storagePostureReport{
		Actual: storagePostureActual{
			BucketReachable:    true,
			VersioningStatus:   "Suspended",
			BucketPolicyPublic: "public",
		},
		Checks: []storagePostureCheck{{Key: "connectivity", Label: "Bucket 访问", Status: "passed"}},
	}
	got := evaluateStoragePostureReport(profile, report, false)
	if got.Status != "warning" {
		t.Fatalf("expected warning, got %s", got.Status)
	}
	summary := got.Summary
	for _, want := range []string{"版本化", "对象锁", "KMS", "公开"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing %q: %s", want, summary)
		}
	}
}

func TestBuildStoragePostureReportUnsupportedStorage(t *testing.T) {
	report, err := (&UseCase{}).buildStorageProfilePostureReport(context.Background(), &DatabaseStorageProfile{
		StorageType: DatabaseBackupStorageLocal,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Status != "unsupported" || len(report.Checks) != 1 {
		t.Fatalf("unexpected unsupported report: %#v", report)
	}
}

func TestStoragePostureS3OptionsRequireTemporaryCredentials(t *testing.T) {
	_, err := storagePostureS3OptionsFromRequest(&DatabaseStorageProfile{StorageType: "s3"}, &DatabaseStorageProfilePostureCheckRequest{})
	if err == nil || !strings.Contains(err.Error(), "临时提供") {
		t.Fatalf("expected temporary credential error, got %v", err)
	}
	useSSL := false
	usePathStyle := true
	opts, err := storagePostureS3OptionsFromRequest(&DatabaseStorageProfile{StorageType: "minio", Endpoint: "192.168.1.30:9000"}, &DatabaseStorageProfilePostureCheckRequest{
		AccessKey:    "ak",
		SecretKey:    "sk",
		UseSSL:       &useSSL,
		UsePathStyle: &usePathStyle,
	})
	if err != nil {
		t.Fatalf("unexpected options error: %v", err)
	}
	if opts.UseSSL || !opts.UsePathStyle {
		t.Fatalf("unexpected options: %#v", opts)
	}
}
