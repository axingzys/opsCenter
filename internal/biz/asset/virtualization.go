// Copyright (c) 2026 DYCloud J.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package asset

import (
	"time"

	"gorm.io/gorm"
)

const (
	VirtualizationProviderESXi = "esxi"
	VirtualizationProviderPVE  = "pve"

	VirtualizationOnboardPolicyStrict = "strict"
	VirtualizationOnboardPolicyWarn   = "warn"

	VirtualizationGuestPowerActionOn     = "power_on"
	VirtualizationGuestPowerActionOff    = "power_off"
	VirtualizationGuestPowerActionReboot = "reboot"

	VirtualizationSnapshotActionCreate   = "snapshot_create"
	VirtualizationSnapshotActionRollback = "snapshot_rollback"
	VirtualizationSnapshotActionDelete   = "snapshot_delete"

	VirtualizationConsoleActionOpen = "console_link"
)

// VirtualizationPlatform 虚拟化平台（ESXi/PVE）连接配置
type VirtualizationPlatform struct {
	gorm.Model
	Name               string     `gorm:"type:varchar(100);not null;comment:平台名称" json:"name"`
	Provider           string     `gorm:"type:varchar(20);not null;comment:平台类型 esxi/pve" json:"provider"`
	Endpoint           string     `gorm:"type:varchar(255);not null;comment:API地址" json:"endpoint"`
	Port               int        `gorm:"type:int;default:443;comment:API端口" json:"port"`
	Username           string     `gorm:"type:varchar(100);not null;comment:用户名" json:"username"`
	Password           string     `gorm:"type:varchar(500);not null;comment:密码或Token" json:"-"`
	InsecureSkipVerify bool       `gorm:"column:insecure_skip_verify;default:false;comment:是否跳过TLS验证" json:"insecureSkipVerify"`
	Status             string     `gorm:"type:varchar(20);default:'enabled';comment:状态 enabled/disabled" json:"status"`
	LastSyncAt         *time.Time `gorm:"column:last_sync_at;comment:最后同步时间" json:"lastSyncAt,omitempty"`
	LastSyncStatus     string     `gorm:"column:last_sync_status;type:varchar(20);default:'idle';comment:最后同步状态 idle/running/success/failed" json:"lastSyncStatus"`
	LastSyncMessage    string     `gorm:"column:last_sync_message;type:varchar(500);comment:最后同步结果" json:"lastSyncMessage"`
	Description        string     `gorm:"type:varchar(500);comment:备注" json:"description"`
}

func (VirtualizationPlatform) TableName() string {
	return "virtualization_platforms"
}

// VirtualizationCluster 虚拟化集群信息快照
type VirtualizationCluster struct {
	gorm.Model
	PlatformID       uint       `gorm:"column:platform_id;not null;index;comment:平台ID" json:"platformId"`
	ExternalID       string     `gorm:"column:external_id;type:varchar(128);not null;comment:平台内集群ID" json:"externalId"`
	Name             string     `gorm:"type:varchar(150);not null;comment:集群名称" json:"name"`
	Datacenter       string     `gorm:"type:varchar(150);comment:数据中心" json:"datacenter"`
	HostCount        int        `gorm:"type:int;default:0;comment:主机数" json:"hostCount"`
	GuestCount       int        `gorm:"type:int;default:0;comment:虚机数" json:"guestCount"`
	Status           string     `gorm:"type:varchar(20);default:'unknown';comment:状态 unknown/normal/warn/error" json:"status"`
	LastCollectedAt  *time.Time `gorm:"column:last_collected_at;comment:最后采集时间" json:"lastCollectedAt,omitempty"`
	RawPayloadDigest string     `gorm:"column:raw_payload_digest;type:varchar(64);comment:原始数据摘要" json:"rawPayloadDigest"`
}

func (VirtualizationCluster) TableName() string {
	return "virtualization_clusters"
}

// VirtualizationHost 虚拟化宿主机信息快照
type VirtualizationHost struct {
	gorm.Model
	PlatformID      uint       `gorm:"column:platform_id;not null;index;comment:平台ID" json:"platformId"`
	ClusterID       uint       `gorm:"column:cluster_id;default:0;index;comment:集群ID" json:"clusterId"`
	ExternalID      string     `gorm:"column:external_id;type:varchar(128);not null;comment:平台内宿主机ID" json:"externalId"`
	Name            string     `gorm:"type:varchar(150);not null;comment:宿主机名称" json:"name"`
	ManagementIP    string     `gorm:"column:management_ip;type:varchar(64);comment:管理IP" json:"managementIp"`
	CPUModel        string     `gorm:"column:cpu_model;type:varchar(255);comment:CPU型号" json:"cpuModel"`
	CPUCores        int        `gorm:"column:cpu_cores;type:int;default:0;comment:CPU核心数" json:"cpuCores"`
	MemoryTotalMB   int64      `gorm:"column:memory_total_mb;type:bigint;default:0;comment:总内存(MB)" json:"memoryTotalMb"`
	MemoryUsedMB    int64      `gorm:"column:memory_used_mb;type:bigint;default:0;comment:已用内存(MB)" json:"memoryUsedMb"`
	GuestCount      int        `gorm:"column:guest_count;type:int;default:0;comment:虚机数" json:"guestCount"`
	Status          string     `gorm:"type:varchar(20);default:'unknown';comment:状态 unknown/online/offline/maintenance" json:"status"`
	LastCollectedAt *time.Time `gorm:"column:last_collected_at;comment:最后采集时间" json:"lastCollectedAt,omitempty"`
}

func (VirtualizationHost) TableName() string {
	return "virtualization_hosts"
}

// VirtualizationGuest 虚拟机实例快照
type VirtualizationGuest struct {
	gorm.Model
	PlatformID      uint       `gorm:"column:platform_id;not null;index;comment:平台ID" json:"platformId"`
	ClusterID       uint       `gorm:"column:cluster_id;default:0;index;comment:集群ID" json:"clusterId"`
	HostID          uint       `gorm:"column:host_id;default:0;index;comment:宿主机ID" json:"hostId"`
	ExternalID      string     `gorm:"column:external_id;type:varchar(128);not null;comment:平台内虚机ID" json:"externalId"`
	Name            string     `gorm:"type:varchar(150);not null;comment:虚机名称" json:"name"`
	OSType          string     `gorm:"column:os_type;type:varchar(100);comment:操作系统" json:"osType"`
	PowerState      string     `gorm:"column:power_state;type:varchar(20);default:'unknown';comment:电源状态 powered_on/powered_off/suspended/unknown" json:"powerState"`
	CPUCount        int        `gorm:"column:cpu_count;type:int;default:0;comment:CPU数量" json:"cpuCount"`
	MemoryMB        int64      `gorm:"column:memory_mb;type:bigint;default:0;comment:内存(MB)" json:"memoryMb"`
	PrimaryIP       string     `gorm:"column:primary_ip;type:varchar(64);comment:主IP" json:"primaryIp"`
	ToolsStatus     string     `gorm:"column:tools_status;type:varchar(50);comment:Tools状态" json:"toolsStatus"`
	BindingStatus   string     `gorm:"column:binding_status;type:varchar(20);default:'unbound';comment:纳管状态 bound/unbound/conflict" json:"bindingStatus"`
	LastCollectedAt *time.Time `gorm:"column:last_collected_at;comment:最后采集时间" json:"lastCollectedAt,omitempty"`
}

func (VirtualizationGuest) TableName() string {
	return "virtualization_guests"
}

// VirtualizationGuestBinding 虚机与资产主机的纳管映射关系
type VirtualizationGuestBinding struct {
	gorm.Model
	GuestID     uint       `gorm:"column:guest_id;not null;index;comment:虚机ID" json:"guestId"`
	AssetHostID uint       `gorm:"column:asset_host_id;not null;index;comment:资产主机ID" json:"assetHostId"`
	BindingType string     `gorm:"column:binding_type;type:varchar(20);default:'manual';comment:纳管类型 manual/auto" json:"bindingType"`
	Status      string     `gorm:"type:varchar(20);default:'active';comment:状态 active/inactive" json:"status"`
	BoundBy     uint       `gorm:"column:bound_by;comment:绑定人" json:"boundBy"`
	BoundAt     *time.Time `gorm:"column:bound_at;comment:绑定时间" json:"boundAt,omitempty"`
	UnboundAt   *time.Time `gorm:"column:unbound_at;comment:解绑时间" json:"unboundAt,omitempty"`
	BindingNote string     `gorm:"column:binding_note;type:varchar(500);comment:备注" json:"bindingNote"`
}

func (VirtualizationGuestBinding) TableName() string {
	return "virtualization_guest_bindings"
}

// VirtualizationSyncJob 虚拟化平台同步任务
type VirtualizationSyncJob struct {
	gorm.Model
	PlatformID    uint       `gorm:"column:platform_id;not null;index;comment:平台ID" json:"platformId"`
	TriggerType   string     `gorm:"column:trigger_type;type:varchar(20);default:'manual';comment:触发方式 manual/schedule" json:"triggerType"`
	Status        string     `gorm:"type:varchar(20);default:'pending';comment:状态 pending/running/success/failed" json:"status"`
	StartedAt     *time.Time `gorm:"column:started_at;comment:开始时间" json:"startedAt,omitempty"`
	FinishedAt    *time.Time `gorm:"column:finished_at;comment:结束时间" json:"finishedAt,omitempty"`
	OperatorID    uint       `gorm:"column:operator_id;comment:操作人ID" json:"operatorId"`
	ItemsTotal    int        `gorm:"column:items_total;type:int;default:0;comment:总条目数" json:"itemsTotal"`
	ItemsCreated  int        `gorm:"column:items_created;type:int;default:0;comment:新增条目数" json:"itemsCreated"`
	ItemsUpdated  int        `gorm:"column:items_updated;type:int;default:0;comment:更新条目数" json:"itemsUpdated"`
	ItemsDeleted  int        `gorm:"column:items_deleted;type:int;default:0;comment:删除条目数" json:"itemsDeleted"`
	FailureReason string     `gorm:"column:failure_reason;type:varchar(500);comment:失败原因" json:"failureReason"`
}

func (VirtualizationSyncJob) TableName() string {
	return "virtualization_sync_jobs"
}

// VirtualizationPlatformMetric 虚拟化平台状态趋势快照
type VirtualizationPlatformMetric struct {
	gorm.Model
	PlatformID          uint       `gorm:"column:platform_id;not null;index;comment:平台ID" json:"platformId"`
	CollectedAt         *time.Time `gorm:"column:collected_at;not null;index;comment:采集时间" json:"collectedAt,omitempty"`
	GuestTotal          int        `gorm:"column:guest_total;type:int;default:0;comment:虚机总数" json:"guestTotal"`
	PoweredOnGuests     int        `gorm:"column:powered_on_guests;type:int;default:0;comment:开机虚机数" json:"poweredOnGuests"`
	PoweredOffGuests    int        `gorm:"column:powered_off_guests;type:int;default:0;comment:关机虚机数" json:"poweredOffGuests"`
	SuspendedGuests     int        `gorm:"column:suspended_guests;type:int;default:0;comment:挂起虚机数" json:"suspendedGuests"`
	BoundGuests         int        `gorm:"column:bound_guests;type:int;default:0;comment:已纳管虚机数" json:"boundGuests"`
	OnlineGuests        int        `gorm:"column:online_guests;type:int;default:0;comment:在线虚机数" json:"onlineGuests"`
	OfflineGuests       int        `gorm:"column:offline_guests;type:int;default:0;comment:离线虚机数" json:"offlineGuests"`
	NotConfiguredGuests int        `gorm:"column:not_configured_guests;type:int;default:0;comment:未配置采集虚机数" json:"notConfiguredGuests"`
	UnknownGuests       int        `gorm:"column:unknown_guests;type:int;default:0;comment:未知虚机数" json:"unknownGuests"`
}

func (VirtualizationPlatformMetric) TableName() string {
	return "virtualization_platform_metrics"
}

// VirtualizationClusterMetric 虚拟化集群状态趋势快照
type VirtualizationClusterMetric struct {
	gorm.Model
	PlatformID          uint       `gorm:"column:platform_id;not null;index;comment:平台ID" json:"platformId"`
	ClusterID           uint       `gorm:"column:cluster_id;not null;index;comment:集群ID" json:"clusterId"`
	CollectedAt         *time.Time `gorm:"column:collected_at;not null;index;comment:采集时间" json:"collectedAt,omitempty"`
	GuestTotal          int        `gorm:"column:guest_total;type:int;default:0;comment:虚机总数" json:"guestTotal"`
	PoweredOnGuests     int        `gorm:"column:powered_on_guests;type:int;default:0;comment:开机虚机数" json:"poweredOnGuests"`
	PoweredOffGuests    int        `gorm:"column:powered_off_guests;type:int;default:0;comment:关机虚机数" json:"poweredOffGuests"`
	SuspendedGuests     int        `gorm:"column:suspended_guests;type:int;default:0;comment:挂起虚机数" json:"suspendedGuests"`
	BoundGuests         int        `gorm:"column:bound_guests;type:int;default:0;comment:已纳管虚机数" json:"boundGuests"`
	OnlineGuests        int        `gorm:"column:online_guests;type:int;default:0;comment:在线虚机数" json:"onlineGuests"`
	OfflineGuests       int        `gorm:"column:offline_guests;type:int;default:0;comment:离线虚机数" json:"offlineGuests"`
	NotConfiguredGuests int        `gorm:"column:not_configured_guests;type:int;default:0;comment:未配置采集虚机数" json:"notConfiguredGuests"`
	UnknownGuests       int        `gorm:"column:unknown_guests;type:int;default:0;comment:未知虚机数" json:"unknownGuests"`
}

func (VirtualizationClusterMetric) TableName() string {
	return "virtualization_cluster_metrics"
}

// VirtualizationActionLog 虚拟化操作审计日志（三期基础表）
type VirtualizationActionLog struct {
	gorm.Model
	PlatformID      uint       `gorm:"column:platform_id;not null;index;comment:平台ID" json:"platformId"`
	ClusterID       uint       `gorm:"column:cluster_id;default:0;index;comment:集群ID" json:"clusterId"`
	HostID          uint       `gorm:"column:host_id;default:0;index;comment:宿主机ID" json:"hostId"`
	GuestID         uint       `gorm:"column:guest_id;default:0;index;comment:虚机ID" json:"guestId"`
	Action          string     `gorm:"column:action;type:varchar(50);not null;comment:动作 power_on/power_off/reboot/snapshot_create..." json:"action"`
	RiskLevel       string     `gorm:"column:risk_level;type:varchar(20);default:'medium';comment:风险等级 low/medium/high" json:"riskLevel"`
	Status          string     `gorm:"column:status;type:varchar(20);default:'pending';comment:状态 pending/success/failed/denied" json:"status"`
	TargetType      string     `gorm:"column:target_type;type:varchar(20);default:'guest';comment:目标类型 platform/cluster/host/guest/snapshot" json:"targetType"`
	TargetName      string     `gorm:"column:target_name;type:varchar(200);comment:目标名称" json:"targetName"`
	OperatorID      uint       `gorm:"column:operator_id;index;comment:操作人ID" json:"operatorId"`
	OperatorName    string     `gorm:"column:operator_name;type:varchar(100);comment:操作人" json:"operatorName"`
	ConfirmRequired bool       `gorm:"column:confirm_required;default:false;comment:是否需要二次确认" json:"confirmRequired"`
	Reason          string     `gorm:"column:reason;type:varchar(500);comment:操作原因" json:"reason"`
	RequestPayload  string     `gorm:"column:request_payload;type:text;comment:请求载荷" json:"requestPayload"`
	ResultMessage   string     `gorm:"column:result_message;type:varchar(500);comment:结果信息" json:"resultMessage"`
	StartedAt       *time.Time `gorm:"column:started_at;comment:开始时间" json:"startedAt,omitempty"`
	FinishedAt      *time.Time `gorm:"column:finished_at;comment:结束时间" json:"finishedAt,omitempty"`
}

func (VirtualizationActionLog) TableName() string {
	return "virtualization_action_logs"
}

// VirtualizationPlatformRequest 平台新增/更新请求
type VirtualizationPlatformRequest struct {
	ID                 uint   `json:"id"`
	Name               string `json:"name" binding:"required,min=2,max=100"`
	Provider           string `json:"provider" binding:"required,oneof=esxi pve"`
	Endpoint           string `json:"endpoint" binding:"required,max=255"`
	Port               int    `json:"port" binding:"omitempty,min=1,max=65535"`
	Username           string `json:"username" binding:"required,max=100"`
	Password           string `json:"password" binding:"omitempty,max=500"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
	Status             string `json:"status" binding:"omitempty,oneof=enabled disabled"`
	Description        string `json:"description" binding:"omitempty,max=500"`
}

// VirtualizationGuestListRequest 虚机列表查询参数
type VirtualizationGuestListRequest struct {
	Page       int    `form:"page" binding:"omitempty,min=1"`
	PageSize   int    `form:"pageSize" binding:"omitempty,min=1,max=100"`
	Keyword    string `form:"keyword"`
	PlatformID uint   `form:"platformId"`
	ClusterID  uint   `form:"clusterId"`
	PowerState string `form:"powerState" binding:"omitempty,oneof=powered_on powered_off suspended unknown"`
	Bound      string `form:"bound" binding:"omitempty,oneof=all bound unbound"`
}

// VirtualizationGuestBindingRequest 虚机纳管绑定请求
type VirtualizationGuestBindingRequest struct {
	AssetHostID uint   `json:"assetHostId" binding:"required"`
	BindingType string `json:"bindingType" binding:"omitempty,oneof=manual auto"`
	BindingNote string `json:"bindingNote" binding:"omitempty,max=500"`
}

// VirtualizationGuestUnbindRequest 虚机解绑请求
type VirtualizationGuestUnbindRequest struct {
	BindingNote string `json:"bindingNote" binding:"omitempty,max=500"`
}

type VirtualizationGuestPowerRequest struct {
	Action      string `json:"action" binding:"required,oneof=power_on power_off reboot"`
	Reason      string `json:"reason" binding:"required,max=500"`
	ConfirmText string `json:"confirmText" binding:"required,max=150"`
}

type VirtualizationGuestSnapshotCreateRequest struct {
	Name          string `json:"name" binding:"required,min=2,max=80"`
	Description   string `json:"description" binding:"omitempty,max=500"`
	IncludeMemory bool   `json:"includeMemory"`
	Quiesce       bool   `json:"quiesce"`
	Reason        string `json:"reason" binding:"required,max=500"`
}

type VirtualizationGuestSnapshotActionRequest struct {
	SnapshotName string `json:"snapshotName" binding:"required,max=80"`
	Reason       string `json:"reason" binding:"required,max=500"`
	ConfirmText  string `json:"confirmText" binding:"omitempty,max=150"`
}

type VirtualizationActionLogListRequest struct {
	Page       int    `form:"page" binding:"omitempty,min=1"`
	PageSize   int    `form:"pageSize" binding:"omitempty,min=1,max=100"`
	PlatformID uint   `form:"platformId"`
	GuestID    uint   `form:"guestId"`
	Action     string `form:"action" binding:"omitempty,oneof=power_on power_off reboot snapshot_create snapshot_rollback snapshot_delete console_link"`
	Status     string `form:"status" binding:"omitempty,oneof=pending success failed denied"`
	Keyword    string `form:"keyword"`
}

type VirtualizationSettingsRequest struct {
	ConflictPolicy         string `json:"conflictPolicy" binding:"required,oneof=strict warn"`
	WriteOperationsEnabled bool   `json:"writeOperationsEnabled"`
}

type VirtualizationPlatformVO struct {
	ID                 uint   `json:"id"`
	Name               string `json:"name"`
	Provider           string `json:"provider"`
	ProviderText       string `json:"providerText"`
	Endpoint           string `json:"endpoint"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
	Status             string `json:"status"`
	LastSyncAt         string `json:"lastSyncAt,omitempty"`
	LastSyncStatus     string `json:"lastSyncStatus"`
	LastSyncMessage    string `json:"lastSyncMessage,omitempty"`
	Description        string `json:"description"`
	CreateTime         string `json:"createTime"`
	UpdateTime         string `json:"updateTime"`
}

type VirtualizationSyncJobVO struct {
	ID            uint   `json:"id"`
	PlatformID    uint   `json:"platformId"`
	TriggerType   string `json:"triggerType"`
	Status        string `json:"status"`
	StartedAt     string `json:"startedAt,omitempty"`
	FinishedAt    string `json:"finishedAt,omitempty"`
	OperatorID    uint   `json:"operatorId"`
	ItemsTotal    int    `json:"itemsTotal"`
	ItemsCreated  int    `json:"itemsCreated"`
	ItemsUpdated  int    `json:"itemsUpdated"`
	ItemsDeleted  int    `json:"itemsDeleted"`
	FailureReason string `json:"failureReason,omitempty"`
	CreateTime    string `json:"createTime"`
}

type VirtualizationTopologyVO struct {
	Platforms []*VirtualizationTopologyPlatformNode `json:"platforms"`
}

type VirtualizationTopologyPlatformNode struct {
	ID           uint                                 `json:"id"`
	Name         string                               `json:"name"`
	Provider     string                               `json:"provider"`
	ProviderText string                               `json:"providerText"`
	Endpoint     string                               `json:"endpoint,omitempty"`
	Port         int                                  `json:"port,omitempty"`
	Status       string                               `json:"status"`
	LastSyncAt   string                               `json:"lastSyncAt,omitempty"`
	Clusters     []*VirtualizationTopologyClusterNode `json:"clusters"`
}

type VirtualizationTopologyClusterNode struct {
	ID         uint                              `json:"id"`
	Name       string                            `json:"name"`
	Datacenter string                            `json:"datacenter"`
	Status     string                            `json:"status"`
	HostCount  int                               `json:"hostCount"`
	GuestCount int                               `json:"guestCount"`
	Hosts      []*VirtualizationTopologyHostNode `json:"hosts"`
}

type VirtualizationTopologyHostNode struct {
	ID              uint                               `json:"id"`
	Name            string                             `json:"name"`
	ClusterID       uint                               `json:"clusterId"`
	ExternalID      string                             `json:"externalId"`
	ManagementIP    string                             `json:"managementIp"`
	CPUModel        string                             `json:"cpuModel"`
	CPUCores        int                                `json:"cpuCores"`
	MemoryTotalMB   int64                              `json:"memoryTotalMb"`
	MemoryUsedMB    int64                              `json:"memoryUsedMb"`
	Status          string                             `json:"status"`
	GuestCount      int                                `json:"guestCount"`
	LastCollectedAt string                             `json:"lastCollectedAt,omitempty"`
	Guests          []*VirtualizationTopologyGuestNode `json:"guests"`
}

type VirtualizationTopologyGuestNode struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	ClusterID     uint   `json:"clusterId"`
	HostID        uint   `json:"hostId"`
	ExternalID    string `json:"externalId"`
	PowerState    string `json:"powerState"`
	CPUCount      int    `json:"cpuCount"`
	MemoryMB      int64  `json:"memoryMb"`
	PrimaryIP     string `json:"primaryIp"`
	ToolsStatus   string `json:"toolsStatus"`
	BindingStatus string `json:"bindingStatus"`
}

type VirtualizationGuestVO struct {
	ID                  uint   `json:"id"`
	PlatformID          uint   `json:"platformId"`
	ClusterID           uint   `json:"clusterId"`
	HostID              uint   `json:"hostId"`
	ExternalID          string `json:"externalId"`
	Name                string `json:"name"`
	OSType              string `json:"osType"`
	PowerState          string `json:"powerState"`
	CPUCount            int    `json:"cpuCount"`
	MemoryMB            int64  `json:"memoryMb"`
	PrimaryIP           string `json:"primaryIp"`
	ToolsStatus         string `json:"toolsStatus"`
	BindingStatus       string `json:"bindingStatus"`
	RuntimeStatus       string `json:"runtimeStatus"`
	RuntimeStatusText   string `json:"runtimeStatusText"`
	RuntimeStatusSource string `json:"runtimeStatusSource,omitempty"`
	LastCollectedAt     string `json:"lastCollectedAt,omitempty"`
	AssetHostID         uint   `json:"assetHostId,omitempty"`
	AssetHostName       string `json:"assetHostName,omitempty"`
}

type VirtualizationGuestDetailVO struct {
	Guest   *VirtualizationGuestVO        `json:"guest"`
	Binding *VirtualizationGuestBindingVO `json:"binding,omitempty"`
}

type VirtualizationGuestOnboardPrecheckVO struct {
	GuestID        uint                             `json:"guestId"`
	GuestName      string                           `json:"guestName"`
	PrimaryIP      string                           `json:"primaryIp"`
	ConflictPolicy string                           `json:"conflictPolicy"`
	RiskLevel      string                           `json:"riskLevel"`
	CurrentBinding *VirtualizationPrecheckHostVO    `json:"currentBinding,omitempty"`
	SelectedHost   *VirtualizationPrecheckHostVO    `json:"selectedHost,omitempty"`
	SuggestedHost  *VirtualizationPrecheckHostVO    `json:"suggestedHost,omitempty"`
	CandidateHosts []*VirtualizationPrecheckHostVO  `json:"candidateHosts,omitempty"`
	Checks         []*VirtualizationPrecheckCheckVO `json:"checks"`
}

type VirtualizationSettingsVO struct {
	ConflictPolicy             string `json:"conflictPolicy"`
	ConflictPolicyText         string `json:"conflictPolicyText"`
	WriteOperationsEnabled     bool   `json:"writeOperationsEnabled"`
	WriteOperationsEnabledText string `json:"writeOperationsEnabledText"`
}

type VirtualizationPlatformTrendVO struct {
	ScopeType  string                                `json:"scopeType"`
	ScopeID    uint                                  `json:"scopeId"`
	ScopeName  string                                `json:"scopeName,omitempty"`
	PlatformID uint                                  `json:"platformId"`
	ClusterID  uint                                  `json:"clusterId,omitempty"`
	Range      string                                `json:"range"`
	Start      string                                `json:"start"`
	End        string                                `json:"end"`
	Points     []*VirtualizationPlatformTrendPointVO `json:"points"`
}

type VirtualizationPlatformTrendPointVO struct {
	Timestamp           int64  `json:"timestamp"`
	Time                string `json:"time"`
	GuestTotal          int    `json:"guestTotal"`
	PoweredOnGuests     int    `json:"poweredOnGuests"`
	PoweredOffGuests    int    `json:"poweredOffGuests"`
	SuspendedGuests     int    `json:"suspendedGuests"`
	BoundGuests         int    `json:"boundGuests"`
	OnlineGuests        int    `json:"onlineGuests"`
	OfflineGuests       int    `json:"offlineGuests"`
	NotConfiguredGuests int    `json:"notConfiguredGuests"`
	UnknownGuests       int    `json:"unknownGuests"`
}

type VirtualizationPrecheckHostVO struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	IP        string `json:"ip"`
	MatchType string `json:"matchType,omitempty"`
}

type VirtualizationPrecheckCheckVO struct {
	Code    string `json:"code"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type VirtualizationGuestBindingVO struct {
	ID          uint   `json:"id"`
	GuestID     uint   `json:"guestId"`
	AssetHostID uint   `json:"assetHostId"`
	BindingType string `json:"bindingType"`
	Status      string `json:"status"`
	BoundBy     uint   `json:"boundBy"`
	BoundAt     string `json:"boundAt,omitempty"`
	UnboundAt   string `json:"unboundAt,omitempty"`
	BindingNote string `json:"bindingNote,omitempty"`
}

type VirtualizationGuestPowerVO struct {
	AuditLogID     uint   `json:"auditLogId"`
	GuestID        uint   `json:"guestId"`
	GuestName      string `json:"guestName"`
	Action         string `json:"action"`
	ActionText     string `json:"actionText"`
	Status         string `json:"status"`
	StatusText     string `json:"statusText"`
	Message        string `json:"message"`
	PowerState     string `json:"powerState"`
	PowerStateText string `json:"powerStateText"`
	RequestedAt    string `json:"requestedAt"`
	FinishedAt     string `json:"finishedAt,omitempty"`
}

type VirtualizationGuestConsoleLinkVO struct {
	AuditLogID    uint   `json:"auditLogId"`
	GuestID       uint   `json:"guestId"`
	GuestName     string `json:"guestName"`
	Provider      string `json:"provider"`
	ProviderText  string `json:"providerText"`
	Mode          string `json:"mode"`
	URL           string `json:"url"`
	RequiresLogin bool   `json:"requiresLogin"`
	Message       string `json:"message,omitempty"`
	RequestedAt   string `json:"requestedAt,omitempty"`
	ExpiresAt     string `json:"expiresAt,omitempty"`
}

type VirtualizationGuestSnapshotVO struct {
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	CreatedAt     string `json:"createdAt,omitempty"`
	ParentName    string `json:"parentName,omitempty"`
	Current       bool   `json:"current"`
	IncludeMemory bool   `json:"includeMemory"`
	Quiesced      bool   `json:"quiesced"`
	PowerState    string `json:"powerState,omitempty"`
	Children      int    `json:"children"`
}

type VirtualizationActionLogVO struct {
	ID              uint   `json:"id"`
	PlatformID      uint   `json:"platformId"`
	PlatformName    string `json:"platformName,omitempty"`
	ClusterID       uint   `json:"clusterId,omitempty"`
	HostID          uint   `json:"hostId,omitempty"`
	GuestID         uint   `json:"guestId,omitempty"`
	Action          string `json:"action"`
	ActionText      string `json:"actionText"`
	RiskLevel       string `json:"riskLevel"`
	Status          string `json:"status"`
	StatusText      string `json:"statusText"`
	TargetType      string `json:"targetType"`
	TargetName      string `json:"targetName"`
	OperatorID      uint   `json:"operatorId"`
	OperatorName    string `json:"operatorName"`
	ConfirmRequired bool   `json:"confirmRequired"`
	Reason          string `json:"reason,omitempty"`
	ResultMessage   string `json:"resultMessage,omitempty"`
	StartedAt       string `json:"startedAt,omitempty"`
	FinishedAt      string `json:"finishedAt,omitempty"`
	CreateTime      string `json:"createTime"`
}
