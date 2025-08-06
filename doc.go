// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package pingcheckreceiver implements a receiver that performs ICMP ping checks
// against configured targets and reports network connectivity metrics.
package pingcheckreceiver // import "github.com/lukeod/pingcheckreceiver"
