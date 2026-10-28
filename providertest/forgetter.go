// SPDX-License-Identifier: Apache-2.0

package providertest

import "go.toshicloud.io/provider"

// Forgetter is optional. When a provider implements it, the suite checks
// that forgotten instances disappear from Get and List.
type Forgetter = provider.Forgetter
