package database

import "gorm.io/gorm"

func applyAllowedInstanceFilter(query *gorm.DB, column string, restricted bool, allowedIDs []uint) *gorm.DB {
	if !restricted {
		return query
	}
	if len(allowedIDs) == 0 {
		return query.Where("1 = 0")
	}
	return query.Where(column+" IN ?", allowedIDs)
}

func applyAllowedRestoreInstanceFilter(query *gorm.DB, restricted bool, allowedIDs []uint) *gorm.DB {
	if !restricted {
		return query
	}
	if len(allowedIDs) == 0 {
		return query.Where("1 = 0")
	}
	return query.Where("source_instance_id IN ? OR target_instance_id IN ?", allowedIDs, allowedIDs)
}
