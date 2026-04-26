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

type oauthStateRepo struct {
	db *gorm.DB
}

// NewOAuthStateRepo 创建OAuth状态仓库
func NewOAuthStateRepo(db *gorm.DB) identity.OAuthStateRepo {
	return &oauthStateRepo{db: db}
}

func (r *oauthStateRepo) Create(ctx context.Context, state *identity.OAuthState) error {
	state.CreatedAt = time.Now()
	return r.db.WithContext(ctx).Create(state).Error
}

func (r *oauthStateRepo) GetByState(ctx context.Context, state string) (*identity.OAuthState, error) {
	var oauthState identity.OAuthState
	err := r.db.WithContext(ctx).Where("state = ?", state).First(&oauthState).Error
	return &oauthState, err
}

func (r *oauthStateRepo) Delete(ctx context.Context, state string) error {
	return r.db.WithContext(ctx).Where("state = ?", state).Delete(&identity.OAuthState{}).Error
}

func (r *oauthStateRepo) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&identity.OAuthState{}).Error
}
