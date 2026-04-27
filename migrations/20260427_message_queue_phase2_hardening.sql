-- Message Queue phase 2 hardening fields.

ALTER TABLE `mq_operation_audits`
  ADD COLUMN `operation_id` varchar(80) DEFAULT NULL COMMENT '操作ID' AFTER `status`,
  ADD COLUMN `idempotency_key` varchar(120) DEFAULT NULL COMMENT '幂等键' AFTER `operation_id`,
  ADD COLUMN `lock_key` varchar(255) DEFAULT NULL COMMENT '资源锁键' AFTER `idempotency_key`,
  ADD COLUMN `confirm_text` varchar(512) DEFAULT NULL COMMENT '确认文本' AFTER `lock_key`,
  ADD COLUMN `before_snapshot_json` text COMMENT '执行前快照' AFTER `result_json`,
  ADD COLUMN `after_snapshot_json` text COMMENT '执行后快照' AFTER `before_snapshot_json`,
  ADD COLUMN `diff_json` text COMMENT '配置差异' AFTER `after_snapshot_json`,
  ADD COLUMN `warnings_json` text COMMENT '风险提示' AFTER `diff_json`,
  ADD COLUMN `impact_summary_json` text COMMENT '影响摘要' AFTER `warnings_json`,
  ADD COLUMN `metadata_refresh_status` varchar(30) DEFAULT NULL COMMENT '后置元数据刷新状态' AFTER `impact_summary_json`,
  ADD COLUMN `metadata_refresh_error` varchar(500) DEFAULT NULL COMMENT '后置元数据刷新错误' AFTER `metadata_refresh_status`,
  ADD KEY `idx_mq_operation_audits_operation_id` (`operation_id`),
  ADD KEY `idx_mq_operation_audits_idempotency_key` (`idempotency_key`),
  ADD KEY `idx_mq_operation_audits_lock_key` (`lock_key`);

INSERT INTO `sys_config` (`key`, `value`, `type`, `group`, `remark`, `created_at`, `updated_at`)
VALUES
  ('messageQueueOperationMaxMetadataAgeMinutes', '30', 'int', 'messagequeue', 'MQ资源操作允许的最大元数据年龄(分钟)', NOW(), NOW())
ON DUPLICATE KEY UPDATE
  `type` = VALUES(`type`),
  `group` = VALUES(`group`),
  `remark` = VALUES(`remark`),
  `updated_at` = NOW();
