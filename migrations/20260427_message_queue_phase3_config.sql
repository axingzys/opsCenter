-- Message Queue phase 3 high-risk operation configuration.

INSERT INTO `sys_config` (`key`, `value`, `type`, `group`, `remark`, `created_at`, `updated_at`)
VALUES
  ('messageQueueHighRiskEnabled', 'false', 'bool', 'messagequeue', 'MQ高危操作总开关', NOW(), NOW()),
  ('messageQueueOperationReasonRequired', 'true', 'bool', 'messagequeue', 'MQ高危操作是否要求填写原因', NOW(), NOW())
ON DUPLICATE KEY UPDATE
  `type` = VALUES(`type`),
  `group` = VALUES(`group`),
  `remark` = VALUES(`remark`),
  `updated_at` = NOW();
