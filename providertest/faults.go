// SPDX-License-Identifier: Apache-2.0

package providertest

import "go.toshicloud.io/provider"

// ResourceCounter is optional. Resources returns how many instances hold
// backend resources (processes, directories, containers), whether or not the
// provider still knows them. With CreateFaults or Restarter, the suite checks
// that a cancelled Create, a crashed one and a restart leave none behind.
type ResourceCounter interface {
	Resources() int
}

// CreateFaults is optional. It stops the next Create part way, after it has
// taken its resources.
type CreateFaults interface {
	// HoldNextCreate makes the next Create close the returned channel once it
	// holds its resources, then wait for its context to end before it
	// returns.
	HoldNextCreate() <-chan struct{}
	// CrashNextCreate makes the next Create return an error after it takes
	// its resources and before it records the instance, as if its process
	// died there. The suite then uses only the provider Restart returns.
	CrashNextCreate()
}

// Restarter is optional. Restart abandons the provider as a process that died
// would, with no Close and no cleanup, and returns a new provider built over
// the same state. The suite does not use the old provider again.
type Restarter interface {
	Restart() provider.Provider
}
