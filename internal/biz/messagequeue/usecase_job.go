package messagequeue

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"
)

func (uc *UseCase) ListJobs(ctx context.Context, req *JobListRequest) ([]*JobVO, int64, error) {
	if uc.jobRepo == nil {
		return nil, 0, fmt.Errorf("MQ任务仓库未配置")
	}
	normalizeJobListRequest(req)
	items, total, err := uc.jobRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*JobVO, 0, len(items))
	for _, item := range items {
		list = append(list, toJobVO(item))
	}
	return list, total, nil
}

func (uc *UseCase) GetJob(ctx context.Context, id uint) (*JobVO, error) {
	if uc.jobRepo == nil {
		return nil, fmt.Errorf("MQ任务仓库未配置")
	}
	if id == 0 {
		return nil, fmt.Errorf("MQ任务ID不能为空")
	}
	item, err := uc.jobRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("MQ任务不存在")
	}
	return toJobVO(item), nil
}

func (uc *UseCase) startMQJob(ctx context.Context, instance *MQInstance, jobType, triggerType string, operator Operator, total int64, stage string) *MQJob {
	if uc.jobRepo == nil || instance == nil {
		return nil
	}
	now := time.Now()
	if strings.TrimSpace(triggerType) == "" {
		triggerType = TriggerManual
	}
	if total < 0 {
		total = 0
	}
	job := &MQJob{
		InstanceID:      instance.ID,
		InstanceName:    instance.Name,
		MQType:          instance.MQType,
		JobType:         strings.TrimSpace(jobType),
		Status:          JobStatusRunning,
		ProgressCurrent: 0,
		ProgressTotal:   total,
		CurrentStage:    trimText(stage, 120),
		TriggerType:     triggerType,
		OperatorID:      operator.ID,
		OperatorName:    operator.Username,
		StartedAt:       &now,
		CorrelationID:   newMQCorrelationID(),
	}
	if err := uc.jobRepo.Create(ctx, job); err != nil {
		return nil
	}
	return job
}

func (uc *UseCase) updateMQJob(ctx context.Context, job *MQJob, current, total int64, stage, message string) {
	if uc.jobRepo == nil || job == nil {
		return
	}
	if current >= 0 {
		job.ProgressCurrent = current
	}
	if total >= 0 {
		job.ProgressTotal = total
	}
	if stage != "" {
		job.CurrentStage = trimText(stage, 120)
	}
	if message != "" {
		job.Message = trimText(message, 500)
	}
	_ = uc.jobRepo.Update(ctx, job)
}

func (uc *UseCase) finishMQJob(ctx context.Context, job *MQJob, status, message string, result any, err error, current, total int64) {
	if uc.jobRepo == nil || job == nil {
		return
	}
	now := time.Now()
	if strings.TrimSpace(status) == "" {
		status = JobStatusSuccess
	}
	if current >= 0 {
		job.ProgressCurrent = current
	}
	if total >= 0 {
		job.ProgressTotal = total
	}
	if status == JobStatusSuccess && job.ProgressTotal > 0 && job.ProgressCurrent < job.ProgressTotal {
		job.ProgressCurrent = job.ProgressTotal
	}
	job.Status = status
	job.Message = trimText(message, 500)
	job.FinishedAt = &now
	if job.StartedAt != nil {
		job.DurationMs = now.Sub(*job.StartedAt).Milliseconds()
	}
	if result != nil {
		job.ResultJSON = mustJSON(result)
	}
	if err != nil {
		job.ErrorJSON = mustJSON(map[string]string{"message": err.Error()})
		if job.Message == "" {
			job.Message = trimText(err.Error(), 500)
		}
	}
	_ = uc.jobRepo.Update(ctx, job)
}

func newMQCorrelationID() string {
	var token [4]byte
	if _, err := rand.Read(token[:]); err == nil {
		return fmt.Sprintf("mq-%d-%x", time.Now().UnixNano(), token[:])
	}
	return fmt.Sprintf("mq-%d", time.Now().UnixNano())
}

func normalizeJobListRequest(req *JobListRequest) {
	if req == nil {
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 500 {
		req.PageSize = 500
	}
	req.Keyword = strings.TrimSpace(req.Keyword)
	req.MQType = NormalizeType(req.MQType)
	req.JobType = strings.TrimSpace(req.JobType)
	req.Status = strings.TrimSpace(req.Status)
	req.StartTime = strings.TrimSpace(req.StartTime)
	req.EndTime = strings.TrimSpace(req.EndTime)
}

func toJobVO(item *MQJob) *JobVO {
	if item == nil {
		return nil
	}
	percent := 0
	if item.ProgressTotal > 0 {
		percent = int(item.ProgressCurrent * 100 / item.ProgressTotal)
		if percent > 100 {
			percent = 100
		}
	}
	return &JobVO{
		ID:              item.ID,
		InstanceID:      item.InstanceID,
		InstanceName:    item.InstanceName,
		MQType:          item.MQType,
		MQTypeText:      TypeText(item.MQType),
		JobType:         item.JobType,
		JobTypeText:     jobTypeText(item.JobType),
		Status:          item.Status,
		StatusText:      jobStatusText(item.Status),
		ProgressCurrent: item.ProgressCurrent,
		ProgressTotal:   item.ProgressTotal,
		ProgressPercent: percent,
		CurrentStage:    item.CurrentStage,
		TriggerType:     item.TriggerType,
		OperatorID:      item.OperatorID,
		OperatorName:    item.OperatorName,
		StartedAt:       formatTimePtr(item.StartedAt),
		FinishedAt:      formatTimePtr(item.FinishedAt),
		DurationMs:      item.DurationMs,
		Message:         item.Message,
		ErrorJSON:       item.ErrorJSON,
		ResultJSON:      item.ResultJSON,
		CorrelationID:   item.CorrelationID,
		CreatedAt:       item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:       item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func jobTypeText(jobType string) string {
	switch jobType {
	case JobTypeSync:
		return "元数据同步"
	case JobTypeMetricCollect:
		return "指标采集"
	case JobTypeInspection:
		return "巡检"
	case JobTypeOperationRefresh:
		return "操作后刷新"
	case JobTypeExport:
		return "导出"
	default:
		return jobType
	}
}

func jobStatusText(status string) string {
	switch status {
	case JobStatusPending:
		return "待执行"
	case JobStatusRunning:
		return "执行中"
	case JobStatusSuccess:
		return "成功"
	case JobStatusFailed:
		return "失败"
	case JobStatusPartial:
		return "部分成功"
	case JobStatusCancel:
		return "已取消"
	case JobStatusTimeout:
		return "超时"
	default:
		return status
	}
}
