package service

import (
	"strings"

	assetbiz "github.com/ydcloud-dy/opshub/internal/biz/asset"
	"gorm.io/gorm"
)

func applyAgentManagedHostScope(query *gorm.DB, osType string) *gorm.DB {
	if query == nil {
		return query
	}

	query = query.Where("(agent_id <> '' OR management_mode = ?)", assetbiz.ManagementModeAgent)

	switch strings.ToLower(strings.TrimSpace(osType)) {
	case assetbiz.OSTypeLinux, assetbiz.OSTypeWindows:
		query = query.Where("os_type = ?", strings.ToLower(strings.TrimSpace(osType)))
	}

	return query
}
