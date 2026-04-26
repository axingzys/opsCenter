-- MySQL dump 10.13  Distrib 8.0.45, for Linux (x86_64)
--
-- Host: localhost    Database: opshub
-- ------------------------------------------------------
-- Server version	8.0.45

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!50503 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Table structure for table `alert_channels`
--

DROP TABLE IF EXISTS `alert_channels`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `alert_channels` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
  `channel_type` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL,
  `enabled` tinyint(1) DEFAULT '1',
  `config` text COLLATE utf8mb4_unicode_ci,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_channel_type` (`channel_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `alert_configs`
--

DROP TABLE IF EXISTS `alert_configs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `alert_configs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
  `alert_type` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL,
  `enabled` tinyint(1) DEFAULT '1',
  `threshold` bigint DEFAULT NULL,
  `domain_monitor_id` bigint unsigned DEFAULT NULL,
  `enable_email` tinyint(1) DEFAULT '0',
  `enable_webhook` tinyint(1) DEFAULT '0',
  `enable_wechat` tinyint DEFAULT '0' COMMENT '企业微信告警',
  `enable_dingtalk` tinyint DEFAULT '0' COMMENT '钉钉告警',
  `enable_feishu` tinyint(1) DEFAULT '0',
  `enable_system_msg` tinyint(1) DEFAULT '0',
  `alert_interval` bigint DEFAULT '600',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `enable_we_chat` tinyint(1) DEFAULT '0',
  `enable_ding_talk` tinyint(1) DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `idx_alert_type` (`alert_type`),
  KEY `idx_domain_monitor_id` (`domain_monitor_id`),
  KEY `idx_alert_configs_domain_monitor_id` (`domain_monitor_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `alert_logs`
--

DROP TABLE IF EXISTS `alert_logs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `alert_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `alert_type` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL,
  `domain_monitor_id` bigint unsigned NOT NULL,
  `domain` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `status` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL,
  `message` text COLLATE utf8mb4_unicode_ci,
  `channel_type` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `error_msg` text COLLATE utf8mb4_unicode_ci,
  `sent_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `resource_type` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'domain',
  `resource_id` bigint unsigned DEFAULT NULL,
  `resource_name` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `resource_target` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `metric` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `severity` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `current_value` double DEFAULT NULL,
  `threshold_value` double DEFAULT NULL,
  `alert_rule_id` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_alert_type` (`alert_type`),
  KEY `idx_domain_monitor_id` (`domain_monitor_id`),
  KEY `idx_sent_at` (`sent_at`),
  KEY `idx_alert_logs_alert_type` (`alert_type`),
  KEY `idx_alert_logs_resource_type` (`resource_type`),
  KEY `idx_alert_logs_resource_id` (`resource_id`),
  KEY `idx_alert_logs_metric` (`metric`),
  KEY `idx_alert_logs_alert_rule_id` (`alert_rule_id`),
  KEY `idx_alert_logs_domain_monitor_id` (`domain_monitor_id`),
  KEY `idx_alert_logs_sent_at` (`sent_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `alert_receiver_channels`
--

DROP TABLE IF EXISTS `alert_receiver_channels`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `alert_receiver_channels` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `receiver_id` bigint unsigned NOT NULL,
  `channel_id` bigint unsigned NOT NULL,
  `config` text COLLATE utf8mb4_unicode_ci,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_receiver_channel` (`receiver_id`,`channel_id`),
  KEY `idx_receiver_id` (`receiver_id`),
  KEY `idx_channel_id` (`channel_id`),
  KEY `idx_alert_receiver_channels_receiver_id` (`receiver_id`),
  KEY `idx_alert_receiver_channels_channel_id` (`channel_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `alert_receivers`
--

DROP TABLE IF EXISTS `alert_receivers`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `alert_receivers` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
  `email` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `phone` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `wechat_id` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '企业微信ID',
  `dingtalk_id` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '钉钉ID',
  `feishu_id` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `user_id` bigint unsigned DEFAULT NULL,
  `enable_email` tinyint(1) DEFAULT '1',
  `enable_webhook` tinyint(1) DEFAULT '0',
  `enable_wechat` tinyint DEFAULT '0' COMMENT '启用企业微信',
  `enable_dingtalk` tinyint DEFAULT '0' COMMENT '启用钉钉',
  `enable_feishu` tinyint(1) DEFAULT '1',
  `enable_system_msg` tinyint(1) DEFAULT '1',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `we_chat_id` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `ding_talk_id` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `enable_we_chat` tinyint(1) DEFAULT '1',
  `enable_ding_talk` tinyint(1) DEFAULT '1',
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_alert_receivers_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `ansible_tasks`
--

DROP TABLE IF EXISTS `ansible_tasks`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `ansible_tasks` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '任务名称',
  `playbook_content` longtext COLLATE utf8mb4_unicode_ci COMMENT 'Playbook内容',
  `playbook_path` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'Playbook路径',
  `inventory` text COLLATE utf8mb4_unicode_ci COMMENT '清单(JSON)',
  `extra_vars` json DEFAULT NULL COMMENT '额外变量',
  `tags` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '标签',
  `fork` int DEFAULT '5' COMMENT '并发数',
  `timeout` int DEFAULT '600' COMMENT '超时时间(秒)',
  `verbose` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'v' COMMENT '日志级别',
  `status` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT 'pending' COMMENT '状态 pending/running/success/failed/cancelled',
  `last_run_time` datetime DEFAULT NULL COMMENT '最后执行时间',
  `last_run_result` json DEFAULT NULL COMMENT '最后执行结果',
  `created_by` bigint unsigned NOT NULL COMMENT '创建者ID',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL COMMENT '删除时间',
  PRIMARY KEY (`id`),
  KEY `idx_status` (`status`),
  KEY `idx_created_at` (`created_at`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `app_permissions`
--

DROP TABLE IF EXISTS `app_permissions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `app_permissions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `app_id` bigint unsigned NOT NULL COMMENT '应用ID',
  `subject_type` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '主体类型(user/role/dept)',
  `subject_id` bigint unsigned NOT NULL COMMENT '主体ID',
  `permission` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'access' COMMENT '权限类型(access/admin)',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_app_id` (`app_id`),
  KEY `idx_subject` (`subject_type`,`subject_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `asset_agent_jobs`
--

DROP TABLE IF EXISTS `asset_agent_jobs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `asset_agent_jobs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `host_id` bigint unsigned NOT NULL COMMENT '主机ID',
  `job_type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '任务类型',
  `status` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending' COMMENT '任务状态',
  `progress` int DEFAULT '0' COMMENT '任务进度',
  `stage` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '当前阶段',
  `message` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '当前提示',
  `error` text COLLATE utf8mb4_unicode_ci COMMENT '错误信息',
  `operator_id` bigint unsigned DEFAULT NULL COMMENT '操作人',
  `started_at` datetime(3) DEFAULT NULL COMMENT '开始时间',
  `finished_at` datetime(3) DEFAULT NULL COMMENT '结束时间',
  PRIMARY KEY (`id`),
  KEY `idx_asset_agent_jobs_host_id` (`host_id`),
  KEY `idx_asset_agent_jobs_status` (`status`),
  KEY `idx_asset_agent_jobs_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `asset_agents`
--

DROP TABLE IF EXISTS `asset_agents`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `asset_agents` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `host_id` bigint unsigned NOT NULL COMMENT '主机ID',
  `agent_id` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'Agent ID',
  `access_token_ciphertext` text COLLATE utf8mb4_unicode_ci COMMENT 'Agent访问令牌密文',
  `version` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'Agent版本',
  `status` varchar(32) COLLATE utf8mb4_unicode_ci DEFAULT 'pending' COMMENT 'Agent状态',
  `listen_port` int DEFAULT '19100' COMMENT 'Agent监听端口',
  `install_path` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '安装目录',
  `service_name` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '服务名',
  `install_progress` int DEFAULT '0' COMMENT '安装进度',
  `install_stage` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '安装阶段',
  `last_heartbeat_at` datetime(3) DEFAULT NULL COMMENT '最后心跳时间',
  `last_report_at` datetime(3) DEFAULT NULL COMMENT '最后上报时间',
  `last_error` text COLLATE utf8mb4_unicode_ci COMMENT '最近错误',
  `deployed_by` bigint unsigned DEFAULT NULL COMMENT '部署人',
  `deployed_at` datetime(3) DEFAULT NULL COMMENT '部署时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_asset_agents_host_id` (`host_id`),
  KEY `idx_asset_agents_agent_id` (`agent_id`),
  KEY `idx_asset_agents_status` (`status`),
  KEY `idx_asset_agents_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `asset_desktop_sessions`
--

DROP TABLE IF EXISTS `asset_desktop_sessions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `asset_desktop_sessions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `session_uuid` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '会话UUID',
  `host_id` bigint unsigned NOT NULL COMMENT '主机ID',
  `host_name` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '主机名称',
  `host_ip` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '主机IP',
  `user_id` bigint unsigned NOT NULL COMMENT '操作用户ID',
  `username` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '用户名',
  `provider` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'guacamole' COMMENT '桌面网关',
  `protocol` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'rdp' COMMENT '桌面协议',
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'creating' COMMENT 'creating/active/closed/failed/timeout',
  `client_ip` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '客户端IP',
  `resolution` varchar(32) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '分辨率',
  `recording_path` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '录屏路径',
  `close_reason` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '结束原因',
  `started_at` datetime(3) DEFAULT NULL COMMENT '开始时间',
  `ended_at` datetime(3) DEFAULT NULL COMMENT '结束时间',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_asset_desktop_sessions_session_uuid` (`session_uuid`),
  KEY `idx_host_id` (`host_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_status` (`status`),
  KEY `idx_created_at` (`created_at`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_asset_desktop_sessions_deleted_at` (`deleted_at`),
  KEY `idx_asset_desktop_sessions_host_id` (`host_id`),
  KEY `idx_asset_desktop_sessions_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `asset_group`
--

DROP TABLE IF EXISTS `asset_group`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `asset_group` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '分组名称',
  `code` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '分组编码',
  `parent_id` bigint unsigned DEFAULT '0' COMMENT '父分组ID',
  `description` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '分组描述',
  `sort` int DEFAULT '0' COMMENT '排序',
  `status` tinyint DEFAULT '1' COMMENT '状态 1:启用 0:禁用',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`,`deleted_at`),
  UNIQUE KEY `idx_asset_group_code` (`code`),
  KEY `idx_parent_id` (`parent_id`),
  KEY `idx_sort` (`sort`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_asset_group_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `asset_host_inventory`
--

DROP TABLE IF EXISTS `asset_host_inventory`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `asset_host_inventory` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `host_id` bigint unsigned NOT NULL COMMENT '主机ID',
  `private_ips_json` json DEFAULT NULL COMMENT '内网IP列表',
  `public_ips_json` json DEFAULT NULL COMMENT '公网IP列表',
  `interfaces_json` json DEFAULT NULL COMMENT '网卡信息',
  `disks_json` json DEFAULT NULL COMMENT '磁盘详情',
  `top_processes_json` json DEFAULT NULL COMMENT '进程快照',
  `listening_ports_json` json DEFAULT NULL COMMENT '监听端口快照',
  `config_summary_json` json DEFAULT NULL COMMENT '配置摘要',
  `collected_at` datetime(3) DEFAULT NULL COMMENT '采集时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_asset_host_inventory_host_id` (`host_id`),
  KEY `idx_asset_host_inventory_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `asset_host_public_ip_history`
--

DROP TABLE IF EXISTS `asset_host_public_ip_history`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `asset_host_public_ip_history` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `host_id` bigint unsigned NOT NULL COMMENT '主机ID',
  `ip` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '公网出口IP',
  `source` varchar(32) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '来源',
  `first_seen_at` datetime(3) NOT NULL COMMENT '首次发现时间',
  `last_seen_at` datetime(3) NOT NULL COMMENT '最近发现时间',
  `seen_count` bigint DEFAULT '1' COMMENT '观测次数',
  `is_current` tinyint(1) DEFAULT '0' COMMENT '是否当前IP',
  PRIMARY KEY (`id`),
  KEY `idx_asset_host_public_ip_history_deleted_at` (`deleted_at`),
  KEY `idx_asset_host_public_ip_history_host_id` (`host_id`),
  KEY `idx_asset_host_public_ip_history_current` (`is_current`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `auth_logs`
--

DROP TABLE IF EXISTS `auth_logs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `auth_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned DEFAULT NULL COMMENT '用户ID',
  `username` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '用户名',
  `action` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '动作(login/logout/access_app)',
  `app_id` bigint unsigned DEFAULT '0' COMMENT '应用ID',
  `app_name` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '应用名称',
  `login_type` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '登录类型(password/oauth/ldap)',
  `ip` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'IP地址',
  `location` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '地理位置',
  `user_agent` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'UserAgent',
  `result` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '结果(success/failed)',
  `fail_reason` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '失败原因',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_action` (`action`),
  KEY `idx_result` (`result`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `cloud_accounts`
--

DROP TABLE IF EXISTS `cloud_accounts`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `cloud_accounts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '账号名称',
  `provider` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '云厂商 aliyun/tencent/aws/huawei',
  `access_key` varchar(200) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'AccessKey',
  `secret_key` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'SecretKey',
  `region` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '默认区域',
  `description` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '备注',
  `status` tinyint DEFAULT '1' COMMENT '状态 1:启用 0:禁用',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_provider` (`provider`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_cloud_accounts_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `credentials`
--

DROP TABLE IF EXISTS `credentials`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `credentials` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '凭证名称',
  `protocol` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'ssh' COMMENT '连接协议 ssh/winrm/rdp',
  `type` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '认证方式 password/key',
  `username` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '用户名',
  `domain` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'Windows域名',
  `password` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '密码(加密)',
  `private_key` text COLLATE utf8mb4_unicode_ci COMMENT '私钥(加密)',
  `passphrase` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '私钥密码(加密)',
  `description` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '备注',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_type` (`type`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_credentials_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `domain_check_histories`
--

DROP TABLE IF EXISTS `domain_check_histories`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `domain_check_histories` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `domain_id` bigint unsigned NOT NULL,
  `domain` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `status` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL,
  `response_time` bigint DEFAULT '0',
  `ssl_valid` tinyint(1) DEFAULT '0',
  `ssl_expiry` datetime DEFAULT NULL,
  `status_code` bigint DEFAULT '0',
  `error_message` text COLLATE utf8mb4_unicode_ci,
  `checked_at` datetime NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_domain_check_histories_domain_id` (`domain_id`),
  KEY `idx_domain_check_histories_checked_at` (`checked_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `domain_monitors`
--

DROP TABLE IF EXISTS `domain_monitors`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `domain_monitors` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `domain` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `status` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'unknown',
  `response_time` bigint DEFAULT '0',
  `ssl_valid` tinyint(1) DEFAULT '0',
  `ssl_expiry` datetime DEFAULT NULL,
  `check_interval` bigint NOT NULL DEFAULT '300',
  `enable_ssl` tinyint(1) DEFAULT '1',
  `enable_alert` tinyint(1) DEFAULT '0',
  `last_check` datetime DEFAULT NULL,
  `next_check` datetime DEFAULT NULL,
  `alert_config_id` bigint unsigned DEFAULT NULL,
  `response_threshold` bigint DEFAULT '1000',
  `ssl_expiry_days` bigint DEFAULT '30',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_domain_monitors_domain` (`domain`),
  KEY `idx_status` (`status`),
  KEY `idx_next_check` (`next_check`),
  KEY `idx_domain_monitors_next_check` (`next_check`),
  KEY `idx_domain_monitors_alert_config_id` (`alert_config_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `host_alert_rules`
--

DROP TABLE IF EXISTS `host_alert_rules`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `host_alert_rules` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
  `host_id` bigint unsigned DEFAULT NULL,
  `metric` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL,
  `threshold` double NOT NULL,
  `alert_interval` bigint DEFAULT '600',
  `severity` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'warning',
  `enabled` tinyint(1) DEFAULT '1',
  `description` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `channel_ids_json` json DEFAULT NULL COMMENT '告警通道ID列表',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_host_alert_rules_host_id` (`host_id`),
  KEY `idx_host_alert_rules_metric` (`metric`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `hosts`
--

DROP TABLE IF EXISTS `hosts`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `hosts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '主机名称',
  `group_id` bigint unsigned DEFAULT NULL COMMENT '分组ID',
  `type` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'self' COMMENT '主机类型 self:自建 cloud:云主机',
  `cloud_provider` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '云厂商 aliyun/tencent/aws',
  `cloud_instance_id` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '云实例ID',
  `cloud_account_id` bigint unsigned DEFAULT NULL COMMENT '云账号ID',
  `os_type` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'linux' COMMENT '操作系统类型 linux/windows',
  `ssh_user` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'SSH用户名',
  `ip` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'IP地址',
  `port` int DEFAULT '22' COMMENT 'SSH端口',
  `credential_id` bigint unsigned DEFAULT NULL COMMENT '凭证ID',
  `management_mode` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'ssh' COMMENT '管理方式 ssh/winrm/agent/none',
  `management_port` int DEFAULT '22' COMMENT '管理端口',
  `management_credential_id` bigint unsigned DEFAULT NULL COMMENT '管理凭证ID',
  `desktop_enabled` tinyint(1) DEFAULT '0' COMMENT '是否启用桌面访问',
  `desktop_protocol` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'rdp' COMMENT '桌面协议 rdp',
  `desktop_port` int DEFAULT '3389' COMMENT '桌面端口',
  `desktop_credential_id` bigint unsigned DEFAULT NULL COMMENT '桌面凭证ID',
  `desktop_security` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'nla' COMMENT '桌面安全模式',
  `desktop_ignore_cert` tinyint(1) DEFAULT '1' COMMENT '桌面是否忽略证书',
  `tags` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '主机标签(逗号分隔)',
  `description` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '备注',
  `status` tinyint DEFAULT '1' COMMENT '状态 1:在线 0:离线 -1:未知',
  `last_seen` datetime(3) DEFAULT NULL COMMENT '最后连接时间',
  `collect_status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'unknown' COMMENT '采集状态 online/offline/unknown/not_configured',
  `collect_error` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '采集错误',
  `last_collect_at` datetime(3) DEFAULT NULL COMMENT '最后采集时间',
  `primary_private_ip` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '主内网IP',
  `primary_public_ip` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '主公网IP',
  `agent_id` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'Agent ID',
  `agent_version` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'Agent 版本',
  `agent_last_heartbeat_at` datetime(3) DEFAULT NULL COMMENT 'Agent 最后心跳时间',
  `agent_port` int DEFAULT '19100' COMMENT 'Agent监听端口',
  `agent_last_report_at` datetime(3) DEFAULT NULL COMMENT 'Agent最后上报时间',
  `agent_last_error` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'Agent最近错误',
  `os` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '操作系统',
  `kernel` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '内核版本',
  `arch` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '架构',
  `cpu_info` text COLLATE utf8mb4_unicode_ci COMMENT 'CPU信息JSON',
  `cpu_cores` bigint DEFAULT NULL COMMENT 'CPU核心数',
  `cpu_usage` double DEFAULT NULL COMMENT 'CPU使用率',
  `memory_total` bigint DEFAULT NULL COMMENT '内存总容量(字节)',
  `memory_used` bigint DEFAULT NULL COMMENT '已用内存(字节)',
  `memory_usage` double DEFAULT NULL COMMENT '内存使用率',
  `disk_total` bigint DEFAULT NULL COMMENT '磁盘总容量(字节)',
  `disk_used` bigint DEFAULT NULL COMMENT '已用磁盘(字节)',
  `disk_usage` double DEFAULT NULL COMMENT '磁盘使用率',
  `uptime` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '运行时间',
  `hostname` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '主机名',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_group_id` (`group_id`),
  KEY `idx_ip` (`ip`),
  KEY `idx_status` (`status`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_hosts_deleted_at` (`deleted_at`),
  CONSTRAINT `fk_hosts_group` FOREIGN KEY (`group_id`) REFERENCES `asset_group` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `identity_sources`
--

DROP TABLE IF EXISTS `identity_sources`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `identity_sources` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '身份源名称',
  `type` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '类型(wechat/dingtalk/feishu/qq/github/ldap/oidc/saml)',
  `icon` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '图标URL',
  `config` text COLLATE utf8mb4_unicode_ci COMMENT '配置JSON',
  `user_mapping` text COLLATE utf8mb4_unicode_ci COMMENT '用户属性映射',
  `auto_create_user` tinyint(1) DEFAULT '0' COMMENT '自动创建用户',
  `default_role_id` bigint unsigned DEFAULT '0' COMMENT '默认角色ID',
  `enabled` tinyint(1) DEFAULT '1' COMMENT '是否启用',
  `sort` int DEFAULT '0' COMMENT '排序',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL COMMENT '删除时间',
  PRIMARY KEY (`id`),
  KEY `idx_type` (`type`),
  KEY `idx_enabled` (`enabled`),
  KEY `idx_sort` (`sort`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `job_tasks`
--

DROP TABLE IF EXISTS `job_tasks`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `job_tasks` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '任务名称',
  `template_id` bigint unsigned DEFAULT NULL COMMENT '模板ID',
  `task_type` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '任务类型 manual/ansible/cron',
  `status` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT 'pending' COMMENT '状态 pending/running/success/failed',
  `target_hosts` text COLLATE utf8mb4_unicode_ci COMMENT '目标主机列表(JSON)',
  `parameters` json DEFAULT NULL COMMENT '执行参数',
  `execute_time` datetime DEFAULT NULL COMMENT '执行时间',
  `result` json DEFAULT NULL COMMENT '执行结果',
  `error_message` text COLLATE utf8mb4_unicode_ci COMMENT '错误信息',
  `created_by` bigint unsigned NOT NULL COMMENT '创建者ID',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL COMMENT '删除时间',
  PRIMARY KEY (`id`),
  KEY `idx_template_id` (`template_id`),
  KEY `idx_task_type` (`task_type`),
  KEY `idx_status` (`status`),
  KEY `idx_created_at` (`created_at`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `job_templates`
--

DROP TABLE IF EXISTS `job_templates`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `job_templates` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '模板名称',
  `code` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '模板编码',
  `description` text COLLATE utf8mb4_unicode_ci COMMENT '模板描述',
  `content` longtext COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '模板内容',
  `variables` json DEFAULT NULL COMMENT '变量定义',
  `category` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '分类 script/ansible/module',
  `platform` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '平台 linux/windows',
  `timeout` int DEFAULT '300' COMMENT '超时时间(秒)',
  `sort` int DEFAULT '0' COMMENT '排序',
  `status` tinyint DEFAULT '1' COMMENT '状态 0:禁用 1:启用',
  `created_by` bigint unsigned NOT NULL COMMENT '创建者ID',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL COMMENT '删除时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`,`deleted_at`),
  KEY `idx_category` (`category`),
  KEY `idx_sort` (`sort`),
  KEY `idx_status` (`status`),
  KEY `idx_created_at` (`created_at`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `k8s_cluster_inspections`
--

DROP TABLE IF EXISTS `k8s_cluster_inspections`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `k8s_cluster_inspections` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id` bigint unsigned NOT NULL COMMENT '集群ID',
  `cluster_name` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '集群名称',
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '状态 running/completed/failed',
  `score` int DEFAULT NULL COMMENT '健康评分',
  `check_count` int DEFAULT NULL COMMENT '检查项总数',
  `pass_count` int DEFAULT NULL COMMENT '通过项数',
  `warning_count` int DEFAULT NULL COMMENT '警告项数',
  `fail_count` int DEFAULT NULL COMMENT '失败项数',
  `duration` int DEFAULT NULL COMMENT '耗时(秒)',
  `report_data` longtext COLLATE utf8mb4_unicode_ci COMMENT '巡检报告',
  `user_id` bigint unsigned DEFAULT NULL COMMENT '执行者ID',
  `start_time` datetime DEFAULT NULL COMMENT '开始时间',
  `end_time` datetime DEFAULT NULL COMMENT '结束时间',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_cluster_id` (`cluster_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `k8s_clusters`
--

DROP TABLE IF EXISTS `k8s_clusters`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `k8s_clusters` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
  `alias` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `api_endpoint` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL,
  `kube_config` text COLLATE utf8mb4_unicode_ci NOT NULL,
  `version` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `status` bigint DEFAULT '1',
  `region` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `provider` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `description` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `created_by` bigint unsigned DEFAULT NULL,
  `node_count` bigint DEFAULT '0',
  `pod_count` bigint DEFAULT '0',
  `status_synced_at` datetime DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_k8s_clusters_name` (`name`),
  KEY `idx_status` (`status`),
  KEY `idx_provider` (`provider`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `k8s_terminal_sessions`
--

DROP TABLE IF EXISTS `k8s_terminal_sessions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `k8s_terminal_sessions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id` bigint unsigned NOT NULL COMMENT '集群ID',
  `cluster_name` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '集群名称',
  `namespace` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '命名空间',
  `pod_name` varchar(200) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'Pod名称',
  `container_name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '容器名称',
  `user_id` bigint unsigned NOT NULL COMMENT '用户ID',
  `username` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '用户名',
  `recording_path` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '录制文件路径',
  `duration` int DEFAULT NULL COMMENT '会话时长(秒)',
  `file_size` bigint DEFAULT NULL COMMENT '文件大小(字节)',
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'completed' COMMENT '状态',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_cluster_id` (`cluster_id`),
  KEY `idx_namespace` (`namespace`),
  KEY `idx_pod_name` (`pod_name`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `k8s_user_kube_configs`
--

DROP TABLE IF EXISTS `k8s_user_kube_configs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `k8s_user_kube_configs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `service_account` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `namespace` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT 'default',
  `is_active` tinyint(1) DEFAULT '1',
  `created_by` bigint unsigned NOT NULL,
  `created_at` datetime DEFAULT NULL,
  `revoked_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cluster_user_sa` (`cluster_id`,`user_id`,`service_account`),
  KEY `idx_cluster_id` (`cluster_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_k8s_user_kube_configs_cluster_id` (`cluster_id`),
  KEY `idx_k8s_user_kube_configs_user_id` (`user_id`),
  KEY `idx_k8s_user_kube_configs_service_account` (`service_account`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `k8s_user_role_bindings`
--

DROP TABLE IF EXISTS `k8s_user_role_bindings`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `k8s_user_role_bindings` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `role_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `role_namespace` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT '',
  `role_type` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL,
  `bound_by` bigint unsigned NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cluster_user_role` (`cluster_id`,`user_id`,`role_name`,`role_namespace`),
  KEY `idx_cluster_id` (`cluster_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_cluster_user_role` (`cluster_id`,`user_id`,`role_name`,`role_namespace`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `ldap_sync_jobs`
--

DROP TABLE IF EXISTS `ldap_sync_jobs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `ldap_sync_jobs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `source_id` bigint unsigned NOT NULL COMMENT '身份源ID',
  `status` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '状态(pending/running/completed/failed)',
  `total_users` int DEFAULT '0' COMMENT '总用户数',
  `synced_users` int DEFAULT '0' COMMENT '已同步用户数',
  `failed_users` int DEFAULT '0' COMMENT '失败用户数',
  `error_message` text COLLATE utf8mb4_unicode_ci COMMENT '错误信息',
  `started_at` datetime DEFAULT NULL COMMENT '开始时间',
  `completed_at` datetime DEFAULT NULL COMMENT '完成时间',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_source_id` (`source_id`),
  KEY `idx_status` (`status`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `mfa_challenges`
--

DROP TABLE IF EXISTS `mfa_challenges`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `mfa_challenges` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL COMMENT '用户ID',
  `token` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '挑战令牌',
  `type` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '类型(login/action)',
  `attempts` int DEFAULT '0' COMMENT '尝试次数',
  `verified` tinyint(1) DEFAULT '0' COMMENT '是否已验证',
  `expires_at` datetime NOT NULL COMMENT '过期时间',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_token` (`token`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_expires_at` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `mfa_settings`
--

DROP TABLE IF EXISTS `mfa_settings`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `mfa_settings` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `totp_enabled` tinyint(1) DEFAULT '0',
  `totp_secret` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `totp_verified` tinyint(1) DEFAULT '0',
  `backup_codes` text COLLATE utf8mb4_unicode_ci,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_mfa_settings_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `mfa_trusted_devices`
--

DROP TABLE IF EXISTS `mfa_trusted_devices`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `mfa_trusted_devices` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `device_token` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `device_name` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `ip_address` varchar(45) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `last_verified_at` datetime(3) DEFAULT NULL,
  `expires_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_mfa_trusted_devices_device_token` (`device_token`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_expires_at` (`expires_at`),
  KEY `idx_mfa_trusted_devices_user_id` (`user_id`),
  KEY `idx_mfa_trusted_devices_expires_at` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `nginx_access_logs`
--

DROP TABLE IF EXISTS `nginx_access_logs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `nginx_access_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `source_id` bigint unsigned NOT NULL,
  `timestamp` datetime NOT NULL,
  `remote_addr` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `remote_user` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `request` varchar(2000) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `method` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `uri` varchar(1000) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `protocol` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `status` bigint DEFAULT NULL,
  `body_bytes_sent` bigint DEFAULT NULL,
  `http_referer` varchar(1000) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `http_user_agent` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `request_time` decimal(10,3) DEFAULT NULL,
  `upstream_time` decimal(10,3) DEFAULT NULL,
  `host` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `country` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `province` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `city` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `isp` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `browser` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `browser_version` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `os` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `os_version` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `device_type` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `ingress_name` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `service_name` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_source_time` (`source_id`,`timestamp`),
  KEY `idx_source_ip` (`source_id`,`remote_addr`),
  KEY `idx_source_status` (`source_id`,`status`),
  KEY `idx_source_country` (`source_id`,`country`),
  KEY `idx_source_device` (`source_id`,`device_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `nginx_agg_daily`
--

DROP TABLE IF EXISTS `nginx_agg_daily`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `nginx_agg_daily` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `source_id` bigint unsigned NOT NULL,
  `date` date NOT NULL,
  `total_requests` bigint DEFAULT '0',
  `pv_count` bigint DEFAULT '0',
  `unique_ips` bigint DEFAULT '0',
  `total_bandwidth` bigint DEFAULT '0',
  `avg_response_time` decimal(10,3) DEFAULT '0.000',
  `max_response_time` decimal(10,3) DEFAULT '0.000',
  `min_response_time` decimal(10,3) DEFAULT '0.000',
  `status_2xx` bigint DEFAULT '0' COMMENT '2xx状态码数',
  `status_3xx` bigint DEFAULT '0' COMMENT '3xx状态码数',
  `status_4xx` bigint DEFAULT '0' COMMENT '4xx状态码数',
  `status_5xx` bigint DEFAULT '0' COMMENT '5xx状态码数',
  `top_urls` text COLLATE utf8mb4_unicode_ci,
  `top_ips` text COLLATE utf8mb4_unicode_ci,
  `top_referers` text COLLATE utf8mb4_unicode_ci,
  `top_countries` text COLLATE utf8mb4_unicode_ci,
  `top_browsers` text COLLATE utf8mb4_unicode_ci,
  `top_devices` text COLLATE utf8mb4_unicode_ci,
  `hourly_traffic` text COLLATE utf8mb4_unicode_ci,
  `method_distribution` text COLLATE utf8mb4_unicode_ci,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `status2xx` bigint DEFAULT '0',
  `status3xx` bigint DEFAULT '0',
  `status4xx` bigint DEFAULT '0',
  `status5xx` bigint DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_source_date` (`source_id`,`date`),
  UNIQUE KEY `idx_source_date` (`source_id`,`date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `nginx_agg_hourly`
--

DROP TABLE IF EXISTS `nginx_agg_hourly`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `nginx_agg_hourly` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `source_id` bigint unsigned NOT NULL,
  `hour` datetime NOT NULL,
  `total_requests` bigint DEFAULT '0',
  `pv_count` bigint DEFAULT '0',
  `unique_ips` bigint DEFAULT '0',
  `total_bandwidth` bigint DEFAULT '0',
  `avg_response_time` decimal(10,3) DEFAULT '0.000',
  `max_response_time` decimal(10,3) DEFAULT '0.000',
  `min_response_time` decimal(10,3) DEFAULT '0.000',
  `status_2xx` bigint DEFAULT '0' COMMENT '2xx状态码数',
  `status_3xx` bigint DEFAULT '0' COMMENT '3xx状态码数',
  `status_4xx` bigint DEFAULT '0' COMMENT '4xx状态码数',
  `status_5xx` bigint DEFAULT '0' COMMENT '5xx状态码数',
  `method_distribution` text COLLATE utf8mb4_unicode_ci,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `status2xx` bigint DEFAULT '0',
  `status3xx` bigint DEFAULT '0',
  `status4xx` bigint DEFAULT '0',
  `status5xx` bigint DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_source_hour` (`source_id`,`hour`),
  UNIQUE KEY `idx_source_hour` (`source_id`,`hour`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `nginx_daily_stats`
--

DROP TABLE IF EXISTS `nginx_daily_stats`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `nginx_daily_stats` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `source_id` bigint unsigned NOT NULL,
  `date` date NOT NULL,
  `total_requests` bigint DEFAULT '0',
  `unique_visitors` bigint DEFAULT '0',
  `total_bandwidth` bigint DEFAULT '0',
  `avg_response_time` decimal(10,3) DEFAULT '0.000',
  `status_2xx` bigint DEFAULT '0' COMMENT '2xx状态码数',
  `status_3xx` bigint DEFAULT '0' COMMENT '3xx状态码数',
  `status_4xx` bigint DEFAULT '0' COMMENT '4xx状态码数',
  `status_5xx` bigint DEFAULT '0' COMMENT '5xx状态码数',
  `top_ur_is` text COLLATE utf8mb4_unicode_ci COMMENT 'Top URI JSON',
  `top_i_ps` text COLLATE utf8mb4_unicode_ci COMMENT 'Top IP JSON',
  `top_referers` text COLLATE utf8mb4_unicode_ci,
  `top_user_agents` text COLLATE utf8mb4_unicode_ci,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `status2xx` bigint DEFAULT '0',
  `status3xx` bigint DEFAULT '0',
  `status4xx` bigint DEFAULT '0',
  `status5xx` bigint DEFAULT '0',
  `top_uris` text COLLATE utf8mb4_unicode_ci,
  `top_ips` text COLLATE utf8mb4_unicode_ci,
  PRIMARY KEY (`id`),
  KEY `idx_source_id` (`source_id`),
  KEY `idx_date` (`date`),
  KEY `idx_nginx_daily_stats_source_id` (`source_id`),
  KEY `idx_nginx_daily_stats_date` (`date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `nginx_dim_ip`
--

DROP TABLE IF EXISTS `nginx_dim_ip`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `nginx_dim_ip` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `ip_address` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL,
  `country` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `province` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `city` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `isp` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `is_bot` tinyint DEFAULT '0',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_nginx_dim_ip_ip_address` (`ip_address`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `nginx_dim_referer`
--

DROP TABLE IF EXISTS `nginx_dim_referer`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `nginx_dim_referer` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `referer_hash` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `referer_url` varchar(2000) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `referer_domain` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `referer_type` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_nginx_dim_referer_referer_hash` (`referer_hash`),
  KEY `idx_referer_domain` (`referer_domain`),
  KEY `idx_nginx_dim_referer_referer_domain` (`referer_domain`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `nginx_dim_url`
--

DROP TABLE IF EXISTS `nginx_dim_url`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `nginx_dim_url` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `url_hash` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `url_path` varchar(2000) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `url_normalized` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `host` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_nginx_dim_url_url_hash` (`url_hash`),
  KEY `idx_url_normalized` (`url_normalized`),
  KEY `idx_host` (`host`),
  KEY `idx_nginx_dim_url_url_normalized` (`url_normalized`),
  KEY `idx_nginx_dim_url_host` (`host`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `nginx_dim_user_agent`
--

DROP TABLE IF EXISTS `nginx_dim_user_agent`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `nginx_dim_user_agent` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `ua_hash` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `user_agent` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `browser` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `browser_version` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `os` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `os_version` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `device_type` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `is_bot` tinyint DEFAULT '0',
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_nginx_dim_user_agent_ua_hash` (`ua_hash`),
  KEY `idx_browser` (`browser`),
  KEY `idx_os` (`os`),
  KEY `idx_device_type` (`device_type`),
  KEY `idx_is_bot` (`is_bot`),
  KEY `idx_nginx_dim_user_agent_browser` (`browser`),
  KEY `idx_nginx_dim_user_agent_os` (`os`),
  KEY `idx_nginx_dim_user_agent_device_type` (`device_type`),
  KEY `idx_nginx_dim_user_agent_is_bot` (`is_bot`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `nginx_fact_access_logs`
--

DROP TABLE IF EXISTS `nginx_fact_access_logs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `nginx_fact_access_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `source_id` bigint unsigned NOT NULL,
  `timestamp` datetime NOT NULL,
  `ip_id` bigint unsigned DEFAULT NULL,
  `url_id` bigint unsigned DEFAULT NULL,
  `referer_id` bigint unsigned DEFAULT NULL,
  `ua_id` bigint unsigned DEFAULT NULL,
  `method` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `protocol` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `status` bigint DEFAULT NULL,
  `body_bytes_sent` bigint DEFAULT NULL,
  `request_time` decimal(10,3) DEFAULT NULL,
  `upstream_time` decimal(10,3) DEFAULT NULL,
  `ingress_name` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `service_name` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `pod_name` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `is_pv` tinyint DEFAULT '1',
  `session_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_source_time` (`source_id`,`timestamp`),
  KEY `idx_ip_id` (`ip_id`),
  KEY `idx_url_id` (`url_id`),
  KEY `idx_referer_id` (`referer_id`),
  KEY `idx_ua_id` (`ua_id`),
  KEY `idx_method` (`method`),
  KEY `idx_status` (`status`),
  KEY `idx_nginx_fact_access_logs_ip_id` (`ip_id`),
  KEY `idx_nginx_fact_access_logs_url_id` (`url_id`),
  KEY `idx_nginx_fact_access_logs_referer_id` (`referer_id`),
  KEY `idx_nginx_fact_access_logs_ua_id` (`ua_id`),
  KEY `idx_nginx_fact_access_logs_method` (`method`),
  KEY `idx_nginx_fact_access_logs_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `nginx_hourly_stats`
--

DROP TABLE IF EXISTS `nginx_hourly_stats`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `nginx_hourly_stats` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `source_id` bigint unsigned NOT NULL,
  `hour` datetime NOT NULL,
  `total_requests` bigint DEFAULT '0',
  `unique_visitors` bigint DEFAULT '0',
  `total_bandwidth` bigint DEFAULT '0',
  `avg_response_time` decimal(10,3) DEFAULT '0.000',
  `status_2xx` bigint DEFAULT '0' COMMENT '2xx状态码数',
  `status_3xx` bigint DEFAULT '0' COMMENT '3xx状态码数',
  `status_4xx` bigint DEFAULT '0' COMMENT '4xx状态码数',
  `status_5xx` bigint DEFAULT '0' COMMENT '5xx状态码数',
  `created_at` datetime(3) DEFAULT NULL,
  `status2xx` bigint DEFAULT '0',
  `status3xx` bigint DEFAULT '0',
  `status4xx` bigint DEFAULT '0',
  `status5xx` bigint DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `idx_source_id` (`source_id`),
  KEY `idx_hour` (`hour`),
  KEY `idx_nginx_hourly_stats_source_id` (`source_id`),
  KEY `idx_nginx_hourly_stats_hour` (`hour`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `nginx_sources`
--

DROP TABLE IF EXISTS `nginx_sources`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `nginx_sources` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
  `type` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL,
  `description` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `status` tinyint DEFAULT '1',
  `host_id` bigint unsigned DEFAULT NULL,
  `log_path` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `log_format` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT 'combined',
  `cluster_id` bigint unsigned DEFAULT NULL,
  `namespace` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `ingress_name` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `k8s_pod_selector` varchar(200) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `k8s_container_name` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `log_format_config` text COLLATE utf8mb4_unicode_ci,
  `geo_enabled` tinyint DEFAULT '1',
  `session_enabled` tinyint DEFAULT '0',
  `collect_interval` bigint DEFAULT '60',
  `retention_days` bigint DEFAULT '30',
  `last_collect_at` datetime DEFAULT NULL,
  `last_collect_logs` bigint DEFAULT '0',
  `last_error` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `last_file_size` bigint DEFAULT '0',
  `last_file_offset` bigint DEFAULT '0',
  `last_file_inode` bigint DEFAULT '0',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_host_id` (`host_id`),
  KEY `idx_cluster_id` (`cluster_id`),
  KEY `idx_status` (`status`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_nginx_sources_host_id` (`host_id`),
  KEY `idx_nginx_sources_cluster_id` (`cluster_id`),
  KEY `idx_nginx_sources_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `oauth2_access_tokens`
--

DROP TABLE IF EXISTS `oauth2_access_tokens`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `oauth2_access_tokens` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `token_hash` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '令牌哈希',
  `client_id` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '客户端ID',
  `user_id` bigint unsigned NOT NULL COMMENT '用户ID',
  `scope` text COLLATE utf8mb4_unicode_ci COMMENT '授权范围',
  `expires_at` datetime NOT NULL COMMENT '过期时间',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_token_hash` (`token_hash`),
  KEY `idx_client_id` (`client_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_expires_at` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `oauth2_authorization_codes`
--

DROP TABLE IF EXISTS `oauth2_authorization_codes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `oauth2_authorization_codes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `code` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '授权码',
  `client_id` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '客户端ID',
  `user_id` bigint unsigned NOT NULL COMMENT '用户ID',
  `scope` text COLLATE utf8mb4_unicode_ci COMMENT '授权范围',
  `redirect_uri` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '重定向URI',
  `code_challenge` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'PKCE挑战码',
  `code_challenge_method` varchar(10) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'PKCE方法(S256/plain)',
  `expires_at` datetime NOT NULL COMMENT '过期时间',
  `used` tinyint(1) DEFAULT '0' COMMENT '是否已使用',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_client_id` (`client_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_expires_at` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `oauth2_refresh_tokens`
--

DROP TABLE IF EXISTS `oauth2_refresh_tokens`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `oauth2_refresh_tokens` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `token_hash` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '令牌哈希',
  `access_token_id` bigint unsigned NOT NULL COMMENT '关联的访问令牌ID',
  `expires_at` datetime NOT NULL COMMENT '过期时间',
  `revoked` tinyint(1) DEFAULT '0' COMMENT '是否已撤销',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_token_hash` (`token_hash`),
  KEY `idx_access_token_id` (`access_token_id`),
  KEY `idx_expires_at` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `oauth_states`
--

DROP TABLE IF EXISTS `oauth_states`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `oauth_states` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `state` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '状态码',
  `provider` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '提供商类型',
  `redirect_url` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '回调后重定向URL',
  `action` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'login' COMMENT '操作类型(login/bind)',
  `user_id` bigint unsigned DEFAULT '0' COMMENT '用户ID(绑定操作时使用)',
  `expires_at` datetime NOT NULL COMMENT '过期时间',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_state` (`state`),
  KEY `idx_expires_at` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `plugin_states`
--

DROP TABLE IF EXISTS `plugin_states`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `plugin_states` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
  `enabled` tinyint(1) NOT NULL DEFAULT '0',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_plugin_states_name` (`name`)
) ENGINE=InnoDB AUTO_INCREMENT=6 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `ssh_terminal_sessions`
--

DROP TABLE IF EXISTS `ssh_terminal_sessions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `ssh_terminal_sessions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `host_id` bigint unsigned NOT NULL COMMENT '主机ID',
  `host_name` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '主机名称',
  `host_ip` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '主机IP',
  `user_id` bigint unsigned NOT NULL COMMENT '操作用户ID',
  `username` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '用户名',
  `recording_path` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '录制文件路径',
  `duration` bigint DEFAULT NULL COMMENT '会话时长(秒)',
  `file_size` bigint DEFAULT NULL COMMENT '文件大小(字节)',
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'recording' COMMENT '会话状态 recording/completed/failed',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_host_id` (`host_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_status` (`status`),
  KEY `idx_created_at` (`created_at`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_ssh_terminal_sessions_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `ssl_certificates`
--

DROP TABLE IF EXISTS `ssl_certificates`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `ssl_certificates` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
  `domain` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `san_domains` text COLLATE utf8mb4_unicode_ci,
  `acme_email` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `ca_provider` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `key_algorithm` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `source_type` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `cloud_account_id` bigint unsigned DEFAULT NULL,
  `cloud_cert_id` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `certificate` text COLLATE utf8mb4_unicode_ci,
  `private_key` text COLLATE utf8mb4_unicode_ci,
  `cert_chain` text COLLATE utf8mb4_unicode_ci,
  `issuer` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `not_before` datetime(3) DEFAULT NULL,
  `not_after` datetime(3) DEFAULT NULL,
  `fingerprint` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'pending',
  `auto_renew` tinyint(1) DEFAULT '1',
  `renew_days_before` bigint DEFAULT '30',
  `dns_provider_id` bigint unsigned DEFAULT NULL,
  `last_renew_at` datetime(3) DEFAULT NULL,
  `last_error` text COLLATE utf8mb4_unicode_ci,
  PRIMARY KEY (`id`),
  KEY `idx_ssl_certificates_deleted_at` (`deleted_at`),
  KEY `idx_ssl_certificates_domain` (`domain`),
  KEY `idx_ssl_certificates_not_after` (`not_after`),
  KEY `idx_ssl_certificates_cloud_account_id` (`cloud_account_id`),
  KEY `idx_ssl_certificates_dns_provider_id` (`dns_provider_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `ssl_deploy_configs`
--

DROP TABLE IF EXISTS `ssl_deploy_configs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `ssl_deploy_configs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `certificate_id` bigint unsigned NOT NULL,
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
  `deploy_type` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL,
  `target_config` text COLLATE utf8mb4_unicode_ci NOT NULL,
  `auto_deploy` tinyint(1) DEFAULT '1',
  `enabled` tinyint(1) DEFAULT '1',
  `last_deploy_at` datetime(3) DEFAULT NULL,
  `last_deploy_ok` tinyint(1) DEFAULT NULL,
  `last_error` text COLLATE utf8mb4_unicode_ci,
  PRIMARY KEY (`id`),
  KEY `idx_ssl_deploy_configs_deleted_at` (`deleted_at`),
  KEY `idx_ssl_deploy_configs_certificate_id` (`certificate_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `ssl_dns_providers`
--

DROP TABLE IF EXISTS `ssl_dns_providers`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `ssl_dns_providers` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
  `provider` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL,
  `config` text COLLATE utf8mb4_unicode_ci NOT NULL,
  `email` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `phone` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `enabled` tinyint(1) DEFAULT '1',
  `last_test_at` datetime(3) DEFAULT NULL,
  `last_test_ok` tinyint(1) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_ssl_dns_providers_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `ssl_renew_tasks`
--

DROP TABLE IF EXISTS `ssl_renew_tasks`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `ssl_renew_tasks` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `certificate_id` bigint unsigned NOT NULL,
  `task_type` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL,
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'pending',
  `trigger_type` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL,
  `started_at` datetime(3) DEFAULT NULL,
  `finished_at` datetime(3) DEFAULT NULL,
  `error_message` text COLLATE utf8mb4_unicode_ci,
  `result` text COLLATE utf8mb4_unicode_ci,
  PRIMARY KEY (`id`),
  KEY `idx_ssl_renew_tasks_deleted_at` (`deleted_at`),
  KEY `idx_ssl_renew_tasks_certificate_id` (`certificate_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sso_applications`
--

DROP TABLE IF EXISTS `sso_applications`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sso_applications` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '应用名称',
  `code` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '应用编码',
  `icon` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '图标URL',
  `description` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '应用描述',
  `category` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '分类(cicd/code/monitor/registry/other)',
  `url` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '应用URL',
  `sso_type` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'SSO类型(oauth2/saml/form/token)',
  `sso_config` text COLLATE utf8mb4_unicode_ci COMMENT 'SSO配置JSON',
  `enabled` tinyint(1) DEFAULT '1' COMMENT '是否启用',
  `sort` int DEFAULT '0' COMMENT '排序',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL COMMENT '删除时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`,`deleted_at`),
  KEY `idx_category` (`category`),
  KEY `idx_enabled` (`enabled`),
  KEY `idx_sort` (`sort`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sys_config`
--

DROP TABLE IF EXISTS `sys_config`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sys_config` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `key` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '配置键',
  `value` text COLLATE utf8mb4_unicode_ci COMMENT '配置值',
  `type` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'string' COMMENT '配置类型(string/int/bool/json)',
  `group` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '配置分组(basic/security)',
  `remark` varchar(200) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '备注说明',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_sys_config_key` (`key`),
  KEY `idx_group` (`group`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_sys_config_group` (`group`),
  KEY `idx_sys_config_deleted_at` (`deleted_at`)
) ENGINE=InnoDB AUTO_INCREMENT=13 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sys_data_log`
--

DROP TABLE IF EXISTS `sys_data_log`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sys_data_log` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned DEFAULT NULL COMMENT '用户ID',
  `username` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '用户名',
  `real_name` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '真实姓名',
  `table_name` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '操作表名',
  `record_id` bigint unsigned DEFAULT NULL COMMENT '记录ID',
  `action` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '操作类型',
  `old_data` longtext COLLATE utf8mb4_unicode_ci COMMENT '旧数据',
  `new_data` longtext COLLATE utf8mb4_unicode_ci COMMENT '新数据',
  `diff_fields` text COLLATE utf8mb4_unicode_ci COMMENT '变更字段',
  `ip` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '客户端IP',
  `user_agent` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '用户代理',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL COMMENT '删除时间',
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_table_name` (`table_name`),
  KEY `idx_record_id` (`record_id`),
  KEY `idx_created_at` (`created_at`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sys_data_logs`
--

DROP TABLE IF EXISTS `sys_data_logs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sys_data_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `user_id` bigint unsigned DEFAULT NULL COMMENT '用户ID',
  `username` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '用户名',
  `real_name` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '真实姓名',
  `table_name` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '表名',
  `record_id` bigint unsigned DEFAULT NULL COMMENT '记录ID',
  `action` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '操作类型',
  `old_data` longtext COLLATE utf8mb4_unicode_ci COMMENT '原始数据',
  `new_data` longtext COLLATE utf8mb4_unicode_ci COMMENT '新数据',
  `diff_fields` text COLLATE utf8mb4_unicode_ci COMMENT '差异字段',
  `ip` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'IP地址',
  `user_agent` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '用户代理',
  PRIMARY KEY (`id`),
  KEY `idx_sys_data_logs_deleted_at` (`deleted_at`),
  KEY `idx_sys_data_logs_user_id` (`user_id`),
  KEY `idx_sys_data_logs_record_id` (`record_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sys_department`
--

DROP TABLE IF EXISTS `sys_department`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sys_department` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '部门名称',
  `code` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '部门编码',
  `parent_id` bigint unsigned DEFAULT '0' COMMENT '父部门ID',
  `dept_type` tinyint DEFAULT '3' COMMENT '部门类型 1:公司 2:中心 3:部门',
  `sort` int DEFAULT '0' COMMENT '排序',
  `status` tinyint DEFAULT '1' COMMENT '状态 1:启用 0:禁用',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`,`deleted_at`),
  UNIQUE KEY `idx_sys_department_code` (`code`),
  KEY `idx_parent_id` (`parent_id`),
  KEY `idx_dept_type` (`dept_type`),
  KEY `idx_sort` (`sort`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_sys_department_deleted_at` (`deleted_at`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sys_login_log`
--

DROP TABLE IF EXISTS `sys_login_log`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sys_login_log` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned DEFAULT NULL COMMENT '用户ID',
  `username` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '用户名',
  `real_name` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '真实姓名',
  `login_type` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '登录类型',
  `login_status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '登录状态',
  `login_time` datetime(3) DEFAULT NULL COMMENT '登录时间',
  `logout_time` datetime(3) DEFAULT NULL COMMENT '登出时间',
  `ip` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'IP地址',
  `location` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '登录地点',
  `user_agent` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '用户代理',
  `fail_reason` varchar(200) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '失败原因',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_username` (`username`),
  KEY `idx_login_time` (`login_time`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_sys_login_log_deleted_at` (`deleted_at`),
  KEY `idx_sys_login_log_user_id` (`user_id`),
  KEY `idx_sys_login_log_username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sys_menu`
--

DROP TABLE IF EXISTS `sys_menu`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sys_menu` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '菜单名称',
  `code` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '菜单编码',
  `type` tinyint NOT NULL COMMENT '类型 1:目录 2:菜单 3:按钮',
  `parent_id` bigint unsigned DEFAULT '0' COMMENT '父菜单ID',
  `path` varchar(200) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '路由路径',
  `component` varchar(200) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '组件路径',
  `icon` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '图标',
  `sort` int DEFAULT '0' COMMENT '排序',
  `visible` tinyint DEFAULT '1' COMMENT '是否显示 1:显示 0:隐藏',
  `status` tinyint DEFAULT '1' COMMENT '状态 1:启用 0:禁用',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`,`deleted_at`),
  UNIQUE KEY `idx_sys_menu_code` (`code`),
  KEY `idx_parent_id` (`parent_id`),
  KEY `idx_type` (`type`),
  KEY `idx_sort` (`sort`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_sys_menu_deleted_at` (`deleted_at`)
) ENGINE=InnoDB AUTO_INCREMENT=100 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sys_mfa_log`
--

DROP TABLE IF EXISTS `sys_mfa_log`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sys_mfa_log` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `mfa_type` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `action` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL,
  `ip_address` varchar(45) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `user_agent` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `success` tinyint(1) DEFAULT '0',
  `message` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_created_at` (`created_at`),
  KEY `idx_sys_mfa_log_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sys_operation_log`
--

DROP TABLE IF EXISTS `sys_operation_log`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sys_operation_log` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned DEFAULT NULL COMMENT '用户ID',
  `username` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '用户名',
  `real_name` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '真实姓名',
  `module` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '模块名称',
  `action` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '操作类型',
  `description` varchar(200) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '操作描述',
  `method` varchar(10) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '请求方法',
  `path` varchar(200) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '请求路径',
  `params` text COLLATE utf8mb4_unicode_ci COMMENT '请求参数',
  `status` bigint DEFAULT NULL COMMENT '状态码',
  `error_msg` text COLLATE utf8mb4_unicode_ci COMMENT '错误信息',
  `cost_time` bigint DEFAULT NULL COMMENT '耗时(毫秒)',
  `ip` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'IP地址',
  `user_agent` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '用户代理',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_username` (`username`),
  KEY `idx_action` (`action`),
  KEY `idx_created_at` (`created_at`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_sys_operation_log_deleted_at` (`deleted_at`),
  KEY `idx_sys_operation_log_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sys_position`
--

DROP TABLE IF EXISTS `sys_position`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sys_position` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `post_name` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '岗位名称',
  `post_code` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '岗位编码',
  `post_status` tinyint DEFAULT '1' COMMENT '状态 1:启用 2:禁用',
  `remark` varchar(200) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '备注',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_sys_position_post_code` (`post_code`),
  UNIQUE KEY `uk_post_code` (`post_code`,`deleted_at`),
  KEY `idx_post_status` (`post_status`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_sys_position_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sys_role`
--

DROP TABLE IF EXISTS `sys_role`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sys_role` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '角色名称',
  `code` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '角色编码',
  `description` varchar(200) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '角色描述',
  `sort` int DEFAULT '0' COMMENT '排序',
  `status` tinyint DEFAULT '1' COMMENT '状态 1:启用 0:禁用',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_sys_role_name` (`name`),
  UNIQUE KEY `idx_sys_role_code` (`code`),
  UNIQUE KEY `uk_name` (`name`,`deleted_at`),
  UNIQUE KEY `uk_code` (`code`,`deleted_at`),
  KEY `idx_sort` (`sort`),
  KEY `idx_status` (`status`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_sys_role_deleted_at` (`deleted_at`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sys_role_asset_permission`
--

DROP TABLE IF EXISTS `sys_role_asset_permission`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sys_role_asset_permission` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `role_id` bigint unsigned NOT NULL,
  `asset_group_id` bigint unsigned NOT NULL,
  `host_ids` json DEFAULT NULL,
  `permissions` int unsigned DEFAULT '1' COMMENT '操作权限位掩码：1=查看,2=编辑,4=删除,8=终端,16=文件,32=采集,64=桌面',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_role_asset` (`role_id`,`asset_group_id`,`deleted_at`),
  KEY `idx_asset_group_id` (`asset_group_id`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_sys_role_asset_permission_deleted_at` (`deleted_at`),
  KEY `idx_role_asset` (`role_id`,`asset_group_id`),
  KEY `idx_sys_role_asset_permission_permissions` (`permissions`),
  CONSTRAINT `fk_role_asset_perm_group` FOREIGN KEY (`asset_group_id`) REFERENCES `asset_group` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_role_asset_perm_role` FOREIGN KEY (`role_id`) REFERENCES `sys_role` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sys_role_menu`
--

DROP TABLE IF EXISTS `sys_role_menu`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sys_role_menu` (
  `role_id` bigint unsigned NOT NULL COMMENT '角色ID',
  `menu_id` bigint unsigned NOT NULL COMMENT '菜单ID',
  PRIMARY KEY (`role_id`,`menu_id`),
  KEY `idx_menu_id` (`menu_id`),
  CONSTRAINT `fk_role_menu_menu` FOREIGN KEY (`menu_id`) REFERENCES `sys_menu` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_role_menu_role` FOREIGN KEY (`role_id`) REFERENCES `sys_role` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sys_user`
--

DROP TABLE IF EXISTS `sys_user`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sys_user` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `username` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '用户名',
  `password` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '密码',
  `real_name` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '真实姓名',
  `email` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '邮箱',
  `phone` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '手机号',
  `avatar` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '头像',
  `status` tinyint DEFAULT '1' COMMENT '状态 1:启用 0:禁用',
  `source` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'local' COMMENT '用户来源 local:本地 ldap:LDAP',
  `department_id` bigint unsigned DEFAULT '0' COMMENT '部门ID',
  `bio` text COLLATE utf8mb4_unicode_ci COMMENT '个人简介',
  `last_login_at` datetime(3) DEFAULT NULL COMMENT '最后登录时间',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `is_deleted` tinyint(1) GENERATED ALWAYS AS ((case when (`deleted_at` is null) then 0 else 1 end)) STORED,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username_deleted` (`username`,`deleted_at`),
  UNIQUE KEY `idx_username_email_is_deleted` (`username`,`email`,`is_deleted`),
  KEY `idx_department_id` (`department_id`),
  KEY `idx_status` (`status`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_sys_user_deleted_at` (`deleted_at`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sys_user_login_attempt`
--

DROP TABLE IF EXISTS `sys_user_login_attempt`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sys_user_login_attempt` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `username` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '用户名',
  `fail_count` int DEFAULT '0' COMMENT '失败次数',
  `last_fail_at` datetime(3) DEFAULT NULL COMMENT '最后失败时间',
  `locked_until` datetime(3) DEFAULT NULL COMMENT '锁定截止时间',
  PRIMARY KEY (`id`),
  KEY `idx_username` (`username`),
  KEY `idx_sys_user_login_attempt_username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sys_user_position`
--

DROP TABLE IF EXISTS `sys_user_position`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sys_user_position` (
  `user_id` bigint unsigned NOT NULL COMMENT '用户ID',
  `position_id` bigint unsigned NOT NULL COMMENT '职位ID',
  PRIMARY KEY (`user_id`,`position_id`),
  KEY `idx_position_id` (`position_id`),
  CONSTRAINT `fk_user_position_position` FOREIGN KEY (`position_id`) REFERENCES `sys_position` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_user_position_user` FOREIGN KEY (`user_id`) REFERENCES `sys_user` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sys_user_role`
--

DROP TABLE IF EXISTS `sys_user_role`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sys_user_role` (
  `user_id` bigint unsigned NOT NULL COMMENT '用户ID',
  `role_id` bigint unsigned NOT NULL COMMENT '角色ID',
  PRIMARY KEY (`user_id`,`role_id`),
  KEY `idx_role_id` (`role_id`),
  CONSTRAINT `fk_user_role_role` FOREIGN KEY (`role_id`) REFERENCES `sys_role` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_user_role_user` FOREIGN KEY (`user_id`) REFERENCES `sys_user` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `user_credentials`
--

DROP TABLE IF EXISTS `user_credentials`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `user_credentials` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL COMMENT '用户ID',
  `app_id` bigint unsigned NOT NULL COMMENT '应用ID',
  `username` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '应用账号',
  `password` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '应用密码(AES加密存储)',
  `extra_data` text COLLATE utf8mb4_unicode_ci COMMENT '额外数据JSON',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL COMMENT '删除时间',
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_app_id` (`app_id`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `user_favorite_apps`
--

DROP TABLE IF EXISTS `user_favorite_apps`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `user_favorite_apps` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL COMMENT '用户ID',
  `app_id` bigint unsigned NOT NULL COMMENT '应用ID',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_app` (`user_id`,`app_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_app_id` (`app_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `user_oauth_bindings`
--

DROP TABLE IF EXISTS `user_oauth_bindings`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `user_oauth_bindings` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL COMMENT '用户ID',
  `source_id` bigint unsigned NOT NULL COMMENT '身份源ID',
  `source_type` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '身份源类型',
  `open_id` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'OpenID',
  `union_id` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'UnionID',
  `nickname` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '昵称',
  `avatar` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '头像URL',
  `extra_info` text COLLATE utf8mb4_unicode_ci COMMENT '额外信息JSON',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL COMMENT '删除时间',
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_source_id` (`source_id`),
  KEY `idx_open_id` (`open_id`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2026-04-09 16:52:06
-- MySQL dump 10.13  Distrib 8.0.45, for Linux (x86_64)
--
-- Host: localhost    Database: opshub
-- ------------------------------------------------------
-- Server version	8.0.45

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!50503 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Dumping data for table `sys_department`
--

LOCK TABLES `sys_department` WRITE;
/*!40000 ALTER TABLE `sys_department` DISABLE KEYS */;
INSERT INTO `sys_department` VALUES (1,'总公司','head',0,1,0,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL);
/*!40000 ALTER TABLE `sys_department` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Dumping data for table `sys_role`
--

LOCK TABLES `sys_role` WRITE;
/*!40000 ALTER TABLE `sys_role` DISABLE KEYS */;
INSERT INTO `sys_role` VALUES (1,'管理员','admin','系统管理员，拥有所有权限',0,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(2,'普通用户','user','普通用户，具有基本操作权限',1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL);
/*!40000 ALTER TABLE `sys_role` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Dumping data for table `sys_menu`
--

LOCK TABLES `sys_menu` WRITE;
/*!40000 ALTER TABLE `sys_menu` DISABLE KEYS */;
INSERT INTO `sys_menu` VALUES (1,'系统管理','system',1,0,'','','Setting',100,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(2,'用户管理','users',2,1,'/users','system/Users','User',1,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(3,'角色管理','roles',2,1,'/roles','system/Roles','UserFilled',2,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(5,'菜单管理','menus',2,1,'/menus','system/Menus','Menu',4,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(10,'仪表盘','dashboard',1,0,'/dashboard','','HomeFilled',0,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(11,'部门信息','dept-info',2,1,'/dept-info','system/DeptInfo','OfficeBuilding',5,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(12,'岗位信息','position-info',2,1,'/position-info','system/PositionInfo','Avatar',6,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(13,'系统配置','system-config',2,1,'/system-config','system/SystemConfig','Setting',7,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(15,'资产管理','asset-management',1,0,'/asset','','Coin',1,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(16,'主机管理','host-management',2,15,'/asset/hosts','asset/Hosts','Monitor',1,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(17,'业务分组','business-group',2,15,'/asset/groups','asset/Groups','Collection',3,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(19,'凭据管理','asset:credentials',3,15,'/asset/credentials','asset/Credentials','Lock',2,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(23,'操作审计','audit',1,0,'/audit','','Document',50,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(24,'操作日志','operation-logs',2,23,'/audit/operation-logs','audit/OperationLogs','Document',1,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(25,'登录日志','login-logs',2,23,'/audit/login-logs','audit/LoginLogs','CircleCheck',2,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(27,'云账号管理','cloud-accounts',2,15,'/asset/cloud-accounts','asset/CloudAccounts','Cloudy',5,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(29,'个人信息','profile',2,0,'/profile','Profile','UserFilled',100,0,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(30,'插件管理','plugin',1,0,'/plugin','','Grid',80,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(32,'插件列表','plugin-list',2,30,'/plugin/list','plugin/PluginList','Grid',1,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(33,'插件安装','plugin-install',2,30,'/plugin/install','plugin/PluginInstall','Upload',2,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(34,'终端审计','asset_terminal_audit',2,15,'/asset/terminal-audit','','View',5,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(35,'Agent管理','plugin-agents',2,30,'/plugin/agents','asset/Agents','Connection',3,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(65,'权限配置','asset_permission',2,15,'/asset/permissions','views/asset/AssetPermission.vue','Lock',6,1,1,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(66,'Nginx统计','_nginx',1,0,'/nginx','','DataLine',50,1,1,'2026-04-09 16:52:02.017','2026-04-09 16:52:02.017',NULL),(67,'概况','_nginx_nginx_overview',2,66,'/nginx/overview','','PieChart',1,1,1,'2026-04-09 16:52:02.020','2026-04-09 16:52:02.020',NULL),(68,'Top分析','_nginx_nginx_top_analysis',2,66,'/nginx/top-analysis','','Histogram',2,1,1,'2026-04-09 16:52:02.022','2026-04-09 16:52:02.022',NULL),(69,'数据日报','_nginx_nginx_daily_report',2,66,'/nginx/daily-report','','Calendar',3,1,1,'2026-04-09 16:52:02.024','2026-04-09 16:52:02.024',NULL),(70,'访问明细','_nginx_nginx_access_logs',2,66,'/nginx/access-logs','','List',4,1,1,'2026-04-09 16:52:02.026','2026-04-09 16:52:02.026',NULL),(71,'数据源配置','_nginx_nginx_config',2,66,'/nginx/config','','Setting',5,1,1,'2026-04-09 16:52:02.029','2026-04-09 16:52:02.029',NULL),(72,'SSL证书','_ssl_cert',1,0,'/ssl-cert','','Key',50,1,1,'2026-04-09 16:52:03.144','2026-04-09 16:52:03.144',NULL),(73,'证书管理','_ssl_cert_ssl_cert_certificates',2,72,'/ssl-cert/certificates','','Document',1,1,1,'2026-04-09 16:52:03.146','2026-04-09 16:52:03.146',NULL),(74,'DNS配置','_ssl_cert_ssl_cert_dns_providers',2,72,'/ssl-cert/dns-providers','','Connection',2,1,1,'2026-04-09 16:52:03.148','2026-04-09 16:52:03.148',NULL),(75,'部署配置','_ssl_cert_ssl_cert_deploy_configs',2,72,'/ssl-cert/deploy-configs','','Upload',3,1,1,'2026-04-09 16:52:03.149','2026-04-09 16:52:03.149',NULL),(76,'任务记录','_ssl_cert_ssl_cert_tasks',2,72,'/ssl-cert/tasks','','List',4,1,1,'2026-04-09 16:52:03.151','2026-04-09 16:52:03.151',NULL),(77,'容器管理','_kubernetes',1,0,'/kubernetes','','Platform',100,1,1,'2026-04-09 16:52:03.171','2026-04-09 16:52:03.171',NULL),(78,'集群管理','_kubernetes_kubernetes_clusters',2,77,'/kubernetes/clusters','','OfficeBuilding',1,1,1,'2026-04-09 16:52:03.172','2026-04-09 16:52:03.172',NULL),(79,'节点管理','_kubernetes_kubernetes_nodes',2,77,'/kubernetes/nodes','','Monitor',2,1,1,'2026-04-09 16:52:03.174','2026-04-09 16:52:03.174',NULL),(80,'命名空间','_kubernetes_kubernetes_namespaces',2,77,'/kubernetes/namespaces','','FolderOpened',3,1,1,'2026-04-09 16:52:03.177','2026-04-09 16:52:03.177',NULL),(81,'工作负载','_kubernetes_kubernetes_workloads',2,77,'/kubernetes/workloads','','Tools',4,1,1,'2026-04-09 16:52:03.179','2026-04-09 16:52:03.179',NULL),(82,'网络管理','_kubernetes_kubernetes_network',2,77,'/kubernetes/network','','Connection',5,1,1,'2026-04-09 16:52:03.181','2026-04-09 16:52:03.181',NULL),(83,'配置管理','_kubernetes_kubernetes_config',2,77,'/kubernetes/config','','Document',6,1,1,'2026-04-09 16:52:03.184','2026-04-09 16:52:03.184',NULL),(84,'存储管理','_kubernetes_kubernetes_storage',2,77,'/kubernetes/storage','','Files',7,1,1,'2026-04-09 16:52:03.186','2026-04-09 16:52:03.186',NULL),(85,'访问控制','_kubernetes_kubernetes_access',2,77,'/kubernetes/access','','Lock',8,1,1,'2026-04-09 16:52:03.188','2026-04-09 16:52:03.188',NULL),(86,'终端审计','_kubernetes_kubernetes_audit',2,77,'/kubernetes/audit','','View',9,1,1,'2026-04-09 16:52:03.189','2026-04-09 16:52:03.189',NULL),(87,'应用诊断','_kubernetes_kubernetes_application_diagnosis',2,77,'/kubernetes/application-diagnosis','','Cpu',10,1,1,'2026-04-09 16:52:03.191','2026-04-09 16:52:03.191',NULL),(88,'集群巡检','_kubernetes_kubernetes_cluster_inspection',2,77,'/kubernetes/cluster-inspection','','DocumentChecked',11,1,1,'2026-04-09 16:52:03.193','2026-04-09 16:52:03.193',NULL),(89,'任务中心','_task',1,0,'/task','','Tickets',90,1,1,'2026-04-09 16:52:03.222','2026-04-09 16:52:03.222',NULL),(90,'执行任务','_task_task_execute',2,89,'/task/execute','','VideoPlay',1,1,1,'2026-04-09 16:52:03.224','2026-04-09 16:52:03.224',NULL),(91,'模板管理','_task_task_templates',2,89,'/task/templates','','Document',2,1,1,'2026-04-09 16:52:03.226','2026-04-09 16:52:03.226',NULL),(92,'文件分发','_task_task_file_distribution',2,89,'/task/file-distribution','','FolderOpened',3,1,1,'2026-04-09 16:52:03.228','2026-04-09 16:52:03.228',NULL),(93,'监控中心','_monitor',1,0,'/monitor','','Monitor',20,1,1,'2026-04-09 16:52:04.955','2026-04-09 16:52:04.955',NULL),(94,'域名监控','_monitor_monitor_domain',2,93,'/monitor/domain','','Monitor',1,1,1,'2026-04-09 16:52:04.959','2026-04-09 16:52:04.959',NULL),(95,'主机监控','_monitor_monitor_hosts',2,93,'/monitor/hosts','','Monitor',2,1,1,'2026-04-09 16:52:04.963','2026-04-09 16:52:04.963',NULL),(96,'告警通道','_monitor_monitor_alert_channels',2,93,'/monitor/alert-channels','','Bell',3,1,1,'2026-04-09 16:52:04.967','2026-04-09 16:52:04.967',NULL),(97,'告警接收人','_monitor_monitor_alert_receivers',2,93,'/monitor/alert-receivers','','User',4,1,1,'2026-04-09 16:52:04.970','2026-04-09 16:52:04.970',NULL),(98,'主机告警','_monitor_monitor_host_alert_rules',2,93,'/monitor/host-alert-rules','','Bell',5,1,1,'2026-04-09 16:52:04.972','2026-04-09 16:52:04.972',NULL),(99,'告警日志','_monitor_monitor_alert_logs',2,93,'/monitor/alert-logs','','Document',6,1,1,'2026-04-09 16:52:04.974','2026-04-09 16:52:04.974',NULL);
/*!40000 ALTER TABLE `sys_menu` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Dumping data for table `sys_role_menu`
--

LOCK TABLES `sys_role_menu` WRITE;
/*!40000 ALTER TABLE `sys_role_menu` DISABLE KEYS */;
INSERT INTO `sys_role_menu` VALUES (1,1),(1,2),(1,3),(1,5),(1,10),(2,10),(1,11),(1,12),(1,13),(1,15),(2,15),(1,16),(2,16),(1,17),(2,17),(1,19),(2,19),(1,23),(2,23),(1,24),(2,24),(1,25),(2,25),(1,27),(2,27),(1,29),(1,30),(1,32),(1,33),(1,34),(2,34),(1,35),(1,65),(2,65),(1,66),(1,67),(1,68),(1,69),(1,70),(1,71),(1,72),(1,73),(1,74),(1,75),(1,76),(1,77),(1,78),(1,79),(1,80),(1,81),(1,82),(1,83),(1,84),(1,85),(1,86),(1,87),(1,88),(1,89),(1,90),(1,91),(1,92),(1,93),(1,94),(1,95),(1,96),(1,97),(1,98),(1,99);
/*!40000 ALTER TABLE `sys_role_menu` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Dumping data for table `sys_position`
--

LOCK TABLES `sys_position` WRITE;
/*!40000 ALTER TABLE `sys_position` DISABLE KEYS */;
/*!40000 ALTER TABLE `sys_position` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Dumping data for table `sys_user`
--

LOCK TABLES `sys_user` WRITE;
/*!40000 ALTER TABLE `sys_user` DISABLE KEYS */;
INSERT INTO `sys_user` (`id`, `username`, `password`, `real_name`, `email`, `phone`, `avatar`, `status`, `source`, `department_id`, `bio`, `last_login_at`, `created_at`, `updated_at`, `deleted_at`) VALUES (1,'admin','$2a$10$RLkgoedTSa0dYj3ujbXMcunSED3c6GLvfdKYsmpz0l0YFZbVrSBqW','系统管理员','admin@opshub.io',NULL,NULL,1,'local',1,NULL,NULL,'2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL);
/*!40000 ALTER TABLE `sys_user` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Dumping data for table `sys_user_role`
--

LOCK TABLES `sys_user_role` WRITE;
/*!40000 ALTER TABLE `sys_user_role` DISABLE KEYS */;
INSERT INTO `sys_user_role` VALUES (1,1);
/*!40000 ALTER TABLE `sys_user_role` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Dumping data for table `sys_user_position`
--

LOCK TABLES `sys_user_position` WRITE;
/*!40000 ALTER TABLE `sys_user_position` DISABLE KEYS */;
/*!40000 ALTER TABLE `sys_user_position` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Dumping data for table `sys_config`
--

LOCK TABLES `sys_config` WRITE;
/*!40000 ALTER TABLE `sys_config` DISABLE KEYS */;
INSERT INTO `sys_config` VALUES (1,'system_name','OpsHub','string','basic','系统名称','2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(2,'system_logo','','string','basic','系统Logo路径','2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(3,'system_description','运维管理平台','string','basic','系统描述','2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(4,'password_min_length','8','int','security','密码最小长度','2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(5,'session_timeout','3600','int','security','Session超时时间(秒)','2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(6,'enable_captcha','true','bool','security','是否开启验证码','2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(7,'max_login_attempts','5','int','security','最大登录失败次数','2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(8,'lockout_duration','300','int','security','账户锁定时间(秒)','2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(9,'mfa_enabled','false','bool','security','是否启用MFA功能','2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(10,'mfa_enforced','false','bool','security','是否强制所有用户启用MFA','2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(11,'mfa_type','totp','string','security','MFA类型(totp)','2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL),(12,'mfa_skip_duration','2592000','int','security','MFA记住设备时长(秒)','2026-04-09 16:51:48.000','2026-04-09 16:51:48.000',NULL);
/*!40000 ALTER TABLE `sys_config` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Dumping data for table `plugin_states`
--

LOCK TABLES `plugin_states` WRITE;
/*!40000 ALTER TABLE `plugin_states` DISABLE KEYS */;
INSERT INTO `plugin_states` VALUES (1,'kubernetes',1,'2026-04-09 16:51:48.000','2026-04-09 16:52:03.215'),(2,'monitor',1,'2026-04-09 16:51:48.000','2026-04-09 16:52:04.991'),(3,'task',1,'2026-04-09 16:51:48.000','2026-04-09 16:52:03.237'),(4,'ssl-cert',1,'2026-04-09 16:51:48.000','2026-04-09 16:52:03.162'),(5,'nginx',1,'2026-04-09 16:51:48.000','2026-04-09 16:52:02.040');
/*!40000 ALTER TABLE `plugin_states` ENABLE KEYS */;
UNLOCK TABLES;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2026-04-09 16:52:06
