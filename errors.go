// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"errors"
	"fmt"
)

// Sentinel errors. Providers wrap them with fmt.Errorf("...: %w", err) so
// that errors.Is keeps working.
var (
	// ErrNotFound means the provider does not know the instance or snapshot,
	// or has already forgotten it.
	ErrNotFound = errors.New("provider: not found")

	// ErrInvalidTransition means the instance's current state does not
	// allow the requested operation.
	ErrInvalidTransition = errors.New("provider: invalid state transition")

	// ErrInvalidSpec means a Spec failed validation.
	ErrInvalidSpec = errors.New("provider: invalid spec")

	// ErrUnsupported means the provider cannot do what was asked, for
	// example run the requested OS.
	ErrUnsupported = errors.New("provider: unsupported")

	// ErrImageChanged means a snapshot was taken from a base image that has
	// changed on disk since: restoring it would put the snapshot's disk on
	// top of a different image.
	ErrImageChanged = errors.New("provider: image changed since the snapshot")

	// ErrInUse means another instance of the provider that is not
	// destroyed holds the state volume a spec names.
	ErrInUse = errors.New("provider: state volume in use")

	// ErrKeyInUse means another instance of the provider that is not
	// destroyed has the spec's Key in its Workspace. The error Create
	// returns is a *KeyInUseError naming that instance.
	ErrKeyInUse = errors.New("provider: key in use")

	// ErrNoReply means a command to an instance got no answer before its
	// context ended or its connection failed: it may still be running.
	ErrNoReply = errors.New("provider: no reply")
)

// KeyInUseError is the error Create returns when the spec's Key is in use in
// its Workspace: ID is the instance that holds it. It wraps ErrKeyInUse.
type KeyInUseError struct {
	Workspace string
	Key       string
	ID        string
}

func (e *KeyInUseError) Error() string {
	return fmt.Sprintf("provider: key %q in workspace %q is in use by %s", e.Key, e.Workspace, e.ID)
}

func (e *KeyInUseError) Unwrap() error { return ErrKeyInUse }
