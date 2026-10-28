// SPDX-License-Identifier: Apache-2.0

package providertest

import (
	"slices"
	"testing"

	"go.toshicloud.io/provider"
)

// CheckFacets fails t unless v lists its facets and the list is exactly
// the facets whose interfaces v implements, each once. A provider in the
// same process as its caller lists what it implements; the check catches
// an interface added without its facet, or a facet left after its methods
// went.
func CheckFacets(t testing.TB, v any) {
	t.Helper()
	l, ok := v.(provider.FacetLister)
	if !ok {
		t.Fatalf("%T lists no facets: it is no provider.FacetLister", v)
	}
	listed := slices.Clone(l.Facets())
	slices.Sort(listed)
	if c := slices.Compact(slices.Clone(listed)); len(c) != len(listed) {
		t.Errorf("%T lists a facet more than once: %v", v, listed)
	}
	implemented := provider.Implemented(v)
	slices.Sort(implemented)
	if !slices.Equal(slices.Compact(listed), implemented) {
		t.Errorf("%T lists facets %v; it implements %v", v, listed, implemented)
	}
}
