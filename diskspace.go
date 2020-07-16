// Copyright 2020 Matthew Holt

package diskspace

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Maintainer keeps disk space utilization under control.
type Maintainer struct {
	// The volume to maintain. Default: "/"
	Volume string

	// How often to check disk space usage.
	// Default: 10m
	CheckInterval time.Duration

	// The ratio of used/total space before
	// disk cleaning. Default: 0.9
	Threshold float64

	// The function that will be called to
	// clean up disk space.
	Clean func() error

	// Custom logger.
	Logger *zap.Logger

	mu sync.Mutex
}

// Maintain maintains disk space. It checks the disk usage
// for m.Volume every m.CheckInterval, and runs m.Clean if
// the disk usage is above m.Threshold. If m.Clean is nil,
// this function panics. Otherwise, it blocks indefinitely
// until ctx is cancelled.
func (m *Maintainer) Maintain(ctx context.Context) {
	if m.Clean == nil {
		panic("nil Clean function")
	}
	if m.Volume == "" {
		m.Volume = defaultVolume
	}
	if m.Threshold <= 0 || m.Threshold >= 1 {
		m.Threshold = defaultThreshold
	}
	if m.CheckInterval <= 0 {
		m.CheckInterval = defaultCheckInterval
	}
	if m.Logger == nil {
		m.Logger = zap.NewNop()
	}

	m.Logger.Info("starting disk usage maintenance goroutine",
		zap.String("volume", m.Volume),
		zap.Float64("threshold", m.Threshold),
		zap.Duration("interval", m.CheckInterval))

	// initial maintenance
	err := m.maintainDiskUsage()
	if err != nil {
		m.Logger.Error("checking disk space", zap.Error(err))
	}

	// start maintenance ticker
	ticker := time.NewTicker(m.CheckInterval)

	// maintain until context is canceled
	for {
		select {
		case <-ticker.C:
			err := m.maintainDiskUsage()
			if err != nil {
				m.Logger.Error("checking disk space", zap.Error(err))
				continue
			}
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func (m *Maintainer) maintainDiskUsage() error {
	// don't allow maintenance ops to overlap
	m.mu.Lock()
	defer m.mu.Unlock()

	du, err := diskUsage(m.Volume)
	if err != nil {
		return err
	}
	totalMB := du.all / MB
	usedMB := du.used / MB
	usedRatio := float64(usedMB) / float64(totalMB)

	// nothing to do if disk is not nearly full
	if usedRatio < m.Threshold {
		return nil
	}

	m.Logger.Warn("disk space usage above threshold",
		zap.Uint64("total_mb", totalMB),
		zap.Uint64("used_mb", usedMB),
		zap.Float64("used_ratio", usedRatio),
		zap.Float64("used_threshold", m.Threshold))

	// run cleaner function
	err = m.Clean()
	if err != nil {
		return fmt.Errorf("clean: %v", err)
	}

	// see how much space is now available
	du, err = diskUsage(m.Volume)
	if err != nil {
		return err
	}
	newUsedMB := du.used / MB
	usedDiff := usedMB - newUsedMB

	m.Logger.Info("disk space cleaned",
		zap.Uint64("used_mb", newUsedMB),
		zap.Uint64("freed_mb", usedDiff))

	return nil
}

const (
	defaultVolume        = "/"
	defaultThreshold     = 0.9
	defaultCheckInterval = 10 * time.Minute
)

// Disk size constants.
const (
	_  = iota
	KB = 1 << (10 * iota)
	MB
	GB
	TB
	PB
	EB
	ZB
	YB
)
