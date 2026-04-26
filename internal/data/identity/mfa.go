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
	"time"

	"github.com/ydcloud-dy/opshub/internal/biz/identity"
	"gorm.io/gorm"
)

type mfaSettingsRepo struct {
	db *gorm.DB
}

// NewMFASettingsRepo 创建MFA设置仓库
func NewMFASettingsRepo(db *gorm.DB) identity.MFASettingsRepo {
	return &mfaSettingsRepo{db: db}
}

func (r *mfaSettingsRepo) Create(ctx context.Context, settings *identity.MFASettings) error {
	return r.db.WithContext(ctx).Create(settings).Error
}

func (r *mfaSettingsRepo) Update(ctx context.Context, settings *identity.MFASettings) error {
	return r.db.WithContext(ctx).Save(settings).Error
}

func (r *mfaSettingsRepo) GetByUserID(ctx context.Context, userID uint) (*identity.MFASettings, error) {
	var settings identity.MFASettings
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&settings).Error
	return &settings, err
}

func (r *mfaSettingsRepo) Delete(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&identity.MFASettings{}).Error
}

type mfaChallengeRepo struct {
	db *gorm.DB
}

// NewMFAChallengeRepo 创建MFA挑战仓库
func NewMFAChallengeRepo(db *gorm.DB) identity.MFAChallengeRepo {
	return &mfaChallengeRepo{db: db}
}

func (r *mfaChallengeRepo) Create(ctx context.Context, challenge *identity.MFAChallenge) error {
	return r.db.WithContext(ctx).Create(challenge).Error
}

func (r *mfaChallengeRepo) GetByToken(ctx context.Context, token string) (*identity.MFAChallenge, error) {
	var challenge identity.MFAChallenge
	err := r.db.WithContext(ctx).Where("token = ?", token).First(&challenge).Error
	return &challenge, err
}

func (r *mfaChallengeRepo) Update(ctx context.Context, challenge *identity.MFAChallenge) error {
	return r.db.WithContext(ctx).Save(challenge).Error
}

func (r *mfaChallengeRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&identity.MFAChallenge{}, id).Error
}

func (r *mfaChallengeRepo) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&identity.MFAChallenge{}).Error
}
