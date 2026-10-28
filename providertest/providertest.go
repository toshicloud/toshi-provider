// SPDX-License-Identifier: Apache-2.0

package providertest

import (
	"context"
	"errors"
	"maps"
	"net"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"go.toshicloud.io/provider"
)

const subtestTimeout = 10 * time.Second

// Option changes how the suite runs.
type Option func(*options)

type options struct {
	guestLink func(ctx context.Context, conn net.Conn) error
}

// WithGuestLink has the suite check the guest protocol on a connection
// DialGuest returns: check speaks to the guest at the connection's far end
// and returns an error if the guest does not answer as it should. The suite
// closes the connection afterwards. The guest protocol belongs to the agent
// and the guest, not to the provider contract, so without this option the
// suite checks only that DialGuest connects, and reports the protocol check
// as skipped.
func WithGuestLink(check func(ctx context.Context, conn net.Conn) error) Option {
	return func(o *options) { o.guestLink = check }
}

// Run executes the provider conformance suite for Linux specs. Each subtest
// calls newProvider for a fresh provider instance.
func Run(t *testing.T, newProvider func(t *testing.T) provider.Provider, opts ...Option) {
	RunOS(t, provider.Linux, newProvider, opts...)
}

// RunOS executes the provider conformance suite using specs for os. Each
// subtest calls newProvider for a fresh provider instance.
func RunOS(t *testing.T, os provider.OS, newProvider func(t *testing.T) provider.Provider, opts ...Option) {
	t.Helper()
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	t.Run("CreateThenWaitReady", func(t *testing.T) {
		p := newProvider(t)
		ctx := testContext(t)
		spec := validSpec(os)

		inst, err := p.Create(ctx, spec.Clone())
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if inst.State != provider.Creating && inst.State != provider.Ready {
			t.Fatalf("Create state = %q, want Creating or Ready", inst.State)
		}

		got, err := p.Wait(ctx, inst.ID, provider.Ready)
		if err != nil {
			t.Fatalf("Wait Ready: %v", err)
		}
		if got.State != provider.Ready {
			t.Fatalf("Wait Ready state = %q, want Ready", got.State)
		}
		assertSpecRoundTrip(t, spec, got.Spec)
	})

	t.Run("StartStopDestroy", func(t *testing.T) {
		p := newProvider(t)
		ctx := testContext(t)
		spec := validSpec(os)

		inst, err := p.Create(ctx, spec.Clone())
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := p.Wait(ctx, inst.ID, provider.Ready); err != nil {
			t.Fatalf("Wait Ready: %v", err)
		}
		if err := p.Start(ctx, inst.ID); err != nil {
			t.Fatalf("Start: %v", err)
		}
		if _, err := p.Wait(ctx, inst.ID, provider.Running); err != nil {
			t.Fatalf("Wait Running: %v", err)
		}
		if err := p.Stop(ctx, inst.ID); err != nil {
			t.Fatalf("Stop: %v", err)
		}
		if _, err := p.Wait(ctx, inst.ID, provider.Stopped); err != nil {
			t.Fatalf("Wait Stopped: %v", err)
		}
		if err := p.Destroy(ctx, inst.ID); err != nil {
			t.Fatalf("Destroy: %v", err)
		}
		snap, err := p.Get(ctx, inst.ID)
		if err != nil {
			if !errors.Is(err, provider.ErrNotFound) {
				t.Fatalf("Get after Destroy: %v", err)
			}
		} else if snap.State != provider.Destroyed {
			t.Fatalf("Get state = %q, want Destroyed or ErrNotFound", snap.State)
		}
	})

	t.Run("DestroyIdempotent", func(t *testing.T) {
		p := newProvider(t)
		ctx := testContext(t)
		spec := validSpec(os)

		inst, err := p.Create(ctx, spec.Clone())
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := p.Wait(ctx, inst.ID, provider.Ready); err != nil {
			t.Fatalf("Wait Ready: %v", err)
		}
		if err := p.Destroy(ctx, inst.ID); err != nil {
			t.Fatalf("first Destroy: %v", err)
		}
		if err := p.Destroy(ctx, inst.ID); err != nil {
			t.Fatalf("second Destroy: %v", err)
		}

		inst2, err := p.Create(ctx, spec.Clone())
		if err != nil {
			t.Fatalf("Create (immediate destroy): %v", err)
		}
		if err := p.Destroy(ctx, inst2.ID); err != nil {
			t.Fatalf("Destroy straight after Create: %v", err)
		}
	})

	t.Run("GetUnknownIsNotFound", func(t *testing.T) {
		p := newProvider(t)
		ctx := testContext(t)

		_, err := p.Get(ctx, "unknown-instance-id")
		if err == nil {
			t.Fatal("Get: want error")
		}
		if !errors.Is(err, provider.ErrNotFound) {
			t.Fatalf("Get: %v, want ErrNotFound", err)
		}
	})

	t.Run("StartRunningIsInvalidTransition", func(t *testing.T) {
		p := newProvider(t)
		ctx := testContext(t)
		spec := validSpec(os)

		inst, err := p.Create(ctx, spec.Clone())
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := p.Wait(ctx, inst.ID, provider.Ready); err != nil {
			t.Fatalf("Wait Ready: %v", err)
		}
		if err := p.Start(ctx, inst.ID); err != nil {
			t.Fatalf("Start: %v", err)
		}
		if _, err := p.Wait(ctx, inst.ID, provider.Running); err != nil {
			t.Fatalf("Wait Running: %v", err)
		}
		err = p.Start(ctx, inst.ID)
		if err == nil {
			t.Fatal("Start on Running: want error")
		}
		if !errors.Is(err, provider.ErrInvalidTransition) {
			t.Fatalf("Start on Running: %v, want ErrInvalidTransition", err)
		}
	})

	t.Run("InvalidSpec", func(t *testing.T) {
		base := validSpec(os)
		cases := []struct {
			name string
			mut  func(*provider.Spec)
		}{
			{
				name: "empty OS",
				mut: func(s *provider.Spec) {
					s.OS = ""
				},
			},
			{
				name: "unknown OS",
				mut: func(s *provider.Spec) {
					s.OS = provider.OS("plan9")
				},
			},
			{
				name: "zero CPUs",
				mut: func(s *provider.Spec) {
					s.CPUs = 0
				},
			},
			{
				name: "zero MemoryMiB",
				mut: func(s *provider.Spec) {
					s.MemoryMiB = 0
				},
			},
			{
				name: "negative DiskGiB",
				mut: func(s *provider.Spec) {
					s.DiskGiB = -1
				},
			},
			{
				name: "relative state disk",
				mut: func(s *provider.Spec) {
					s.SetDisk(provider.Disk{Kind: provider.DiskState, Path: "volume.qcow2"})
				},
			},
			{
				name: "unclean state disk",
				mut: func(s *provider.Spec) {
					s.SetDisk(provider.Disk{Kind: provider.DiskState, Path: "/volumes/../volumes/home.qcow2"})
				},
			},
			{
				name: "unknown disk kind",
				mut: func(s *provider.Spec) {
					s.Disks = []provider.Disk{{Kind: "scratch", Path: "/volumes/scratch.qcow2"}}
				},
			},
			{
				name: "two state disks",
				mut: func(s *provider.Spec) {
					s.Disks = []provider.Disk{
						{Kind: provider.DiskState, Path: "/volumes/a.qcow2"},
						{Kind: provider.DiskState, Path: "/volumes/b.qcow2"},
					}
				},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Helper()
				p := newProvider(t)
				ctx := testContext(t)
				spec := base.Clone()
				tc.mut(&spec)

				_, err := p.Create(ctx, spec)
				if err == nil {
					t.Fatal("Create: want error")
				}
				if !errors.Is(err, provider.ErrInvalidSpec) {
					t.Fatalf("Create: %v, want ErrInvalidSpec", err)
				}
				list, err := p.List(ctx)
				if err != nil {
					t.Fatalf("List: %v", err)
				}
				if len(list) != 0 {
					t.Fatalf("List len = %d, want 0", len(list))
				}
			})
		}
	})

	t.Run("ListShowsCreated", func(t *testing.T) {
		p := newProvider(t)
		ctx := testContext(t)
		spec := validSpec(os)

		inst1, err := p.Create(ctx, spec.Clone())
		if err != nil {
			t.Fatalf("Create 1: %v", err)
		}
		if _, err := p.Wait(ctx, inst1.ID, provider.Ready); err != nil {
			t.Fatalf("Wait Ready 1: %v", err)
		}

		spec2 := spec.Clone()
		spec2.Labels = maps.Clone(spec2.Labels)
		spec2.Labels["pool"] = "secondary"
		inst2, err := p.Create(ctx, spec2)
		if err != nil {
			t.Fatalf("Create 2: %v", err)
		}
		if _, err := p.Wait(ctx, inst2.ID, provider.Ready); err != nil {
			t.Fatalf("Wait Ready 2: %v", err)
		}

		list, err := p.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		byID := indexByID(list)
		if len(byID) < 2 {
			t.Fatalf("List contains %d instances, want at least 2", len(byID))
		}
		checkInst, ok := byID[inst1.ID]
		if !ok {
			t.Fatalf("List missing instance %q", inst1.ID)
		}
		if !reflect.DeepEqual(checkInst.Spec.Labels, spec.Labels) {
			t.Fatalf("instance 1 Labels = %v, want %v", checkInst.Spec.Labels, spec.Labels)
		}
		checkInst2, ok := byID[inst2.ID]
		if !ok {
			t.Fatalf("List missing instance %q", inst2.ID)
		}
		if !reflect.DeepEqual(checkInst2.Spec.Labels, spec2.Labels) {
			t.Fatalf("instance 2 Labels = %v, want %v", checkInst2.Spec.Labels, spec2.Labels)
		}
	})

	t.Run("ListHidesForgotten", func(t *testing.T) {
		p := newProvider(t)
		ctx := testContext(t)
		spec := validSpec(os)

		inst, err := p.Create(ctx, spec.Clone())
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := p.Wait(ctx, inst.ID, provider.Ready); err != nil {
			t.Fatalf("Wait Ready: %v", err)
		}
		if err := p.Destroy(ctx, inst.ID); err != nil {
			t.Fatalf("Destroy: %v", err)
		}

		if f, ok := provider.As[Forgetter](p); ok {
			if err := f.Forget(inst.ID); err != nil {
				t.Fatalf("Forget: %v", err)
			}
			_, err := p.Get(ctx, inst.ID)
			if err == nil {
				t.Fatal("Get after Forget: want error")
			}
			if !errors.Is(err, provider.ErrNotFound) {
				t.Fatalf("Get after Forget: %v, want ErrNotFound", err)
			}
			list, err := p.List(ctx)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			for _, item := range list {
				if item.ID == inst.ID {
					t.Fatalf("List still contains forgotten id %q", inst.ID)
				}
			}
			return
		}

		got, getErr := p.Get(ctx, inst.ID)
		if getErr != nil {
			if !errors.Is(getErr, provider.ErrNotFound) {
				t.Fatalf("Get after Destroy: %v", getErr)
			}
			assertAbsentFromList(t, p, ctx, inst.ID)
			return
		}
		if got.State != provider.Destroyed {
			t.Fatalf("Get state = %q, want Destroyed or ErrNotFound", got.State)
		}
		assertPresentDestroyedInList(t, p, ctx, inst.ID)
	})

	t.Run("WaitCancelledContext", func(t *testing.T) {
		p := newProvider(t)
		ctx := testContext(t)
		spec := validSpec(os)

		inst, err := p.Create(ctx, spec.Clone())
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := p.Wait(ctx, inst.ID, provider.Ready); err != nil {
			t.Fatalf("Wait Ready: %v", err)
		}

		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		got, err := p.Wait(cancelled, inst.ID, provider.Running)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Wait: %v, want context.Canceled", err)
		}
		if got.ID != inst.ID || got.State != provider.Ready {
			t.Fatalf("Wait snapshot = %q in %q, want %q in Ready", got.ID, got.State, inst.ID)
		}
	})

	t.Run("WaitDestroyedFails", func(t *testing.T) {
		p := newProvider(t)
		ctx := testContext(t)
		spec := validSpec(os)

		inst, err := p.Create(ctx, spec.Clone())
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := p.Wait(ctx, inst.ID, provider.Ready); err != nil {
			t.Fatalf("Wait Ready: %v", err)
		}

		waitCtx, waitCancel := context.WithTimeout(ctx, subtestTimeout)
		defer waitCancel()
		done := make(chan struct{})
		var waitInst provider.Instance
		var waitErr error
		go func() {
			waitInst, waitErr = p.Wait(waitCtx, inst.ID, provider.Running)
			close(done)
		}()

		time.Sleep(50 * time.Millisecond)
		select {
		case <-done:
			t.Fatal("Wait returned before Destroy")
		default:
		}

		if err := p.Destroy(ctx, inst.ID); err != nil {
			t.Fatalf("Destroy: %v", err)
		}
		select {
		case <-done:
		case <-time.After(subtestTimeout):
			t.Fatal("Wait did not return after Destroy")
		}
		if waitErr == nil {
			t.Fatal("Wait: want error after Destroy")
		}
		if waitInst.State != provider.Destroyed {
			t.Fatalf("Wait snapshot state = %q, want Destroyed", waitInst.State)
		}
	})

	t.Run("WaitUnreachableIsInvalidTransition", func(t *testing.T) {
		p := newProvider(t)
		ctx := testContext(t)
		spec := validSpec(os)

		inst, err := p.Create(ctx, spec.Clone())
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := p.Wait(ctx, inst.ID, provider.Ready); err != nil {
			t.Fatalf("Wait Ready: %v", err)
		}
		if err := p.Start(ctx, inst.ID); err != nil {
			t.Fatalf("Start: %v", err)
		}
		if _, err := p.Wait(ctx, inst.ID, provider.Running); err != nil {
			t.Fatalf("Wait Running: %v", err)
		}

		_, err = p.Wait(ctx, inst.ID, provider.Ready)
		if err == nil {
			t.Fatal("Wait Ready from Running: want error")
		}
		if !errors.Is(err, provider.ErrInvalidTransition) {
			t.Fatalf("Wait Ready from Running: %v, want ErrInvalidTransition", err)
		}
	})

	t.Run("InstanceIsValue", func(t *testing.T) {
		p := newProvider(t)
		ctx := testContext(t)
		spec := validSpec(os)

		inst, err := p.Create(ctx, spec.Clone())
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		inst, err = p.Wait(ctx, inst.ID, provider.Ready)
		if err != nil {
			t.Fatalf("Wait Ready: %v", err)
		}
		wantMeta := maps.Clone(inst.Spec.Metadata)
		wantLabels := maps.Clone(inst.Spec.Labels)

		inst.Spec.Metadata["mutated"] = "yes"
		inst.Spec.Labels["mutated"] = "yes"

		got, err := p.Get(ctx, inst.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if !reflect.DeepEqual(got.Spec.Metadata, wantMeta) {
			t.Fatalf("Get Metadata = %v, want %v", got.Spec.Metadata, wantMeta)
		}
		if !reflect.DeepEqual(got.Spec.Labels, wantLabels) {
			t.Fatalf("Get Labels = %v, want %v", got.Spec.Labels, wantLabels)
		}
		assertSpecRoundTrip(t, spec, got.Spec)
	})

	t.Run("SameKey", func(t *testing.T) {
		p := newProvider(t)
		ctx := testContext(t)
		spec := validSpec(os)
		spec.Key = "runner-1"
		spec.Workspace = "ws-a"

		a, err := p.Create(ctx, spec.Clone())
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		_, err = p.Create(ctx, spec.Clone())
		assertKeyInUse(t, err, spec, a.ID)
		assertKeyHolders(t, p, ctx, spec, a.ID)

		if err := p.Destroy(ctx, a.ID); err != nil {
			t.Fatalf("Destroy: %v", err)
		}
		// With the key free, of concurrent Creates with it exactly one
		// succeeds and every other names the winner.
		const racers = 4
		insts := make([]provider.Instance, racers)
		errs := make([]error, racers)
		var wg sync.WaitGroup
		for i := range racers {
			wg.Go(func() { insts[i], errs[i] = p.Create(ctx, spec.Clone()) })
		}
		wg.Wait()
		var winner string
		for i, err := range errs {
			if err != nil {
				continue
			}
			if winner != "" {
				t.Fatalf("concurrent Creates with one key made %s and %s", winner, insts[i].ID)
			}
			winner = insts[i].ID
		}
		if winner == "" {
			t.Fatalf("no concurrent Create with a free key succeeded: %v", errs)
		}
		if winner == a.ID {
			t.Fatalf("Create after Destroy returned the destroyed instance %s", a.ID)
		}
		for _, err := range errs {
			if err != nil {
				assertKeyInUse(t, err, spec, winner)
			}
		}
		assertKeyHolders(t, p, ctx, spec, winner)
		if err := p.Destroy(ctx, winner); err != nil {
			t.Fatalf("Destroy winner: %v", err)
		}

		// An empty Key is never in use.
		blank := validSpec(os)
		blank.Workspace = spec.Workspace
		b1, err := p.Create(ctx, blank.Clone())
		if err != nil {
			t.Fatalf("Create without a key: %v", err)
		}
		b2, err := p.Create(ctx, blank.Clone())
		if err != nil {
			t.Fatalf("second Create without a key: %v", err)
		}
		if b1.ID == b2.ID {
			t.Fatalf("two Creates without a key returned one instance %s", b1.ID)
		}
		for _, id := range []string{b1.ID, b2.ID} {
			if err := p.Destroy(ctx, id); err != nil {
				t.Fatalf("Destroy %s: %v", id, err)
			}
		}
	})

	t.Run("KeyPerWorkspace", func(t *testing.T) {
		p := newProvider(t)
		ctx := testContext(t)
		spec1 := validSpec(os)
		spec1.Key = "runner-1"
		spec1.Workspace = "ws-a"
		spec2 := spec1.Clone()
		spec2.Workspace = "ws-b"

		x, err := p.Create(ctx, spec1.Clone())
		if err != nil {
			t.Fatalf("Create in %s: %v", spec1.Workspace, err)
		}
		y, err := p.Create(ctx, spec2.Clone())
		if err != nil {
			t.Fatalf("Create with the same key in %s: %v", spec2.Workspace, err)
		}
		if x.ID == y.ID {
			t.Fatalf("one key in two workspaces returned one instance %s", x.ID)
		}
		for id, want := range map[string]provider.Spec{x.ID: spec1, y.ID: spec2} {
			got, err := p.Get(ctx, id)
			if err != nil {
				t.Fatalf("Get %s: %v", id, err)
			}
			assertSpecRoundTrip(t, want, got.Spec)
		}
		list, err := p.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		byID := indexByID(list)
		for id, want := range map[string]provider.Spec{x.ID: spec1, y.ID: spec2} {
			got, ok := byID[id]
			if !ok {
				t.Fatalf("List missing instance %q", id)
			}
			assertSpecRoundTrip(t, want, got.Spec)
		}

		_, err = p.Create(ctx, spec1.Clone())
		assertKeyInUse(t, err, spec1, x.ID)
		assertKeyHolders(t, p, ctx, spec2, y.ID)
		for _, id := range []string{x.ID, y.ID} {
			if err := p.Destroy(ctx, id); err != nil {
				t.Fatalf("Destroy %s: %v", id, err)
			}
		}
	})

	t.Run("CancelDuringCreate", func(t *testing.T) {
		p := newProvider(t)
		faults, ok := p.(CreateFaults)
		if !ok {
			t.Skip("provider does not implement CreateFaults")
		}
		counter := resourceCounter(t, p)
		ctx := testContext(t)
		spec := validSpec(os)
		spec.Key = "runner-cancel"
		spec.Workspace = "ws-a"
		base := counter.Resources()

		createCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		held := faults.HoldNextCreate()
		done := make(chan error, 1)
		go func() {
			_, err := p.Create(createCtx, spec.Clone())
			done <- err
		}()
		select {
		case <-held:
		case err := <-done:
			t.Fatalf("held Create returned before it held its resources: %v", err)
		case <-ctx.Done():
			t.Fatal("held Create never held its resources")
		}
		cancel()
		var err error
		select {
		case err = <-done:
		case <-ctx.Done():
			t.Fatal("cancelled Create did not return")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled Create: %v, want context.Canceled", err)
		}
		assertNoKeyHolder(t, p, ctx, spec)
		if got := counter.Resources(); got != base {
			t.Fatalf("Resources after a cancelled Create = %d, want %d", got, base)
		}

		inst, err := p.Create(ctx, spec.Clone())
		if err != nil {
			t.Fatalf("Create after a cancelled Create: %v", err)
		}
		if err := p.Destroy(ctx, inst.ID); err != nil {
			t.Fatalf("Destroy: %v", err)
		}
		if got := counter.Resources(); got != base {
			t.Fatalf("Resources after Destroy = %d, want %d", got, base)
		}
	})

	t.Run("CrashAfterSideEffect", func(t *testing.T) {
		p := newProvider(t)
		faults, ok := p.(CreateFaults)
		if !ok {
			t.Skip("provider does not implement CreateFaults")
		}
		restarter := providerRestarter(t, p)
		counter := resourceCounter(t, p)
		ctx := testContext(t)
		spec := validSpec(os)
		spec.Key = "runner-crash"
		spec.Workspace = "ws-a"
		base := counter.Resources()

		faults.CrashNextCreate()
		if inst, err := p.Create(ctx, spec.Clone()); err == nil {
			t.Fatalf("crashed Create returned %s and no error", inst.ID)
		}
		p2 := restarter.Restart()
		counter = resourceCounter(t, p2)
		// The orphan is either released or shown Destroyed: never live.
		assertNoKeyHolder(t, p2, ctx, spec)
		if got := counter.Resources(); got != base {
			t.Fatalf("Resources after a restart over a crashed Create = %d, want %d", got, base)
		}

		inst, err := p2.Create(ctx, spec.Clone())
		if err != nil {
			t.Fatalf("Create with the crashed Create's key after the restart: %v", err)
		}
		if err := p2.Destroy(ctx, inst.ID); err != nil {
			t.Fatalf("Destroy: %v", err)
		}
		if got := counter.Resources(); got != base {
			t.Fatalf("Resources after Destroy = %d, want %d", got, base)
		}
	})

	t.Run("RestartRecovery", func(t *testing.T) {
		p := newProvider(t)
		restarter := providerRestarter(t, p)
		counter := resourceCounter(t, p)
		ctx := testContext(t)
		base := counter.Resources()

		ready := validSpec(os)
		ready.Key = "runner-ready"
		ready.Workspace = "ws-a"
		r, err := p.Create(ctx, ready.Clone())
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := p.Wait(ctx, r.ID, provider.Ready); err != nil {
			t.Fatalf("Wait Ready: %v", err)
		}
		running := ready.Clone()
		running.Key = "runner-running"
		s, err := p.Create(ctx, running.Clone())
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := p.Wait(ctx, s.ID, provider.Ready); err != nil {
			t.Fatalf("Wait Ready: %v", err)
		}
		if err := p.Start(ctx, s.ID); err != nil {
			t.Fatalf("Start: %v", err)
		}
		if _, err := p.Wait(ctx, s.ID, provider.Running); err != nil {
			t.Fatalf("Wait Running: %v", err)
		}

		p2 := restarter.Restart()
		counter = resourceCounter(t, p2)
		// Each instance is either recovered, in the state it was in (a
		// running one may have failed with its process), or released.
		for _, c := range []struct {
			id   string
			spec provider.Spec
			want []provider.State
		}{
			{r.ID, ready, []provider.State{provider.Ready}},
			{s.ID, running, []provider.State{provider.Running, provider.Failed}},
		} {
			got, err := p2.Get(ctx, c.id)
			if errors.Is(err, provider.ErrNotFound) {
				// Released: its key is free.
				inst, err := p2.Create(ctx, c.spec.Clone())
				if err != nil {
					t.Fatalf("Create with released %s's key: %v", c.id, err)
				}
				if err := p2.Destroy(ctx, inst.ID); err != nil {
					t.Fatalf("Destroy: %v", err)
				}
				continue
			}
			if err != nil {
				t.Fatalf("Get %s after the restart: %v", c.id, err)
			}
			if !slices.Contains(c.want, got.State) {
				t.Fatalf("recovered %s is %s, want one of %v", c.id, got.State, c.want)
			}
			if got.State == provider.Failed && got.Error == "" {
				t.Fatalf("recovered %s is Failed with no Error", c.id)
			}
			assertSpecRoundTrip(t, c.spec, got.Spec)
			_, err = p2.Create(ctx, c.spec.Clone())
			assertKeyInUse(t, err, c.spec, c.id)
			if err := p2.Destroy(ctx, c.id); err != nil {
				t.Fatalf("Destroy recovered %s: %v", c.id, err)
			}
			assertNoKeyHolder(t, p2, ctx, c.spec)
		}
		if got := counter.Resources(); got != base {
			t.Fatalf("Resources after the restart and Destroy = %d, want %d", got, base)
		}
	})

	t.Run("SnapshotRestore", func(t *testing.T) {
		p := newProvider(t)
		snap, ok := provider.As[provider.Snapshotter](p)
		if !ok {
			t.Skip("provider does not implement Snapshotter")
		}
		ctx := testContext(t)
		spec := validSpec(os)

		inst, err := p.Create(ctx, spec.Clone())
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := p.Wait(ctx, inst.ID, provider.Ready); err != nil {
			t.Fatalf("Wait Ready: %v", err)
		}
		if err := p.Start(ctx, inst.ID); err != nil {
			t.Fatalf("Start: %v", err)
		}
		if _, err := p.Wait(ctx, inst.ID, provider.Running); err != nil {
			t.Fatalf("Wait Running: %v", err)
		}

		snapshotID, err := snap.Snapshot(ctx, inst.ID)
		if err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		if snapshotID == "" {
			t.Fatal("Snapshot: empty id")
		}

		restored, err := snap.Restore(ctx, snapshotID, spec.Clone())
		if err != nil {
			t.Fatalf("Restore: %v", err)
		}
		if restored.State != provider.Ready {
			t.Fatalf("Restore state = %q, want Ready", restored.State)
		}

		_, err = snap.Restore(ctx, "unknown-snapshot-id", spec.Clone())
		if err == nil {
			t.Fatal("Restore unknown: want error")
		}
		if !errors.Is(err, provider.ErrNotFound) {
			t.Fatalf("Restore unknown: %v, want ErrNotFound", err)
		}

		wrongOS := spec.Clone()
		wrongOS.OS = provider.Windows
		_, err = snap.Restore(ctx, snapshotID, wrongOS)
		if err == nil {
			t.Fatal("Restore wrong OS: want error")
		}
		if !errors.Is(err, provider.ErrInvalidSpec) {
			t.Fatalf("Restore wrong OS: %v, want ErrInvalidSpec", err)
		}

		readyInst, err := p.Create(ctx, spec.Clone())
		if err != nil {
			t.Fatalf("Create for Ready snapshot: %v", err)
		}
		if _, err := p.Wait(ctx, readyInst.ID, provider.Ready); err != nil {
			t.Fatalf("Wait Ready: %v", err)
		}
		_, err = snap.Snapshot(ctx, readyInst.ID)
		if err == nil {
			t.Fatal("Snapshot on Ready: want error")
		}
		if !errors.Is(err, provider.ErrInvalidTransition) {
			t.Fatalf("Snapshot on Ready: %v, want ErrInvalidTransition", err)
		}
	})

	t.Run("GuestDial", func(t *testing.T) {
		p := newProvider(t)
		gc, ok := provider.As[provider.GuestConnector](p)
		if !ok {
			t.Skip("provider does not implement GuestConnector")
		}
		ctx := testContext(t)
		spec := validSpec(os)

		inst, err := p.Create(ctx, spec.Clone())
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := p.Wait(ctx, inst.ID, provider.Ready); err != nil {
			t.Fatalf("Wait Ready: %v", err)
		}
		if err := p.Start(ctx, inst.ID); err != nil {
			t.Fatalf("Start: %v", err)
		}
		if _, err := p.Wait(ctx, inst.ID, provider.Running); err != nil {
			t.Fatalf("Wait Running: %v", err)
		}

		conn, err := gc.DialGuest(ctx, inst.ID)
		if errors.Is(err, provider.ErrUnsupported) {
			t.Skip("provider cannot reach guests")
		}
		if err != nil {
			t.Fatalf("DialGuest: %v", err)
		}

		t.Run("Link", func(t *testing.T) {
			if o.guestLink == nil {
				t.Skip("no guest link check: WithGuestLink checks the guest protocol on the connection")
			}
			if err := o.guestLink(ctx, conn); err != nil {
				t.Fatalf("guest link: %v", err)
			}
		})
		_ = conn.Close()

		_, err = gc.DialGuest(ctx, "unknown-instance-id")
		if err == nil {
			t.Fatal("DialGuest unknown: want error")
		}
		if !errors.Is(err, provider.ErrNotFound) {
			t.Fatalf("DialGuest unknown: %v, want ErrNotFound", err)
		}
	})
}

// RunStateVolume checks the one-instance rule of a state disk in
// Spec.Disks on a provider that attaches state volumes, and that the
// provider attaches one only from boot. newVolume returns a new volume's
// host path in the form the provider takes: a disk file or a directory.
func RunStateVolume(t *testing.T, newProvider func(t *testing.T) provider.Provider, newVolume func(t *testing.T) string) {
	t.Helper()

	t.Run("OneInstanceAtATime", func(t *testing.T) {
		p := newProvider(t)
		ctx := testContext(t)
		vol, other := newVolume(t), newVolume(t)
		spec := validSpec(provider.Linux)
		spec.SetDisk(provider.Disk{Kind: provider.DiskState, Path: vol})

		first, err := p.Create(ctx, spec.Clone())
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if got, err := p.Get(ctx, first.ID); err != nil || !slices.Equal(got.Spec.Disks, spec.Disks) {
			t.Fatalf("Get = %+v, %v, want Disks %+v", got.Spec, err, spec.Disks)
		}
		if _, err := p.Create(ctx, spec.Clone()); !errors.Is(err, provider.ErrInUse) {
			t.Fatalf("second Create with the same volume = %v, want ErrInUse", err)
		}
		otherSpec := spec.Clone()
		otherSpec.SetDisk(provider.Disk{Kind: provider.DiskState, Path: other})
		if _, err := p.Create(ctx, otherSpec); err != nil {
			t.Fatalf("Create with another volume: %v", err)
		}

		if err := p.Destroy(ctx, first.ID); err != nil {
			t.Fatalf("Destroy: %v", err)
		}
		if _, err := p.Wait(ctx, first.ID, provider.Destroyed); err != nil {
			t.Fatalf("Wait destroyed: %v", err)
		}
		if _, err := p.Create(ctx, spec.Clone()); err != nil {
			t.Fatalf("Create after the holder was destroyed: %v", err)
		}
	})

	t.Run("NotSnapshotted", func(t *testing.T) {
		p := newProvider(t)
		snap, ok := provider.As[provider.Snapshotter](p)
		if !ok {
			t.Skip("provider takes no snapshots")
		}
		ctx := testContext(t)
		spec := validSpec(provider.Linux)
		spec.SetDisk(provider.Disk{Kind: provider.DiskState, Path: newVolume(t)})
		inst, err := p.Create(ctx, spec)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := p.Wait(ctx, inst.ID, provider.Ready); err != nil {
			t.Fatalf("Wait ready: %v", err)
		}
		if err := p.Start(ctx, inst.ID); err != nil {
			t.Fatalf("Start: %v", err)
		}
		if _, err := p.Wait(ctx, inst.ID, provider.Running); err != nil {
			t.Fatalf("Wait running: %v", err)
		}
		if _, err := snap.Snapshot(ctx, inst.ID); !errors.Is(err, provider.ErrUnsupported) {
			t.Fatalf("Snapshot = %v, want ErrUnsupported", err)
		}
		if _, err := snap.Restore(ctx, "missing", spec); !errors.Is(err, provider.ErrUnsupported) {
			t.Fatalf("Restore with a state volume = %v, want ErrUnsupported", err)
		}
	})

	t.Run("NotHotPlugged", func(t *testing.T) {
		p := newProvider(t)
		disks, ok := provider.As[provider.Disks](p)
		if !ok {
			t.Skip("provider attaches no disks to a running instance")
		}
		ctx := testContext(t)
		inst, err := p.Create(ctx, validSpec(provider.Linux))
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		state := provider.Disk{Kind: provider.DiskState, Path: newVolume(t)}
		if err := disks.AttachDisk(ctx, inst.ID, state); !errors.Is(err, provider.ErrUnsupported) {
			t.Fatalf("AttachDisk(state) = %v, want ErrUnsupported", err)
		}
	})
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), subtestTimeout)
	t.Cleanup(cancel)
	return ctx
}

func validSpec(os provider.OS) provider.Spec {
	return provider.Spec{
		OS:        os,
		CPUs:      2,
		MemoryMiB: 1024,
		Metadata:  map[string]string{"role": "runner"},
		Labels:    map[string]string{"pool": "default"},
	}
}

func assertSpecRoundTrip(t *testing.T, want, got provider.Spec) {
	t.Helper()
	if got.OS != want.OS {
		t.Fatalf("Spec.OS = %q, want %q", got.OS, want.OS)
	}
	if got.CPUs != want.CPUs {
		t.Fatalf("Spec.CPUs = %d, want %d", got.CPUs, want.CPUs)
	}
	if got.MemoryMiB != want.MemoryMiB {
		t.Fatalf("Spec.MemoryMiB = %d, want %d", got.MemoryMiB, want.MemoryMiB)
	}
	if !reflect.DeepEqual(got.Metadata, want.Metadata) {
		t.Fatalf("Spec.Metadata = %v, want %v", got.Metadata, want.Metadata)
	}
	if !reflect.DeepEqual(got.Labels, want.Labels) {
		t.Fatalf("Spec.Labels = %v, want %v", got.Labels, want.Labels)
	}
	if got.Key != want.Key {
		t.Fatalf("Spec.Key = %q, want %q", got.Key, want.Key)
	}
	if got.Workspace != want.Workspace {
		t.Fatalf("Spec.Workspace = %q, want %q", got.Workspace, want.Workspace)
	}
}

// assertKeyInUse checks that err is the *KeyInUseError for spec's key, naming
// instance id, and wraps ErrKeyInUse.
func assertKeyInUse(t *testing.T, err error, spec provider.Spec, id string) {
	t.Helper()
	if !errors.Is(err, provider.ErrKeyInUse) {
		t.Fatalf("Create with key %q in use: %v, want ErrKeyInUse", spec.Key, err)
	}
	var inUse *provider.KeyInUseError
	if !errors.As(err, &inUse) {
		t.Fatalf("Create with key %q in use: %v, want a *KeyInUseError", spec.Key, err)
	}
	if inUse.ID != id || inUse.Key != spec.Key || inUse.Workspace != spec.Workspace {
		t.Fatalf("KeyInUseError = %+v, want ID %s, Key %q, Workspace %q", *inUse, id, spec.Key, spec.Workspace)
	}
}

// resourceCounter returns p as a ResourceCounter, or skips the subtest.
func resourceCounter(t *testing.T, p provider.Provider) ResourceCounter {
	t.Helper()
	counter, ok := p.(ResourceCounter)
	if !ok {
		t.Skip("provider does not implement ResourceCounter")
	}
	return counter
}

// providerRestarter returns p as a Restarter, or skips the subtest.
func providerRestarter(t *testing.T, p provider.Provider) Restarter {
	t.Helper()
	restarter, ok := p.(Restarter)
	if !ok {
		t.Skip("provider does not implement Restarter")
	}
	return restarter
}

// assertNoKeyHolder checks that List shows no instance that is not destroyed
// with spec's Key in its Workspace.
func assertNoKeyHolder(t *testing.T, p provider.Provider, ctx context.Context, spec provider.Spec) {
	t.Helper()
	list, err := p.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, inst := range list {
		if inst.State != provider.Destroyed && spec.SameKey(inst.Spec) {
			t.Fatalf("List shows %s (%s) with key %q in %q, want none live", inst.ID, inst.State, spec.Key, spec.Workspace)
		}
	}
}

// assertKeyHolders checks that List shows exactly one instance that is not
// destroyed with spec's Key in its Workspace, and that it is id.
func assertKeyHolders(t *testing.T, p provider.Provider, ctx context.Context, spec provider.Spec, id string) {
	t.Helper()
	list, err := p.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var holders []string
	for _, inst := range list {
		if inst.State != provider.Destroyed && spec.SameKey(inst.Spec) {
			holders = append(holders, inst.ID)
		}
	}
	if len(holders) != 1 || holders[0] != id {
		t.Fatalf("live instances with key %q in %q = %v, want [%s]", spec.Key, spec.Workspace, holders, id)
	}
}

func indexByID(list []provider.Instance) map[string]provider.Instance {
	m := make(map[string]provider.Instance, len(list))
	for _, inst := range list {
		m[inst.ID] = inst
	}
	return m
}

func assertAbsentFromList(t *testing.T, p provider.Provider, ctx context.Context, id string) {
	t.Helper()
	list, err := p.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, item := range list {
		if item.ID == id {
			t.Fatalf("List contains id %q, want absent", id)
		}
	}
}

func assertPresentDestroyedInList(t *testing.T, p provider.Provider, ctx context.Context, id string) {
	t.Helper()
	list, err := p.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, item := range list {
		if item.ID == id {
			if item.State != provider.Destroyed {
				t.Fatalf("List entry state = %q, want Destroyed", item.State)
			}
			return
		}
	}
	t.Fatalf("List missing destroyed id %q", id)
}
