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
