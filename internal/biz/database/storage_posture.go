package database

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

const storagePostureCheckTimeout = 20 * time.Second

type storagePostureReport struct {
	CheckedAt   string                 `json:"checkedAt"`
	Provider    string                 `json:"provider"`
	Endpoint    string                 `json:"endpoint"`
	Bucket      string                 `json:"bucket"`
	Expected    storagePostureExpected `json:"expected"`
	Actual      storagePostureActual   `json:"actual"`
	Checks      []storagePostureCheck  `json:"checks"`
	Status      string                 `json:"status"`
	StatusText  string                 `json:"statusText"`
	Summary     string                 `json:"summary"`
	Warnings    []string               `json:"warnings,omitempty"`
	Unsupported []string               `json:"unsupported,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

type storagePostureExpected struct {
	VersioningEnabled   bool   `json:"versioningEnabled"`
	ImmutabilityEnabled bool   `json:"immutabilityEnabled"`
	KMSKeyID            string `json:"kmsKeyId"`
	RetentionLockDays   int    `json:"retentionLockDays"`
}

type storagePostureActual struct {
	BucketReachable             bool   `json:"bucketReachable"`
	BucketRegion                string `json:"bucketRegion,omitempty"`
	VersioningStatus            string `json:"versioningStatus,omitempty"`
	VersioningEnabled           bool   `json:"versioningEnabled"`
	ObjectLockEnabled           bool   `json:"objectLockEnabled"`
	ObjectLockMode              string `json:"objectLockMode,omitempty"`
	ObjectLockRetentionDays     int    `json:"objectLockRetentionDays,omitempty"`
	EncryptionEnabled           bool   `json:"encryptionEnabled"`
	EncryptionAlgorithm         string `json:"encryptionAlgorithm,omitempty"`
	EncryptionKMSKeyID          string `json:"encryptionKmsKeyId,omitempty"`
	BucketPolicyPublic          string `json:"bucketPolicyPublic,omitempty"`
	PublicAccessBlockConfigured bool   `json:"publicAccessBlockConfigured"`
	BlockPublicACLs             bool   `json:"blockPublicAcls"`
	IgnorePublicACLs            bool   `json:"ignorePublicAcls"`
	BlockPublicPolicy           bool   `json:"blockPublicPolicy"`
	RestrictPublicBuckets       bool   `json:"restrictPublicBuckets"`
}

type storagePostureCheck struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Status   string `json:"status"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Message  string `json:"message"`
}

type storagePostureS3Options struct {
	AccessKey          string
	SecretKey          string
	SessionToken       string
	UseSSL             bool
	UsePathStyle       bool
	InsecureSkipVerify bool
}

type storagePostureS3Observation struct {
	Actual      storagePostureActual
	Checks      []storagePostureCheck
	Warnings    []string
	Unsupported []string
	Failed      bool
}

func (uc *UseCase) CheckStorageProfilePosture(ctx context.Context, id uint, req *DatabaseStorageProfilePostureCheckRequest) (*DatabaseStorageProfileVO, error) {
	if uc.storageProfileRepo == nil {
		return nil, fmt.Errorf("存储配置仓库未配置")
	}
	item, err := uc.storageProfileRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("存储配置不存在")
	}
	report, err := uc.buildStorageProfilePostureReport(ctx, item, req)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	item.PostureStatus = normalizeStoragePostureStatus(report.Status)
	item.PostureSummary = trimText(report.Summary, 1000)
	item.PostureJSON = string(payload)
	item.LastPostureCheckAt = &now
	if err := uc.storageProfileRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	return toStorageProfileVO(item), nil
}

func (uc *UseCase) buildStorageProfilePostureReport(ctx context.Context, item *DatabaseStorageProfile, req *DatabaseStorageProfilePostureCheckRequest) (storagePostureReport, error) {
	provider := normalizeStorageProfileType(item.StorageType)
	expected := storagePostureExpected{
		VersioningEnabled:   item.VersioningEnabled,
		ImmutabilityEnabled: item.ImmutabilityEnabled,
		KMSKeyID:            strings.TrimSpace(item.KMSKeyID),
		RetentionLockDays:   item.RetentionLockDays,
	}
	report := storagePostureReport{
		CheckedAt:  time.Now().Format(time.RFC3339),
		Provider:   provider,
		Endpoint:   strings.TrimSpace(item.Endpoint),
		Bucket:     strings.TrimSpace(item.Bucket),
		Expected:   expected,
		Status:     "unknown",
		StatusText: StoragePostureStatusText("unknown"),
		Metadata: map[string]string{
			"pathPrefix": strings.TrimSpace(item.PathPrefix),
		},
	}
	if provider != "s3" && provider != "minio" {
		report.Status = "unsupported"
		report.StatusText = StoragePostureStatusText(report.Status)
		report.Summary = "当前存储类型暂不支持自动检测，请按存储平台控制台人工核验版本化、不可变保留、默认加密和访问策略。"
		report.Checks = []storagePostureCheck{{
			Key:      "provider",
			Label:    "存储类型",
			Status:   "unsupported",
			Expected: "S3 或 MinIO",
			Actual:   StorageProfileTypeText(provider),
			Message:  "P2.6.12 第一版仅对 S3/MinIO 执行主动检测。",
		}}
		return report, nil
	}
	if report.Bucket == "" {
		return report, fmt.Errorf("对象存储 bucket 不能为空")
	}
	opts, err := storagePostureS3OptionsFromRequest(item, req)
	if err != nil {
		return report, err
	}
	checkCtx, cancel := context.WithTimeout(ctx, storagePostureCheckTimeout)
	defer cancel()
	observation := inspectS3StoragePosture(checkCtx, item, opts)
	report.Actual = observation.Actual
	report.Checks = append(report.Checks, observation.Checks...)
	report.Warnings = append(report.Warnings, observation.Warnings...)
	report.Unsupported = append(report.Unsupported, observation.Unsupported...)
	report = evaluateStoragePostureReport(item, report, observation.Failed)
	return report, nil
}

func storagePostureS3OptionsFromRequest(item *DatabaseStorageProfile, req *DatabaseStorageProfilePostureCheckRequest) (storagePostureS3Options, error) {
	if req == nil {
		req = &DatabaseStorageProfilePostureCheckRequest{}
	}
	opts := storagePostureS3Options{
		AccessKey:    strings.TrimSpace(req.AccessKey),
		SecretKey:    strings.TrimSpace(req.SecretKey),
		SessionToken: strings.TrimSpace(req.SessionToken),
		UseSSL:       normalizeStorageProfileType(item.StorageType) == "s3",
		UsePathStyle: normalizeStorageProfileType(item.StorageType) == "minio" || strings.TrimSpace(item.Endpoint) != "",
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(item.Endpoint)), "https://") {
		opts.UseSSL = true
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(item.Endpoint)), "http://") {
		opts.UseSSL = false
	}
	if req.UseSSL != nil {
		opts.UseSSL = *req.UseSSL
	}
	if req.UsePathStyle != nil {
		opts.UsePathStyle = *req.UsePathStyle
	}
	if req.InsecureSkipVerify != nil {
		opts.InsecureSkipVerify = *req.InsecureSkipVerify
	}
	if opts.AccessKey == "" || opts.SecretKey == "" {
		return opts, fmt.Errorf("对象存储安全姿态检测需要临时提供 accessKey 和 secretKey，凭据仅用于本次检测且不会落库")
	}
	return opts, nil
}

func inspectS3StoragePosture(ctx context.Context, item *DatabaseStorageProfile, opts storagePostureS3Options) storagePostureS3Observation {
	client := newStoragePostureS3Client(item, opts)
	bucket := strings.TrimSpace(item.Bucket)
	result := storagePostureS3Observation{}
	if _, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)}); err != nil {
		result.Failed = true
		message := "无法访问 bucket: " + sanitizeStoragePostureError(err)
		result.Checks = append(result.Checks, storagePostureCheck{
			Key:      "connectivity",
			Label:    "Bucket 访问",
			Status:   "failed",
			Expected: "可访问",
			Actual:   "不可访问",
			Message:  message,
		})
		result.Warnings = append(result.Warnings, message)
		return result
	}
	result.Actual.BucketReachable = true
	result.Checks = append(result.Checks, storagePostureCheck{
		Key:      "connectivity",
		Label:    "Bucket 访问",
		Status:   "passed",
		Expected: "可访问",
		Actual:   "可访问",
		Message:  "Bucket 可访问，后续检测基于只读 Bucket API。",
	})
	result.inspectS3BucketLocation(ctx, client, bucket)
	result.inspectS3BucketVersioning(ctx, client, bucket)
	result.inspectS3ObjectLock(ctx, client, bucket)
	result.inspectS3BucketEncryption(ctx, client, bucket)
	result.inspectS3BucketPolicyStatus(ctx, client, bucket)
	result.inspectS3PublicAccessBlock(ctx, client, bucket)
	return result
}

func (r *storagePostureS3Observation) inspectS3BucketLocation(ctx context.Context, client *s3.Client, bucket string) {
	out, err := client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: aws.String(bucket)})
	if err != nil {
		r.Unsupported = append(r.Unsupported, "bucket location: "+sanitizeStoragePostureError(err))
		return
	}
	r.Actual.BucketRegion = string(out.LocationConstraint)
}

func (r *storagePostureS3Observation) inspectS3BucketVersioning(ctx context.Context, client *s3.Client, bucket string) {
	out, err := client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: aws.String(bucket)})
	if err != nil {
		r.Checks = append(r.Checks, storagePostureCheck{
			Key:      "versioning",
			Label:    "版本化",
			Status:   "unsupported",
			Expected: "可检测",
			Actual:   "未返回",
			Message:  sanitizeStoragePostureError(err),
		})
		r.Unsupported = append(r.Unsupported, "versioning: "+sanitizeStoragePostureError(err))
		return
	}
	r.Actual.VersioningStatus = string(out.Status)
	r.Actual.VersioningEnabled = out.Status == types.BucketVersioningStatusEnabled
}

func (r *storagePostureS3Observation) inspectS3ObjectLock(ctx context.Context, client *s3.Client, bucket string) {
	out, err := client.GetObjectLockConfiguration(ctx, &s3.GetObjectLockConfigurationInput{Bucket: aws.String(bucket)})
	if err != nil {
		r.Checks = append(r.Checks, storagePostureCheck{
			Key:      "object_lock",
			Label:    "对象锁",
			Status:   optionalStoragePostureStatus(err),
			Expected: "可检测",
			Actual:   "未启用或未返回",
			Message:  sanitizeStoragePostureError(err),
		})
		if optionalStoragePostureStatus(err) == "unsupported" {
			r.Unsupported = append(r.Unsupported, "object lock: "+sanitizeStoragePostureError(err))
		}
		return
	}
	if out.ObjectLockConfiguration == nil {
		return
	}
	r.Actual.ObjectLockEnabled = out.ObjectLockConfiguration.ObjectLockEnabled == types.ObjectLockEnabledEnabled
	if out.ObjectLockConfiguration.Rule == nil || out.ObjectLockConfiguration.Rule.DefaultRetention == nil {
		return
	}
	retention := out.ObjectLockConfiguration.Rule.DefaultRetention
	r.Actual.ObjectLockMode = string(retention.Mode)
	if retention.Days != nil {
		r.Actual.ObjectLockRetentionDays = int(*retention.Days)
	} else if retention.Years != nil {
		r.Actual.ObjectLockRetentionDays = int(*retention.Years) * 365
	}
}

func (r *storagePostureS3Observation) inspectS3BucketEncryption(ctx context.Context, client *s3.Client, bucket string) {
	out, err := client.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{Bucket: aws.String(bucket)})
	if err != nil {
		r.Checks = append(r.Checks, storagePostureCheck{
			Key:      "encryption",
			Label:    "默认加密",
			Status:   optionalStoragePostureStatus(err),
			Expected: "可检测",
			Actual:   "未启用或未返回",
			Message:  sanitizeStoragePostureError(err),
		})
		if optionalStoragePostureStatus(err) == "unsupported" {
			r.Unsupported = append(r.Unsupported, "encryption: "+sanitizeStoragePostureError(err))
		}
		return
	}
	if out.ServerSideEncryptionConfiguration == nil || len(out.ServerSideEncryptionConfiguration.Rules) == 0 {
		return
	}
	for _, rule := range out.ServerSideEncryptionConfiguration.Rules {
		defaultEncryption := rule.ApplyServerSideEncryptionByDefault
		if defaultEncryption == nil {
			continue
		}
		r.Actual.EncryptionEnabled = true
		r.Actual.EncryptionAlgorithm = string(defaultEncryption.SSEAlgorithm)
		if defaultEncryption.KMSMasterKeyID != nil {
			r.Actual.EncryptionKMSKeyID = *defaultEncryption.KMSMasterKeyID
		}
		return
	}
}

func (r *storagePostureS3Observation) inspectS3BucketPolicyStatus(ctx context.Context, client *s3.Client, bucket string) {
	out, err := client.GetBucketPolicyStatus(ctx, &s3.GetBucketPolicyStatusInput{Bucket: aws.String(bucket)})
	if err != nil {
		r.Checks = append(r.Checks, storagePostureCheck{
			Key:      "bucket_policy",
			Label:    "Bucket Policy",
			Status:   optionalStoragePostureStatus(err),
			Expected: "非公开",
			Actual:   "无法判断",
			Message:  sanitizeStoragePostureError(err),
		})
		r.Unsupported = append(r.Unsupported, "bucket policy status: "+sanitizeStoragePostureError(err))
		return
	}
	if out.PolicyStatus != nil && out.PolicyStatus.IsPublic != nil {
		if *out.PolicyStatus.IsPublic {
			r.Actual.BucketPolicyPublic = "public"
		} else {
			r.Actual.BucketPolicyPublic = "private"
		}
	}
}

func (r *storagePostureS3Observation) inspectS3PublicAccessBlock(ctx context.Context, client *s3.Client, bucket string) {
	out, err := client.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: aws.String(bucket)})
	if err != nil {
		r.Checks = append(r.Checks, storagePostureCheck{
			Key:      "public_access_block",
			Label:    "Public Access Block",
			Status:   optionalStoragePostureStatus(err),
			Expected: "已配置阻断公开访问",
			Actual:   "无法判断",
			Message:  sanitizeStoragePostureError(err),
		})
		r.Unsupported = append(r.Unsupported, "public access block: "+sanitizeStoragePostureError(err))
		return
	}
	if out.PublicAccessBlockConfiguration == nil {
		return
	}
	cfg := out.PublicAccessBlockConfiguration
	r.Actual.PublicAccessBlockConfigured = true
	r.Actual.BlockPublicACLs = cfg.BlockPublicAcls != nil && *cfg.BlockPublicAcls
	r.Actual.IgnorePublicACLs = cfg.IgnorePublicAcls != nil && *cfg.IgnorePublicAcls
	r.Actual.BlockPublicPolicy = cfg.BlockPublicPolicy != nil && *cfg.BlockPublicPolicy
	r.Actual.RestrictPublicBuckets = cfg.RestrictPublicBuckets != nil && *cfg.RestrictPublicBuckets
}

func evaluateStoragePostureReport(item *DatabaseStorageProfile, report storagePostureReport, probeFailed bool) storagePostureReport {
	if probeFailed {
		report.Status = "failed"
		report.StatusText = StoragePostureStatusText(report.Status)
		report.Summary = firstStoragePostureText(report.Warnings, "对象存储安全姿态检测失败。")
		return report
	}
	actual := report.Actual
	checks := make([]storagePostureCheck, 0, len(report.Checks)+6)
	for _, check := range report.Checks {
		if check.Key != "versioning" && check.Key != "object_lock" && check.Key != "encryption" && check.Key != "bucket_policy" && check.Key != "public_access_block" {
			checks = append(checks, check)
		}
	}
	checks = append(checks, evaluateVersioningPosture(item.VersioningEnabled, actual))
	checks = append(checks, evaluateObjectLockPosture(item.ImmutabilityEnabled, item.RetentionLockDays, actual))
	checks = append(checks, evaluateEncryptionPosture(item.KMSKeyID, actual))
	checks = append(checks, evaluateBucketPolicyPosture(actual))
	checks = append(checks, evaluatePublicAccessBlockPosture(actual))
	report.Checks = checks
	report.Status = aggregateStoragePostureStatus(checks)
	report.StatusText = StoragePostureStatusText(report.Status)
	report.Summary = summarizeStoragePosture(report)
	return report
}

func evaluateVersioningPosture(expected bool, actual storagePostureActual) storagePostureCheck {
	check := storagePostureCheck{
		Key:      "versioning",
		Label:    "版本化",
		Expected: expectedEnabledText(expected),
		Actual:   actual.VersioningStatus,
	}
	if check.Actual == "" {
		check.Actual = "未启用"
	}
	if actual.VersioningEnabled {
		check.Status = "passed"
		check.Message = "Bucket 已启用版本化，可降低误删覆盖后无法找回对象的风险。"
		return check
	}
	check.Status = "warning"
	if expected {
		check.Message = "登记要求启用版本化，但检测值未启用。"
	} else {
		check.Message = "未检测到版本化；长期备份仓库建议启用版本化。"
	}
	return check
}

func evaluateObjectLockPosture(expected bool, expectedDays int, actual storagePostureActual) storagePostureCheck {
	check := storagePostureCheck{
		Key:      "object_lock",
		Label:    "对象锁/不可变保留",
		Expected: objectLockExpectedText(expected, expectedDays),
		Actual:   objectLockActualText(actual),
	}
	if actual.ObjectLockEnabled && (expectedDays <= 0 || actual.ObjectLockRetentionDays >= expectedDays) {
		check.Status = "passed"
		check.Message = "Bucket 对象锁或默认保留配置满足当前登记要求。"
		return check
	}
	check.Status = "warning"
	if expected {
		check.Message = "登记要求不可变保留，但检测值未满足对象锁或保留天数要求。"
	} else {
		check.Message = "未检测到不可变保留；关键备份建议启用对象锁或平台侧不可变策略。"
	}
	return check
}

func evaluateEncryptionPosture(expectedKMS string, actual storagePostureActual) storagePostureCheck {
	expectedKMS = strings.TrimSpace(expectedKMS)
	check := storagePostureCheck{
		Key:      "encryption",
		Label:    "默认加密/KMS",
		Expected: encryptionExpectedText(expectedKMS),
		Actual:   encryptionActualText(actual),
	}
	if expectedKMS != "" {
		if actual.EncryptionEnabled && strings.EqualFold(strings.TrimSpace(actual.EncryptionKMSKeyID), expectedKMS) {
			check.Status = "passed"
			check.Message = "默认加密 KMS Key 与登记值一致。"
			return check
		}
		check.Status = "warning"
		check.Message = "登记了 KMS Key，但检测值为空或不一致。"
		return check
	}
	if actual.EncryptionEnabled {
		check.Status = "passed"
		check.Message = "Bucket 已启用默认服务端加密。"
		return check
	}
	check.Status = "warning"
	check.Message = "未检测到默认服务端加密；备份仓库建议启用默认加密。"
	return check
}

func evaluateBucketPolicyPosture(actual storagePostureActual) storagePostureCheck {
	check := storagePostureCheck{
		Key:      "bucket_policy",
		Label:    "Bucket Policy 公开性",
		Expected: "非公开",
		Actual:   firstStoragePostureValue(actual.BucketPolicyPublic, "无法判断"),
	}
	switch actual.BucketPolicyPublic {
	case "public":
		check.Status = "warning"
		check.Message = "检测到 Bucket Policy 可能公开，请确认备份仓库只允许受控主体读写。"
	case "private":
		check.Status = "passed"
		check.Message = "Bucket Policy 状态为非公开。"
	default:
		check.Status = "unsupported"
		check.Message = "当前平台或凭据未返回 Bucket Policy 公开性，需在对象存储控制台人工核验。"
	}
	return check
}

func evaluatePublicAccessBlockPosture(actual storagePostureActual) storagePostureCheck {
	check := storagePostureCheck{
		Key:      "public_access_block",
		Label:    "公开访问阻断",
		Expected: "四项阻断均开启",
		Actual:   publicAccessBlockActualText(actual),
	}
	if actual.PublicAccessBlockConfigured && actual.BlockPublicACLs && actual.IgnorePublicACLs && actual.BlockPublicPolicy && actual.RestrictPublicBuckets {
		check.Status = "passed"
		check.Message = "Public Access Block 已完整开启。"
		return check
	}
	if !actual.PublicAccessBlockConfigured {
		check.Status = "unsupported"
		check.Message = "当前平台或凭据未返回 Public Access Block 配置，MinIO 等兼容存储可能不支持该 API。"
		return check
	}
	check.Status = "warning"
	check.Message = "Public Access Block 未完整开启。"
	return check
}

func aggregateStoragePostureStatus(checks []storagePostureCheck) string {
	hasWarning := false
	hasUnsupported := false
	for _, check := range checks {
		switch normalizeStoragePostureStatus(check.Status) {
		case "failed":
			return "failed"
		case "warning":
			hasWarning = true
		case "unsupported":
			hasUnsupported = true
		}
	}
	if hasWarning || hasUnsupported {
		return "warning"
	}
	return "passed"
}

func summarizeStoragePosture(report storagePostureReport) string {
	var warnings []string
	var unsupported int
	for _, check := range report.Checks {
		switch normalizeStoragePostureStatus(check.Status) {
		case "warning", "failed":
			warnings = append(warnings, check.Label+": "+check.Message)
		case "unsupported":
			unsupported++
		}
	}
	if len(warnings) == 0 && unsupported == 0 {
		return "对象存储安全姿态检测通过，登记值与检测值未发现明显差异。"
	}
	if len(warnings) == 0 {
		return fmt.Sprintf("对象存储可访问；%d 项能力未被当前平台或凭据支持，需要人工核验。", unsupported)
	}
	summary := strings.Join(warnings, "；")
	if unsupported > 0 {
		summary += fmt.Sprintf("；另有 %d 项能力需人工核验", unsupported)
	}
	return trimText(summary, 1000)
}

func newStoragePostureS3Client(item *DatabaseStorageProfile, opts storagePostureS3Options) *s3.Client {
	region := strings.TrimSpace(item.Region)
	if region == "" {
		region = "us-east-1"
	}
	awsCfg := aws.Config{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider(opts.AccessKey, opts.SecretKey, opts.SessionToken),
	}
	if opts.InsecureSkipVerify {
		awsCfg.HTTPClient = &http.Client{Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // 用户显式选择用于内网自签名 MinIO 检测。
		}}
	}
	endpoint := normalizeStoragePostureEndpoint(item.Endpoint, opts.UseSSL)
	return s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.UsePathStyle = opts.UsePathStyle
		if endpoint != "" {
			options.BaseEndpoint = aws.String(endpoint)
		}
	})
}

func normalizeStoragePostureEndpoint(endpoint string, useSSL bool) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	lower := strings.ToLower(endpoint)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return endpoint
	}
	if useSSL {
		return "https://" + endpoint
	}
	return "http://" + endpoint
}

func optionalStoragePostureStatus(err error) string {
	code := storagePostureErrorCode(err)
	switch strings.ToLower(code) {
	case "notimplemented", "not supported", "xnotimplemented", "unsupported", "nosuchbucketpolicy", "nosuchpublicaccessblockconfiguration", "objectlockconfigurationnotfounderror", "serversideencryptionconfigurationnotfounderror", "notfound", "404":
		return "unsupported"
	default:
		return "warning"
	}
}

func storagePostureErrorCode(err error) string {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return strings.TrimSpace(apiErr.ErrorCode())
	}
	return ""
}

func sanitizeStoragePostureError(err error) string {
	if err == nil {
		return ""
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := strings.TrimSpace(apiErr.ErrorCode())
		message := strings.TrimSpace(apiErr.ErrorMessage())
		if message == "" {
			message = err.Error()
		}
		return trimText(strings.TrimSpace(code+" "+message), 300)
	}
	return trimText(err.Error(), 300)
}

func expectedEnabledText(enabled bool) string {
	if enabled {
		return "启用"
	}
	return "未登记要求"
}

func objectLockExpectedText(enabled bool, days int) string {
	if !enabled && days <= 0 {
		return "未登记要求"
	}
	if days > 0 {
		return fmt.Sprintf("启用，默认保留不少于 %d 天", days)
	}
	return "启用"
}

func objectLockActualText(actual storagePostureActual) string {
	if !actual.ObjectLockEnabled {
		return "未启用"
	}
	parts := []string{"已启用"}
	if actual.ObjectLockMode != "" {
		parts = append(parts, actual.ObjectLockMode)
	}
	if actual.ObjectLockRetentionDays > 0 {
		parts = append(parts, fmt.Sprintf("%d 天", actual.ObjectLockRetentionDays))
	}
	return strings.Join(parts, " / ")
}

func encryptionExpectedText(kms string) string {
	kms = strings.TrimSpace(kms)
	if kms == "" {
		return "启用默认加密"
	}
	return "KMS: " + kms
}

func encryptionActualText(actual storagePostureActual) string {
	if !actual.EncryptionEnabled {
		return "未启用"
	}
	parts := []string{"已启用"}
	if actual.EncryptionAlgorithm != "" {
		parts = append(parts, actual.EncryptionAlgorithm)
	}
	if actual.EncryptionKMSKeyID != "" {
		parts = append(parts, actual.EncryptionKMSKeyID)
	}
	return strings.Join(parts, " / ")
}

func publicAccessBlockActualText(actual storagePostureActual) string {
	if !actual.PublicAccessBlockConfigured {
		return "未配置或无法判断"
	}
	return fmt.Sprintf("blockACL=%t ignoreACL=%t blockPolicy=%t restrict=%t", actual.BlockPublicACLs, actual.IgnorePublicACLs, actual.BlockPublicPolicy, actual.RestrictPublicBuckets)
}

func firstStoragePostureText(values []string, fallback string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return fallback
}

func firstStoragePostureValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
