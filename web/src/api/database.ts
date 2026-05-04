import request from '@/utils/request'

export interface DatabaseSupportedType {
  type: string
  name: string
  defaultPort: number
  metadataEnabled: boolean
  queryEnabled: boolean
  testEnabled: boolean
  topologyEnabled?: boolean
  phase: string
}

export interface DatabaseInstancePayload {
  name: string
  dbType: string
  host: string
  port?: number
  defaultDatabase?: string
  credentialId: number
  tlsEnabled?: boolean
  connectionParams?: string
  status?: 'enabled' | 'disabled'
  environment?: string
  businessSystem?: string
  owner?: string
  tags?: string
  remark?: string
}

export interface DatabaseInstanceResult extends DatabaseInstancePayload {
  id: number
  dbTypeText: string
  engine: string
  version: string
  endpoint: string
  status: 'enabled' | 'disabled'
  statusText: string
  lastTestAt?: string
  lastSyncAt?: string
  permissions?: number
  createdAt: string
  updatedAt: string
}

export const DATABASE_PERMISSION = {
  VIEW: 1,
  QUERY: 2,
  EXPORT: 4,
  WRITE: 8,
  BACKUP: 16,
  RESTORE: 32,
  DIAGNOSIS: 64,
  TOPOLOGY: 128,
  MANAGE: 256,
  QUERY_UNLIMITED: 512,
  WRITE_EXPLAIN: 1024,
  DDL: 2048,
  ALL: 4095
} as const

export interface DatabaseInstancePermissionPayload {
  roleId: number
  instanceId: number
  permissions: number
}

export interface DatabaseQueryPayload {
  schemaName?: string
  sqlText: string
  limit?: number
  timeoutSeconds?: number
  unlimitedRows?: boolean
}

export interface DatabaseQueryFormatPayload {
  sqlText: string
}

export interface DatabaseWriteValidatePayload {
  schemaName?: string
  sqlText: string
}

export interface DatabaseWriteValidateResult {
  instanceId: number
  instanceName: string
  dbType: string
  dbTypeText: string
  schemaName: string
  sqlType: string
  riskLevel: string
  riskLevelText: string
  allowed: boolean
  confirmRequired: boolean
  reasonRequired: boolean
  rowsAffectedLimit: number
  message: string
}

export interface DatabaseDDLValidateResult {
  instanceId: number
  instanceName: string
  dbType: string
  dbTypeText: string
  schemaName: string
  sqlType: string
  riskLevel: string
  riskLevelText: string
  allowed: boolean
  confirmRequired: boolean
  reasonRequired: boolean
  backupRequired: boolean
  message: string
}

export interface DatabaseWriteExecutePayload {
  schemaName?: string
  sqlText: string
  reason?: string
  confirmed?: boolean
  timeoutSeconds?: number
}

export interface DatabaseWriteExecuteResult {
  auditId: number
  auditAction?: string
  instanceId: number
  instanceName: string
  dbType: string
  dbTypeText: string
  schemaName: string
  sqlType: string
  riskLevel: string
  riskLevelText: string
  executedSql: string
  rowsAffected: number
  rowsAffectedLimit: number
  reason: string
  confirmRequired: boolean
  confirmed: boolean
  rollbackSql: string
  durationMs: number
  message: string
  executedAt: string
}

export interface DatabaseBackupTaskPayload {
  instanceId: number
  name: string
  backupType?: string
  backupMethod?: string
  backupLevel?: string
  backupEngine?: string
  sourceInstanceId?: number
  sourceRole?: string
  storageProfileId?: number
  secretProfileId?: number
  backupScope?: string
  scopeConfig?: string
  rpoMinutes?: number
  rtoMinutes?: number
  schedule?: string
  storageType?: string
  storageConfig?: string
  retentionDays?: number
  maxDurationMinutes?: number
  compression?: string
  encryptionEnabled?: boolean
  enabled: boolean
}

export interface DatabaseBackupTaskResult {
  id: number
  instanceId: number
  instanceName: string
  instanceDbType: string
  instanceDbTypeText: string
  name: string
  backupType: string
  backupTypeText: string
  backupMethod: string
  backupMethodText: string
  backupLevel: string
  backupLevelText: string
  backupEngine: string
  sourceInstanceId: number
  sourceRole: string
  storageProfileId: number
  secretProfileId: number
  backupScope: string
  scopeConfig: string
  rpoMinutes: number
  rtoMinutes: number
  schedule: string
  storageType: string
  storageTypeText: string
  storageConfig?: string
  retentionDays: number
  maxDurationMinutes: number
  compression?: string
  encryptionEnabled?: boolean
  enabled: boolean
  nextRunAt: string
  lastRunAt: string
  lastSuccessAt: string
  lastRestoreTestAt: string
  lastStatus: string
  lastStatusText: string
  lastMessage: string
  restoreCapability: string
  restoreCapabilityText: string
  pitrSupported: boolean
  pitrStatusText: string
  strategyText: string
  instanceCapacitySizeBytes: number
  instanceCapacitySizeText: string
  largeDataWarning: boolean
  largeDataWarningText: string
  createdAt: string
  updatedAt: string
}

export interface DatabaseBackupAlertThreshold {
  noSuccessBackupHours: number
  restoreDrillStaleDays: number
  rpoLagGraceMinutes: number
  includeNonProduction: boolean
  minRiskLevel: string
}

export interface DatabaseBackupAlertRulePayload {
  name: string
  enabled: boolean
  scopeType?: string
  instanceId?: number
  engine?: string
  businessSystem?: string
  owner?: string
  issueTypes: string[]
  severity?: string
  alertInterval: number
  recoveryNotify: boolean
  channelIds: number[]
  threshold: DatabaseBackupAlertThreshold
  description?: string
}

export interface DatabaseBackupAlertRuleResult extends DatabaseBackupAlertRulePayload {
  id: number
  scopeTypeText: string
  issueTypeTexts: string[]
  severityText: string
  createdAt: string
  updatedAt: string
}

export interface DatabaseBackupAlertStateResult {
  id: number
  ruleId: number
  fingerprint: string
  instanceId: number
  resourceType: string
  resourceId: number
  resourceName: string
  resourceTarget: string
  issueType: string
  issueTypeText: string
  alertType: string
  metric: string
  metricText: string
  severity: string
  severityText: string
  status: string
  statusText: string
  message: string
  suggestion: string
  firstFiredAt: string
  lastFiredAt: string
  lastNotifiedAt?: string
  resolvedAt?: string
  notifyCount: number
  createdAt: string
  updatedAt: string
}

export interface DatabaseBackupAlertSummaryResult {
  ruleTotal: number
  ruleEnabled: number
  firingTotal: number
  criticalFiring: number
  warningFiring: number
  notified24h: number
}

export interface DatabaseBackupAlertTestResult {
  status: string
  message: string
  channel: string
  error: string
}

export interface DatabaseBackupPolicyPayload {
  instanceId: number
  sourceInstanceId?: number
  sourceRole?: string
  name: string
  backupEngine?: string
  toolExecutionMode?: string
  toolImage?: string
  toolImageDigest?: string
  containerDatadirPath?: string
  containerWorkdirPath?: string
  containerNetworkMode?: string
  containerDatadirRo?: boolean
  runnerHostId: number
  storageProfileId?: number
  secretProfileId?: number
  binlogStreamId?: number
  fullSchedule?: string
  incrementalSchedule?: string
  syntheticEnabled?: boolean
  syntheticRuleJson?: string
  restoreDrillRequired?: boolean
  retentionJson?: string
  enabled: boolean
}

export interface DatabaseBackupChainStateResult {
  id: number
  policyId: number
  instanceId: number
  chainId: string
  currentBaseRecordId: number
  latestRecordId: number
  latestFullRecordId: number
  latestSyntheticRecordId: number
  incrementalCount: number
  chainStartedAt: string
  lastSuccessAt: string
  recoverableUntil: string
  status: string
  statusText: string
  lastValidationStatus: string
  lastError: string
}

export interface DatabaseBackupChainRecordResult {
  id: number
  chainId: string
  baseRecordId: number
  parentRecordId: number
  backupLevel: string
  backupLevelText: string
  backupOrigin: string
  checkpointFromLsn: string
  checkpointToLsn: string
  checkpointLastLsn: string
  fileName: string
  fileSize: number
  checksumSha256: string
  artifactState: string
  storageUri: string
  recoverableFrom: string
  recoverableUntil: string
  status: string
  statusText: string
  startedAt: string
  finishedAt: string
  serverUuid: string
  backupBinlogFile: string
  backupBinlogPos: number
  backupGtidSet: string
  syntheticSourceRecordIds: string
  supersededByRecordId: number
}

export interface DatabaseBackupPolicyChainValidationResult {
  policyId: number
  status: string
  statusText: string
  backupChainStatus: string
  backupChainStatusText: string
  selectedRecordIds: number[]
  baseRecord?: DatabaseBackupChainRecordResult
  latestRecord?: DatabaseBackupChainRecordResult
  records: DatabaseBackupChainRecordResult[]
  blockingReasons: string[]
  warnings: string[]
  messages: string[]
  checkedAt: string
  chain?: DatabaseBackupChainStateResult
}

export interface DatabaseSyntheticFullPreviewResult {
  policyId: number
  status: string
  statusText: string
  selectedBaseRecordId: number
  selectedIncrementalRecordIds: number[]
  selectedRecordIds: number[]
  selectedRecords: DatabaseBackupChainRecordResult[]
  newSyntheticFullAfterRecordId: number
  estimatedInputSize: number
  estimatedInputSizeText: string
  estimatedWorkdirSize: number
  estimatedWorkdirSizeText: string
  mergeIncrementalCount: number
  requiresRestoreProof: boolean
  blockingReasons: string[]
  warnings: string[]
  messages: string[]
  checkedAt: string
}

export interface DatabaseSyntheticFullJobsResult {
  items: DatabaseRunnerJobResult[]
  total: number
}

export interface DatabaseBackupPolicyPurgeRecordResult {
  recordId: number
  backupLevel: string
  backupOrigin: string
  fileName: string
  fileSize: number
  fileSizeText: string
  storageUri: string
  filePath: string
  checksumSha256: string
  status: string
  statusText: string
  artifactState: string
  supersededByRecordId: number
  purgeEligibleAt: string
  protectedUntil: string
  restoreTestStatus: string
  restoreTestStatusText: string
  blockingReason?: string
}

export interface DatabaseBackupPolicyPurgePreviewResult {
  policyId: number
  syntheticRecordId: number
  requiredProofRecordId: number
  proofStatus: string
  proofStatusText: string
  binlogCoverageStatus: string
  binlogCoverageText: string
  retentionDays: number
  neverDeleteWithoutProof: boolean
  eligibleRecordIds: number[]
  blockedRecordIds: number[]
  storageDeletePlan: DatabaseBackupPolicyPurgeRecordResult[]
  blockedRecords: DatabaseBackupPolicyPurgeRecordResult[]
  blockingReasons: string[]
  warnings: string[]
  messages: string[]
  checkedAt: string
}

export interface DatabaseBackupPolicyPurgeRunResult {
  policyId: number
  syntheticRecordId: number
  purgedRecordIds: number[]
  skippedRecordIds: number[]
  storageDeletePlan: DatabaseBackupPolicyPurgeRecordResult[]
  message: string
  executedAt: string
}

export interface DatabaseBackupPolicyResult {
  id: number
  instanceId: number
  instanceName: string
  instanceDbType: string
  sourceInstanceId: number
  sourceRole: string
  name: string
  engine: string
  backupEngine: string
  toolExecutionMode: string
  toolImage: string
  toolImageDigest: string
  containerDatadirPath: string
  containerWorkdirPath: string
  containerNetworkMode: string
  containerDatadirRo: boolean
  runnerHostId: number
  runnerHostName: string
  storageProfileId: number
  secretProfileId: number
  binlogStreamId: number
  fullSchedule: string
  incrementalSchedule: string
  syntheticEnabled: boolean
  syntheticRuleJson: string
  restoreDrillRequired: boolean
  retentionJson: string
  enabled: boolean
  status: string
  statusText: string
  nextFullRunAt: string
  nextIncrementalRunAt: string
  lastRunAt: string
  lastFullAt: string
  lastIncrementalAt: string
  lastSyntheticAt: string
  lastRestoreDrillAt: string
  lastStatus: string
  lastStatusText: string
  lastMessage: string
  lastError: string
  chain?: DatabaseBackupChainStateResult
  createdAt: string
  updatedAt: string
}

export interface DatabaseBackupPolicyRunResult {
  policyId: number
  policyName: string
  recordId: number
  runnerJobId: number
  instanceId: number
  instanceName: string
  backupLevel: string
  status: string
  statusText: string
  fileName: string
  message: string
  triggeredAt: string
}

export interface DatabaseProtectionActionResult {
  level: string
  action: string
  text: string
  blocking: boolean
}

export interface DatabaseProtectionProfileResult {
  profileId: string
  instanceId: number
  instanceName: string
  engine: string
  engineText: string
  version: string
  endpoint: string
  environment: string
  businessSystem: string
  owner: string
  protectionMode: string
  protectionModeText: string
  protectionLevel: string
  protectionLevelText: string
  healthStatus: string
  healthStatusText: string
  riskLevel: string
  riskLevelText: string
  riskMessages: string[]
  recoverableFrom: string
  recoverableUntil: string
  rpoLagSeconds: number
  lastFullAt: string
  lastIncrementalAt: string
  lastSyntheticAt: string
  lastLogArchiveAt: string
  lastRestoreDrillAt: string
  restoreDrillStatus: string
  restoreDrillStatusText: string
  runnerStatus: string
  runnerStatusText: string
  storageStatus: string
  storageStatusText: string
  backupChainStatus: string
  backupChainStatusText: string
  logChainStatus: string
  logChainStatusText: string
  replicaProtectionStatus: string
  replicaProtectionText: string
  pgSystemIdentifier?: string
  timelineId?: string
  walStart?: string
  walEnd?: string
  walGapCount?: number
  timelineMismatch?: boolean
  backupPolicy?: DatabaseBackupPolicyResult
  logicalTask?: DatabaseBackupTaskResult
  latestBackupRecord?: DatabaseBackupRecordResult
  latestLogArchive?: DatabaseLogArchiveResult
  latestRestoreJob?: DatabaseRestoreJobResult
  logArchiveStream?: DatabaseLogArchiveStreamResult
  barmanServer?: DatabaseBarmanServerResult
  runnerHost?: DatabaseRunnerHostResult
  storageProfile?: DatabaseStorageProfileResult
  replicaProtection?: DatabaseReplicaProtectionResult
  recommendedActions: DatabaseProtectionActionResult[]
  validatedAt: string
}

export interface DatabaseMySQLPITRWizardPayload {
  instanceId: number
  sourceInstanceId?: number
  sourceRole?: string
  runnerHostId: number
  storageProfileId?: number
  secretProfileId?: number
  templateKey?: string
  policyName?: string
  backupEngine?: string
  toolExecutionMode?: string
  toolImage?: string
  toolImageDigest?: string
  containerDatadirPath?: string
  containerWorkdirPath?: string
  containerNetworkMode?: string
  containerDatadirRo?: boolean
  reuseLogArchiveStreamId?: number
  reuseBackupPolicyId?: number
  fullSchedule?: string
  incrementalSchedule?: string
  runInitialFullNow?: boolean
  binlogArchiveMode?: string
  binlogRpoTargetSeconds?: number
  binlogRetentionDays?: number
  syntheticEnabled?: boolean
  syntheticAutoRun?: boolean
  syntheticTriggerAfterIncrementals?: number
  syntheticMergeOldestIncrementals?: number
  syntheticRequireRestoreProof?: boolean
  syntheticNeverDeleteWithoutProof?: boolean
  syntheticMarkSupersededAfterProof?: boolean
  syntheticSupersededKeepDays?: number
  restoreDrillRequired?: boolean
  retentionFullKeepMonths?: number
  retentionIncrementalKeepDays?: number
  retentionBinlogKeepDays?: number
  retentionNeverDeleteWithoutProof?: boolean
  archiveConfigJson?: string
}

export interface DatabaseProtectionWizardActionResult {
  action: string
  resourceType: string
  resourceId?: number
  name: string
  status: string
  message: string
  blocking: boolean
}

export interface DatabaseMySQLPITRWizardResult {
  templateKey: string
  templateName: string
  instanceId: number
  instanceName: string
  engine: string
  engineText: string
  version: string
  runnerHostId: number
  runnerHostName: string
  backupEngine: string
  fullSchedule: string
  incrementalSchedule: string
  runInitialFullNow: boolean
  syntheticRuleJson: string
  retentionJson: string
  canApply: boolean
  blockingReasons: string[]
  warnings: string[]
  messages: string[]
  actions: DatabaseProtectionWizardActionResult[]
  logArchiveStream?: DatabaseLogArchiveStreamResult
  backupPolicy?: DatabaseBackupPolicyResult
  initialFullRun?: DatabaseBackupPolicyRunResult
  profile?: DatabaseProtectionProfileResult
}

export interface DatabasePostgresBarmanPITRWizardPayload {
  instanceId: number
  runnerHostId: number
  reuseBarmanServerId?: number
  name?: string
  barmanServerName?: string
  barmanHome?: string
  configPath?: string
  retentionPolicy?: string
  backupMethod?: string
  streamingArchiverEnabled?: boolean
  archiverEnabled?: boolean
  slotName?: string
  configJson?: string
  runCheckNow?: boolean
  syncCatalogNow?: boolean
  syncWalNow?: boolean
  runInitialBackupNow?: boolean
}

export interface DatabasePostgresBarmanPITRWizardResult {
  templateKey: string
  templateName: string
  instanceId: number
  instanceName: string
  engine: string
  engineText: string
  version: string
  runnerHostId: number
  runnerHostName: string
  barmanServerName: string
  canApply: boolean
  blockingReasons: string[]
  warnings: string[]
  messages: string[]
  actions: DatabaseProtectionWizardActionResult[]
  barmanServer?: DatabaseBarmanServerResult
  logArchiveStream?: DatabaseLogArchiveStreamResult
  checkJob?: DatabaseRunnerJobResult
  catalogSyncJob?: DatabaseRunnerJobResult
  walSyncJob?: DatabaseRunnerJobResult
  initialBackupJob?: DatabaseRunnerJobResult
  profile?: DatabaseProtectionProfileResult
}

export interface DatabaseProtectionRiskResult {
  id: string
  profileId: string
  instanceId: number
  instanceName: string
  engine: string
  engineText: string
  endpoint: string
  environment: string
  businessSystem: string
  owner: string
  protectionMode: string
  protectionModeText: string
  protectionLevel: string
  protectionLevelText: string
  riskLevel: string
  riskLevelText: string
  issueType: string
  issueTypeText: string
  message: string
  action: string
  actionText: string
  blocking: boolean
  recoverableUntil: string
  lastFullAt: string
  lastLogArchiveAt: string
  checkedAt: string
}

export interface DatabaseProtectionRestoreDrillPayload {
  restoreTargetType?: string
  restoreTargetValue?: string
  targetTimelineId?: string
  restoreTargetInclusive?: boolean
  runnerHostId?: number
  containerImage?: string
  listenPort?: number
  expiresInHours?: number
  validationSql?: string[]
  validationAssertions?: DatabaseRestoreValidationAssertionPayload[]
  cleanupOnFailure?: boolean
  postgresStartInstance?: boolean
  targetAction?: string
  barmanGetWal?: boolean
}

export interface DatabaseProtectionRestoreDrillResult {
  profileId: string
  plan?: DatabaseRestorePlanResult
  job?: DatabaseRestoreJobResult
  profile?: DatabaseProtectionProfileResult
  status: string
  message: string
}

export interface DatabaseBackupRecordResult {
  id: number
  taskId: number
  policyId?: number
  taskName: string
  instanceId: number
  instanceName: string
  triggerType: string
  triggerTypeText: string
  backupType: string
  backupTypeText: string
  chainId?: string
  baseRecordId?: number
  parentRecordId?: number
  backupMethod?: string
  backupMethodText?: string
  backupLevel?: string
  backupLevelText?: string
  backupEngine?: string
  externalBackupId?: string
  externalServerName?: string
  backupScope?: string
  toolName?: string
  toolVersion?: string
  sourceInstanceId?: number
  sourceRole?: string
  storageProfileId?: number
  storageType: string
  storageTypeText: string
  storageUri?: string
  manifestJson?: string
  prepareStatus?: string
  backupOrigin?: string
  checkpointFromLsn?: string
  checkpointToLsn?: string
  checkpointLastLsn?: string
  checkpointBackupType?: string
  artifactState?: string
  artifactCacheUri?: string
  syntheticSourceRecordIds?: string
  supersededByRecordId?: number
  purgeEligibleAt?: string
  protectedUntil?: string
  status: string
  statusText: string
  fileName: string
  fileSize: number
  checksumSha256?: string
  encrypted?: boolean
  compression?: string
  expiresAt?: string
  verifiedAt?: string
  verifyStatus?: string
  verifyStatusText?: string
  verifyMessage?: string
  restoreTestedAt?: string
  restoreTestStatus?: string
  restoreTestStatusText?: string
  startedAt: string
  lastHeartbeatAt?: string
  finishedAt: string
  recoverableFrom?: string
  recoverableUntil?: string
  durationMs: number
  message: string
  createdAt: string
  updatedAt: string
  serverUuid?: string
  serverId?: string
  gtidMode?: string
  binlogFormat?: string
  backupBinlogFile?: string
  backupBinlogPos?: number
  backupGtidSet?: string
  pgSystemIdentifier?: string
  timelineId?: string
  walSegmentSize?: number
  startLsn?: string
  endLsn?: string
  walStart?: string
  walEnd?: string
  backupManifestChecksum?: string
}

export interface DatabaseExternalBackupRecordPayload {
  taskId?: number
  instanceId: number
  sourceInstanceId?: number
  sourceRole?: string
  chainId?: string
  baseRecordId?: number
  parentRecordId?: number
  backupMethod?: string
  backupLevel?: string
  backupEngine?: string
  externalBackupId?: string
  externalServerName?: string
  backupScope?: string
  toolName?: string
  toolVersion?: string
  storageProfileId?: number
  storageUri: string
  manifestJson?: string
  prepareStatus?: string
  fileName: string
  fileSize?: number
  checksumSha256?: string
  compression?: string
  encrypted?: boolean
  recoverableFrom?: string
  recoverableUntil?: string
  startedAt?: string
  finishedAt?: string
  status?: string
  verifyStatus?: string
  serverUuid?: string
  backupBinlogFile?: string
  backupBinlogPos?: number
  backupGtidSet?: string
  pgSystemIdentifier?: string
  timelineId?: string
  startLsn?: string
  endLsn?: string
  walStart?: string
  walEnd?: string
}

export interface DatabaseLogArchiveStreamPayload {
  instanceId: number
  sourceInstanceId?: number
  engine?: string
  archiveType?: string
  archiveMode?: string
  archiveEngine?: string
  runnerHostId?: number
  storageProfileId?: number
  secretProfileId?: number
  rpoTargetSeconds?: number
  retentionDays?: number
  enabled: boolean
  configJson?: string
}

export interface DatabaseLogArchiveStreamControlPayload {
  runnerHostId?: number
  archiveMode?: string
  reason?: string
}

export interface DatabaseLogArchiveStreamResult {
  id: number
  instanceId: number
  instanceName: string
  sourceInstanceId: number
  sourceInstanceName: string
  engine: string
  engineText: string
  archiveType: string
  archiveTypeText: string
  archiveMode: string
  archiveModeText: string
  archiveEngine: string
  runnerHostId: number
  runnerHostName: string
  storageProfileId: number
  secretProfileId: number
  rpoTargetSeconds: number
  retentionDays: number
  enabled: boolean
  status: string
  statusText: string
  desiredState: string
  desiredStateText: string
  daemonStatus: string
  daemonStatusText: string
  cursorFile: string
  cursorPos: number
  cursorGtidSet: string
  activeFile: string
  lastSourceFile: string
  lastSourcePos: number
  lastEventTime: string
  archiveLagSeconds: number
  lastHeartbeatAt: string
  consecutiveFailures: number
  leaseOwner: string
  leaseExpiresAt: string
  pausedAt: string
  pausedReason: string
  lastArchivedAt: string
  lastArchiveName: string
  lastError: string
  configJson: string
  createdAt: string
  updatedAt: string
}

export interface DatabaseExternalLogArchivePayload {
  streamId: number
  fileName: string
  storageUri: string
  fileSize?: number
  checksumSha256?: string
  firstEventTime: string
  lastEventTime: string
  status?: string
  serverUuid?: string
  startPos?: number
  endPos?: number
  startGtidSet?: string
  endGtidSet?: string
  previousFileName?: string
  nextFileName?: string
  pgSystemIdentifier?: string
  timelineId?: string
  startLsn?: string
  endLsn?: string
  segmentNo?: string
  timelineHistoryUri?: string
}

export interface DatabaseLogArchiveResult {
  id: number
  streamId: number
  instanceId: number
  instanceName: string
  sourceInstanceId: number
  sourceInstanceName: string
  engine: string
  engineText: string
  archiveType: string
  archiveTypeText: string
  fileName: string
  storageUri: string
  fileSize: number
  checksumSha256?: string
  firstEventTime: string
  lastEventTime: string
  status: string
  statusText: string
  archivedAt: string
  pgSystemIdentifier?: string
  timelineId?: string
  walSegmentSize?: number
  externalServerName?: string
  startLsn?: string
  endLsn?: string
  segmentNo?: string
  timelineHistoryUri?: string
  createdAt: string
  updatedAt: string
}

export interface DatabaseLogArchiveEventResult {
  id: number
  streamId: number
  instanceId: number
  instanceName: string
  sourceInstanceId: number
  sourceInstanceName: string
  runnerHostId: number
  runnerHostName: string
  runnerId: string
  eventType: string
  eventTypeText: string
  level: string
  levelText: string
  message: string
  fileName: string
  cursorFile: string
  cursorPos: number
  activeFile: string
  archiveLagSeconds: number
  payloadJson: string
  occurredAt: string
  createdAt: string
}

export interface DatabaseStorageProfilePayload {
  name: string
  storageType: string
  endpoint?: string
  bucket?: string
  region?: string
  pathPrefix?: string
  secretProfileId?: number
  versioningEnabled?: boolean
  immutabilityEnabled?: boolean
  kmsKeyId?: string
  retentionLockDays?: number
  status?: string
}

export interface DatabaseStorageProfileResult {
  id: number
  name: string
  storageType: string
  storageTypeText: string
  endpoint: string
  bucket: string
  region: string
  pathPrefix: string
  secretProfileId: number
  versioningEnabled: boolean
  immutabilityEnabled: boolean
  kmsKeyId: string
  retentionLockDays: number
  status: string
  statusText: string
  lastTestAt: string
  postureStatus: string
  postureStatusText: string
  postureSummary: string
  postureJson: string
  lastPostureCheckAt: string
  createdAt: string
  updatedAt: string
}

export interface DatabaseStorageProfilePostureCheckPayload {
  accessKey?: string
  secretKey?: string
  sessionToken?: string
  useSsl?: boolean
  usePathStyle?: boolean
  insecureSkipVerify?: boolean
}

export interface DatabaseRunLogArchiveOncePayload {
  runnerHostId: number
  fileName?: string
}

export interface DatabaseRunLogArchiveCatchUpPayload {
  runnerHostId: number
  maxFiles?: number
  includeCurrent?: boolean
}

export interface DatabaseRestorePlanPayload {
  sourceInstanceId: number
  targetInstanceId?: number
  baseRecordId?: number
  restoreMode?: string
  restoreTargetType?: string
  restoreTargetValue: string
  targetTimelineId?: string
  restoreTargetInclusive?: boolean
}

export interface DatabaseRestoreValidationAssertionPayload {
  sql: string
  expectedRows?: number
  expectedContains?: string
  expectedScalar?: string
}

export interface DatabaseRestorePlanRunPayload {
  runnerHostId: number
  containerImage?: string
  listenPort?: number
  expiresInHours?: number
  validationSql?: string[]
  validationAssertions?: DatabaseRestoreValidationAssertionPayload[]
  cleanupOnFailure?: boolean
  postgresStartInstance?: boolean
  targetTimelineId?: string
  targetAction?: string
  barmanGetWal?: boolean
}

export interface DatabaseRestorePlanResult {
  id: number
  sourceInstanceId: number
  sourceInstanceName: string
  targetInstanceId: number
  targetInstanceName: string
  runnerHostId: number
  runnerHostName: string
  restoreMode: string
  restoreModeText: string
  restoreTargetType: string
  restoreTargetValue: string
  restoreTargetInclusive: boolean
  selectedBaseRecordId: number
  selectedBackupRecordIds: string
  selectedLogArchiveIds: string
  backupChainStatus: string
  backupChainStatusText: string
  logChainStatus: string
  logChainStatusText: string
  storageStatus: string
  storageStatusText: string
  toolStatus: string
  toolStatusText: string
  validationStatus: string
  validationStatusText: string
  restoreStatus: string
  restoreStatusText: string
  requiredToolJson: string
  requiredArtifactJson: string
  estimatedRestoreBytes: number
  estimatedRestoreMinutes: number
  planJson: string
  proofJson: string
  operatorName: string
  durationMs: number
  message: string
  createdAt: string
  updatedAt: string
}

export interface DatabaseRunnerHostPayload {
  name: string
  runnerType?: string
  host?: string
  port?: number
  credentialId?: number
  workDir?: string
  storageMountPath?: string
  maxConcurrentJobs?: number
  cpuLimit?: string
  ioLimit?: string
  bandwidthLimit?: string
  timeoutMinutes?: number
  enabled: boolean
  configJson?: string
}

export interface DatabaseRunnerHostResult {
  id: number
  name: string
  runnerType: string
  runnerTypeText: string
  host: string
  port: number
  credentialId: number
  workDir: string
  storageMountPath: string
  maxConcurrentJobs: number
  cpuLimit: string
  ioLimit: string
  bandwidthLimit: string
  timeoutMinutes: number
  enabled: boolean
  status: string
  statusText: string
  lastHeartbeatAt: string
  lastTestAt: string
  lastError: string
  configJson: string
  createdAt: string
  updatedAt: string
}

export interface DatabaseRunnerJobResult {
  id: number
  jobType: string
  jobTypeText: string
  runnerHostId: number
  runnerHostName: string
  runnerId: string
  sourceInstanceId: number
  targetInstanceId: number
  status: string
  statusText: string
  allowedCommand: string
  commandSummary: string
  workDir: string
  logPath: string
  exitCode: number
  operatorId: number
  operatorName: string
  requestJson: string
  resultJson: string
  heartbeatAt: string
  startedAt: string
  finishedAt: string
  durationMs: number
  errorMessage: string
  createdAt: string
  updatedAt: string
}

export interface DatabaseRunnerToolInfo {
  name: string
  path: string
  version: string
  installed: boolean
}

export interface DatabaseRunnerToolCapabilitySummary {
  available: string[]
  missing: string[]
  warnings: string[]
}

export interface DatabaseRunnerToolCompatibilitySummary {
  os: string
  arch: string
  supported: string[]
  warnings: string[]
}

export interface DatabaseRunnerToolProfileResult {
  id: number
  runnerHostId: number
  runnerName: string
  osFamily: string
  osVersion: string
  osPrettyName: string
  arch: string
  packageManager: string
  isRoot: boolean
  hasSudo: boolean
  hasSystemd: boolean
  hasDocker: boolean
  networkAccess: string
  tools: Record<string, DatabaseRunnerToolInfo>
  capability: DatabaseRunnerToolCapabilitySummary
  compatibility: DatabaseRunnerToolCompatibilitySummary
  lastProbeAt: string
  lastStatus: string
  lastError: string
  createdAt: string
  updatedAt: string
}

export interface DatabaseRunnerToolInstallScriptPayload {
  profiles: string[]
  targetDbVersions?: Record<string, string>
  installMode?: string
  executionMode?: string
  toolImage?: string
  toolImageDigest?: string
  datadirMount?: string
  workdirMount?: string
  networkMode?: string
  readOnlyDatadir?: boolean
  dryRun?: boolean
}

export interface DatabaseRunnerToolInstallPayload extends DatabaseRunnerToolInstallScriptPayload {
  confirmInstall?: boolean
  confirmPackages?: boolean
  reason?: string
  offlinePackageId?: number
  allowRiskyOs?: boolean
}

export interface DatabaseRunnerToolInstallScriptResult {
  runnerHostId: number
  runnerHostName: string
  osFamily: string
  osVersion: string
  osPrettyName: string
  arch: string
  packageManager: string
  profiles: string[]
  targetDbVersions?: Record<string, string>
  installMode: string
  executionMode: string
  toolImage: string
  toolImageDigest: string
  datadirMount: string
  workdirMount: string
  networkMode: string
  readOnlyDatadir: boolean
  dryRun: boolean
  warnings: string[]
  unsupported: string[]
  script: string
  generatedAt: string
}

export interface DatabaseRunnerAgentLifecyclePayload {
  serverUrl?: string
  installPath?: string
  serviceName?: string
  listenAddr?: string
  intervalSeconds?: number
  databaseArchiverEnabled?: boolean
  dryRun?: boolean
  regenerateAuth?: boolean
  confirm?: boolean
  reason?: string
}

export interface DatabaseRunnerAgentConfigSnippetResult {
  runnerHostId: number
  runnerId: string
  runnerAuthSha256: string
  configJson: string
  serviceName: string
  installPath: string
  generatedAt: string
  message: string
}

export interface DatabaseRunnerAgentLogsResult {
  runnerHostId: number
  runnerId: string
  serviceName: string
  stdout: string
  stderr: string
  exitCode: number
  fetchedAt: string
}

export interface DatabaseRunnerToolOfflinePackageResult {
  id: number
  name: string
  packageVersion: string
  osFamily: string
  osVersion: string
  arch: string
  packageManager: string
  profiles: string[]
  fileName: string
  fileSize: number
  checksumSha256: string
  storagePath: string
  status: string
  manifestJson: string
  uploadedById: number
  uploadedByName: string
  uploadedAt: string
  lastVerifiedAt: string
  lastError: string
  createdAt: string
  updatedAt: string
}

export interface DatabaseBarmanServerPayload {
  sourceInstanceId: number
  runnerHostId: number
  name: string
  barmanServerName: string
  barmanHome?: string
  configPath?: string
  retentionPolicy?: string
  backupMethod?: string
  streamingArchiverEnabled?: boolean
  archiverEnabled?: boolean
  slotName?: string
  status?: string
  configJson?: string
}

export interface DatabaseBarmanServerResult {
  id: number
  sourceInstanceId: number
  sourceInstanceName: string
  runnerHostId: number
  runnerHostName: string
  name: string
  barmanServerName: string
  barmanHome: string
  configPath: string
  retentionPolicy: string
  backupMethod: string
  streamingArchiverEnabled: boolean
  archiverEnabled: boolean
  slotName: string
  barmanVersion: string
  pgVersion: string
  pgSystemIdentifier: string
  walSegmentSize: number
  status: string
  statusText: string
  lastCheckAt: string
  lastCheckStatus: string
  lastCheckStatusText: string
  lastCatalogSyncAt: string
  lastWalSyncAt: string
  lastError: string
  configJson: string
  createdAt: string
  updatedAt: string
}

export interface DatabaseInstanceReplicaResult {
  id: number
  primaryInstanceId: number
  primaryInstanceName: string
  primaryEndpoint: string
  replicaInstanceId: number
  replicaInstanceName: string
  replicaEndpoint: string
  engine: string
  engineText: string
  replicaRole: string
  replicaRoleText: string
  sourceHost: string
  sourcePort: number
  sourceServerUuid?: string
  pgSystemIdentifier?: string
  applicationName?: string
  configuredDelaySeconds: number
  discoverySource: string
  discoverySourceText: string
  status: string
  statusText: string
  applyState: string
  applyStateText: string
  applyPaused: boolean
  lastCheckId: number
  lastCheckedAt: string
  lastError: string
  createdAt: string
  updatedAt: string
}

export interface DatabaseReplicationCheckResult {
  id: number
  instanceId: number
  instanceName: string
  instanceEndpoint: string
  replicaId: number
  engine: string
  engineText: string
  roleDetected: string
  roleDetectedText: string
  sourceInstanceId: number
  sourceInstanceName: string
  replicaIoRunning: string
  replicaSqlRunning: string
  secondsBehindSource: number
  configuredDelaySeconds: number
  remainingDelaySeconds: number
  relayLogBytes: number
  pgWriteLagMs: number
  pgFlushLagMs: number
  pgReplayLagMs: number
  pgLastWalReplayLsn: string
  pgLastXactReplayTimestamp: string
  walBacklogBytes: number
  healthStatus: string
  healthStatusText: string
  riskFlagsJson: string
  rawStatusJson: string
  checkedAt: string
  errorMessage: string
  createdAt: string
}

export interface DatabaseReplicationStatusResult {
  instance?: DatabaseInstanceResult
  replicas: DatabaseInstanceReplicaResult[]
  checks: DatabaseReplicationCheckResult[]
  lastCheck?: DatabaseReplicationCheckResult
  message: string
}

export interface DatabaseReplicaProtectionResult {
  primaryInstanceId: number
  primaryInstanceName: string
  primaryEndpoint: string
  engine: string
  engineText: string
  hasReplica: boolean
  hasDelayedReplica: boolean
  delayedReplicaCount: number
  preferredReplicaId: number
  preferredReplicaInstanceId: number
  preferredReplicaInstanceName: string
  preferredReplicaEndpoint: string
  preferredReplicaStatus: string
  preferredReplicaStatusText: string
  configuredDelaySeconds: number
  remainingDelaySeconds: number
  remainingDelayEstimated: boolean
  applyLagSeconds: number
  applyTime: string
  lastCheckId: number
  lastCheckedAt: string
  protectionStatus: string
  protectionStatusText: string
  riskLevel: string
  riskLevelText: string
  riskMessages: string[]
  riskFlagsJson: string
  lagWarningSeconds: number
  lagCriticalSeconds: number
  remainingDelayWarningSeconds: number
  relayLogBacklogWarningBytes: number
  walBacklogWarningBytes: number
}

export interface DatabaseReplicaIncidentGuidePayload {
  instanceId: number
  incidentTime?: string
  incidentType: string
  affectedSummary: string
  incidentReason: string
  expectedRecoveryMethod?: string
  confirmNoAutoPause: boolean
}

export interface DatabaseReplicaIncidentGuideResult {
  id: number
  instanceId: number
  instanceName: string
  instanceEndpoint: string
  engine: string
  engineText: string
  incidentTime: string
  incidentType: string
  incidentTypeText: string
  affectedSummary: string
  incidentReason: string
  expectedRecoveryMethod: string
  expectedRecoveryMethodText: string
  preferredReplicaId: number
  preferredReplicaInstanceId: number
  preferredReplicaName: string
  preferredReplicaEndpoint: string
  preferredCheckId: number
  canIntercept: boolean
  remainingDelaySeconds: number
  guideMarkdown: string
  guideJson: string
  status: string
  statusText: string
  operatorId: number
  operatorName: string
  clientIp: string
  createdAt: string
  updatedAt: string
}

export interface DatabaseReplicaActionPayload {
  incidentGuideId?: number
  incidentNo: string
  reason: string
  confirmImpact: string
  confirmed: boolean
  maxCheckAgeSeconds?: number
}

export interface DatabaseReplicaActionResult {
  id: number
  replicaId: number
  primaryInstanceId: number
  primaryInstanceName: string
  primaryEndpoint: string
  replicaInstanceId: number
  replicaInstanceName: string
  replicaEndpoint: string
  incidentGuideId: number
  incidentNo: string
  action: string
  actionText: string
  engine: string
  engineText: string
  allowedCommand: string
  commandTemplate: string
  reason: string
  confirmImpact: string
  confirmed: boolean
  beforeCheckId: number
  afterCheckId: number
  beforeStatusJson: string
  afterStatusJson: string
  stdout: string
  stderr: string
  exitCode: number
  status: string
  statusText: string
  errorMessage: string
  operatorId: number
  operatorName: string
  clientIp: string
  startedAt: string
  finishedAt: string
  durationMs: number
  createdAt: string
  updatedAt: string
}

export interface DatabaseBackupRunResult {
  taskId: number
  taskName: string
  recordId: number
  instanceId: number
  instanceName: string
  status: string
  statusText: string
  fileName: string
  fileSize: number
  checksumSha256?: string
  durationMs: number
  message: string
  triggeredAt: string
}

export interface DatabaseRestoreDryRunPayload {
  targetInstanceId: number
  restoreMode?: string
  restoreStrategy?: string
}

export interface DatabaseRestoreJobResult {
  id: number
  backupRecordId: number
  restorePlanId: number
  runnerHostId: number
  runnerHostName: string
  runnerJobId: number
  sourceInstanceId: number
  sourceInstanceName: string
  targetInstanceId: number
  targetInstanceName: string
  targetEnvironment: string
  restoreMode: string
  restoreModeText: string
  restoreStrategy: string
  restoreStrategyText: string
  restoreTargetType: string
  restoreTargetValue: string
  status: string
  statusText: string
  fileName: string
  fileSize: number
  workDir: string
  preparedDatadir: string
  containerName: string
  containerImage: string
  listenHost: string
  listenPort: number
  stepJson: string
  validationJson: string
  proofJson: string
  logPath: string
  artifactUri: string
  expiresAt: string
  cleanupStatus: string
  operatorId: number
  operatorName: string
  startedAt: string
  finishedAt: string
  durationMs: number
  message: string
  createdAt: string
  updatedAt: string
}

export interface DatabaseCapacityPoint {
  collectedAt: string
  schemaCount: number
  tableCount: number
  rowCount: number
  dataSizeBytes: number
  indexSizeBytes: number
  totalSizeBytes: number
  totalSizeText: string
}

export interface DatabaseCapacityObject {
  objectType: string
  schemaName: string
  tableName: string
  tableCount: number
  rowCount: number
  dataSizeBytes: number
  indexSizeBytes: number
  totalSizeBytes: number
  totalSizeText: string
  collectedAt: string
}

export interface DatabaseCapacityTrendResult {
  instanceId: number
  instanceName: string
  dbType: string
  dbTypeText: string
  range: string
  rangeText: string
  collectedAt: string
  latestSizeBytes: number
  latestSizeText: string
  growthBytes: number
  growthText: string
  growthPercent: number
  points: DatabaseCapacityPoint[]
  topSchemas: DatabaseCapacityObject[]
  topTables: DatabaseCapacityObject[]
  message: string
}

export interface DatabaseCapacityCollectResult {
  instanceId: number
  instanceName: string
  snapshotsCount: number
  schemaCount: number
  tableCount: number
  totalSizeBytes: number
  totalSizeText: string
  collectedAt: string
  message: string
}

export interface DatabaseInspectionMetric {
  key: string
  label: string
  value: string
  status: string
}

export interface DatabaseInspectionSection {
  key: string
  label: string
  status: string
  summary: string
  metrics: DatabaseInspectionMetric[]
}

export interface DatabaseInspectionFinding {
  severity: string
  category: string
  title: string
  message: string
  resourceType: string
  resourceId: string
}

export interface DatabaseInspectionReportResult {
  id: number
  instanceId: number
  instanceName: string
  dbType: string
  dbTypeText: string
  reportType: string
  status: string
  statusText: string
  healthScore: number
  riskLevel: string
  riskLevelText: string
  summary: string
  capacitySummary: DatabaseInspectionSection
  performanceSummary: DatabaseInspectionSection
  securitySummary: DatabaseInspectionSection
  backupSummary: DatabaseInspectionSection
  findings: DatabaseInspectionFinding[]
  operatorId: number
  operatorName: string
  generatedAt: string
  durationMs: number
  errorMessage: string
  createdAt: string
  updatedAt: string
}

export interface DatabaseDiagnosisListParams {
  limit?: number
}

export interface DatabaseTopologyCard {
  key: string
  label: string
  value: string
  description: string
}

export interface DatabaseTopologyNode {
  id: string
  name: string
  role: string
  roleText: string
  address: string
  state: string
  version: string
  slots: string
  lagBytes: number
  lagText: string
  message: string
  metrics?: Record<string, string>
  updatedAt: string
}

export interface DatabaseTopologyLink {
  source: string
  target: string
  sourceName?: string
  targetName?: string
  label: string
  state: string
  lagText?: string
  message?: string
  metrics?: Record<string, string>
}

export interface DatabaseTopologyFinding {
  level: string
  category: string
  title: string
  description: string
  suggestion: string
  nodeId?: string
  linkId?: string
}

export interface DatabaseShard {
  index: string
  shard: string
  primary: boolean
  state: string
  node: string
  address: string
  docs: number
  storeBytes: number
}

export interface DatabaseTopologyResult {
  instanceId: number
  instanceName: string
  dbType: string
  dbTypeText: string
  topologyType: string
  topologyTypeText: string
  collectedAt: string
  cards: DatabaseTopologyCard[]
  nodes: DatabaseTopologyNode[]
  links: DatabaseTopologyLink[]
  shards: DatabaseShard[]
  findings?: DatabaseTopologyFinding[]
  message: string
}

export interface DatabaseTableRelationResult {
  id: number
  instanceId: number
  schemaName: string
  tableName: string
  columnName: string
  referencedSchemaName: string
  referencedTableName: string
  referencedColumnName: string
  constraintName: string
  relationType: string
  relationTypeText: string
  relationSource: string
  relationSourceText: string
  confidence: number
  onUpdate: string
  onDelete: string
  cardinality: string
  cardinalityText: string
  comment: string
  direction: string
  joinSql: string
  reverseJoinSql: string
  orphanCheckSql: string
  dependencyCheckSql: string
  impactLevel: string
  impactText: string
  lastSyncAt?: string
}

export interface DatabaseTableRelationNode {
  id: string
  schemaName: string
  tableName: string
  label: string
  current: boolean
  incoming: number
  outgoing: number
  relationType: string
}

export interface DatabaseTableRelationLink {
  id: string
  source: string
  target: string
  sourceLabel: string
  targetLabel: string
  label: string
  relationType: string
  confidence: number
}

export interface DatabaseTableRelationSummary {
  total: number
  foreignKeys: number
  inferred: number
  incoming: number
  outgoing: number
  lowConfidence: number
}

export interface DatabaseTableRelationGraphResult {
  relations: DatabaseTableRelationResult[]
  nodes: DatabaseTableRelationNode[]
  links: DatabaseTableRelationLink[]
  summary: DatabaseTableRelationSummary
}

export interface DatabaseTableDDLPayload {
  schemaName?: string
  tableName: string
}

export const getDatabaseSupportedTypes = () =>
  request.get('/api/v1/databases/supported-types')

export const getDatabaseUIPermissions = () =>
  request.get('/api/v1/databases/ui-permissions')

export const listDatabaseInstancePermissions = (params?: {
  page?: number
  pageSize?: number
  roleId?: number
  instanceId?: number
  keyword?: string
}) => request.get('/api/v1/databases/instance-permissions', { params })

export const upsertDatabaseInstancePermission = (data: DatabaseInstancePermissionPayload) =>
  request.post('/api/v1/databases/instance-permissions', data)

export const deleteDatabaseInstancePermission = (id: number) =>
  request.delete(`/api/v1/databases/instance-permissions/${id}`)

export const listDatabaseInstances = (params: {
  page?: number
  pageSize?: number
  keyword?: string
  dbType?: string
  status?: string
  environment?: string
}) => request.get('/api/v1/databases/instances', { params })

export const listDatabaseQueryAudits = (params: {
  page?: number
  pageSize?: number
  keyword?: string
  instanceId?: number
  action?: string
  status?: string
  riskLevel?: string
  sqlType?: string
  startTime?: string
  endTime?: string
}) => request.get('/api/v1/databases/query-audits', { params })

export const listDatabaseQueryHistory = (params?: {
  instanceId?: number
  schemaName?: string
  keyword?: string
  limit?: number
}) => request.get('/api/v1/databases/query-history', { params })

export const exportDatabaseQueryAudits = (params: {
  keyword?: string
  instanceId?: number
  action?: string
  status?: string
  riskLevel?: string
  sqlType?: string
  startTime?: string
  endTime?: string
}) => request.get('/api/v1/databases/query-audits/export', {
  params,
  responseType: 'blob'
})

export const listDatabaseBackupTasks = (params?: {
  page?: number
  pageSize?: number
  keyword?: string
  instanceId?: number
  enabled?: string
}) => request.get('/api/v1/databases/backup-tasks', { params })

export const createDatabaseBackupTask = (data: DatabaseBackupTaskPayload) =>
  request.post('/api/v1/databases/backup-tasks', data)

export const updateDatabaseBackupTask = (id: number, data: DatabaseBackupTaskPayload) =>
  request.put(`/api/v1/databases/backup-tasks/${id}`, data)

export const deleteDatabaseBackupTask = (id: number) =>
  request.delete(`/api/v1/databases/backup-tasks/${id}`)

export const runDatabaseBackupTask = (id: number) =>
  request.post(`/api/v1/databases/backup-tasks/${id}/run`)

export const getDatabaseBackupAlertSummary = () =>
  request.get('/api/v1/databases/backup-alert-summary')

export const listDatabaseBackupAlertRules = (params?: {
  page?: number
  pageSize?: number
  keyword?: string
  enabled?: string
  scopeType?: string
  instanceId?: number
  engine?: string
  issueType?: string
}) => request.get('/api/v1/databases/backup-alert-rules', { params })

export const createDatabaseBackupAlertRule = (data: DatabaseBackupAlertRulePayload) =>
  request.post('/api/v1/databases/backup-alert-rules', data)

export const updateDatabaseBackupAlertRule = (id: number, data: DatabaseBackupAlertRulePayload) =>
  request.put(`/api/v1/databases/backup-alert-rules/${id}`, data)

export const deleteDatabaseBackupAlertRule = (id: number) =>
  request.delete(`/api/v1/databases/backup-alert-rules/${id}`)

export const testDatabaseBackupAlertRule = (id: number) =>
  request.post(`/api/v1/databases/backup-alert-rules/${id}/test`)

export const listDatabaseBackupAlertStates = (params?: {
  page?: number
  pageSize?: number
  ruleId?: number
  instanceId?: number
  status?: string
  severity?: string
  issueType?: string
  keyword?: string
}) => request.get('/api/v1/databases/backup-alert-states', { params })

export const listDatabaseBackupPolicies = (params?: {
  page?: number
  pageSize?: number
  keyword?: string
  instanceId?: number
  status?: string
  enabled?: string
}) => request.get('/api/v1/databases/backup-policies', { params })

export const listDatabaseProtectionProfiles = (params?: {
  page?: number
  pageSize?: number
  keyword?: string
  instanceId?: number
  engine?: string
  protectionMode?: string
  protectionLevel?: string
  riskLevel?: string
}) => request.get('/api/v1/databases/protection-profiles', { params })

export const listDatabaseProtectionRisks = (params?: {
  page?: number
  pageSize?: number
  keyword?: string
  instanceId?: number
  engine?: string
  riskLevel?: string
  issueType?: string
  productionOnly?: string
}) => request.get('/api/v1/databases/protection-risks', { params })

export const getDatabaseProtectionProfile = (id: string) =>
  request.get(`/api/v1/databases/protection-profiles/${id}`)

export const validateDatabaseProtectionProfile = (id: string) =>
  request.post(`/api/v1/databases/protection-profiles/${id}/validate`)

export const previewDatabaseMySQLPITRWizard = (data: DatabaseMySQLPITRWizardPayload) =>
  request.post('/api/v1/databases/protection-wizards/mysql-pitr/preview', data)

export const applyDatabaseMySQLPITRWizard = (data: DatabaseMySQLPITRWizardPayload) =>
  request.post('/api/v1/databases/protection-wizards/mysql-pitr/apply', data)

export const previewDatabasePostgresBarmanPITRWizard = (data: DatabasePostgresBarmanPITRWizardPayload) =>
  request.post('/api/v1/databases/protection-wizards/postgresql-barman/preview', data)

export const applyDatabasePostgresBarmanPITRWizard = (data: DatabasePostgresBarmanPITRWizardPayload) =>
  request.post('/api/v1/databases/protection-wizards/postgresql-barman/apply', data)

export const runDatabaseProtectionRestoreDrill = (id: string, data: DatabaseProtectionRestoreDrillPayload) =>
  request.post(`/api/v1/databases/protection-profiles/${id}/run-restore-drill`, data)

export const createDatabaseBackupPolicy = (data: DatabaseBackupPolicyPayload) =>
  request.post('/api/v1/databases/backup-policies', data)

export const updateDatabaseBackupPolicy = (id: number, data: DatabaseBackupPolicyPayload) =>
  request.put(`/api/v1/databases/backup-policies/${id}`, data)

export const deleteDatabaseBackupPolicy = (id: number) =>
  request.delete(`/api/v1/databases/backup-policies/${id}`)

export const getDatabaseBackupPolicyChain = (id: number) =>
  request.get(`/api/v1/databases/backup-policies/${id}/chain`)

export const validateDatabaseBackupPolicyChain = (id: number) =>
  request.post(`/api/v1/databases/backup-policies/${id}/validate-chain`)

export const previewDatabaseBackupPolicySyntheticFull = (id: number) =>
  request.post(`/api/v1/databases/backup-policies/${id}/synthetic-full/preview`)

export const runDatabaseBackupPolicySyntheticFull = (id: number, data?: { reason?: string }) =>
  request.post(`/api/v1/databases/backup-policies/${id}/synthetic-full/run`, data || {})

export const listDatabaseBackupPolicySyntheticJobs = (id: number, params?: {
  page?: number
  pageSize?: number
}) => request.get(`/api/v1/databases/backup-policies/${id}/synthetic-full/jobs`, { params })

export const previewDatabaseBackupPolicyPurge = (id: number) =>
  request.post(`/api/v1/databases/backup-policies/${id}/purge-preview`)

export const runDatabaseBackupPolicyPurge = (id: number, data?: { reason?: string }) =>
  request.post(`/api/v1/databases/backup-policies/${id}/purge`, data || {})

export const runDatabaseBackupPolicyFull = (id: number, data?: { reason?: string }) =>
  request.post(`/api/v1/databases/backup-policies/${id}/run-full`, data || {})

export const runDatabaseBackupPolicyIncremental = (id: number, data?: { reason?: string }) =>
  request.post(`/api/v1/databases/backup-policies/${id}/run-incremental`, data || {})

export const listDatabaseBackupRecords = (params?: {
  page?: number
  pageSize?: number
  taskId?: number
  instanceId?: number
  status?: string
  triggerType?: string
  backupMethod?: string
  backupEngine?: string
  backupScope?: string
  dateFrom?: string
  dateTo?: string
}) => request.get('/api/v1/databases/backup-records', { params })

export const registerExternalDatabaseBackupRecord = (data: DatabaseExternalBackupRecordPayload) =>
  request.post('/api/v1/databases/backup-records/external', data)

export const downloadDatabaseBackupRecord = (id: number) =>
  request.get(`/api/v1/databases/backup-records/${id}/download`, { responseType: 'blob' })

export const verifyDatabaseBackupRecord = (id: number) =>
  request.post(`/api/v1/databases/backup-records/${id}/verify`)

export const runDatabaseRestoreDryRun = (id: number, data: DatabaseRestoreDryRunPayload) =>
  request.post(`/api/v1/databases/backup-records/${id}/restore-dry-run`, data)

export const listDatabaseRestoreJobs = (params?: {
  page?: number
  pageSize?: number
  backupRecordId?: number
  sourceInstanceId?: number
  targetInstanceId?: number
  status?: string
}) => request.get('/api/v1/databases/restore-jobs', { params })

export const listDatabaseLogArchiveStreams = (params?: {
  page?: number
  pageSize?: number
  instanceId?: number
  archiveType?: string
  status?: string
}) => request.get('/api/v1/databases/log-archive-streams', { params })

export const createDatabaseLogArchiveStream = (data: DatabaseLogArchiveStreamPayload) =>
  request.post('/api/v1/databases/log-archive-streams', data)

export const getDatabaseLogArchiveStreamStatus = (id: number) =>
  request.get(`/api/v1/databases/log-archive-streams/${id}/status`)

export const startDatabaseLogArchiveStream = (id: number, data?: DatabaseLogArchiveStreamControlPayload) =>
  request.post(`/api/v1/databases/log-archive-streams/${id}/start`, data || {})

export const pauseDatabaseLogArchiveStream = (id: number, data?: DatabaseLogArchiveStreamControlPayload) =>
  request.post(`/api/v1/databases/log-archive-streams/${id}/pause`, data || {})

export const resumeDatabaseLogArchiveStream = (id: number, data?: DatabaseLogArchiveStreamControlPayload) =>
  request.post(`/api/v1/databases/log-archive-streams/${id}/resume`, data || {})

export const stopDatabaseLogArchiveStream = (id: number, data?: DatabaseLogArchiveStreamControlPayload) =>
  request.post(`/api/v1/databases/log-archive-streams/${id}/stop`, data || {})

export const runDatabaseLogArchiveOnce = (id: number, data: DatabaseRunLogArchiveOncePayload) =>
  request.post(`/api/v1/databases/log-archive-streams/${id}/run-once`, data)

export const runDatabaseLogArchiveCatchUp = (id: number, data: DatabaseRunLogArchiveCatchUpPayload) =>
  request.post(`/api/v1/databases/log-archive-streams/${id}/catch-up`, data)

export const listDatabaseLogArchives = (params?: {
  page?: number
  pageSize?: number
  streamId?: number
  instanceId?: number
  archiveType?: string
  status?: string
}) => request.get('/api/v1/databases/log-archives', { params })

export const registerExternalDatabaseLogArchive = (data: DatabaseExternalLogArchivePayload) =>
  request.post('/api/v1/databases/log-archives/external', data)

export const listDatabaseLogArchiveEvents = (params?: {
  page?: number
  pageSize?: number
  streamId?: number
  instanceId?: number
  runnerHostId?: number
  level?: string
  eventType?: string
}) => request.get('/api/v1/databases/log-archive-events', { params })

export const listDatabaseStorageProfiles = (params?: {
  page?: number
  pageSize?: number
  keyword?: string
  storageType?: string
  status?: string
}) => request.get('/api/v1/databases/storage-profiles', { params })

export const createDatabaseStorageProfile = (data: DatabaseStorageProfilePayload) =>
  request.post('/api/v1/databases/storage-profiles', data)

export const checkDatabaseStorageProfilePosture = (id: number, data: DatabaseStorageProfilePostureCheckPayload) =>
  request.post(`/api/v1/databases/storage-profiles/${id}/posture-check`, data)

export const listDatabaseRestorePlans = (params?: {
  page?: number
  pageSize?: number
  sourceInstanceId?: number
  targetInstanceId?: number
  validationStatus?: string
  restoreStatus?: string
}) => request.get('/api/v1/databases/restore-plans', { params })

export const createDatabaseRestorePlan = (data: DatabaseRestorePlanPayload) =>
  request.post('/api/v1/databases/restore-plans', data)

export const runDatabaseRestorePlan = (id: number, data: DatabaseRestorePlanRunPayload) =>
  request.post(`/api/v1/databases/restore-plans/${id}/run`, data)

export const getDatabaseRestoreJob = (id: number) =>
  request.get(`/api/v1/databases/restore-jobs/${id}`)

export const cancelDatabaseRestoreJob = (id: number) =>
  request.post(`/api/v1/databases/restore-jobs/${id}/cancel`)

export const cleanupDatabaseRestoreJob = (id: number) =>
  request.post(`/api/v1/databases/restore-jobs/${id}/cleanup`)

export const getDatabaseRestoreJobProof = (id: number) =>
  request.get(`/api/v1/databases/restore-jobs/${id}/proof`)

export const listDatabaseRunnerHosts = (params?: {
  page?: number
  pageSize?: number
  keyword?: string
  runnerType?: string
  status?: string
  enabled?: string
}) => request.get('/api/v1/databases/runner-hosts', { params })

export const createDatabaseRunnerHost = (data: DatabaseRunnerHostPayload) =>
  request.post('/api/v1/databases/runner-hosts', data)

export const updateDatabaseRunnerHost = (id: number, data: DatabaseRunnerHostPayload) =>
  request.put(`/api/v1/databases/runner-hosts/${id}`, data)

export const deleteDatabaseRunnerHost = (id: number) =>
  request.delete(`/api/v1/databases/runner-hosts/${id}`)

export const testDatabaseRunnerHost = (id: number) =>
  request.post(`/api/v1/databases/runner-hosts/${id}/test`)

export const getDatabaseRunnerToolProfile = (id: number) =>
  request.get(`/api/v1/databases/runner-hosts/${id}/tool-profile`)

export const probeDatabaseRunnerTools = (id: number) =>
  request.post(`/api/v1/databases/runner-hosts/${id}/tool-probe`)

export const generateDatabaseRunnerToolInstallScript = (id: number, data: DatabaseRunnerToolInstallScriptPayload) =>
  request.post(`/api/v1/databases/runner-hosts/${id}/tool-install-script`, data)

export const installDatabaseRunnerTools = (id: number, data: DatabaseRunnerToolInstallPayload) =>
  request.post(`/api/v1/databases/runner-hosts/${id}/tool-install`, data)

export const generateDatabaseRunnerAgentConfigSnippet = (id: number, data: DatabaseRunnerAgentLifecyclePayload) =>
  request.post(`/api/v1/databases/runner-hosts/${id}/agent-config-snippet`, data)

export const installDatabaseRunnerAgent = (id: number, data: DatabaseRunnerAgentLifecyclePayload) =>
  request.post(`/api/v1/databases/runner-hosts/${id}/agent-install`, data)

export const upgradeDatabaseRunnerAgent = (id: number, data: DatabaseRunnerAgentLifecyclePayload) =>
  request.post(`/api/v1/databases/runner-hosts/${id}/agent-upgrade`, data)

export const restartDatabaseRunnerAgent = (id: number, data: DatabaseRunnerAgentLifecyclePayload) =>
  request.post(`/api/v1/databases/runner-hosts/${id}/agent-restart`, data)

export const getDatabaseRunnerAgentLogs = (id: number, params?: { lines?: number }) =>
  request.get(`/api/v1/databases/runner-hosts/${id}/agent-logs`, { params })

export const listDatabaseRunnerToolOfflinePackages = (params?: {
  page?: number
  pageSize?: number
  keyword?: string
  osFamily?: string
  arch?: string
  status?: string
}) => request.get('/api/v1/databases/runner-tool-offline-packages', { params })

export const uploadDatabaseRunnerToolOfflinePackage = (data: FormData) =>
  request.post('/api/v1/databases/runner-tool-offline-packages', data, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })

export const downloadDatabaseRunnerToolOfflinePackage = (id: number) =>
  request.get(`/api/v1/databases/runner-tool-offline-packages/${id}/download`, { responseType: 'blob' })

export const listDatabaseRunnerJobs = (params?: {
  page?: number
  pageSize?: number
  runnerHostId?: number
  jobType?: string
  status?: string
  sourceInstanceId?: number
  targetInstanceId?: number
}) => request.get('/api/v1/databases/runner-jobs', { params })

export const listDatabaseBarmanServers = (params?: {
  page?: number
  pageSize?: number
  keyword?: string
  sourceInstanceId?: number
  runnerHostId?: number
  status?: string
}) => request.get('/api/v1/databases/barman-servers', { params })

export const createDatabaseBarmanServer = (data: DatabaseBarmanServerPayload) =>
  request.post('/api/v1/databases/barman-servers', data)

export const updateDatabaseBarmanServer = (id: number, data: DatabaseBarmanServerPayload) =>
  request.put(`/api/v1/databases/barman-servers/${id}`, data)

export const deleteDatabaseBarmanServer = (id: number) =>
  request.delete(`/api/v1/databases/barman-servers/${id}`)

export const checkDatabaseBarmanServer = (id: number) =>
  request.post(`/api/v1/databases/barman-servers/${id}/check`)

export const syncDatabaseBarmanCatalog = (id: number) =>
  request.post(`/api/v1/databases/barman-servers/${id}/sync-catalog`)

export const syncDatabaseBarmanWAL = (id: number) =>
  request.post(`/api/v1/databases/barman-servers/${id}/sync-wal`)

export const backupDatabaseBarmanServer = (id: number) =>
  request.post(`/api/v1/databases/barman-servers/${id}/backup`)

export const listDatabaseInspectionReports = (params?: {
  page?: number
  pageSize?: number
  instanceId?: number
  status?: string
  riskLevel?: string
}) => request.get('/api/v1/databases/inspection-reports', { params })

export const getDatabaseInspectionReport = (id: number) =>
  request.get(`/api/v1/databases/inspection-reports/${id}`)

export const generateDatabaseInspectionReport = (data: { instanceId: number }) =>
  request.post('/api/v1/databases/inspection-reports', data)

export const listDatabaseReplicas = (params?: {
  page?: number
  pageSize?: number
  instanceId?: number
  primaryInstanceId?: number
  replicaInstanceId?: number
  engine?: string
  replicaRole?: string
  status?: string
}) => request.get('/api/v1/databases/replicas', { params })

export const deleteDatabaseReplicaRelation = (id: number) =>
  request.delete(`/api/v1/databases/replicas/${id}`)

export const listDatabaseReplicationChecks = (params?: {
  page?: number
  pageSize?: number
  instanceId?: number
  replicaId?: number
  engine?: string
  roleDetected?: string
  healthStatus?: string
}) => request.get('/api/v1/databases/replication-checks', { params })

export const listDatabaseReplicaProtections = (params?: {
  page?: number
  pageSize?: number
  instanceId?: number
  engine?: string
  riskLevel?: string
  protectionStatus?: string
  lagWarningSeconds?: number
  lagCriticalSeconds?: number
  remainingDelayWarningSeconds?: number
  relayLogBacklogWarningBytes?: number
  walBacklogWarningBytes?: number
}) => request.get('/api/v1/databases/replica-protections', { params })

export const createDatabaseReplicaIncidentGuide = (data: DatabaseReplicaIncidentGuidePayload) =>
  request.post('/api/v1/databases/replica-incident-guides', data)

export const listDatabaseReplicaIncidentGuides = (params?: {
  page?: number
  pageSize?: number
  instanceId?: number
  incidentType?: string
  status?: string
  canIntercept?: string
}) => request.get('/api/v1/databases/replica-incident-guides', { params })

export const getDatabaseReplicaIncidentGuide = (id: number) =>
  request.get(`/api/v1/databases/replica-incident-guides/${id}`)

export const deleteDatabaseReplicaIncidentGuide = (id: number) =>
  request.delete(`/api/v1/databases/replica-incident-guides/${id}`)

export const listDatabaseReplicaActions = (params?: {
  page?: number
  pageSize?: number
  replicaId?: number
  instanceId?: number
  action?: string
  status?: string
}) => request.get('/api/v1/databases/replica-actions', { params })

export const deleteDatabaseReplicaAction = (id: number) =>
  request.delete(`/api/v1/databases/replica-actions/${id}`)

export const pauseDatabaseReplicaApply = (id: number, data: DatabaseReplicaActionPayload) =>
  request.post(`/api/v1/databases/replicas/${id}/pause-apply`, data)

export const resumeDatabaseReplicaApply = (id: number, data: DatabaseReplicaActionPayload) =>
  request.post(`/api/v1/databases/replicas/${id}/resume-apply`, data)

export const createDatabaseInstance = (data: DatabaseInstancePayload) =>
  request.post('/api/v1/databases/instances', data)

export const getDatabaseInstance = (id: number) =>
  request.get(`/api/v1/databases/instances/${id}`)

export const updateDatabaseInstance = (id: number, data: DatabaseInstancePayload) =>
  request.put(`/api/v1/databases/instances/${id}`, data)

export const deleteDatabaseInstance = (id: number) =>
  request.delete(`/api/v1/databases/instances/${id}`)

export const enableDatabaseInstance = (id: number) =>
  request.post(`/api/v1/databases/instances/${id}/enable`)

export const disableDatabaseInstance = (id: number) =>
  request.post(`/api/v1/databases/instances/${id}/disable`)

export const testDatabaseInstance = (id: number) =>
  request.post(`/api/v1/databases/instances/${id}/test`)

export const syncDatabaseMetadata = (id: number) =>
  request.post(`/api/v1/databases/instances/${id}/sync-metadata`)

export const listDatabaseSchemas = (id: number) =>
  request.get(`/api/v1/databases/instances/${id}/schemas`)

export const listDatabaseTables = (id: number, params?: { schemaName?: string }) =>
  request.get(`/api/v1/databases/instances/${id}/tables`, { params })

export const listDatabaseColumns = (id: number, params: { schemaName?: string; tableName: string }) =>
  request.get(`/api/v1/databases/instances/${id}/columns`, { params })

export const listDatabaseIndexes = (id: number, params: { schemaName?: string; tableName: string }) =>
  request.get(`/api/v1/databases/instances/${id}/indexes`, { params })

export const listDatabaseTableRelations = (id: number, params?: {
  schemaName?: string
  tableName?: string
  direction?: string
  source?: string
  includeInferred?: boolean
}) => request.get(`/api/v1/databases/instances/${id}/table-relations`, { params })

export const getDatabaseTableDDL = (id: number, params: DatabaseTableDDLPayload) =>
  request.get(`/api/v1/databases/instances/${id}/ddl`, { params })

export const exportDatabaseTableDictionary = (id: number, params: DatabaseTableDDLPayload) =>
  request.get(`/api/v1/databases/instances/${id}/dictionary/export`, {
    params,
    responseType: 'blob'
  })

export const getDatabaseDiagnosisMetrics = (id: number) =>
  request.get(`/api/v1/databases/instances/${id}/metrics`)

export const listDatabaseDiagnosisSessions = (id: number, params?: DatabaseDiagnosisListParams) =>
  request.get(`/api/v1/databases/instances/${id}/sessions`, { params })

export const listDatabaseSlowQueries = (id: number, params?: DatabaseDiagnosisListParams) =>
  request.get(`/api/v1/databases/instances/${id}/slow-queries`, { params })

export const getDatabaseTopology = (id: number) =>
  request.get(`/api/v1/databases/instances/${id}/topology`)

export const listDatabaseInstanceReplicas = (id: number, params?: {
  page?: number
  pageSize?: number
  status?: string
  replicaRole?: string
}) => request.get(`/api/v1/databases/instances/${id}/replicas`, { params })

export const getDatabaseReplicationStatus = (id: number) =>
  request.get(`/api/v1/databases/instances/${id}/replication-status`)

export const checkDatabaseReplication = (id: number) =>
  request.post(`/api/v1/databases/instances/${id}/replication-check`)

export const getDatabaseCapacityTrend = (id: number, params?: { range?: string; topLimit?: number }) =>
  request.get(`/api/v1/databases/instances/${id}/capacity-trend`, { params })

export const collectDatabaseCapacitySnapshot = (id: number) =>
  request.post(`/api/v1/databases/instances/${id}/capacity-snapshots`)

export const formatDatabaseQuery = (id: number, data: DatabaseQueryFormatPayload) =>
  request.post(`/api/v1/databases/instances/${id}/query/format`, data)

export const validateDatabaseWriteQuery = (id: number, data: DatabaseWriteValidatePayload) =>
  request.post(`/api/v1/databases/instances/${id}/query/write/validate`, data)

export const executeDatabaseWriteQuery = (id: number, data: DatabaseWriteExecutePayload) =>
  request.post(`/api/v1/databases/instances/${id}/query/write`, data)

export const validateDatabaseDDLQuery = (id: number, data: DatabaseWriteValidatePayload) =>
  request.post(`/api/v1/databases/instances/${id}/query/ddl/validate`, data)

export const executeDatabaseDDLQuery = (id: number, data: DatabaseWriteExecutePayload) =>
  request.post(`/api/v1/databases/instances/${id}/query/ddl`, data)

export const executeDatabaseQuery = (id: number, data: DatabaseQueryPayload) =>
  request.post(`/api/v1/databases/instances/${id}/query`, data)

export const explainDatabaseQuery = (id: number, data: DatabaseQueryPayload) =>
  request.post(`/api/v1/databases/instances/${id}/query/explain`, data)

export const explainDatabaseWriteQuery = (id: number, data: DatabaseQueryPayload) =>
  request.post(`/api/v1/databases/instances/${id}/query/write/explain`, data)

export const exportDatabaseQueryResult = (id: number, data: DatabaseQueryPayload) =>
  request.post(`/api/v1/databases/instances/${id}/query/export`, data, {
    responseType: 'blob'
  })
