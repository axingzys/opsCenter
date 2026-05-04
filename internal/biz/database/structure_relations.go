package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

func (uc *UseCase) ListTableRelations(ctx context.Context, req *DatabaseTableRelationListRequest) (*DatabaseTableRelationGraphVO, error) {
	if req == nil {
		req = &DatabaseTableRelationListRequest{}
	}
	instance, err := uc.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	if normalizeDBType(instance.DBType) == DBTypeRedis {
		return &DatabaseTableRelationGraphVO{
			Relations: []*DatabaseTableRelationVO{},
			Nodes:     []*DatabaseTableRelationNodeVO{},
			Links:     []*DatabaseTableRelationLinkVO{},
		}, nil
	}
	if uc.tableRelationRepo == nil {
		return nil, fmt.Errorf("表关系仓储未配置")
	}

	req.SchemaName = strings.TrimSpace(req.SchemaName)
	req.TableName = strings.TrimSpace(req.TableName)
	req.Direction = normalizeTableRelationDirection(req.Direction)
	req.Source = strings.TrimSpace(req.Source)
	req.CurrentTableName = req.TableName

	items, err := uc.tableRelationRepo.List(ctx, req)
	if err != nil {
		return nil, err
	}
	relations := make([]*DatabaseTableRelationVO, 0, len(items))
	for _, item := range items {
		relations = append(relations, uc.toTableRelationVO(item, req, instance.DBType))
	}
	return buildTableRelationGraph(relations, req), nil
}

func (uc *UseCase) toTableRelationVO(item *DatabaseTableRelation, req *DatabaseTableRelationListRequest, dbType string) *DatabaseTableRelationVO {
	if item == nil {
		return nil
	}
	direction := resolveTableRelationDirection(item, req)
	impactLevel, impactText := buildTableRelationImpact(item, direction)
	return &DatabaseTableRelationVO{
		ID:                   item.ID,
		InstanceID:           item.InstanceID,
		SchemaName:           item.SchemaName,
		TableName:            item.Table,
		ColumnName:           item.ColumnName,
		ReferencedSchemaName: item.ReferencedSchemaName,
		ReferencedTableName:  item.ReferencedTableName,
		ReferencedColumnName: item.ReferencedColumnName,
		ConstraintName:       item.ConstraintName,
		RelationType:         item.RelationType,
		RelationTypeText:     TableRelationTypeText(item.RelationType),
		RelationSource:       item.RelationSource,
		RelationSourceText:   TableRelationSourceText(item.RelationSource),
		Confidence:           item.Confidence,
		OnUpdate:             item.OnUpdate,
		OnDelete:             item.OnDelete,
		Cardinality:          item.Cardinality,
		CardinalityText:      TableRelationCardinalityText(item.Cardinality),
		Comment:              item.Comment,
		Direction:            direction,
		JoinSQL:              buildTableRelationJoinSQL(item, dbType),
		ReverseJoinSQL:       buildTableRelationReverseJoinSQL(item, dbType),
		OrphanCheckSQL:       buildTableRelationOrphanCheckSQL(item, dbType),
		DependencyCheckSQL:   buildTableRelationDependencyCheckSQL(item, dbType),
		ImpactLevel:          impactLevel,
		ImpactText:           impactText,
		LastSyncAt:           formatTime(item.LastSyncAt),
	}
}

func buildTableRelationGraph(relations []*DatabaseTableRelationVO, req *DatabaseTableRelationListRequest) *DatabaseTableRelationGraphVO {
	nodeMap := make(map[string]*DatabaseTableRelationNodeVO)
	links := make([]*DatabaseTableRelationLinkVO, 0, len(relations))
	summary := DatabaseTableRelationSummaryVO{Total: len(relations)}
	for _, relation := range relations {
		if relation == nil {
			continue
		}
		if relation.RelationType == DatabaseTableRelationTypeForeignKey {
			summary.ForeignKeys++
		}
		if relation.RelationType == DatabaseTableRelationTypeInferred {
			summary.Inferred++
		}
		if relation.Confidence > 0 && relation.Confidence < 80 {
			summary.LowConfidence++
		}
		switch relation.Direction {
		case "incoming":
			summary.Incoming++
		case "outgoing":
			summary.Outgoing++
		}

		sourceID := tableRelationNodeID(relation.SchemaName, relation.TableName)
		targetID := tableRelationNodeID(relation.ReferencedSchemaName, relation.ReferencedTableName)
		source := ensureTableRelationNode(nodeMap, relation.SchemaName, relation.TableName, req)
		target := ensureTableRelationNode(nodeMap, relation.ReferencedSchemaName, relation.ReferencedTableName, req)
		source.Outgoing++
		target.Incoming++
		if relation.RelationType == DatabaseTableRelationTypeForeignKey {
			source.RelationType = DatabaseTableRelationTypeForeignKey
			target.RelationType = DatabaseTableRelationTypeForeignKey
		} else if source.RelationType == "" {
			source.RelationType = relation.RelationType
		} else if target.RelationType == "" {
			target.RelationType = relation.RelationType
		}

		links = append(links, &DatabaseTableRelationLinkVO{
			ID:           fmt.Sprintf("%d:%s:%s:%s", relation.ID, relation.TableName, relation.ColumnName, relation.ReferencedTableName),
			Source:       sourceID,
			Target:       targetID,
			SourceLabel:  source.Label,
			TargetLabel:  target.Label,
			Label:        fmt.Sprintf("%s -> %s", relation.ColumnName, relation.ReferencedColumnName),
			RelationType: relation.RelationType,
			Confidence:   relation.Confidence,
		})
	}

	nodes := make([]*DatabaseTableRelationNodeVO, 0, len(nodeMap))
	for _, node := range nodeMap {
		if node.RelationType == "" {
			node.RelationType = DatabaseTableRelationTypeInferred
		}
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Current != nodes[j].Current {
			return nodes[i].Current
		}
		return nodes[i].ID < nodes[j].ID
	})
	return &DatabaseTableRelationGraphVO{
		Relations: relations,
		Nodes:     nodes,
		Links:     links,
		Summary:   summary,
	}
}

func ensureTableRelationNode(nodes map[string]*DatabaseTableRelationNodeVO, schemaName, tableName string, req *DatabaseTableRelationListRequest) *DatabaseTableRelationNodeVO {
	id := tableRelationNodeID(schemaName, tableName)
	if node, ok := nodes[id]; ok {
		return node
	}
	node := &DatabaseTableRelationNodeVO{
		ID:         id,
		SchemaName: schemaName,
		TableName:  tableName,
		Label:      tableRelationNodeLabel(schemaName, tableName),
		Current: strings.EqualFold(strings.TrimSpace(req.SchemaName), strings.TrimSpace(schemaName)) &&
			strings.EqualFold(strings.TrimSpace(req.TableName), strings.TrimSpace(tableName)),
	}
	nodes[id] = node
	return node
}

func buildInferredTableRelations(tables []*DatabaseTable, columns []*DatabaseColumn, indexes []*DatabaseIndex, existing []*DatabaseTableRelation) []*DatabaseTableRelation {
	if len(tables) == 0 || len(columns) == 0 {
		return nil
	}

	tableMap := make(map[string]*DatabaseTable)
	for _, table := range tables {
		if table == nil {
			continue
		}
		tableMap[tableRelationNodeID(table.SchemaName, table.Name)] = table
	}
	uniqueColumns := buildUniqueTableColumns(columns, indexes)
	existingKeys := make(map[string]struct{})
	for _, relation := range existing {
		if relation == nil {
			continue
		}
		existingKeys[tableRelationSignature(
			relation.SchemaName,
			relation.Table,
			relation.ColumnName,
			relation.ReferencedSchemaName,
			relation.ReferencedTableName,
			relation.ReferencedColumnName,
		)] = struct{}{}
	}

	result := make([]*DatabaseTableRelation, 0)
	for _, column := range columns {
		if column == nil {
			continue
		}
		targetBase, targetColumn, ok := inferRelationTarget(column.ColumnName)
		if !ok {
			continue
		}
		sourceKey := tableRelationNodeID(column.SchemaName, column.Table)
		if _, ok := tableMap[sourceKey]; !ok {
			continue
		}
		target := chooseInferredRelationTarget(column.SchemaName, column.Table, targetBase, targetColumn, tableMap, uniqueColumns)
		if target == nil {
			continue
		}
		signature := tableRelationSignature(column.SchemaName, column.Table, column.ColumnName, target.SchemaName, target.Name, targetColumn)
		if _, ok := existingKeys[signature]; ok {
			continue
		}
		existingKeys[signature] = struct{}{}
		confidence := 78
		if strings.EqualFold(column.SchemaName, target.SchemaName) {
			confidence += 6
		}
		if isPrimaryOrUniqueColumn(uniqueColumns, target.SchemaName, target.Name, targetColumn, true) {
			confidence += 6
		}
		if confidence > 92 {
			confidence = 92
		}
		result = append(result, &DatabaseTableRelation{
			SchemaName:           column.SchemaName,
			Table:                column.Table,
			ColumnName:           column.ColumnName,
			ReferencedSchemaName: target.SchemaName,
			ReferencedTableName:  target.Name,
			ReferencedColumnName: targetColumn,
			ConstraintName:       trimText(fmt.Sprintf("inferred_%s_%s_%s", column.Table, column.ColumnName, target.Name), 150),
			RelationType:         DatabaseTableRelationTypeInferred,
			RelationSource:       DatabaseTableRelationSourceUniqueIndex,
			Confidence:           confidence,
			Cardinality:          DatabaseTableRelationCardinalityManyToOne,
			Comment:              "基于字段命名和目标主键/唯一索引推断，建议人工确认",
		})
	}
	return result
}

type tableRelationTarget struct {
	*DatabaseTable
	score int
}

func chooseInferredRelationTarget(sourceSchema, sourceTable, targetBase, targetColumn string, tables map[string]*DatabaseTable, uniqueColumns map[string]map[string]bool) *DatabaseTable {
	candidates := make([]tableRelationTarget, 0)
	for _, table := range tables {
		if table == nil || strings.EqualFold(table.SchemaName, sourceSchema) && strings.EqualFold(table.Name, sourceTable) {
			continue
		}
		score := matchRelationTableScore(table.Name, targetBase)
		if score == 0 {
			continue
		}
		if !isPrimaryOrUniqueColumn(uniqueColumns, table.SchemaName, table.Name, targetColumn, false) {
			continue
		}
		if strings.EqualFold(table.SchemaName, sourceSchema) {
			score += 20
		}
		if isPrimaryOrUniqueColumn(uniqueColumns, table.SchemaName, table.Name, targetColumn, true) {
			score += 10
		}
		candidates = append(candidates, tableRelationTarget{DatabaseTable: table, score: score})
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return tableRelationNodeID(candidates[i].SchemaName, candidates[i].Name) < tableRelationNodeID(candidates[j].SchemaName, candidates[j].Name)
	})
	if len(candidates) > 1 && candidates[0].score == candidates[1].score {
		return nil
	}
	return candidates[0].DatabaseTable
}

func buildUniqueTableColumns(columns []*DatabaseColumn, indexes []*DatabaseIndex) map[string]map[string]bool {
	result := make(map[string]map[string]bool)
	for _, column := range columns {
		if column == nil {
			continue
		}
		if strings.EqualFold(column.ColumnKey, "PRI") {
			markTableRelationUniqueColumn(result, column.SchemaName, column.Table, column.ColumnName)
		}
	}
	for _, index := range indexes {
		if index == nil {
			continue
		}
		cols := splitRelationIndexColumns(index.Columns)
		if len(cols) != 1 {
			continue
		}
		if index.IsUnique || strings.EqualFold(index.IndexType, "PRIMARY") || strings.EqualFold(index.IndexName, "PRIMARY") {
			markTableRelationUniqueColumn(result, index.SchemaName, index.Table, cols[0])
		}
	}
	return result
}

func markTableRelationUniqueColumn(items map[string]map[string]bool, schemaName, tableName, columnName string) {
	key := tableRelationNodeID(schemaName, tableName)
	if _, ok := items[key]; !ok {
		items[key] = make(map[string]bool)
	}
	items[key][normalizeRelationName(columnName)] = true
}

func isPrimaryOrUniqueColumn(items map[string]map[string]bool, schemaName, tableName, columnName string, strictPrimaryName bool) bool {
	cols := items[tableRelationNodeID(schemaName, tableName)]
	if cols == nil {
		return false
	}
	normalized := normalizeRelationName(columnName)
	if strictPrimaryName {
		return cols[normalized] && (normalized == "id" || normalized == "uuid")
	}
	return cols[normalized]
}

func inferRelationTarget(columnName string) (string, string, bool) {
	normalized := normalizeRelationName(columnName)
	if normalized == "" {
		return "", "", false
	}
	for _, blocked := range []string{"id", "uuid", "trace_id", "request_id", "session_id", "batch_id", "token_id", "message_id", "event_id"} {
		if normalized == blocked {
			return "", "", false
		}
	}
	switch {
	case strings.HasSuffix(normalized, "_id"):
		base := strings.TrimSuffix(normalized, "_id")
		if isWeakRelationBase(base) {
			return "", "", false
		}
		return base, "id", true
	case strings.HasSuffix(normalized, "_uuid"):
		base := strings.TrimSuffix(normalized, "_uuid")
		if isWeakRelationBase(base) {
			return "", "", false
		}
		return base, "uuid", true
	default:
		return "", "", false
	}
}

func isWeakRelationBase(base string) bool {
	if base == "" {
		return true
	}
	switch base {
	case "trace", "request", "session", "batch", "token", "message", "event", "file", "path", "image", "icon", "logo", "hash":
		return true
	default:
		return false
	}
}

func matchRelationTableScore(tableName, targetBase string) int {
	table := normalizeRelationName(tableName)
	base := normalizeRelationName(targetBase)
	if table == "" || base == "" {
		return 0
	}
	plural := pluralRelationBase(base)
	switch {
	case table == base:
		return 70
	case table == plural:
		return 68
	case strings.HasSuffix(table, "_"+base):
		return 52
	case strings.HasSuffix(table, "_"+plural):
		return 50
	default:
		return 0
	}
}

func pluralRelationBase(base string) string {
	if strings.HasSuffix(base, "s") {
		return base
	}
	if strings.HasSuffix(base, "y") && len(base) > 1 {
		return strings.TrimSuffix(base, "y") + "ies"
	}
	if strings.HasSuffix(base, "x") || strings.HasSuffix(base, "ch") || strings.HasSuffix(base, "sh") {
		return base + "es"
	}
	return base + "s"
}

func splitRelationIndexColumns(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		column := normalizeRelationName(part)
		if column == "" || strings.ContainsAny(column, " ()") {
			continue
		}
		result = append(result, column)
	}
	return result
}

func tableRelationNodeID(schemaName, tableName string) string {
	return strings.ToLower(strings.TrimSpace(schemaName)) + "." + strings.ToLower(strings.TrimSpace(tableName))
}

func tableRelationNodeLabel(schemaName, tableName string) string {
	schemaName = strings.TrimSpace(schemaName)
	tableName = strings.TrimSpace(tableName)
	if schemaName == "" {
		return tableName
	}
	return schemaName + "." + tableName
}

func tableRelationSignature(schemaName, tableName, columnName, refSchema, refTable, refColumn string) string {
	return strings.Join([]string{
		tableRelationNodeID(schemaName, tableName),
		normalizeRelationName(columnName),
		tableRelationNodeID(refSchema, refTable),
		normalizeRelationName(refColumn),
	}, "|")
}

func buildTableRelationKey(item *DatabaseTableRelation) string {
	if item == nil {
		return ""
	}
	raw := tableRelationSignature(
		item.SchemaName,
		item.Table,
		item.ColumnName,
		item.ReferencedSchemaName,
		item.ReferencedTableName,
		item.ReferencedColumnName,
	) + "|" + normalizeRelationName(item.RelationType) + "|" + normalizeRelationName(item.RelationSource)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func normalizeRelationName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`\"[]")
	if value == "" {
		return ""
	}
	var b strings.Builder
	runes := []rune(value)
	for i, r := range runes {
		if r == '-' || r == ' ' || r == '.' {
			r = '_'
		}
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := runes[i-1]
			if prev >= 'a' && prev <= 'z' || prev >= '0' && prev <= '9' {
				b.WriteRune('_')
			}
		}
		b.WriteRune(r)
	}
	return strings.ToLower(strings.Trim(b.String(), "_"))
}

func normalizeTableRelationDirection(value string) string {
	switch strings.TrimSpace(value) {
	case "incoming", "outgoing":
		return strings.TrimSpace(value)
	default:
		return "all"
	}
}

func resolveTableRelationDirection(item *DatabaseTableRelation, req *DatabaseTableRelationListRequest) string {
	if item == nil || req == nil || strings.TrimSpace(req.TableName) == "" {
		return "related"
	}
	if strings.EqualFold(item.SchemaName, req.SchemaName) && strings.EqualFold(item.Table, req.TableName) {
		return "outgoing"
	}
	if strings.EqualFold(item.ReferencedSchemaName, req.SchemaName) && strings.EqualFold(item.ReferencedTableName, req.TableName) {
		return "incoming"
	}
	return "related"
}

func buildTableRelationJoinSQL(item *DatabaseTableRelation, dbType string) string {
	if item == nil {
		return ""
	}
	target := tableRelationQualifiedTable(dbType, item.ReferencedSchemaName, item.ReferencedTableName)
	return fmt.Sprintf(
		"LEFT JOIN %s ref ON src.%s = ref.%s",
		target,
		tableRelationQuoteIdentifier(dbType, item.ColumnName),
		tableRelationQuoteIdentifier(dbType, item.ReferencedColumnName),
	)
}

func buildTableRelationReverseJoinSQL(item *DatabaseTableRelation, dbType string) string {
	if item == nil {
		return ""
	}
	source := tableRelationQualifiedTable(dbType, item.SchemaName, item.Table)
	return fmt.Sprintf(
		"LEFT JOIN %s child ON child.%s = parent.%s",
		source,
		tableRelationQuoteIdentifier(dbType, item.ColumnName),
		tableRelationQuoteIdentifier(dbType, item.ReferencedColumnName),
	)
}

func buildTableRelationOrphanCheckSQL(item *DatabaseTableRelation, dbType string) string {
	if item == nil {
		return ""
	}
	source := tableRelationQualifiedTable(dbType, item.SchemaName, item.Table)
	target := tableRelationQualifiedTable(dbType, item.ReferencedSchemaName, item.ReferencedTableName)
	selectPrefix := "SELECT src.*"
	limitSuffix := tableRelationLimitSuffix(dbType, 100)
	if normalizeDBType(dbType) == DBTypeSQLServer {
		selectPrefix = "SELECT TOP (100) src.*"
		limitSuffix = ""
	}
	sql := fmt.Sprintf(
		"%s\nFROM %s src\nLEFT JOIN %s ref ON src.%s = ref.%s\nWHERE src.%s IS NOT NULL\n  AND ref.%s IS NULL",
		selectPrefix,
		source,
		target,
		tableRelationQuoteIdentifier(dbType, item.ColumnName),
		tableRelationQuoteIdentifier(dbType, item.ReferencedColumnName),
		tableRelationQuoteIdentifier(dbType, item.ColumnName),
		tableRelationQuoteIdentifier(dbType, item.ReferencedColumnName),
	)
	if limitSuffix != "" {
		sql += "\n" + limitSuffix
	}
	return sql
}

func buildTableRelationDependencyCheckSQL(item *DatabaseTableRelation, dbType string) string {
	if item == nil {
		return ""
	}
	source := tableRelationQualifiedTable(dbType, item.SchemaName, item.Table)
	target := tableRelationQualifiedTable(dbType, item.ReferencedSchemaName, item.ReferencedTableName)
	return fmt.Sprintf(
		"SELECT COUNT(*) AS dependent_rows\nFROM %s child\nJOIN %s parent ON child.%s = parent.%s",
		source,
		target,
		tableRelationQuoteIdentifier(dbType, item.ColumnName),
		tableRelationQuoteIdentifier(dbType, item.ReferencedColumnName),
	)
}

func buildTableRelationImpact(item *DatabaseTableRelation, direction string) (string, string) {
	if item == nil {
		return "info", ""
	}
	source := tableRelationNodeLabel(item.SchemaName, item.Table)
	target := tableRelationNodeLabel(item.ReferencedSchemaName, item.ReferencedTableName)
	isForeignKey := item.RelationType == DatabaseTableRelationTypeForeignKey
	if direction == "incoming" {
		if isForeignKey {
			return "high", fmt.Sprintf("当前表被 %s.%s 真实外键引用，删除或修改 %s.%s 前应先检查依赖行。", source, item.ColumnName, target, item.ReferencedColumnName)
		}
		if item.Confidence >= 80 {
			return "warning", fmt.Sprintf("当前表可能被 %s.%s 引用，推断可信度 %d，建议人工确认后再评估影响。", source, item.ColumnName, item.Confidence)
		}
		return "info", fmt.Sprintf("当前表可能被 %s.%s 引用，推断可信度较低，仅作为排查线索。", source, item.ColumnName)
	}
	if direction == "outgoing" {
		if isForeignKey {
			return "warning", fmt.Sprintf("当前表通过 %s.%s 依赖 %s.%s，写入或清理来源字段前应避免孤儿记录。", source, item.ColumnName, target, item.ReferencedColumnName)
		}
		if item.Confidence >= 80 {
			return "warning", fmt.Sprintf("当前表可能通过 %s.%s 依赖 %s.%s，推断可信度 %d。", source, item.ColumnName, target, item.ReferencedColumnName, item.Confidence)
		}
		return "info", fmt.Sprintf("当前表可能通过 %s.%s 依赖 %s.%s，推断可信度较低。", source, item.ColumnName, target, item.ReferencedColumnName)
	}
	if isForeignKey {
		return "warning", "该关系来自数据库外键约束，变更前建议检查双向依赖。"
	}
	return "info", "该关系来自命名和唯一键推断，仅作为结构排查线索。"
}

func tableRelationQualifiedTable(dbType, schemaName, tableName string) string {
	schemaName = strings.TrimSpace(schemaName)
	tableName = strings.TrimSpace(tableName)
	quotedTable := tableRelationQuoteIdentifier(dbType, tableName)
	if schemaName == "" {
		return quotedTable
	}
	return tableRelationQuoteIdentifier(dbType, schemaName) + "." + quotedTable
}

func tableRelationQuoteIdentifier(dbType, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	switch normalizeDBType(dbType) {
	case DBTypePostgreSQL, DBTypeOpenGauss, DBTypeKingbase, DBTypeOracle:
		return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
	case DBTypeSQLServer:
		return `[` + strings.ReplaceAll(value, `]`, `]]`) + `]`
	default:
		return "`" + strings.ReplaceAll(value, "`", "``") + "`"
	}
}

func tableRelationLimitSuffix(dbType string, limit int) string {
	if limit <= 0 {
		limit = 100
	}
	switch normalizeDBType(dbType) {
	case DBTypeOracle:
		return fmt.Sprintf("FETCH FIRST %d ROWS ONLY", limit)
	case DBTypeSQLServer:
		return ""
	default:
		return fmt.Sprintf("LIMIT %d", limit)
	}
}

func TableRelationTypeText(value string) string {
	switch value {
	case DatabaseTableRelationTypeForeignKey:
		return "外键"
	case DatabaseTableRelationTypeInferred:
		return "推断"
	default:
		return "未知"
	}
}

func TableRelationSourceText(value string) string {
	switch value {
	case DatabaseTableRelationSourceConstraint:
		return "数据库约束"
	case DatabaseTableRelationSourceNamingRule:
		return "命名规则"
	case DatabaseTableRelationSourceUniqueIndex:
		return "唯一键推断"
	default:
		return "未知"
	}
}

func TableRelationCardinalityText(value string) string {
	switch value {
	case DatabaseTableRelationCardinalityManyToOne:
		return "多对一"
	case DatabaseTableRelationCardinalityOneToOne:
		return "一对一"
	default:
		return "未知"
	}
}
