package database

import (
	"context"
	"fmt"
	"sync"
	"time"

	appLogger "github.com/ydcloud-dy/opshub/pkg/logger"
	"go.uber.org/zap"
)

type BackupAlertSchedulerOptions struct {
	Interval time.Duration
	Now      func() time.Time
}

type BackupAlertScheduler struct {
	useCase  *UseCase
	interval time.Duration
	now      func() time.Time

	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	started bool
}

func NewBackupAlertScheduler(useCase *UseCase, options BackupAlertSchedulerOptions) *BackupAlertScheduler {
	interval := options.Interval
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	nowFn := options.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	return &BackupAlertScheduler{useCase: useCase, interval: interval, now: nowFn}
}

func (s *BackupAlertScheduler) Start(ctx context.Context) {
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

func (s *BackupAlertScheduler) Stop(ctx context.Context) error {
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
	s.cancel = nil
	s.done = nil
	s.started = false
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
		return fmt.Errorf("数据库备份告警调度器停止超时: %w", ctx.Err())
	}
}

func (s *BackupAlertScheduler) loop(ctx context.Context, done chan struct{}) {
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

func (s *BackupAlertScheduler) runOnce(ctx context.Context) {
	if err := s.useCase.RunBackupAlertScan(ctx); err != nil {
		appLogger.Warn("数据库备份告警扫描失败", zap.Error(err))
	}
}
