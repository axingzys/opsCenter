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

package identity

import (
	"context"

	"github.com/ydcloud-dy/opshub/internal/biz/identity"
	"gorm.io/gorm"
)

type ldapSyncJobRepo struct {
	db *gorm.DB
}

// NewLDAPSyncJobRepo 创建LDAP同步任务仓库
func NewLDAPSyncJobRepo(db *gorm.DB) identity.LDAPSyncJobRepo {
	return &ldapSyncJobRepo{db: db}
}

func (r *ldapSyncJobRepo) Create(ctx context.Context, job *identity.LDAPSyncJob) error {
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *ldapSyncJobRepo) Update(ctx context.Context, job *identity.LDAPSyncJob) error {
	return r.db.WithContext(ctx).Save(job).Error
}

func (r *ldapSyncJobRepo) GetByID(ctx context.Context, id uint) (*identity.LDAPSyncJob, error) {
	var job identity.LDAPSyncJob
	err := r.db.WithContext(ctx).First(&job, id).Error
	return &job, err
}

func (r *ldapSyncJobRepo) GetLatestBySourceID(ctx context.Context, sourceID uint) (*identity.LDAPSyncJob, error) {
	var job identity.LDAPSyncJob
	err := r.db.WithContext(ctx).
		Where("source_id = ?", sourceID).
		Order("created_at DESC").
		First(&job).Error
	return &job, err
}

func (r *ldapSyncJobRepo) List(ctx context.Context, sourceID uint, page, pageSize int) ([]*identity.LDAPSyncJob, int64, error) {
	var jobs []*identity.LDAPSyncJob
	var total int64

	query := r.db.WithContext(ctx).Model(&identity.LDAPSyncJob{})
	if sourceID > 0 {
		query = query.Where("source_id = ?", sourceID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&jobs).Error; err != nil {
		return nil, 0, err
	}

	return jobs, total, nil
}
