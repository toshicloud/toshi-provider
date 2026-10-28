// SPDX-License-Identifier: Apache-2.0

package provider_test

import (
	"context"
	"slices"
	"testing"

	"go.toshicloud.io/provider"
)

// closing has a Close and a Forget, and lists no facets.
type closing struct{}

func (closing) Close(context.Context) error { return nil }
func (closing) Forget(string) error         { return nil }

// listing has the same methods and lists only the closer facet.
type listing struct{ closing }

func (listing) Facets() []provider.Facet { return []provider.Facet{provider.FacetCloser} }

// overListing lists a facet whose interface it lacks.
type overListing struct{ closing }

func (overListing) Facets() []provider.Facet {
	return []provider.Facet{provider.FacetCloser, provider.FacetForgetter, provider.FacetSnapshotter}
}

func TestAsAndHasFollowTheList(t *testing.T) {
	for _, tc := range []struct {
		name            string
		v               any
		closer, forgets bool
		snapshots       bool
	}{
		{"no list: what it implements", closing{}, true, true, false},
		{"a list narrows", listing{}, true, false, false},
		{"a list never widens", overListing{}, true, true, false},
		{"nil", nil, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := provider.As[provider.Closer](tc.v); ok != tc.closer {
				t.Errorf("As[Closer] = %v, want %v", ok, tc.closer)
			}
			if _, ok := provider.As[provider.Forgetter](tc.v); ok != tc.forgets {
				t.Errorf("As[Forgetter] = %v, want %v", ok, tc.forgets)
			}
			if _, ok := provider.As[provider.Snapshotter](tc.v); ok != tc.snapshots {
				t.Errorf("As[Snapshotter] = %v, want %v", ok, tc.snapshots)
			}
			for f, want := range map[provider.Facet]bool{
				provider.FacetCloser:      tc.closer,
				provider.FacetForgetter:   tc.forgets,
				provider.FacetSnapshotter: tc.snapshots,
				"no_such_facet":           false,
			} {
				if got := provider.Has(tc.v, f); got != want {
					t.Errorf("Has(%q) = %v, want %v", f, got, want)
				}
			}
		})
	}
}

func TestAsRefusesAListerAnUnknownInterface(t *testing.T) {
	if _, ok := provider.As[interface{ Close(context.Context) error }](listing{}); ok {
		t.Fatal("As of an interface no facet names passed a value that lists its facets")
	}
	if _, ok := provider.As[interface{ Close(context.Context) error }](closing{}); !ok {
		t.Fatal("As of an unnamed interface failed a value that lists no facets")
	}
}

func TestImplementedIgnoresTheList(t *testing.T) {
	want := []provider.Facet{provider.FacetCloser, provider.FacetForgetter}
	if got := provider.Implemented(listing{}); !slices.Equal(got, want) {
		t.Fatalf("Implemented = %v, want %v", got, want)
	}
}
