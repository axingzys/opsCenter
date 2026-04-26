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

func TestSupportedTypeCapabilityMatrix(t *testing.T) {
	uc := &UseCase{}
	byType := make(map[string]*SupportedTypeVO)
	for _, item := range uc.SupportedTypes() {
		byType[item.Type] = item
	}

	if mongodb := byType[DBTypeMongoDB]; mongodb == nil || !mongodb.TopologyEnabled || mongodb.TestEnabled || mongodb.MetadataEnabled || mongodb.QueryEnabled {
		t.Fatalf("mongodb capabilities should be topology-only, got %#v", mongodb)
	}
	if dameng := byType[DBTypeDameng]; dameng == nil || dameng.TestEnabled || dameng.MetadataEnabled || dameng.QueryEnabled || dameng.TopologyEnabled {
		t.Fatalf("dameng capabilities should be disabled until dedicated driver support, got %#v", dameng)
	}
	if redis := byType[DBTypeRedis]; redis == nil || !redis.TestEnabled || !redis.MetadataEnabled || !redis.QueryEnabled || !redis.TopologyEnabled {
		t.Fatalf("redis capabilities should be fully enabled, got %#v", redis)
	}
}
