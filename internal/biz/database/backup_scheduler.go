package database

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	appLogger "github.com/ydcloud-dy/opshub/pkg/logger"
	"go.uber.org/zap"
)

var backupCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

type BackupSchedulerNotice struct {
	Task      *DatabaseBackupTask
	Instance  *DatabaseInstance
	Operation string
	Message   string
	Error     error
}

type BackupSchedulerOptions struct {
	Interval          time.Duration
	CleanupInterval   time.Duration
	MaxConcurrentRuns int
	NotifyFailure     func(ctx context.Context, notice *BackupSchedulerNotice) error
	Now               func() time.Time
}

type BackupScheduler struct {
	useCase         *UseCase
	interval        time.Duration
	cleanupInterval time.Duration
	runSlots        chan struct{}
	notifyFailure   func(ctx context.Context, notice *BackupSchedulerNotice) error
	now             func() time.Time

	mu            sync.Mutex
	cancel        context.CancelFunc
	done          chan struct{}
	started       bool
	lastCleanupAt time.Time
}

func NewBackupScheduler(useCase *UseCase, options BackupSchedulerOptions) *BackupScheduler {
	interval := options.Interval
	if interval <= 0 {
		interval = time.Minute
	}
	cleanupInterval := options.CleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = 6 * time.Hour
	}
	maxConcurrentRuns := options.MaxConcurrentRuns
	if maxConcurrentRuns <= 0 {
		maxConcurrentRuns = 2
	}
	nowFn := options.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	return &BackupScheduler{
		useCase:         useCase,
		interval:        interval,
		cleanupInterval: cleanupInterval,
		runSlots:        make(chan struct{}, maxConcurrentRuns),
		notifyFailure:   options.NotifyFailure,
		now:             nowFn,
	}
}

func (s *BackupScheduler) Start(ctx context.Context) {
	if s == nil || s.useCase == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return
	}

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	s.cancel = cancel
	s.done = done
	s.started = true

	go s.loop(runCtx, done)
}

func (s *BackupScheduler) Stop(ctx context.Context) error {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	cancel := s.cancel
	done := s.done
	s.started = false
	s.cancel = nil
	s.done = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if done == nil {
		return nil
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("备份调度器停止超时: %w", ctx.Err())
	}
}

func (s *BackupScheduler) loop(ctx context.Context, done chan struct{}) {
	defer close(done)

	s.runOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *BackupScheduler) runOnce(ctx context.Context) {
	tasks, err := s.useCase.ListEnabledBackupTasks(ctx)
	if err != nil {
		appLogger.Error("读取数据库备份调度任务失败", zap.Error(err))
		return
	}

	now := s.now()
	for _, task := range tasks {
		due, nextRun, scheduleErr := shouldTriggerBackupTask(task, now)
		if scheduleErr != nil {
			appLogger.Warn("数据库备份任务 Cron 表达式无效",
				zap.Uint("taskID", task.ID),
				zap.String("taskName", task.Name),
				zap.String("schedule", task.Schedule),
				zap.Error(scheduleErr),
			)
			continue
		}
		s.persistTaskNextRun(ctx, task, nextRun)
		if !due {
			continue
		}
		go s.executeScheduledTask(ctx, task.ID, nextRun)
	}

	if s.shouldRunCleanup(now) {
		s.runCleanupSweep(ctx)
	}
}

func (s *BackupScheduler) shouldRunCleanup(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastCleanupAt.IsZero() || now.Sub(s.lastCleanupAt) >= s.cleanupInterval {
		s.lastCleanupAt = now
		return true
	}
	return false
}

func (s *BackupScheduler) executeScheduledTask(ctx context.Context, taskID uint, nextRun time.Time) {
	if !s.tryAcquireRunSlot() {
		appLogger.Warn("数据库定时备份跳过，调度器全局并发已满",
			zap.Uint("taskID", taskID),
			zap.Time("scheduledAt", nextRun),
		)
		return
	}
	defer s.releaseRunSlot()

	result, err := s.useCase.RunScheduledBackupTask(ctx, taskID)
	if err != nil {
		if isBackupTaskRunningError(err) {
			appLogger.Info("数据库定时备份跳过，任务仍在执行中",
				zap.Uint("taskID", taskID),
				zap.Time("scheduledAt", nextRun),
			)
			return
		}
		task, instance := s.loadTaskContext(ctx, taskID)
		appLogger.Error("数据库定时备份执行失败",
			zap.Uint("taskID", taskID),
			zap.Time("scheduledAt", nextRun),
			zap.Error(err),
		)
		s.notifySchedulerFailure(ctx, &BackupSchedulerNotice{
			Task:      task,
			Instance:  instance,
			Operation: "schedule_run",
			Message:   "数据库定时备份执行失败",
			Error:     err,
		})
		return
	}

	appLogger.Info("数据库定时备份执行完成",
		zap.Uint("taskID", taskID),
		zap.Uint("recordID", result.RecordID),
		zap.String("status", result.Status),
		zap.String("message", result.Message),
	)
}

func (s *BackupScheduler) tryAcquireRunSlot() bool {
	if s == nil || s.runSlots == nil {
		return true
	}
	select {
	case s.runSlots <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *BackupScheduler) releaseRunSlot() {
	if s == nil || s.runSlots == nil {
		return
	}
	select {
	case <-s.runSlots:
	default:
	}
}

func (s *BackupScheduler) persistTaskNextRun(ctx context.Context, task *DatabaseBackupTask, nextRun time.Time) {
	if s == nil || s.useCase == nil || s.useCase.backupTaskRepo == nil || task == nil || task.ID == 0 || nextRun.IsZero() {
		return
	}
	if task.NextRunAt != nil && task.NextRunAt.Equal(nextRun) {
		return
	}
	task.NextRunAt = &nextRun
	_ = s.useCase.backupTaskRepo.Update(ctx, task)
}

func (s *BackupScheduler) runCleanupSweep(ctx context.Context) {
	tasks, err := s.useCase.ListAllBackupTasks(ctx)
	if err != nil {
		appLogger.Error("读取数据库备份清理任务失败", zap.Error(err))
		return
	}

	totalCleaned := 0
	for _, task := range tasks {
		cleanedCount, cleanupErr := s.useCase.cleanupExpiredBackupFilesByTask(ctx, task)
		totalCleaned += cleanedCount
		if cleanupErr != nil {
			instance, _ := s.loadInstance(ctx, task.InstanceID)
			appLogger.Error("数据库备份保留策略清理失败",
				zap.Uint("taskID", task.ID),
				zap.String("taskName", task.Name),
				zap.Error(cleanupErr),
			)
			s.notifySchedulerFailure(ctx, &BackupSchedulerNotice{
				Task:      task,
				Instance:  instance,
				Operation: "retention_cleanup",
				Message:   "数据库备份保留策略清理失败",
				Error:     cleanupErr,
			})
		}
	}

	if totalCleaned > 0 {
		appLogger.Info("数据库备份保留策略清理完成", zap.Int("cleanedFiles", totalCleaned))
	}
}

func (s *BackupScheduler) notifySchedulerFailure(ctx context.Context, notice *BackupSchedulerNotice) {
	if s.notifyFailure == nil || notice == nil {
		return
	}
	if err := s.notifyFailure(ctx, notice); err != nil {
		appLogger.Warn("发送数据库备份失败通知失败",
			zap.String("operation", notice.Operation),
			zap.Error(err),
		)
	}
}

func (s *BackupScheduler) loadTaskContext(ctx context.Context, taskID uint) (*DatabaseBackupTask, *DatabaseInstance) {
	task, _ := s.useCase.getBackupTask(ctx, taskID)
	if task == nil {
		return nil, nil
	}
	instance, _ := s.loadInstance(ctx, task.InstanceID)
	return task, instance
}

func (s *BackupScheduler) loadInstance(ctx context.Context, instanceID uint) (*DatabaseInstance, error) {
	if s.useCase == nil || s.useCase.instanceRepo == nil || instanceID == 0 {
		return nil, nil
	}
	return s.useCase.instanceRepo.GetByID(ctx, instanceID)
}

func parseBackupSchedule(schedule string) (cron.Schedule, error) {
	schedule = strings.TrimSpace(schedule)
	if schedule == "" {
		return nil, nil
	}
	return backupCronParser.Parse(schedule)
}

func nextBackupRunAt(task *DatabaseBackupTask) (time.Time, error) {
	if task == nil {
		return time.Time{}, nil
	}
	schedule, err := parseBackupSchedule(task.Schedule)
	if err != nil || schedule == nil {
		return time.Time{}, err
	}
	baseTime := task.CreatedAt
	if task.LastRunAt != nil && !task.LastRunAt.IsZero() {
		baseTime = *task.LastRunAt
	}
	return schedule.Next(baseTime), nil
}

func shouldTriggerBackupTask(task *DatabaseBackupTask, now time.Time) (bool, time.Time, error) {
	nextRun, err := nextBackupRunAt(task)
	if err != nil || nextRun.IsZero() {
		return false, nextRun, err
	}
	return !now.Before(nextRun), nextRun, nil
}
