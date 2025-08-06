// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pingcheckreceiver

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/lukeod/pingcheckreceiver/internal/metadata"
)

var errConfigNotPing = errors.New("config was not a Ping receiver config")

// NewFactory creates a new factory for Ping receiver
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = 60 * time.Second

	return &Config{
		ControllerConfig:     cfg,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Targets:              []Target{},
		Privileged:           false,
	}
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	pCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errConfigNotPing
	}

	pingScraperInstance := newScraper(pCfg, settings)
	scraperInstance, err := scraper.NewMetrics(
		pingScraperInstance.scrape,
		scraper.WithStart(pingScraperInstance.start),
		scraper.WithShutdown(pingScraperInstance.shutdown),
	)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(
		&pCfg.ControllerConfig,
		settings,
		consumer,
		scraperhelper.AddScraper(metadata.Type, scraperInstance),
	)
}
