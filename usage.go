// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"time"
)

// UsageReader is implemented by providers that can measure what a running
// instance uses. Callers find it with a type assertion on a
// Provider.
type UsageReader interface {
	// Usage returns the counters of Running instance id now. From any other
	// state it wraps ErrInvalidTransition; an unknown id wraps ErrNotFound.
	Usage(ctx context.Context, id string) (Usage, error)
}

// HostMemoryReader is implemented by providers that can read what an
// instance holds on the host without asking its guest, such as a warm VM
// waiting for a job. Callers find it with a type assertion on a Provider.
type HostMemoryReader interface {
	// HostMemory returns the memory the processes that run Running instance
	// id hold now (resident), in bytes. From any other state it wraps
	// ErrInvalidTransition; an unknown id wraps ErrNotFound.
	HostMemory(ctx context.Context, id string) (uint64, error)
}

// Usage is what an instance has used since it started. The counters only
// grow, except that CPUWait can drop when one of the instance's threads
// exits.
type Usage struct {
	// CPUTime is the CPU time, user and system, of the processes that run
	// the instance.
	CPUTime time.Duration
	// CPUWait is how long those processes' threads were ready to run but
	// waited for a host CPU.
	CPUWait time.Duration
	// HostMemoryBytes is the memory those processes hold now (resident).
	HostMemoryBytes uint64
	// DiskReadBytes and DiskWriteBytes are what they read from and wrote to
	// storage.
	DiskReadBytes  uint64
	DiskWriteBytes uint64
	// GuestMemoryUsedBytes is the memory in use inside the guest, its total
	// less what it reports available. GuestMemoryKnown is false when the
	// guest does not report it.
	GuestMemoryUsedBytes uint64
	GuestMemoryKnown     bool
}
