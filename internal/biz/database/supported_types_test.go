package database

import "testing"

func TestPhase4SupportedTypes(t *testing.T) {
	for _, dbType := range []string{DBTypeTiDB, DBTypeOceanBase, DBTypeOpenGauss, DBTypeKingbase, DBTypeDameng, DBTypeElasticsearch, DBTypeOpenSearch} {
		if !IsSupportedType(dbType) {
			t.Fatalf("%s should be supported", dbType)
		}
		if DefaultPort(dbType) <= 0 {
			t.Fatalf("%s should have default port", dbType)
		}
		if DBTypeText(dbType) == "" || DBTypeText(dbType) == dbType {
			t.Fatalf("%s should have display text, got %q", dbType, DBTypeText(dbType))
		}
	}
}

func TestCompatibleReadOnlyCapabilities(t *testing.T) {
	for _, dbType := range []string{DBTypeTiDB, DBTypeOceanBase, DBTypeOpenGauss, DBTypeKingbase} {
		if !supportsAutoAppendLimit(dbType) {
			t.Fatalf("%s should support read-only LIMIT append", dbType)
		}
		if !supportsExplain(dbType) {
			t.Fatalf("%s should support explain validation", dbType)
		}
		if supportsWriteValidation(dbType) {
			t.Fatalf("%s write validation should wait for dedicated compatibility verification", dbType)
		}
	}
}
