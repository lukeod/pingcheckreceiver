// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pingcheckreceiver

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/lukeod/pingcheckreceiver/internal/metadata"
)

// Config defines the configuration for the Ping receiver
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`

	// Targets to ping
	Targets []Target `mapstructure:"targets"`

	// Privileged mode for raw ICMP sockets
	Privileged bool `mapstructure:"privileged"`
}

// Target defines a ping target configuration
type Target struct {
	// Endpoint to ping (hostname or IP)
	Endpoint string `mapstructure:"endpoint"`

	// Number of packets to send (default: 4)
	Count int `mapstructure:"count"`

	// Timeout for ping operation (default: 5s)
	Timeout time.Duration `mapstructure:"timeout"`

	// Interval between packets (default: 1s)
	Interval time.Duration `mapstructure:"interval"`
}

// Validate implements component.Config
func (cfg *Config) Validate() error {
	var err error

	if len(cfg.Targets) == 0 {
		err = multierr.Append(err, errors.New("at least one target must be specified"))
	}

	for i, target := range cfg.Targets {
		if target.Endpoint == "" {
			err = multierr.Append(err, fmt.Errorf("targets[%d]: endpoint cannot be empty", i))
		}
		if target.Count < 0 {
			err = multierr.Append(err, fmt.Errorf("targets[%d]: count cannot be negative", i))
		}
		if target.Timeout < 0 {
			err = multierr.Append(err, fmt.Errorf("targets[%d]: timeout cannot be negative", i))
		}
		if target.Interval < 0 {
			err = multierr.Append(err, fmt.Errorf("targets[%d]: interval cannot be negative", i))
		}
	}

	return err
}
