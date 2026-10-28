// SPDX-License-Identifier: Apache-2.0

package provider

import "context"

// MemoryCapReasonPrefix starts the Error of an instance that failed because
// the kernel stopped it at its memory cap.
const MemoryCapReasonPrefix = "memory cap reached"

// PasstExitReasonPrefix starts the Error of an instance that failed because
// passt, its network helper, exited under it.
const PasstExitReasonPrefix = "passt exited unexpectedly"

// MemoryHighSetter is implemented by providers that can soft-cap a running
// instance's memory (cgroup v2 memory.high). 0 removes the cap.
type MemoryHighSetter interface {
	SetMemoryHigh(ctx context.Context, id string, highMiB int) error
}

// TierContainer is the tier of a host whose jobs run in containers that
// share its kernel. A host that runs each job in a VM has the tier "".
const TierContainer = "container"

// Capabilities are what a provider's hosts can do that a caller asks about
// instead of comparing the provider's kind. The zero value is a host that
// runs each job in a VM and has none of them.
type Capabilities struct {
	// Tier is the tier the host registers as: TierContainer or "".
	Tier string
	// SharedJobMemory is set when a job's memory is its share of the
	// host's by CPUs rather than its size's.
	SharedJobMemory bool
	// InstanceLimits is set when the provider can give an instance a cgroup
	// of its own with a lower CPU weight and a memory cap.
	InstanceLimits bool
	// StateDisk is set when the provider keeps its instances' disks under
	// its state directory, so the free space there limits the host.
	StateDisk bool
	// StandIn is set when the provider starts nothing on the host: it
	// stands in for a tier, and the host has nothing of it to check.
	StandIn bool
}

// Capable is a Backend that reports its Capabilities. A Backend that does
// not has the zero Capabilities.
type Capable interface {
	Capabilities() Capabilities
}
