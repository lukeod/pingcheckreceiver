// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pingcheckreceiver

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/lukeod/pingcheckreceiver/internal/metadata"
)

type pingScraper struct {
	cfg      *Config
	settings receiver.Settings
	logger   *zap.Logger
	mb       *metadata.MetricsBuilder
	pingers  map[string]*probing.Pinger
	mu       sync.RWMutex
}

func newScraper(cfg *Config, settings receiver.Settings) *pingScraper {
	return &pingScraper{
		cfg:      cfg,
		settings: settings,
		logger:   settings.Logger,
		pingers:  make(map[string]*probing.Pinger),
	}
}

// start initializes resources
func (s *pingScraper) start(ctx context.Context, host component.Host) error {
	s.mb = metadata.NewMetricsBuilder(s.cfg.MetricsBuilderConfig, s.settings)

	// Initialize pingers for all targets
	for _, target := range s.cfg.Targets {
		pinger, err := probing.NewPinger(target.Endpoint)
		if err != nil {
			s.logger.Error("Failed to create pinger",
				zap.String("endpoint", target.Endpoint),
				zap.Error(err))
			continue // Skip this target but don't fail startup
		}

		// Apply default values if not set
		if target.Count == 0 {
			target.Count = 4
		}
		if target.Timeout == 0 {
			target.Timeout = 5 * time.Second
		}
		if target.Interval == 0 {
			target.Interval = time.Second
		}

		// Configure pinger
		pinger.Count = target.Count
		pinger.Timeout = target.Timeout
		pinger.Interval = target.Interval

		// Platform-specific privilege configuration
		if runtime.GOOS == "windows" {
			// Windows requires privileged mode
			pinger.SetPrivileged(true)
			s.logger.Debug("Windows detected, using privileged mode",
				zap.String("endpoint", target.Endpoint))
		} else {
			pinger.SetPrivileged(s.cfg.Privileged)
		}

		// Prevent memory growth for long-running operations
		pinger.RecordRtts = false

		// Set callbacks for debugging
		pinger.OnRecv = func(pkt *probing.Packet) {
			s.logger.Debug("Received packet",
				zap.String("endpoint", target.Endpoint),
				zap.Int("seq", pkt.Seq),
				zap.Duration("rtt", pkt.Rtt))
		}

		s.mu.Lock()
		s.pingers[target.Endpoint] = pinger
		s.mu.Unlock()
	}

	if len(s.pingers) == 0 {
		return fmt.Errorf("no valid pingers could be created")
	}

	return nil
}

// shutdown cleans up resources
func (s *pingScraper) shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for endpoint, pinger := range s.pingers {
		pinger.Stop()
		s.logger.Debug("Stopped pinger", zap.String("endpoint", endpoint))
	}
	s.pingers = nil

	return nil
}

// scrape performs ping checks for all targets
func (s *pingScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, len(s.cfg.Targets))

	wg.Add(len(s.cfg.Targets))
	for _, target := range s.cfg.Targets {
		go func(t Target) {
			defer wg.Done()

			if err := s.pingTarget(ctx, t, &mu); err != nil {
				select {
				case errChan <- fmt.Errorf("target %s: %w", t.Endpoint, err):
				case <-ctx.Done():
				}
			}
		}(target)
	}

	// Wait for all pings to complete
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Collect errors
	var errs error
	for err := range errChan {
		errs = multierr.Append(errs, err)
		s.logger.Warn("Ping failed", zap.Error(err))
	}

	return s.mb.Emit(), errs
}

func (s *pingScraper) pingTarget(ctx context.Context, target Target, mu *sync.Mutex) error {
	s.mu.RLock()
	pinger, ok := s.pingers[target.Endpoint]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("pinger not found for target: %s", target.Endpoint)
	}

	// Run ping with native context support (pro-bing v0.7.0+)
	err := pinger.RunWithContext(ctx)
	if err != nil {
		// Record error metrics if enabled
		if s.cfg.Metrics.PingErrors.Enabled {
			now := pcommon.NewTimestampFromTime(time.Now())
			mu.Lock()
			s.mb.RecordPingErrorsDataPoint(
				now,
				1,
				target.Endpoint,
				"", // IP will be empty on error
				categorizeError(err),
			)
			mu.Unlock()
		}
		return fmt.Errorf("ping failed: %w", err)
	}

	stats := pinger.Statistics()
	now := pcommon.NewTimestampFromTime(time.Now())

	// Record metrics with lock
	mu.Lock()
	defer mu.Unlock()

	// Note: stats.Rtts will be empty since RecordRtts=false
	// Recording only aggregate metrics which are always available

	// Record individual RTT metrics if available (for future when RecordRtts might be enabled)
	if s.cfg.Metrics.PingDuration.Enabled && len(stats.Rtts) > 0 {
		for _, rtt := range stats.Rtts {
			s.mb.RecordPingDurationDataPoint(
				now,
				float64(rtt.Milliseconds()),
				target.Endpoint,
				stats.IPAddr.String(),
			)
		}
	}

	// Record aggregate metrics
	if stats.MinRtt > 0 && s.cfg.Metrics.PingDurationMin.Enabled {
		s.mb.RecordPingDurationMinDataPoint(
			now,
			float64(stats.MinRtt.Milliseconds()),
			target.Endpoint,
			stats.IPAddr.String(),
		)
	}

	if stats.MaxRtt > 0 && s.cfg.Metrics.PingDurationMax.Enabled {
		s.mb.RecordPingDurationMaxDataPoint(
			now,
			float64(stats.MaxRtt.Milliseconds()),
			target.Endpoint,
			stats.IPAddr.String(),
		)
	}

	if stats.AvgRtt > 0 && s.cfg.Metrics.PingDurationAvg.Enabled {
		s.mb.RecordPingDurationAvgDataPoint(
			now,
			float64(stats.AvgRtt.Milliseconds()),
			target.Endpoint,
			stats.IPAddr.String(),
		)
	}

	if stats.StdDevRtt > 0 && s.cfg.Metrics.PingDurationStddev.Enabled {
		s.mb.RecordPingDurationStddevDataPoint(
			now,
			float64(stats.StdDevRtt.Milliseconds()),
			target.Endpoint,
			stats.IPAddr.String(),
		)
	}

	// Record packet loss as ratio (0.0 to 1.0)
	if s.cfg.Metrics.PingPacketLoss.Enabled {
		s.mb.RecordPingPacketLossDataPoint(
			now,
			stats.PacketLoss/100.0,
			target.Endpoint,
			stats.IPAddr.String(),
		)
	}

	// Record packet counts
	if s.cfg.Metrics.PingPacketsSent.Enabled {
		s.mb.RecordPingPacketsSentDataPoint(
			now,
			int64(stats.PacketsSent),
			target.Endpoint,
			stats.IPAddr.String(),
		)
	}

	if s.cfg.Metrics.PingPacketsReceived.Enabled {
		s.mb.RecordPingPacketsReceivedDataPoint(
			now,
			int64(stats.PacketsRecv),
			target.Endpoint,
			stats.IPAddr.String(),
		)
	}

	return nil
}

// categorizeError categorizes errors for metrics
func categorizeError(err error) metadata.AttributeErrorType {
	if err == nil {
		return metadata.AttributeErrorTypeUnknown
	}

	errMsg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errMsg, "timeout"):
		return metadata.AttributeErrorTypeTimeout
	case strings.Contains(errMsg, "no such host"):
		return metadata.AttributeErrorTypeDNSFailure
	case strings.Contains(errMsg, "network is unreachable"):
		return metadata.AttributeErrorTypeNetworkUnreachable
	case strings.Contains(errMsg, "permission denied") || strings.Contains(errMsg, "operation not permitted"):
		return metadata.AttributeErrorTypePermissionDenied
	default:
		return metadata.AttributeErrorTypeUnknown
	}
}
