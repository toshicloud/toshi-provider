// SPDX-License-Identifier: Apache-2.0

package provider_test

import (
	"slices"
	"testing"

	"go.toshicloud.io/provider"
)

// A clone given another disk leaves the spec it came from as it was: the
// agent sets a job's cache disk on a clone of its pool's spec.
func TestSetDiskLeavesTheOriginal(t *testing.T) {
	cache := provider.Disk{Kind: provider.DiskCache, Path: "/cache/gen-1"}
	state := provider.Disk{Kind: provider.DiskState, Path: "/volumes/home.qcow2"}
	base := provider.Spec{OS: provider.Linux}
	base.SetDisk(state)
	base.SetDisk(cache)

	job := base.Clone()
	job.SetDisk(provider.Disk{Kind: provider.DiskCache, Path: "/cache/job-1"})
	alias := base
	alias.RemoveDisk(provider.DiskState)

	if want := []provider.Disk{state, cache}; !slices.Equal(base.Disks, want) {
		t.Fatalf("base Disks = %+v, want %+v", base.Disks, want)
	}
	if d, ok := job.Disk(provider.DiskCache); !ok || d.Path != "/cache/job-1" {
		t.Fatalf("job cache disk = %+v, %v, want /cache/job-1", d, ok)
	}
	if _, ok := alias.Disk(provider.DiskState); ok {
		t.Fatalf("RemoveDisk left the state disk: %+v", alias.Disks)
	}
	if d, _ := job.Disk(provider.DiskState); d != state {
		t.Fatalf("job state disk = %+v, want %+v", d, state)
	}
}
