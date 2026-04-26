package database

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type DatabaseCapacityTrendRequest struct {
	Range    string `form:"range"`
	TopLimit int    `form:"topLimit"`
}

type DatabaseCapacityPointVO struct {
	CollectedAt    string `json:"collectedAt"`
	SchemaCount    int    `json:"schemaCount"`
	TableCount     int    `json:"tableCount"`
	RowCount       int64  `json:"rowCount"`
	DataSizeBytes  int64  `json:"dataSizeBytes"`
	IndexSizeBytes int64  `json:"indexSizeBytes"`
	TotalSizeBytes int64  `json:"totalSizeBytes"`
	TotalSizeText  string `json:"totalSizeText"`
}

type DatabaseCapacityObjectVO struct {
	ObjectType     string `json:"objectType"`
	SchemaName     string `json:"schemaName"`
	TableName      string `json:"tableName"`
	TableCount     int    `json:"tableCount"`
	RowCount       int64  `json:"rowCount"`
	DataSizeBytes  int64  `json:"dataSizeBytes"`
	IndexSizeBytes int64  `json:"indexSizeBytes"`
	TotalSizeBytes int64  `json:"totalSizeBytes"`
	TotalSizeText  string `json:"totalSizeText"`
	CollectedAt    string `json:"collectedAt"`
}

type DatabaseCapacityTrendVO struct {
	InstanceID      uint                        `json:"instanceId"`
	InstanceName    string                      `json:"instanceName"`
	DBType          string                      `json:"dbType"`
	DBTypeText      string                      `json:"dbTypeText"`
	Range           string                      `json:"range"`
	RangeText       string                      `json:"rangeText"`
	CollectedAt     string                      `json:"collectedAt"`
	LatestSizeBytes int64                       `json:"latestSizeBytes"`
	LatestSizeText  string                      `json:"latestSizeText"`
	GrowthBytes     int64                       `json:"growthBytes"`
	GrowthText      string                      `json:"growthText"`
	GrowthPercent   float64                     `json:"growthPercent"`
	Points          []*DatabaseCapacityPointVO  `json:"points"`
	TopSchemas      []*DatabaseCapacityObjectVO `json:"topSchemas"`
	TopTables       []*DatabaseCapacityObjectVO `json:"topTables"`
	Message         string                      `json:"message"`
}

type DatabaseCapacityCollectVO struct {
	InstanceID     uint   `json:"instanceId"`
	InstanceName   string `json:"instanceName"`
	SnapshotsCount int    `json:"snapshotsCount"`
	SchemaCount    int    `json:"schemaCount"`
	TableCount     int    `json:"tableCount"`
	TotalSizeBytes int64  `json:"totalSizeBytes"`
	TotalSizeText  string `json:"totalSizeText"`
	CollectedAt    string `json:"collectedAt"`
	Message        string `json:"message"`
}

func (uc *UseCase) CollectCapacitySnapshot(ctx context.Context, instanceID uint) (*DatabaseCapacityCollectVO, error) {
	if uc.capacitySnapshotRepo == nil {
		return nil, fmt.Errorf("容量快照仓库未配置")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	if normalizeDBType(instance.DBType) == DBTypeRedis {
		return uc.collectRedisCapacitySnapshot(ctx, instance)
	}
	schemas, err := uc.schemaRepo.ListByInstanceID(ctx, instance.ID)
	if err != nil {
		return nil, fmt.Errorf("读取 Schema 元数据失败: %w", err)
	}
	tables, err := uc.tableRepo.List(ctx, instance.ID, "")
	if err != nil {
		return nil, fmt.Errorf("读取表元数据失败: %w", err)
	}

	collectedAt := time.Now()
	snapshots := buildCapacitySnapshots(instance, schemas, tables, collectedAt)
	if err := uc.capacitySnapshotRepo.CreateBatch(ctx, snapshots); err != nil {
		return nil, fmt.Errorf("写入容量快照失败: %w", err)
	}

	instanceSnapshot := findInstanceCapacitySnapshot(snapshots)
	return &DatabaseCapacityCollectVO{
		InstanceID:     instance.ID,
		InstanceName:   instance.Name,
		SnapshotsCount: len(snapshots),
		SchemaCount:    len(schemas),
		TableCount:     len(tables),
		TotalSizeBytes: instanceSnapshot.TotalSizeBytes,
		TotalSizeText:  humanizeBytes(instanceSnapshot.TotalSizeBytes),
		CollectedAt:    collectedAt.Format("2006-01-02 15:04:05"),
		Message:        "容量快照采集完成",
	}, nil
}

func (uc *UseCase) CollectCapacitySnapshotForUser(ctx context.Context, instanceID uint, operator QueryOperator) (*DatabaseCapacityCollectVO, error) {
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	audit := uc.startCapacityAudit(ctx, instance, "COLLECT CAPACITY SNAPSHOT", operator)
	start := time.Now()
	result, err := uc.CollectCapacitySnapshot(ctx, instanceID)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusFailed, 0, duration, err.Error())
		return nil, err
	}
	uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusSuccess, result.SnapshotsCount, duration, "")
	return result, nil
}

func (uc *UseCase) CollectCapacitySnapshots(ctx context.Context) (int, error) {
	if uc.instanceRepo == nil {
		return 0, fmt.Errorf("实例仓库未配置")
	}
	items, err := uc.instanceRepo.ListEnabled(ctx)
	if err != nil {
		return 0, err
	}
	collected := 0
	var firstErr error
	for _, item := range items {
		if item == nil {
			continue
		}
		if _, err := uc.CollectCapacitySnapshot(ctx, item.ID); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		collected++
	}
	return collected, firstErr
}

func (uc *UseCase) GetCapacityTrend(ctx context.Context, instanceID uint, req *DatabaseCapacityTrendRequest, operator QueryOperator) (*DatabaseCapacityTrendVO, error) {
	if uc.capacitySnapshotRepo == nil {
		return nil, fmt.Errorf("容量快照仓库未配置")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	normalizedRange, since := normalizeCapacityRange(req)
	topLimit := normalizeTopLimit(req)

	audit := uc.startCapacityAudit(ctx, instance, "VIEW CAPACITY TREND "+normalizedRange, operator)
	start := time.Now()
	points, err := uc.capacitySnapshotRepo.ListInstanceTrend(ctx, instance.ID, since)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusFailed, 0, duration, err.Error())
		return nil, err
	}

	topSchemas, _ := uc.capacitySnapshotRepo.LatestTopObjects(ctx, instance.ID, DatabaseCapacityObjectSchema, topLimit)
	topTables, _ := uc.capacitySnapshotRepo.LatestTopObjects(ctx, instance.ID, DatabaseCapacityObjectTable, topLimit)
	result := buildCapacityTrendVO(instance, normalizedRange, points, topSchemas, topTables)
	uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusSuccess, len(result.Points), duration, "")
	return result, nil
}

func buildCapacitySnapshots(instance *DatabaseInstance, schemas []*DatabaseSchema, tables []*DatabaseTable, collectedAt time.Time) []*DatabaseCapacitySnapshot {
	schemaStats := make(map[string]*DatabaseCapacitySnapshot)
	schemaFallbackSize := make(map[string]int64)
	schemaFallbackTableCount := make(map[string]int)
	for _, schema := range schemas {
		if schema == nil {
			continue
		}
		name := strings.TrimSpace(schema.SchemaName)
		schemaFallbackSize[name] = schema.SizeBytes
		schemaFallbackTableCount[name] = schema.TableCount
		schemaStats[name] = &DatabaseCapacitySnapshot{
			InstanceID:  instance.ID,
			ObjectType:  DatabaseCapacityObjectSchema,
			SchemaName:  name,
			CollectedAt: collectedAt,
		}
	}

	snapshots := make([]*DatabaseCapacitySnapshot, 0, len(schemas)+len(tables)+1)
	totalDataSize := int64(0)
	totalIndexSize := int64(0)
	totalRows := int64(0)
	for _, table := range tables {
		if table == nil {
			continue
		}
		totalSize := table.DataSizeBytes + table.IndexSizeBytes
		totalDataSize += table.DataSizeBytes
		totalIndexSize += table.IndexSizeBytes
		totalRows += table.RowCount
		schemaName := strings.TrimSpace(table.SchemaName)
		stat := schemaStats[schemaName]
		if stat == nil {
			stat = &DatabaseCapacitySnapshot{
				InstanceID:  instance.ID,
				ObjectType:  DatabaseCapacityObjectSchema,
				SchemaName:  schemaName,
				CollectedAt: collectedAt,
			}
			schemaStats[schemaName] = stat
		}
		stat.TableCount++
		stat.RowCount += table.RowCount
		stat.DataSizeBytes += table.DataSizeBytes
		stat.IndexSizeBytes += table.IndexSizeBytes
		stat.TotalSizeBytes = stat.DataSizeBytes + stat.IndexSizeBytes

		snapshots = append(snapshots, &DatabaseCapacitySnapshot{
			InstanceID:     instance.ID,
			ObjectType:     DatabaseCapacityObjectTable,
			SchemaName:     schemaName,
			Table:          strings.TrimSpace(table.Name),
			RowCount:       table.RowCount,
			DataSizeBytes:  table.DataSizeBytes,
			IndexSizeBytes: table.IndexSizeBytes,
			TotalSizeBytes: totalSize,
			CollectedAt:    collectedAt,
		})
	}

	schemaNames := make([]string, 0, len(schemaStats))
	for name := range schemaStats {
		schemaNames = append(schemaNames, name)
	}
	sort.Strings(schemaNames)
	for _, name := range schemaNames {
		stat := schemaStats[name]
		if stat.TableCount == 0 && schemaFallbackTableCount[name] > 0 {
			stat.TableCount = schemaFallbackTableCount[name]
		}
		if stat.DataSizeBytes == 0 && stat.IndexSizeBytes == 0 && schemaFallbackSize[name] > 0 {
			stat.DataSizeBytes = schemaFallbackSize[name]
		}
		if stat.TotalSizeBytes == 0 {
			stat.TotalSizeBytes = stat.DataSizeBytes + stat.IndexSizeBytes
		}
		snapshots = append(snapshots, stat)
	}

	totalSize := totalDataSize + totalIndexSize
	if totalSize == 0 {
		for _, schema := range schemas {
			if schema != nil {
				totalSize += schema.SizeBytes
			}
		}
		totalDataSize = totalSize
	}
	instanceSnapshot := &DatabaseCapacitySnapshot{
		InstanceID:     instance.ID,
		ObjectType:     DatabaseCapacityObjectInstance,
		SchemaCount:    len(schemas),
		TableCount:     len(tables),
		RowCount:       totalRows,
		DataSizeBytes:  totalDataSize,
		IndexSizeBytes: totalIndexSize,
		TotalSizeBytes: totalSize,
		CollectedAt:    collectedAt,
	}
	return append([]*DatabaseCapacitySnapshot{instanceSnapshot}, snapshots...)
}

func buildCapacityTrendVO(instance *DatabaseInstance, capacityRange string, points []*DatabaseCapacitySnapshot, topSchemas, topTables []*DatabaseCapacitySnapshot) *DatabaseCapacityTrendVO {
	vo := &DatabaseCapacityTrendVO{
		InstanceID:   instance.ID,
		InstanceName: instance.Name,
		DBType:       instance.DBType,
		DBTypeText:   DBTypeText(instance.DBType),
		Range:        capacityRange,
		RangeText:    capacityRangeText(capacityRange),
		CollectedAt:  time.Now().Format("2006-01-02 15:04:05"),
		Points:       make([]*DatabaseCapacityPointVO, 0, len(points)),
		TopSchemas:   capacityObjectsToVO(topSchemas),
		TopTables:    capacityObjectsToVO(topTables),
	}
	for _, point := range points {
		if point == nil {
			continue
		}
		vo.Points = append(vo.Points, capacityPointToVO(point))
	}
	if len(vo.Points) == 0 {
		if normalizeDBType(instance.DBType) == DBTypeRedis {
			vo.Message = "暂无 Redis 内存采样数据，请先手动采集或等待后台采样"
		} else {
			vo.Message = "暂无容量采样数据，请先手动采集或等待后台采样"
		}
		return vo
	}
	first := vo.Points[0]
	last := vo.Points[len(vo.Points)-1]
	vo.LatestSizeBytes = last.TotalSizeBytes
	vo.LatestSizeText = humanizeBytes(last.TotalSizeBytes)
	vo.GrowthBytes = last.TotalSizeBytes - first.TotalSizeBytes
	vo.GrowthText = humanizeSignedBytes(vo.GrowthBytes)
	if first.TotalSizeBytes > 0 {
		vo.GrowthPercent = float64(vo.GrowthBytes) / float64(first.TotalSizeBytes) * 100
	}
	return vo
}

func capacityPointToVO(item *DatabaseCapacitySnapshot) *DatabaseCapacityPointVO {
	return &DatabaseCapacityPointVO{
		CollectedAt:    item.CollectedAt.Format("2006-01-02 15:04:05"),
		SchemaCount:    item.SchemaCount,
		TableCount:     item.TableCount,
		RowCount:       item.RowCount,
		DataSizeBytes:  item.DataSizeBytes,
		IndexSizeBytes: item.IndexSizeBytes,
		TotalSizeBytes: item.TotalSizeBytes,
		TotalSizeText:  humanizeBytes(item.TotalSizeBytes),
	}
}

func capacityObjectsToVO(items []*DatabaseCapacitySnapshot) []*DatabaseCapacityObjectVO {
	list := make([]*DatabaseCapacityObjectVO, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		list = append(list, &DatabaseCapacityObjectVO{
			ObjectType:     item.ObjectType,
			SchemaName:     item.SchemaName,
			TableName:      item.Table,
			TableCount:     item.TableCount,
			RowCount:       item.RowCount,
			DataSizeBytes:  item.DataSizeBytes,
			IndexSizeBytes: item.IndexSizeBytes,
			TotalSizeBytes: item.TotalSizeBytes,
			TotalSizeText:  humanizeBytes(item.TotalSizeBytes),
			CollectedAt:    item.CollectedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return list
}

func findInstanceCapacitySnapshot(items []*DatabaseCapacitySnapshot) *DatabaseCapacitySnapshot {
	for _, item := range items {
		if item != nil && item.ObjectType == DatabaseCapacityObjectInstance {
			return item
		}
	}
	return &DatabaseCapacitySnapshot{}
}

func normalizeCapacityRange(req *DatabaseCapacityTrendRequest) (string, time.Time) {
	value := "7d"
	if req != nil && strings.TrimSpace(req.Range) != "" {
		value = strings.ToLower(strings.TrimSpace(req.Range))
	}
	now := time.Now()
	switch value {
	case "24h", "1d":
		return "24h", now.Add(-24 * time.Hour)
	case "30d":
		return "30d", now.Add(-30 * 24 * time.Hour)
	default:
		return "7d", now.Add(-7 * 24 * time.Hour)
	}
}

func normalizeTopLimit(req *DatabaseCapacityTrendRequest) int {
	if req == nil || req.TopLimit <= 0 {
		return 10
	}
	if req.TopLimit > 50 {
		return 50
	}
	return req.TopLimit
}

func capacityRangeText(value string) string {
	switch value {
	case "24h":
		return "近 24 小时"
	case "30d":
		return "近 30 天"
	default:
		return "近 7 天"
	}
}

func humanizeSignedBytes(size int64) string {
	if size == 0 {
		return "0 B"
	}
	prefix := "+"
	value := size
	if size < 0 {
		prefix = "-"
		value = -size
	}
	return prefix + humanizeBytes(value)
}

func (uc *UseCase) startCapacityAudit(ctx context.Context, item *DatabaseInstance, sqlText string, operator QueryOperator) *DatabaseQueryAudit {
	if uc.auditRepo == nil || item == nil {
		return nil
	}
	audit := &DatabaseQueryAudit{
		InstanceID:     item.ID,
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 100),
		AuditAction:    DatabaseAuditActionCapacityView,
		SQLText:        trimText(sqlText, 20000),
		SQLFingerprint: sqlFingerprint(sqlText),
		SQLType:        "CAPACITY",
		RiskLevel:      DatabaseQueryRiskLow,
		Status:         DatabaseQueryStatusPending,
		ClientIP:       trimText(operator.ClientIP, 64),
	}
	if err := uc.auditRepo.Create(ctx, audit); err != nil {
		return nil
	}
	return audit
}
