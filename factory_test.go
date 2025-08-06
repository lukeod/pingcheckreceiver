// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pingcheckreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/lukeod/pingcheckreceiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, metadata.Type, factory.Type())
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.NotNil(t, cfg)
	assert.IsType(t, &Config{}, cfg)

	pCfg := cfg.(*Config)
	assert.Equal(t, 60*time.Second, pCfg.CollectionInterval)
	assert.Equal(t, time.Second, pCfg.InitialDelay)
	assert.False(t, pCfg.Privileged)
	assert.Empty(t, pCfg.Targets)
}

func TestCreateMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Targets = []Target{
		{
			Endpoint: "localhost",
			Count:    1,
			Timeout:  time.Second,
		},
	}

	receiver, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		consumertest.NewNop(),
	)

	require.NoError(t, err)
	assert.NotNil(t, receiver)
}

func TestCreateMetricsReceiverInvalidConfig(t *testing.T) {
	factory := NewFactory()

	receiver, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(metadata.Type),
		nil,
		consumertest.NewNop(),
	)

	assert.ErrorIs(t, err, errConfigNotPing)
	assert.Nil(t, receiver)
}

func TestCreateMetricsReceiverWithEmptyTargets(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	receiver, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		consumertest.NewNop(),
	)

	// Should create receiver even with empty targets
	require.NoError(t, err)
	assert.NotNil(t, receiver)
}
