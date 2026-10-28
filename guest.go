// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"net"
)

// GuestConnector is an optional interface implemented by providers that can
// open a connection to the guest agent inside one of their instances.
// Callers find it with a type assertion on a Provider. The connection
// carries the guest protocol. An unknown id wraps ErrNotFound; an instance
// that is not running wraps ErrInvalidTransition.
type GuestConnector interface {
	DialGuest(ctx context.Context, id string) (net.Conn, error)
}

// DisplayConnector is an optional interface implemented by providers that
// serve their instances' screens. The connection carries the
// RFB protocol (RFC 6143) from the instance's VNC server. An unknown id
// wraps ErrNotFound; an instance that is not running wraps
// ErrInvalidTransition.
type DisplayConnector interface {
	DialDisplay(ctx context.Context, id string) (net.Conn, error)
}

// ScreenCapturer is an optional interface implemented by providers that
// can take a picture of a running instance's screen: the frames of a run's
// screenshot strip. CaptureScreen has the screen written to
// path as one PNG of at most MaxScreenBytes bytes; path is in a directory
// of the caller's that the provider's processes can write. On nil the
// file is complete and the caller removes it. An error that wraps
// ErrNoReply means no answer came: the picture may still be written to
// path until the instance is destroyed, so the caller removes it only
// then. Any other error leaves nothing at path. An unknown id wraps
// ErrNotFound; an instance that is not running wraps ErrInvalidTransition.
type ScreenCapturer interface {
	CaptureScreen(ctx context.Context, id, path string) error
	MaxScreenBytes() int64
}

// GuestExecutions is an optional interface implemented by a GuestConnector
// that can say before a guest boots whether its guests run executions,
// which guest protocol 2 adds. A GuestConnector without it is
// taken to run them, as the guest agent does.
type GuestExecutions interface {
	GuestsRunExecutions() bool
}

// DebugForwarder is an optional interface implemented by providers that
// forward a host loopback port to each guest's debug ssh port.
// Only the host can reach the address; the guest's ssh server accepts only
// the key the host gave it for the job.
type DebugForwarder interface {
	// DebugSSHAddr returns the host loopback address that reaches the
	// instance's debug ssh port, or false when the instance has none.
	DebugSSHAddr(id string) (string, bool)
}

// PreviewForwarder is an optional interface implemented by providers that
// forward a host loopback port to the guest port an image declares for
// previews. Only the host can reach the address; the agent dials
// it for a preview relay and for nothing else.
type PreviewForwarder interface {
	// PreviewAddr returns the host loopback address that reaches the
	// instance's preview port and that port, or false when the instance
	// has none.
	PreviewAddr(id string) (addr string, guestPort int, ok bool)
}
