// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"errors"
)

// ConsoleLogKeeper is implemented by providers that can keep a VM's console
// output after the VM is destroyed, so an operator can read why a job's VM
// died. Callers find it with a type assertion on a Provider.
type ConsoleLogKeeper interface {
	// KeepConsoleLog copies the end of instance id's console output to a
	// file that outlives the instance and returns its path. It returns ""
	// when the provider is set up to keep no console logs. The provider
	// deletes kept logs after a day. An unknown id wraps ErrNotFound.
	KeepConsoleLog(ctx context.Context, id string) (path string, err error)
}

// ConsoleReader is implemented by providers that let a VM's serial console
// be read while it runs. Callers find it with a type assertion on a
// Provider.
type ConsoleReader interface {
	// ReadConsole returns at most max bytes of instance id's console from
	// position (start, offset) on.
	ReadConsole(id, start string, offset int64, max int) (ConsoleChunk, error)
}

// ConsoleChunk is one read of a VM's serial console while it runs.
type ConsoleChunk struct {
	// Start names the VM start the offsets count in; a new start of the
	// instance has a different one and counts from zero again.
	Start string
	// Offset is the position of Data's first byte: the bytes the VM wrote
	// to its console before it, in this start.
	Offset int64
	// Data is empty when there is nothing past Offset yet.
	Data []byte
	// Wait is closed once there may be more to read: a write, a rotation,
	// the log's close; nil when Closed.
	Wait <-chan struct{}
	// Closed says no more bytes will come in this start (QEMU exited, or
	// the log stopped after an I/O error).
	Closed bool
}

// ErrConsoleOffset means a read asked for a position past what the VM has
// written in the start named by the read.
var ErrConsoleOffset = errors.New("provider: console offset past what was written")
