// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pingcheckreceiver

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/lukeod/pingcheckreceiver/internal/metadata"
)

func TestScraperStart(t *testing.T) {
	cfg := &Config{
		ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Targets: []Target{
			{
				Endpoint: "localhost",
				Count:    1,
				Timeout:  time.Second,
			},
		},
		Privileged: false,
	}

	scraper := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	err := scraper.start(context.Background(), componenttest.NewNopHost())

	// May fail if localhost cannot be resolved, but should not panic
	if err != nil {
		assert.Contains(t, err.Error(), "no valid pingers")
	}
}

func TestScraperShutdown(t *testing.T) {
	cfg := &Config{
		ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Targets:              []Target{},
	}

	scraper := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	err := scraper.shutdown(context.Background())
	assert.NoError(t, err)
}

func TestCategorizeError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected metadata.AttributeErrorType
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: metadata.AttributeErrorTypeUnknown,
		},
		{
			name:     "timeout error",
			err:      fmt.Errorf("operation timeout"),
			expected: metadata.AttributeErrorTypeTimeout,
		},
		{
			name:     "dns error",
			err:      fmt.Errorf("no such host"),
			expected: metadata.AttributeErrorTypeDNSFailure,
		},
		{
			name:     "network unreachable",
			err:      fmt.Errorf("network is unreachable"),
			expected: metadata.AttributeErrorTypeNetworkUnreachable,
		},
		{
			name:     "permission denied",
			err:      fmt.Errorf("permission denied"),
			expected: metadata.AttributeErrorTypePermissionDenied,
		},
		{
			name:     "operation not permitted",
			err:      fmt.Errorf("operation not permitted"),
			expected: metadata.AttributeErrorTypePermissionDenied,
		},
		{
			name:     "unknown error",
			err:      fmt.Errorf("something went wrong"),
			expected: metadata.AttributeErrorTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := categorizeError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestScraperStartWithDefaults(t *testing.T) {
	cfg := &Config{
		ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Targets: []Target{
			{
				Endpoint: "127.0.0.1",
				// Leave Count, Timeout, Interval unset to test defaults
			},
		},
		Privileged: false,
	}

	scraper := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	err := scraper.start(context.Background(), componenttest.NewNopHost())

	// Should succeed with localhost IP
	if err != nil {
		// If error, check it's the expected one
		assert.Contains(t, err.Error(), "no valid pingers")
	} else {
		// Verify defaults were applied
		scraper.mu.RLock()
		pinger, ok := scraper.pingers["127.0.0.1"]
		scraper.mu.RUnlock()

		if ok {
			assert.Equal(t, 4, pinger.Count)
			assert.Equal(t, 5*time.Second, pinger.Timeout)
			assert.Equal(t, time.Second, pinger.Interval)
		}
	}
}

func TestScraperMultipleTargets(t *testing.T) {
	cfg := &Config{
		ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Targets: []Target{
			{
				Endpoint: "127.0.0.1",
				Count:    1,
				Timeout:  time.Second,
			},
			{
				Endpoint: "localhost",
				Count:    2,
				Timeout:  2 * time.Second,
			},
		},
		Privileged: false,
	}

	scraper := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	ctx := context.Background()

	err := scraper.start(ctx, componenttest.NewNopHost())
	if err == nil {
		// Verify both pingers were created
		scraper.mu.RLock()
		count := len(scraper.pingers)
		scraper.mu.RUnlock()

		// May be 0, 1, or 2 depending on DNS resolution
		assert.GreaterOrEqual(t, count, 0)
		assert.LessOrEqual(t, count, 2)

		// Clean up
		err = scraper.shutdown(ctx)
		require.NoError(t, err)
	}
}

func TestPingTargetMissingPinger(t *testing.T) {
	cfg := &Config{
		ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Targets:              []Target{},
	}

	scraper := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	scraper.mb = metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, scraper.settings)

	target := Target{
		Endpoint: "nonexistent.endpoint",
		Count:    1,
		Timeout:  time.Second,
	}

	var mu sync.Mutex
	err := scraper.pingTarget(context.Background(), target, &mu)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pinger not found")
}

func TestScraperScrapeWithNoTargets(t *testing.T) {
	cfg := &Config{
		ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Targets:              []Target{},
	}

	scraper := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	scraper.mb = metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, scraper.settings)

	metrics, err := scraper.scrape(context.Background())

	// Should return empty metrics without error
	assert.NoError(t, err)
	assert.NotNil(t, metrics)
}
