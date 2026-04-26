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

type oauth2AuthCodeRepo struct {
	db *gorm.DB
}

// NewOAuth2AuthCodeRepo 创建OAuth2授权码仓库
func NewOAuth2AuthCodeRepo(db *gorm.DB) identity.OAuth2AuthCodeRepo {
	return &oauth2AuthCodeRepo{db: db}
}

func (r *oauth2AuthCodeRepo) Create(ctx context.Context, code *identity.OAuth2AuthorizationCode) error {
	return r.db.WithContext(ctx).Create(code).Error
}

func (r *oauth2AuthCodeRepo) GetByCode(ctx context.Context, code string) (*identity.OAuth2AuthorizationCode, error) {
	var authCode identity.OAuth2AuthorizationCode
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&authCode).Error
	return &authCode, err
}

func (r *oauth2AuthCodeRepo) MarkUsed(ctx context.Context, code string) error {
	return r.db.WithContext(ctx).Model(&identity.OAuth2AuthorizationCode{}).
		Where("code = ?", code).
		Update("used", true).Error
}

func (r *oauth2AuthCodeRepo) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&identity.OAuth2AuthorizationCode{}).Error
}

type oauth2TokenRepo struct {
	db *gorm.DB
}

// NewOAuth2TokenRepo 创建OAuth2令牌仓库
func NewOAuth2TokenRepo(db *gorm.DB) identity.OAuth2TokenRepo {
	return &oauth2TokenRepo{db: db}
}

func (r *oauth2TokenRepo) CreateAccessToken(ctx context.Context, token *identity.OAuth2AccessToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *oauth2TokenRepo) CreateRefreshToken(ctx context.Context, token *identity.OAuth2RefreshToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *oauth2TokenRepo) GetAccessTokenByHash(ctx context.Context, tokenHash string) (*identity.OAuth2AccessToken, error) {
	var token identity.OAuth2AccessToken
	err := r.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&token).Error
	return &token, err
}

func (r *oauth2TokenRepo) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*identity.OAuth2RefreshToken, error) {
	var token identity.OAuth2RefreshToken
	err := r.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&token).Error
	return &token, err
}

func (r *oauth2TokenRepo) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	return r.db.WithContext(ctx).Model(&identity.OAuth2RefreshToken{}).
		Where("token_hash = ?", tokenHash).
		Update("revoked", true).Error
}

func (r *oauth2TokenRepo) DeleteExpiredTokens(ctx context.Context) error {
	// 删除过期的访问令牌
	if err := r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&identity.OAuth2AccessToken{}).Error; err != nil {
		return err
	}

	// 删除过期的刷新令牌
	return r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&identity.OAuth2RefreshToken{}).Error
}
