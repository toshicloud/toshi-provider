# go.toshicloud.io/provider

The contract every Toshi VM provider implements: the `Provider` interface
and its optional facets, the spec, instance and error types, the host
check report, the registry of provider kinds, and `providertest`, the
conformance suite every provider passes. The host agent opens a provider
through this contract and never depends on a particular one
([docs/architecture.md](../docs/architecture.md#the-provider-contract)).

The module depends on nothing of Toshi's own module, so a provider built
in another module requires only this one:

```
require go.toshicloud.io/provider v0.1.0
```

## Versions

`VERSION` is the module's version. It is raised when the contract or its
suite changes, once per release of the module, not with every change.

A provider program speaks version 1 of the VM provider protocol
(`api/proto/toshi/backend/provider/v1/provider.proto`) and says so in the
shared handshake (`toshi.backend.v1`, type `vm_provider`). The two rules
that keep a provider and the agent compatible:

- At run time the handshake's version list rules: an agent that speaks
  none of the versions a program serves refuses it when it dials, and the
  error names both lists.
- At build time minimal version selection rules: a distribution links one
  version of this module, the highest any of its modules requires, so a
  provider built against an older contract that no longer matches it
  fails when the distribution is built, not when the agent runs.

## Conformance

A provider linked into the agent runs the suite in its own tests:

```go
func TestConformance(t *testing.T) {
	providertest.Run(t, func(t *testing.T) provider.Provider {
		return newProvider(t)
	})
}
```

A provider program runs it through the agent's adapter, from a checkout of
the Toshi repository. `TOSHI_PROVIDER_COMMAND` starts the program, its
arguments separated by spaces, and `TOSHI_PROVIDER_SETTINGS` is its own
settings, one JSON object. Each subtest starts the program afresh, as the
agent does when its configuration names a `command`. The suite does not
attach to a program already running, since that program would keep one
subtest's instances into the next and the suite expects a provider that
holds none:

```sh
TOSHI_PROVIDER_COMMAND=/path/to/your-provider \
TOSHI_PROVIDER_SETTINGS='{}' \
  go test -run TestConformanceOfAProgram ./internal/agent/providerproc
```

Through the adapter the suite also checks the guest link: a provider that
gives guests a connection reaches the guest protocol on it. In a provider's
own tests that check runs only with `providertest.WithGuestLink`, since
the guest protocol is internal to Toshi; subtests that need a hook the
provider does not implement are skipped.
