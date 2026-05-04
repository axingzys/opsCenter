package database

import (
	"context"
	"fmt"
	"sync"
	"time"

	appLogger "github.com/ydcloud-dy/opshub/pkg/logger"
	"go.uber.org/zap"
)

type ReplicationCheckSchedulerOptions struct {
	Interval       time.Duration
	StaleAfter     time.Duration
	InitialDelay   time.Duration
	MaxConcurrency int
}

type ReplicationCheckScheduler struct {
	useCase        *UseCase
	interval       time.Duration
	staleAfter     time.Duration
	initialDelay   time.Duration
	maxConcurrency int

	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	started bool
}

func NewReplicationCheckScheduler(useCase *UseCase, options ReplicationCheckSchedulerOptions) *ReplicationCheckScheduler {
	interval := options.Interval
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	staleAfter := options.StaleAfter
	if staleAfter <= 0 {
		staleAfter = 5 * time.Minute
	}
	maxConcurrency := options.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = 2
	}
	if maxConcurrency > 5 {
		maxConcurrency = 5
	}
	return &ReplicationCheckScheduler{
		useCase:        useCase,
		interval:       interval,
		staleAfter:     staleAfter,
		initialDelay:   options.InitialDelay,
		maxConcurrency: maxConcurrency,
	}
}

func (s *ReplicationCheckScheduler) Start(ctx context.Context) {
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

func (s *ReplicationCheckScheduler) Stop(ctx context.Context) error {
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
		return fmt.Errorf("数据库副本状态调度器停止超时: %w", ctx.Err())
	}
}

func (s *ReplicationCheckScheduler) loop(ctx context.Context, done chan struct{}) {
	defer close(done)
	if s.initialDelay > 0 {
		timer := time.NewTimer(s.initialDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
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

func (s *ReplicationCheckScheduler) runOnce(ctx context.Context) {
	result, err := s.useCase.RunReplicationCheckBatch(ctx, &DatabaseReplicationCheckBatchRequest{
		OnlyStale:      true,
		StaleSeconds:   int(s.staleAfter.Seconds()),
		IncludeRelated: true,
		MaxConcurrency: s.maxConcurrency,
		TriggerSource:  DatabaseReplicationCheckTriggerScheduler,
	}, scheduledReplicationCheckOperator())
	if err != nil {
		appLogger.Warn("数据库副本状态自动采集失败", zap.Error(err))
		return
	}
	if result == nil {
		return
	}
	appLogger.Info(
		"数据库副本状态自动采集完成",
		zap.Int("success", result.Success),
		zap.Int("failed", result.Failed),
		zap.Int("skipped", result.Skipped),
	)
}

func scheduledReplicationCheckOperator() QueryOperator {
	return QueryOperator{
		ID:       0,
		Username: "system-replication-scheduler",
		ClientIP: "system",
	}
}
