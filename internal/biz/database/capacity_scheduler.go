package database

import (
	"context"
	"fmt"
	"sync"
	"time"

	appLogger "github.com/ydcloud-dy/opshub/pkg/logger"
	"go.uber.org/zap"
)

type CapacitySchedulerOptions struct {
	Interval time.Duration
}

type CapacityScheduler struct {
	useCase  *UseCase
	interval time.Duration

	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	started bool
}

func NewCapacityScheduler(useCase *UseCase, options CapacitySchedulerOptions) *CapacityScheduler {
	interval := options.Interval
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	return &CapacityScheduler{
		useCase:  useCase,
		interval: interval,
	}
}

func (s *CapacityScheduler) Start(ctx context.Context) {
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

func (s *CapacityScheduler) Stop(ctx context.Context) error {
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
		return fmt.Errorf("容量采样调度器停止超时: %w", ctx.Err())
	}
}

func (s *CapacityScheduler) loop(ctx context.Context, done chan struct{}) {
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

func (s *CapacityScheduler) runOnce(ctx context.Context) {
	count, err := s.useCase.CollectCapacitySnapshots(ctx)
	if err != nil {
		appLogger.Warn("数据库容量采样存在失败项", zap.Int("successCount", count), zap.Error(err))
		return
	}
	appLogger.Info("数据库容量采样完成", zap.Int("successCount", count))
}
