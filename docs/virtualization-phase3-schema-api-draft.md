# 虚拟化平台管理三期：表结构与接口草案

## 1. 审计表
### `virtualization_action_logs`
字段建议：
1. `platform_id`
2. `cluster_id`
3. `host_id`
4. `guest_id`
5. `action`
6. `risk_level`
7. `status`
8. `target_type`
9. `target_name`
10. `operator_id`
11. `operator_name`
12. `confirm_required`
13. `reason`
14. `request_payload`
15. `result_message`
16. `started_at`
17. `finished_at`

## 2. 电源操作接口
### `POST /api/v1/virtualization/guests/{id}/power`
请求：
```json
{
  "action": "power_on",
  "reason": "夜间维护恢复业务",
  "confirmToken": "optional"
}
```

返回：
```json
{
  "jobId": 1001,
  "status": "success",
  "message": "虚机已开机"
}
```

## 3. 快照接口
### `GET /api/v1/virtualization/guests/{id}/snapshots`
### `POST /api/v1/virtualization/guests/{id}/snapshots`
### `POST /api/v1/virtualization/guests/{id}/snapshots/{snapshotId}/restore`
### `DELETE /api/v1/virtualization/guests/{id}/snapshots/{snapshotId}`

## 4. 控制台跳转接口
### `POST /api/v1/virtualization/guests/{id}/console-link`
返回：
```json
{
  "provider": "pve",
  "url": "https://...",
  "expiresAt": "2026-04-20 18:30:00"
}
```

## 5. 审计查询接口
### `GET /api/v1/virtualization/action-logs`
筛选参数：
1. `platformId`
2. `guestId`
3. `action`
4. `status`
5. `riskLevel`
6. `operatorId`
7. `dateFrom`
8. `dateTo`
