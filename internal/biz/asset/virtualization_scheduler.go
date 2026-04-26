package asset

import (
	"context"
	"errors"
	"time"

	appLogger "github.com/ydcloud-dy/opshub/pkg/logger"
	"go.uber.org/zap"
)

var ErrVirtualizationSyncAlreadyRunning = errors.New("虚拟化平台同步任务正在执行")

const (
	defaultVirtualizationAutoSyncInterval        = time.Minute
	defaultVirtualizationAutoSyncInitialDelay    = 15 * time.Second
	defaultVirtualizationAutoSyncPlatformTimeout = 5 * time.Minute
	defaultVirtualizationAutoSyncConcurrency     = 4
)

type VirtualizationAutoSyncOptions struct {
	Enabled         bool
	Interval        time.Duration
	InitialDelay    time.Duration
	PlatformTimeout time.Duration
	Concurrency     int
}

type VirtualizationAutoSyncScheduler struct {
	uc     *VirtualizationUseCase
	opts   VirtualizationAutoSyncOptions
	cancel context.CancelFunc
	done   chan struct{}
}

func (uc *VirtualizationUseCase) StartAutoSyncScheduler(ctx context.Context, opts VirtualizationAutoSyncOptions) *VirtualizationAutoSyncScheduler {
	opts = normalizeVirtualizationAutoSyncOptions(opts)
	if !opts.Enabled {
		appLogger.Info("虚拟化平台自动同步未启用")
		return nil
	}

	schedulerCtx, cancel := context.WithCancel(ctx)
	scheduler := &VirtualizationAutoSyncScheduler{
		uc:     uc,
		opts:   opts,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	go scheduler.run(schedulerCtx)

	appLogger.Info("虚拟化平台自动同步已启动",
		zap.Duration("interval", opts.Interval),
		zap.Duration("initialDelay", opts.InitialDelay),
		zap.Duration("platformTimeout", opts.PlatformTimeout),
		zap.Int("concurrency", opts.Concurrency),
	)
	return scheduler
}

func (s *VirtualizationAutoSyncScheduler) Stop(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.cancel()
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *VirtualizationAutoSyncScheduler) run(ctx context.Context) {
	defer close(s.done)

	if s.opts.InitialDelay > 0 {
		timer := time.NewTimer(s.opts.InitialDelay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return
		}
	}

	s.syncEnabledPlatforms(ctx)

	ticker := time.NewTicker(s.opts.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.syncEnabledPlatforms(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (s *VirtualizationAutoSyncScheduler) syncEnabledPlatforms(ctx context.Context) {
	platforms, err := s.uc.platformRepo.ListEnabled(ctx)
	if err != nil {
		appLogger.Error("虚拟化平台自动同步扫描失败", zap.Error(err))
		return
	}
	if len(platforms) == 0 {
		return
	}

	sem := make(chan struct{}, s.opts.Concurrency)
	done := make(chan struct{}, len(platforms))

	for _, platform := range platforms {
		platformID := platform.ID
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return
		}

		go func() {
			defer func() {
				<-sem
				done <- struct{}{}
			}()
			s.syncPlatform(ctx, platformID)
		}()
	}

	for range platforms {
		select {
		case <-done:
		case <-ctx.Done():
			return
		}
	}
}

func (s *VirtualizationAutoSyncScheduler) syncPlatform(ctx context.Context, platformID uint) {
	syncCtx, cancel := context.WithTimeout(ctx, s.opts.PlatformTimeout)
	defer cancel()

	if _, err := s.uc.TriggerScheduledPlatformSync(syncCtx, platformID); err != nil {
		if errors.Is(err, ErrVirtualizationSyncAlreadyRunning) {
			appLogger.Warn("虚拟化平台自动同步跳过，已有同步任务在执行", zap.Uint("platformID", platformID))
			return
		}
		appLogger.Error("虚拟化平台自动同步失败", zap.Uint("platformID", platformID), zap.Error(err))
	}
}

func normalizeVirtualizationAutoSyncOptions(opts VirtualizationAutoSyncOptions) VirtualizationAutoSyncOptions {
	if opts.Interval <= 0 {
		opts.Interval = defaultVirtualizationAutoSyncInterval
	}
	if opts.InitialDelay < 0 {
		opts.InitialDelay = defaultVirtualizationAutoSyncInitialDelay
	}
	if opts.PlatformTimeout <= 0 {
		opts.PlatformTimeout = defaultVirtualizationAutoSyncPlatformTimeout
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = defaultVirtualizationAutoSyncConcurrency
	}
	return opts
}
