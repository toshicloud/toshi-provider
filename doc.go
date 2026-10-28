// SPDX-License-Identifier: Apache-2.0

// Package provider defines the interface every compute provider implements.
// Each way of starting a runner VM (a local hypervisor or a cloud account)
// sits in its own subpackage behind the Provider interface, so the scheduler
// and the host agent never depend on a particular provider.
//
// The host agent's providers register a kind with Register from their
// packages' init, and the agent opens the kind its configuration names with
// Open, which hands the provider's Factory a decode for its own
// configuration. A Factory returns a Backend, which starts nothing until
// Open: the agent asks it for its Capabilities and its host checks without
// starting the provider.
//
// A provider's state outlives its process. A provider built over the state
// of one that ended without Close, at any point, either recovers each
// instance it finds, so that Get and List show its true state and Destroy
// releases it, or releases the instance at start and then no longer knows it
// (Get wraps ErrNotFound). An instance whose Create never recorded it is
// never shown live, and its Key is free.
//
// Subpackage providertest is the conformance suite every provider must
// pass.
package provider
