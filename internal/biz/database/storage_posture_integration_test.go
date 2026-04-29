package database

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

func TestStoragePostureMinIOIntegration(t *testing.T) {
	endpoint := strings.TrimSpace(os.Getenv("OPSHUB_TEST_MINIO_ENDPOINT"))
	if endpoint == "" {
		t.Skip("set OPSHUB_TEST_MINIO_ENDPOINT to run MinIO posture integration test")
	}
	accessKey := firstStoragePostureValue(os.Getenv("OPSHUB_TEST_MINIO_ACCESS_KEY"), "minioadmin")
	secretKey := firstStoragePostureValue(os.Getenv("OPSHUB_TEST_MINIO_SECRET_KEY"), "minioadmin123")
	bucket := firstStoragePostureValue(os.Getenv("OPSHUB_TEST_MINIO_BUCKET"), "opshub-posture-test")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	profile := &DatabaseStorageProfile{
		Name:              "minio-posture-test",
		StorageType:       "minio",
		Endpoint:          endpoint,
		Bucket:            bucket,
		Region:            "us-east-1",
		VersioningEnabled: true,
	}
	opts := storagePostureS3Options{AccessKey: accessKey, SecretKey: secretKey, UsePathStyle: true}
	client := newStoragePostureS3Client(profile, opts)
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket:                     aws.String(bucket),
		ObjectLockEnabledForBucket: aws.Bool(true),
	})
	if err != nil && !isStoragePostureBucketAlreadyExists(err) {
		t.Fatalf("create minio bucket: %v", err)
	}
	_, _ = client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: aws.String(bucket),
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusEnabled,
		},
	})
	_, encryptionErr := client.PutBucketEncryption(ctx, &s3.PutBucketEncryptionInput{
		Bucket: aws.String(bucket),
		ServerSideEncryptionConfiguration: &types.ServerSideEncryptionConfiguration{
			Rules: []types.ServerSideEncryptionRule{{
				ApplyServerSideEncryptionByDefault: &types.ServerSideEncryptionByDefault{
					SSEAlgorithm: types.ServerSideEncryptionAes256,
				},
			}},
		},
	})

	report, err := (&UseCase{}).buildStorageProfilePostureReport(ctx, profile, &DatabaseStorageProfilePostureCheckRequest{
		AccessKey:    accessKey,
		SecretKey:    secretKey,
		UseSSL:       aws.Bool(false),
		UsePathStyle: aws.Bool(true),
	})
	if err != nil {
		t.Fatalf("build posture report: %v", err)
	}
	if report.Status == "failed" || !report.Actual.BucketReachable {
		t.Fatalf("expected reachable non-failed report, got status=%s summary=%s", report.Status, report.Summary)
	}
	if !storagePostureCheckPassed(report.Checks, "versioning") {
		t.Fatalf("expected versioning check to pass: %#v", report.Checks)
	}
	if encryptionErr != nil {
		t.Logf("minio default encryption setup skipped or unsupported: %v", encryptionErr)
	} else {
		t.Logf("minio default encryption setup accepted; reported encryption check can still vary by MinIO release: %#v", report.Checks)
	}
}

func isStoragePostureBucketAlreadyExists(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := strings.ToLower(apiErr.ErrorCode())
		return code == "bucketalreadyownedbyyou" || code == "bucketalreadyexists"
	}
	return false
}

func storagePostureCheckPassed(checks []storagePostureCheck, key string) bool {
	for _, check := range checks {
		if check.Key == key {
			return check.Status == "passed"
		}
	}
	return false
}
