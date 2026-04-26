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
	"context"
	"time"
)

type AssetGroupRepo interface {
	Create(ctx context.Context, group *AssetGroup) error
	Update(ctx context.Context, group *AssetGroup) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*AssetGroup, error)
	GetTree(ctx context.Context) ([]*AssetGroup, error)
	GetAll(ctx context.Context) ([]*AssetGroup, error)
	List(ctx context.Context, page, pageSize int, keyword string) ([]*AssetGroup, int64, error)
	GetDescendantIDs(ctx context.Context, id uint) ([]uint, error)
}

type HostRepo interface {
	Create(ctx context.Context, host *Host) error
	CreateOrUpdate(ctx context.Context, host *Host) error
	Update(ctx context.Context, host *Host) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*Host, error)
	List(ctx context.Context, page, pageSize int, keyword string, groupIDs []uint, accessibleHostIDs []uint, status *int) ([]*Host, int64, error)
	GetByGroupID(ctx context.Context, groupID uint) ([]*Host, error)
	GetByIP(ctx context.Context, ip string) (*Host, error)
	GetByCloudInstanceID(ctx context.Context, instanceID string) (*Host, error)
	CountByCredentialID(ctx context.Context, credentialID uint) (int64, error)
}

type AssetAgentRepo interface {
	Create(ctx context.Context, agent *AssetAgent) error
	Update(ctx context.Context, agent *AssetAgent) error
	GetByHostID(ctx context.Context, hostID uint) (*AssetAgent, error)
	GetByAgentID(ctx context.Context, agentID string) (*AssetAgent, error)
	List(ctx context.Context, page, pageSize int, keyword string, accessibleHostIDs []uint, status string) ([]*AssetAgent, int64, error)
	DeleteByHostID(ctx context.Context, hostID uint) error
}

type AssetHostInventoryRepo interface {
	Upsert(ctx context.Context, inventory *AssetHostInventory) error
	GetByHostID(ctx context.Context, hostID uint) (*AssetHostInventory, error)
	DeleteByHostID(ctx context.Context, hostID uint) error
}

type AssetHostPublicIPHistoryRepo interface {
	Observe(ctx context.Context, hostID uint, ip, source string, observedAt time.Time) error
	ListByHostID(ctx context.Context, hostID uint, limit int) ([]*AssetHostPublicIPHistory, error)
}

type AssetAgentJobRepo interface {
	Create(ctx context.Context, job *AssetAgentJob) error
	Update(ctx context.Context, job *AssetAgentJob) error
	GetByID(ctx context.Context, id uint) (*AssetAgentJob, error)
	GetLatestByHostID(ctx context.Context, hostID uint) (*AssetAgentJob, error)
}

type CredentialRepo interface {
	Create(ctx context.Context, credential *Credential) error
	Update(ctx context.Context, credential *Credential) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*Credential, error)
	GetByIDDecrypted(ctx context.Context, id uint) (*Credential, error)
	List(ctx context.Context, page, pageSize int, keyword string) ([]*Credential, int64, error)
	GetAll(ctx context.Context) ([]*Credential, error)
}

type CloudAccountRepo interface {
	Create(ctx context.Context, account *CloudAccount) error
	Update(ctx context.Context, account *CloudAccount) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*CloudAccount, error)
	List(ctx context.Context, page, pageSize int) ([]*CloudAccount, int64, error)
	GetAll(ctx context.Context) ([]*CloudAccount, error)
}

type DesktopSessionRepo interface {
	Create(ctx context.Context, session *DesktopSession) error
	Update(ctx context.Context, session *DesktopSession) error
	Touch(ctx context.Context, id uint, at time.Time) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*DesktopSession, error)
	GetBySessionUUID(ctx context.Context, sessionUUID string) (*DesktopSession, error)
	List(ctx context.Context, page, pageSize int, keyword, status string, userID uint) ([]*DesktopSession, int64, error)
	ListStaleOpen(ctx context.Context, cutoff time.Time, limit int) ([]*DesktopSession, error)
}

type VirtualizationPlatformRepo interface {
	Create(ctx context.Context, platform *VirtualizationPlatform) error
	Update(ctx context.Context, platform *VirtualizationPlatform) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*VirtualizationPlatform, error)
	List(ctx context.Context, page, pageSize int, keyword string) ([]*VirtualizationPlatform, int64, error)
}

type VirtualizationClusterRepo interface {
	UpsertBatch(ctx context.Context, items []*VirtualizationCluster) error
	GetByID(ctx context.Context, id uint) (*VirtualizationCluster, error)
	ListByPlatformID(ctx context.Context, platformID uint) ([]*VirtualizationCluster, error)
}

type VirtualizationHostRepo interface {
	UpsertBatch(ctx context.Context, items []*VirtualizationHost) error
	GetByID(ctx context.Context, id uint) (*VirtualizationHost, error)
	ListByPlatformID(ctx context.Context, platformID uint) ([]*VirtualizationHost, error)
}

type VirtualizationGuestRepo interface {
	UpsertBatch(ctx context.Context, items []*VirtualizationGuest) error
	GetByID(ctx context.Context, id uint) (*VirtualizationGuest, error)
	List(ctx context.Context, req *VirtualizationGuestListRequest) ([]*VirtualizationGuest, int64, error)
	ListByPlatformID(ctx context.Context, platformID uint) ([]*VirtualizationGuest, error)
}

type VirtualizationGuestBindingRepo interface {
	Create(ctx context.Context, binding *VirtualizationGuestBinding) error
	Update(ctx context.Context, binding *VirtualizationGuestBinding) error
	GetActiveByGuestID(ctx context.Context, guestID uint) (*VirtualizationGuestBinding, error)
	GetByGuestID(ctx context.Context, guestID uint) (*VirtualizationGuestBinding, error)
}

type VirtualizationSyncJobRepo interface {
	Create(ctx context.Context, job *VirtualizationSyncJob) error
	Update(ctx context.Context, job *VirtualizationSyncJob) error
	ListByPlatformID(ctx context.Context, platformID uint, page, pageSize int) ([]*VirtualizationSyncJob, int64, error)
}

type VirtualizationPlatformMetricRepo interface {
	Create(ctx context.Context, item *VirtualizationPlatformMetric) error
	ListByPlatformID(ctx context.Context, platformID uint, start, end time.Time) ([]*VirtualizationPlatformMetric, error)
}

type VirtualizationClusterMetricRepo interface {
	CreateBatch(ctx context.Context, items []*VirtualizationClusterMetric) error
	ListByClusterID(ctx context.Context, clusterID uint, start, end time.Time) ([]*VirtualizationClusterMetric, error)
}

type VirtualizationActionLogRepo interface {
	Create(ctx context.Context, item *VirtualizationActionLog) error
	Update(ctx context.Context, item *VirtualizationActionLog) error
	List(ctx context.Context, req *VirtualizationActionLogListRequest) ([]*VirtualizationActionLog, int64, error)
}

type VirtualizationPolicyRepo interface {
	GetOnboardConflictPolicy(ctx context.Context) (string, error)
	SaveOnboardConflictPolicy(ctx context.Context, policy string) error
	GetWriteOperationsEnabled(ctx context.Context) (bool, error)
	SaveWriteOperationsEnabled(ctx context.Context, enabled bool) error
}
