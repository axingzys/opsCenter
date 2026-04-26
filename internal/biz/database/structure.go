package database

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type DatabaseTableDDLVO struct {
	InstanceID     uint   `json:"instanceId"`
	InstanceName   string `json:"instanceName"`
	DBType         string `json:"dbType"`
	DBTypeText     string `json:"dbTypeText"`
	SchemaName     string `json:"schemaName"`
	TableName      string `json:"tableName"`
	TableType      string `json:"tableType"`
	Engine         string `json:"engine"`
	Comment        string `json:"comment"`
	RowCount       int64  `json:"rowCount"`
	DataSizeBytes  int64  `json:"dataSizeBytes"`
	IndexSizeBytes int64  `json:"indexSizeBytes"`
	ColumnsCount   int    `json:"columnsCount"`
	IndexesCount   int    `json:"indexesCount"`
	DDL            string `json:"ddl"`
	GeneratedAt    string `json:"generatedAt"`
}

type DatabaseTableDictionaryVO struct {
	InstanceID   uint                            `json:"instanceId"`
	InstanceName string                          `json:"instanceName"`
	DBType       string                          `json:"dbType"`
	DBTypeText   string                          `json:"dbTypeText"`
	SchemaName   string                          `json:"schemaName"`
	TableName    string                          `json:"tableName"`
	ExportedAt   string                          `json:"exportedAt"`
	Rows         []*DatabaseTableDictionaryRowVO `json:"rows"`
}

type DatabaseTableDictionaryRowVO struct {
	SchemaName     string `json:"schemaName"`
	TableName      string `json:"tableName"`
	TableType      string `json:"tableType"`
	Engine         string `json:"engine"`
	TableComment   string `json:"tableComment"`
	RowCount       int64  `json:"rowCount"`
	DataSizeBytes  int64  `json:"dataSizeBytes"`
	IndexSizeBytes int64  `json:"indexSizeBytes"`
	ColumnOrder    int    `json:"columnOrder"`
	ColumnName     string `json:"columnName"`
	DataType       string `json:"dataType"`
	IsNullable     bool   `json:"isNullable"`
	DefaultValue   string `json:"defaultValue"`
	ColumnKey      string `json:"columnKey"`
	IsSensitive    bool   `json:"isSensitive"`
	ColumnComment  string `json:"columnComment"`
	IndexSummary   string `json:"indexSummary"`
}

func (uc *UseCase) GetTableDDL(ctx context.Context, instanceID uint, schemaName, tableName string) (*DatabaseTableDDLVO, error) {
	tableName = strings.TrimSpace(tableName)
	if tableName == "" {
		return nil, fmt.Errorf("表名不能为空")
	}

	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	if normalizeDBType(instance.DBType) == DBTypeRedis {
		return nil, fmt.Errorf("Redis Key 不支持 DDL 预览")
	}
	table, err := uc.tableRepo.Get(ctx, instanceID, strings.TrimSpace(schemaName), tableName)
	if err != nil {
		return nil, fmt.Errorf("表不存在或请先同步元数据")
	}
	columns, err := uc.columnRepo.List(ctx, instanceID, table.SchemaName, table.Name)
	if err != nil {
		return nil, err
	}
	indexes, err := uc.indexRepo.List(ctx, instanceID, table.SchemaName, table.Name)
	if err != nil {
		return nil, err
	}

	return &DatabaseTableDDLVO{
		InstanceID:     instance.ID,
		InstanceName:   instance.Name,
		DBType:         instance.DBType,
		DBTypeText:     DBTypeText(instance.DBType),
		SchemaName:     table.SchemaName,
		TableName:      table.Name,
		TableType:      table.TableType,
		Engine:         table.Engine,
		Comment:        table.Comment,
		RowCount:       table.RowCount,
		DataSizeBytes:  table.DataSizeBytes,
		IndexSizeBytes: table.IndexSizeBytes,
		ColumnsCount:   len(columns),
		IndexesCount:   len(indexes),
		DDL:            buildGeneratedDDL(instance.DBType, table, columns, indexes),
		GeneratedAt:    time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

func (uc *UseCase) GetTableDictionary(ctx context.Context, instanceID uint, schemaName, tableName string) (*DatabaseTableDictionaryVO, error) {
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	return uc.getTableDictionaryWithInstance(ctx, instance, schemaName, tableName)
}

func (uc *UseCase) ExportTableDictionary(ctx context.Context, instanceID uint, schemaName, tableName string, operator QueryOperator) (*DatabaseTableDictionaryVO, error) {
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	if normalizeDBType(instance.DBType) == DBTypeRedis {
		return nil, fmt.Errorf("Redis Key 不支持数据字典导出")
	}

	requestedSchema := strings.TrimSpace(schemaName)
	auditSQLText := buildMetadataExportAuditSQL(requestedSchema, tableName)
	audit := &DatabaseQueryAudit{
		InstanceID:     instance.ID,
		SchemaName:     requestedSchema,
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 100),
		AuditAction:    DatabaseAuditActionMetadataExport,
		SQLText:        trimText(auditSQLText, 20000),
		SQLFingerprint: sqlFingerprint(auditSQLText),
		SQLType:        "EXPORT",
		RiskLevel:      DatabaseQueryRiskLow,
		Status:         DatabaseQueryStatusPending,
		ClientIP:       trimText(operator.ClientIP, 64),
	}
	if uc.auditRepo != nil {
		if err := uc.auditRepo.Create(ctx, audit); err != nil {
			return nil, fmt.Errorf("创建导出审计失败: %w", err)
		}
	}

	start := time.Now()
	data, err := uc.getTableDictionaryWithInstance(ctx, instance, schemaName, tableName)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusFailed, 0, duration, err.Error())
		return nil, err
	}

	audit.SchemaName = data.SchemaName
	uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusSuccess, len(data.Rows), duration, "")
	return data, nil
}

func (uc *UseCase) getTableDictionaryWithInstance(ctx context.Context, instance *DatabaseInstance, schemaName, tableName string) (*DatabaseTableDictionaryVO, error) {
	if instance != nil && normalizeDBType(instance.DBType) == DBTypeRedis {
		return nil, fmt.Errorf("Redis Key 不支持数据字典导出")
	}
	tableName = strings.TrimSpace(tableName)
	if tableName == "" {
		return nil, fmt.Errorf("表名不能为空")
	}
	table, err := uc.tableRepo.Get(ctx, instance.ID, strings.TrimSpace(schemaName), tableName)
	if err != nil {
		return nil, fmt.Errorf("表不存在或请先同步元数据")
	}
	columns, err := uc.columnRepo.List(ctx, instance.ID, table.SchemaName, table.Name)
	if err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("表字段不存在或请先同步元数据")
	}
	indexes, err := uc.indexRepo.List(ctx, instance.ID, table.SchemaName, table.Name)
	if err != nil {
		return nil, err
	}

	indexSummaryMap := buildIndexSummaryMap(indexes)
	rows := make([]*DatabaseTableDictionaryRowVO, 0, len(columns))
	for _, column := range columns {
		rows = append(rows, &DatabaseTableDictionaryRowVO{
			SchemaName:     table.SchemaName,
			TableName:      table.Name,
			TableType:      table.TableType,
			Engine:         table.Engine,
			TableComment:   table.Comment,
			RowCount:       table.RowCount,
			DataSizeBytes:  table.DataSizeBytes,
			IndexSizeBytes: table.IndexSizeBytes,
			ColumnOrder:    column.OrdinalPosition,
			ColumnName:     column.ColumnName,
			DataType:       column.DataType,
			IsNullable:     column.IsNullable,
			DefaultValue:   column.DefaultValue,
			ColumnKey:      column.ColumnKey,
			IsSensitive:    column.IsSensitive,
			ColumnComment:  column.Comment,
			IndexSummary:   strings.Join(indexSummaryMap[column.ColumnName], " | "),
		})
	}

	return &DatabaseTableDictionaryVO{
		InstanceID:   instance.ID,
		InstanceName: instance.Name,
		DBType:       instance.DBType,
		DBTypeText:   DBTypeText(instance.DBType),
		SchemaName:   table.SchemaName,
		TableName:    table.Name,
		ExportedAt:   time.Now().Format("2006-01-02 15:04:05"),
		Rows:         rows,
	}, nil
}

func buildGeneratedDDL(dbType string, table *DatabaseTable, columns []*DatabaseColumn, indexes []*DatabaseIndex) string {
	if table == nil {
		return ""
	}

	var lines []string
	lines = append(lines, "-- 基于已同步元数据生成，仅用于结构查看与评审。")
	lines = append(lines, fmt.Sprintf("-- 对象类型: %s", fallbackText(table.TableType, "TABLE")))
	if strings.TrimSpace(table.Comment) != "" {
		lines = append(lines, fmt.Sprintf("-- 对象说明: %s", escapeDDLComment(table.Comment)))
	}
	if strings.TrimSpace(table.TableType) != "" && !strings.Contains(strings.ToUpper(table.TableType), "TABLE") {
		lines = append(lines, fmt.Sprintf("-- 当前对象为 %s，系统暂未持久化原始定义。", table.TableType))
		if len(columns) > 0 {
			lines = append(lines, "-- 同步字段如下：")
			for _, column := range columns {
				lines = append(lines, fmt.Sprintf("--   %s %s", column.ColumnName, column.DataType))
			}
		}
		return strings.Join(lines, "\n")
	}

	qualifiedName := qualifiedDDLName(dbType, table.SchemaName, table.Name)
	lines = append(lines, fmt.Sprintf("CREATE TABLE %s (", qualifiedName))

	definitions := make([]string, 0, len(columns)+len(indexes))
	for _, column := range columns {
		definitions = append(definitions, buildDDLColumnLine(dbType, column))
	}
	if primary := buildDDLPrimaryKeyLine(dbType, indexes, columns); primary != "" {
		definitions = append(definitions, primary)
	}
	for _, index := range indexes {
		if isPrimaryIndex(index) {
			continue
		}
		definitions = append(definitions, buildDDLIndexLine(dbType, index))
	}

	for i, definition := range definitions {
		suffix := ","
		if i == len(definitions)-1 {
			suffix = ""
		}
		lines = append(lines, definition+suffix)
	}
	lines = append(lines, ");")

	if strings.TrimSpace(table.Engine) != "" {
		lines = append(lines, fmt.Sprintf("-- Engine: %s", table.Engine))
	}
	return strings.Join(lines, "\n")
}

func buildDDLColumnLine(dbType string, column *DatabaseColumn) string {
	definition := fmt.Sprintf("  %s %s", quoteDDLIdentifier(dbType, column.ColumnName), fallbackText(column.DataType, "text"))
	if !column.IsNullable {
		definition += " NOT NULL"
	}
	if strings.TrimSpace(column.DefaultValue) != "" {
		definition += " DEFAULT " + strings.TrimSpace(column.DefaultValue)
	}

	notes := make([]string, 0, 2)
	if strings.TrimSpace(column.ColumnKey) != "" {
		notes = append(notes, "key="+column.ColumnKey)
	}
	if column.IsSensitive {
		notes = append(notes, "sensitive")
	}
	if strings.TrimSpace(column.Comment) != "" {
		notes = append(notes, escapeDDLComment(column.Comment))
	}
	if len(notes) > 0 {
		definition += " -- " + strings.Join(notes, "; ")
	}
	return definition
}

func buildDDLPrimaryKeyLine(dbType string, indexes []*DatabaseIndex, columns []*DatabaseColumn) string {
	for _, index := range indexes {
		if isPrimaryIndex(index) {
			return fmt.Sprintf("  PRIMARY KEY (%s)", formatIndexColumns(dbType, index.Columns))
		}
	}

	pkColumns := make([]string, 0)
	for _, column := range columns {
		if strings.EqualFold(strings.TrimSpace(column.ColumnKey), "PRI") {
			pkColumns = append(pkColumns, quoteDDLIdentifier(dbType, column.ColumnName))
		}
	}
	if len(pkColumns) == 0 {
		return ""
	}
	return fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(pkColumns, ", "))
}

func buildDDLIndexLine(dbType string, index *DatabaseIndex) string {
	keyword := "INDEX"
	if index.IsUnique {
		keyword = "UNIQUE INDEX"
	}
	switch normalizeDBType(dbType) {
	case DBTypeMySQL, DBTypeMariaDB, DBTypeTiDB, DBTypeOceanBase:
		if index.IsUnique {
			keyword = "UNIQUE KEY"
		}
	}
	return fmt.Sprintf("  %s %s (%s)", keyword, quoteDDLIdentifier(dbType, index.IndexName), formatIndexColumns(dbType, index.Columns))
}

func buildIndexSummaryMap(indexes []*DatabaseIndex) map[string][]string {
	summaryMap := make(map[string][]string)
	for _, index := range indexes {
		summary := index.IndexName
		if index.IsUnique {
			summary += "(UNIQUE)"
		}
		if strings.TrimSpace(index.IndexType) != "" {
			summary += ":" + strings.TrimSpace(index.IndexType)
		}
		if strings.TrimSpace(index.Columns) != "" {
			summary += " [" + strings.TrimSpace(index.Columns) + "]"
		}
		for _, column := range splitIndexColumns(index.Columns) {
			summaryMap[column] = append(summaryMap[column], summary)
		}
	}
	return summaryMap
}

func formatIndexColumns(dbType, raw string) string {
	parts := splitIndexColumns(raw)
	if len(parts) == 0 {
		return quoteDDLIdentifier(dbType, raw)
	}
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		quoted = append(quoted, quoteDDLIdentifier(dbType, part))
	}
	return strings.Join(quoted, ", ")
}

func splitIndexColumns(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	columns := make([]string, 0, len(parts))
	for _, part := range parts {
		column := strings.TrimSpace(part)
		column = strings.Trim(column, "`\"[]")
		if column == "" {
			continue
		}
		columns = append(columns, column)
	}
	return columns
}

func qualifiedDDLName(dbType, schemaName, tableName string) string {
	if strings.TrimSpace(schemaName) == "" {
		return quoteDDLIdentifier(dbType, tableName)
	}
	return quoteDDLIdentifier(dbType, schemaName) + "." + quoteDDLIdentifier(dbType, tableName)
}

func quoteDDLIdentifier(dbType, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	switch normalizeDBType(dbType) {
	case DBTypePostgreSQL, DBTypeOracle, DBTypeOpenGauss, DBTypeKingbase:
		return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
	case DBTypeSQLServer:
		return `[` + strings.ReplaceAll(value, `]`, `]]`) + `]`
	default:
		return "`" + strings.ReplaceAll(value, "`", "``") + "`"
	}
}

func isPrimaryIndex(index *DatabaseIndex) bool {
	if index == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(index.IndexName), "PRIMARY") || strings.EqualFold(strings.TrimSpace(index.IndexType), "PRIMARY")
}

func escapeDDLComment(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\r\n", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	return strings.TrimSpace(value)
}

func fallbackText(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
