// SPDX-License-Identifier: Apache-2.0

package provider

// ProcessLimitReader is implemented by providers that hold each instance to
// a process limit: the container tier's pids.max. Callers find
// it with a type assertion on a Provider.
type ProcessLimitReader interface {
	// ProcessLimit returns what instance id's process limit says. ok is
	// false when the instance has no limit or it cannot be read, as after
	// Destroy.
	ProcessLimit(id string) (l ProcessLimit, ok bool)
}

// ProcessLimit is what a provider read of an instance's process limit.
type ProcessLimit struct {
	// Limit is how many processes and threads the instance may hold.
	Limit int
	// Hits is how many forks and clones failed at the limit, as the kernel
	// counted them.
	Hits int
	// Peak is the most processes and threads the instance held at once; 0
	// when it cannot be read.
	Peak int
	// Signal names the reading that says the instance reached its limit:
	// SignalEvents when Hits is above 0, else SignalPeak when Peak reached
	// Limit, else "".
	Signal string
}

// The readings that can say an instance reached its process limit.
const (
	SignalEvents = "pids.events"
	SignalPeak   = "pids.peak"
)

// Reached reports whether the instance reached its process limit.
func (l ProcessLimit) Reached() bool { return l.Signal != "" }
