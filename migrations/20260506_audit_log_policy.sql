-- Operation audit log policy configuration.

INSERT INTO `sys_config` (`key`, `value`, `type`, `group`, `remark`, `created_at`, `updated_at`)
VALUES
  ('audit_log_enabled', 'true', 'bool', 'audit_log', '操作日志记录开关', NOW(), NOW()),
  ('audit_log_retention_days', '30', 'int', 'audit_log', '操作日志保留天数', NOW(), NOW()),
  ('audit_log_auto_cleanup_enabled', 'true', 'bool', 'audit_log', '操作日志自动清理开关', NOW(), NOW()),
  ('audit_log_excluded_path_prefixes', '["/metrics","/api/v1/public/agents/report","/api/v1/public/agents/echo-ip","/api/v1/public/databases/runner-agents/"]', 'json', 'audit_log', '操作日志排除路径前缀(JSON数组)', NOW(), NOW())
ON DUPLICATE KEY UPDATE
  `type` = VALUES(`type`),
  `group` = VALUES(`group`),
  `remark` = VALUES(`remark`),
  `updated_at` = NOW();
