// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"errors"
	"slices"
	"testing"
)

func nopFactory(func(v any) error) (Backend, error) { return nil, nil }

// mustPanic runs f and returns what it panicked with, failing the test when
// it did not panic.
func mustPanic(t *testing.T, f func()) (got any) {
	t.Helper()
	defer func() {
		got = recover()
		if got == nil {
			t.Fatal("no panic")
		}
	}()
	f()
	return nil
}

func TestRegistryRefusesASecondRegister(t *testing.T) {
	var r registry
	r.register("a", nopFactory)
	got := mustPanic(t, func() { r.register("a", nopFactory) })
	if want := `provider: Register "a" twice`; got != want {
		t.Fatalf("panic = %v, want %q", got, want)
	}
}

func TestRegistryRefusesAnEmptyKindAndANilFactory(t *testing.T) {
	var r registry
	if got, want := mustPanic(t, func() { r.register("", nopFactory) }), "provider: Register with an empty kind"; got != want {
		t.Fatalf("panic = %v, want %q", got, want)
	}
	if got, want := mustPanic(t, func() { r.register("a", nil) }), `provider: Register "a" with a nil factory`; got != want {
		t.Fatalf("panic = %v, want %q", got, want)
	}
	if kinds := r.kinds(); len(kinds) != 0 {
		t.Fatalf("kinds = %v after refused registers, want none", kinds)
	}
}

// NewBackend hands the factory the caller's decode and returns what the
// factory returns.
func TestRegistryBackendCallsTheFactoryWithDecode(t *testing.T) {
	var r registry
	errBuilt := errors.New("built")
	var decoded string
	r.register("a", func(decode func(v any) error) (Backend, error) {
		if err := decode(&decoded); err != nil {
			return nil, err
		}
		return nil, errBuilt
	})
	_, err := r.backend("a", func(v any) error {
		*v.(*string) = "config of a"
		return nil
	})
	if !errors.Is(err, errBuilt) {
		t.Fatalf("backend = %v, want the factory's error", err)
	}
	if decoded != "config of a" {
		t.Fatalf("factory decoded %q, want %q", decoded, "config of a")
	}
}

func TestRegistryUnknownKindNamesTheKinds(t *testing.T) {
	var r registry
	if err := r.check("b"); err == nil || err.Error() != `"b": no provider kind is registered` {
		t.Fatalf("check on an empty registry = %v", err)
	}
	r.register("c", nopFactory)
	r.register("a", nopFactory)
	if got := r.kinds(); !slices.Equal(got, []string{"a", "c"}) {
		t.Fatalf("kinds = %v, want [a c]", got)
	}
	want := `"b" is not one of a, c`
	if err := r.check("b"); err == nil || err.Error() != want {
		t.Fatalf("check = %v, want %q", err, want)
	}
	if _, err := r.backend("b", nil); err == nil || err.Error() != want {
		t.Fatalf("backend = %v, want %q", err, want)
	}
	if err := r.check("a"); err != nil {
		t.Fatalf("check of a registered kind = %v", err)
	}
}
