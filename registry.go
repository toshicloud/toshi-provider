// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
)

// The provider registry: each backend package registers its kind from its
// init, and the host agent opens the kind its configuration names, so the
// agent selects a provider without naming the kinds it knows.

// Factory builds a backend of one kind: decode fills the provider's own
// configuration from the caller's, as a key provider's Factory reads its
// table (internal/keyprovider/factory.go), and fills a *Host with what the
// host gives every provider. A Factory only decodes and starts nothing, so
// a caller asks a backend what it can do and what to check on the host
// (backend.go) without starting the provider.
type Factory func(decode func(v any) error) (Backend, error)

// Backend is a provider built from its configuration that has not started.
// It may implement the optional interfaces in backend.go.
type Backend interface {
	// Open starts the provider. Starting may act on the host: a provider
	// may release what one that ended without Close left there.
	Open() (Provider, error)
}

// Host is what a Factory decodes besides its own configuration: what the
// host gives every provider it opens.
type Host struct {
	// Log is the logger the provider logs to.
	Log *slog.Logger
}

// registry is the kinds and their factories. The package's one registry is
// filled at init and read when the agent starts.
type registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

var kinds registry

// Register makes factory the provider of kind. A backend calls it from its
// package's init, so a binary has the kinds of the backends it links. It
// panics when kind is empty, factory is nil or kind is registered already:
// each is a programming error.
func Register(kind string, factory Factory) { kinds.register(kind, factory) }

// NewBackend builds a backend of kind with its registered Factory, which
// reads its configuration through decode. A kind nobody registered is an
// error that names the kinds there are.
func NewBackend(kind string, decode func(v any) error) (Backend, error) {
	return kinds.backend(kind, decode)
}

// Open builds a backend of kind as NewBackend does and starts it.
func Open(kind string, decode func(v any) error) (Provider, error) {
	b, err := kinds.backend(kind, decode)
	if err != nil {
		return nil, err
	}
	return b.Open()
}

// CheckKind returns nil when kind is registered, and otherwise the error Open
// returns for it.
func CheckKind(kind string) error { return kinds.check(kind) }

// Kinds returns the registered kinds, sorted.
func Kinds() []string { return kinds.kinds() }

func (r *registry) register(kind string, factory Factory) {
	if kind == "" {
		panic("provider: Register with an empty kind")
	}
	if factory == nil {
		panic(fmt.Sprintf("provider: Register %q with a nil factory", kind))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.factories[kind]; ok {
		panic(fmt.Sprintf("provider: Register %q twice", kind))
	}
	if r.factories == nil {
		r.factories = make(map[string]Factory)
	}
	r.factories[kind] = factory
}

func (r *registry) backend(kind string, decode func(v any) error) (Backend, error) {
	r.mu.RLock()
	factory, ok := r.factories[kind]
	r.mu.RUnlock()
	if !ok {
		return nil, r.unknown(kind)
	}
	return factory(decode)
}

func (r *registry) check(kind string) error {
	r.mu.RLock()
	_, ok := r.factories[kind]
	r.mu.RUnlock()
	if !ok {
		return r.unknown(kind)
	}
	return nil
}

func (r *registry) kinds() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.factories))
	for kind := range r.factories {
		out = append(out, kind)
	}
	slices.Sort(out)
	return out
}

// unknown is the one error for a kind nobody registered.
func (r *registry) unknown(kind string) error {
	names := r.kinds()
	if len(names) == 0 {
		return fmt.Errorf("%q: no provider kind is registered", kind)
	}
	return fmt.Errorf("%q is not one of %s", kind, strings.Join(names, ", "))
}
