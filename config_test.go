// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pingcheckreceiver

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/lukeod/pingcheckreceiver/internal/metadata"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectedErr error
	}{
		{
			name: "valid config",
			config: Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Targets: []Target{
					{
						Endpoint: "google.com",
						Count:    4,
						Timeout:  5 * time.Second,
						Interval: time.Second,
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "no targets",
			config: Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Targets:              []Target{},
			},
			expectedErr: errors.New("at least one target must be specified"),
		},
		{
			name: "empty endpoint",
			config: Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Targets: []Target{
					{
						Endpoint: "",
						Count:    4,
						Timeout:  5 * time.Second,
					},
				},
			},
			expectedErr: multierr.Combine(
				errors.New("targets[0]: endpoint cannot be empty"),
			),
		},
		{
			name: "negative count",
			config: Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Targets: []Target{
					{
						Endpoint: "google.com",
						Count:    -1,
						Timeout:  5 * time.Second,
					},
				},
			},
			expectedErr: multierr.Combine(
				errors.New("targets[0]: count cannot be negative"),
			),
		},
		{
			name: "zero count allowed",
			config: Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Targets: []Target{
					{
						Endpoint: "google.com",
						Count:    0,
						Timeout:  5 * time.Second,
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "negative timeout",
			config: Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Targets: []Target{
					{
						Endpoint: "google.com",
						Count:    4,
						Timeout:  -1 * time.Second,
					},
				},
			},
			expectedErr: multierr.Combine(
				errors.New("targets[0]: timeout cannot be negative"),
			),
		},
		{
			name: "negative interval",
			config: Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Targets: []Target{
					{
						Endpoint: "google.com",
						Count:    4,
						Timeout:  5 * time.Second,
						Interval: -1 * time.Second,
					},
				},
			},
			expectedErr: multierr.Combine(
				errors.New("targets[0]: interval cannot be negative"),
			),
		},
		{
			name: "multiple errors",
			config: Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Targets: []Target{
					{
						Endpoint: "",
						Count:    -1,
						Timeout:  -1 * time.Second,
						Interval: -1 * time.Second,
					},
				},
			},
			expectedErr: multierr.Combine(
				errors.New("targets[0]: endpoint cannot be empty"),
				errors.New("targets[0]: count cannot be negative"),
				errors.New("targets[0]: timeout cannot be negative"),
				errors.New("targets[0]: interval cannot be negative"),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
