package asset

import (
	"testing"
	"time"
)

func TestNormalizeVirtualizationAutoSyncOptions(t *testing.T) {
	opts := normalizeVirtualizationAutoSyncOptions(VirtualizationAutoSyncOptions{
		Enabled:      true,
		InitialDelay: -1,
	})

	if !opts.Enabled {
		t.Fatal("expected auto sync to stay enabled")
	}
	if opts.Interval != defaultVirtualizationAutoSyncInterval {
		t.Fatalf("expected default interval %s, got %s", defaultVirtualizationAutoSyncInterval, opts.Interval)
	}
	if opts.InitialDelay != defaultVirtualizationAutoSyncInitialDelay {
		t.Fatalf("expected default initial delay %s, got %s", defaultVirtualizationAutoSyncInitialDelay, opts.InitialDelay)
	}
	if opts.PlatformTimeout != defaultVirtualizationAutoSyncPlatformTimeout {
		t.Fatalf("expected default platform timeout %s, got %s", defaultVirtualizationAutoSyncPlatformTimeout, opts.PlatformTimeout)
	}
	if opts.Concurrency != defaultVirtualizationAutoSyncConcurrency {
		t.Fatalf("expected default concurrency %d, got %d", defaultVirtualizationAutoSyncConcurrency, opts.Concurrency)
	}
}

func TestVirtualizationPlatformSyncLockTryMode(t *testing.T) {
	uc := &VirtualizationUseCase{}

	unlock, ok := uc.lockPlatformSync(1, true)
	if !ok {
		t.Fatal("expected blocking lock acquisition to succeed")
	}

	if unlockTry, ok := uc.lockPlatformSync(1, false); ok {
		unlockTry()
		t.Fatal("expected try lock to fail while platform sync is locked")
	}

	unlock()

	unlockTry, ok := uc.lockPlatformSync(1, false)
	if !ok {
		t.Fatal("expected try lock to succeed after platform sync lock is released")
	}
	unlockTry()
}

func TestNormalizeVirtualizationAutoSyncOptionsKeepsExplicitValues(t *testing.T) {
	opts := normalizeVirtualizationAutoSyncOptions(VirtualizationAutoSyncOptions{
		Enabled:         true,
		Interval:        2 * time.Minute,
		InitialDelay:    0,
		PlatformTimeout: 30 * time.Second,
		Concurrency:     8,
	})

	if opts.Interval != 2*time.Minute {
		t.Fatalf("expected explicit interval to be preserved, got %s", opts.Interval)
	}
	if opts.InitialDelay != 0 {
		t.Fatalf("expected explicit zero initial delay to be preserved, got %s", opts.InitialDelay)
	}
	if opts.PlatformTimeout != 30*time.Second {
		t.Fatalf("expected explicit timeout to be preserved, got %s", opts.PlatformTimeout)
	}
	if opts.Concurrency != 8 {
		t.Fatalf("expected explicit concurrency to be preserved, got %d", opts.Concurrency)
	}
}
