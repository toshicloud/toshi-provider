// SPDX-License-Identifier: Apache-2.0

package provider_test

import (
	"regexp"
	"testing"

	"go.toshicloud.io/provider"
)

func TestVersionIsMajorMinorPatch(t *testing.T) {
	if v := provider.Version(); !regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`).MatchString(v) {
		t.Fatalf("VERSION = %q, want MAJOR.MINOR.PATCH", v)
	}
}
