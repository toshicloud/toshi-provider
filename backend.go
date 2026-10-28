// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"time"
)

// The optional interfaces a Backend implements for the host checks only
// some providers have: a caller asks for one with a type assertion, before
// or without starting the provider. Their rows are a CheckReport.

// PreflightInput is what the host knows that a provider's preflight checks.
type PreflightInput struct {
	// Images are the images the host's pools use, sorted.
	Images []string
	// BeforeStart is set when no agent serves the host yet, as in a doctor
	// run before the first start: what the agent's unit makes when it
	// starts, such as the VM cgroup, may not exist yet, so a row about it
	// warns instead of failing.
	BeforeStart bool
}

// Preflighter is a Backend that checks the host before the provider starts:
// the agent refuses to start on a failed row, and its doctor prints them.
// Preflight creates nothing and starts nothing.
type Preflighter interface {
	Preflight(ctx context.Context, in PreflightInput) CheckReport
}

// StatusInput is what the host knows that a provider's status rows report.
type StatusInput struct {
	// Snapshots is set when the host's warm pools restore from snapshots.
	Snapshots bool
	// Pools maps each runner size to the name of its image.
	Pools map[string]string
	// Now is the time the rows are for.
	Now time.Time
	// Grace and MaxPerPass are the collector's, as Collector.Collect takes
	// them.
	Grace      time.Duration
	MaxPerPass int
}

// StatusReporter is a Backend that reports what the provider keeps on the
// host, such as snapshots and what a collection would remove. Only the
// agent's doctor prints the rows; Status changes nothing.
type StatusReporter interface {
	Status(in StatusInput) CheckReport
}
