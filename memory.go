// SPDX-License-Identifier: Apache-2.0

package provider

import "context"

// MemoryReclaimer is implemented by providers that can take memory back
// from a running guest while it waits for work and give it back before the
// work starts, such as the QEMU provider's balloon. Callers find it
// with a type assertion on a Provider.
type MemoryReclaimer interface {
	// IdleMemoryMiB returns the memory an idle instance of spec keeps, or 0
	// when the provider reclaims none for it.
	IdleMemoryMiB(spec Spec) int

	// SetMemoryTarget asks the guest of Running instance id to shrink or grow
	// to targetMiB. It returns once the request is made; the guest gets there
	// on its own. A target of the spec's MemoryMiB gives all memory back.
	// From any state but Running it wraps ErrInvalidTransition.
	SetMemoryTarget(ctx context.Context, id string, targetMiB int) error

	// MemoryMiB returns the memory the guest of Running instance id holds
	// now. From any state but Running it wraps ErrInvalidTransition.
	MemoryMiB(ctx context.Context, id string) (int, error)

	// GuestAvailableMiB returns free memory inside the guest from balloon
	// guest-stats. From any state but Running it wraps ErrInvalidTransition.
	GuestAvailableMiB(ctx context.Context, id string) (int, error)
}
