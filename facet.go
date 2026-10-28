// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"reflect"
	"slices"
)

// Facet names one of the optional interfaces of this package, which a
// Provider or a Backend implements beyond its own methods.
type Facet string

// The facets, one per optional interface. The first three are a Backend's;
// the others a Provider's.
const (
	FacetPreflighter    Facet = "preflighter"
	FacetCapable        Facet = "capable"
	FacetStatusReporter Facet = "status_reporter"

	FacetCloser             Facet = "closer"
	FacetDisks              Facet = "disks"
	FacetCacheDirectories   Facet = "cache_directories"
	FacetDiskData           Facet = "disk_data"
	FacetDiskExporter       Facet = "disk_exporter"
	FacetInstancesGone      Facet = "instances_gone"
	FacetSnapshotter        Facet = "snapshotter"
	FacetImageTracker       Facet = "image_tracker"
	FacetSnapshotReferrer   Facet = "snapshot_referrer"
	FacetCollector          Facet = "collector"
	FacetForgetter          Facet = "forgetter"
	FacetGuestConnector     Facet = "guest_connector"
	FacetDisplayConnector   Facet = "display_connector"
	FacetScreenCapturer     Facet = "screen_capturer"
	FacetGuestExecutions    Facet = "guest_executions"
	FacetDebugForwarder     Facet = "debug_forwarder"
	FacetPreviewForwarder   Facet = "preview_forwarder"
	FacetConsoleLogKeeper   Facet = "console_log_keeper"
	FacetConsoleReader      Facet = "console_reader"
	FacetMemoryReclaimer    Facet = "memory_reclaimer"
	FacetMemoryHighSetter   Facet = "memory_high_setter"
	FacetProcessLimitReader Facet = "process_limit_reader"
	FacetUsageReader        Facet = "usage_reader"
	FacetHostMemoryReader   Facet = "host_memory_reader"
	FacetDiskCapturer       Facet = "disk_capturer"
)

// FacetLister is a Provider or a Backend that lists the facets it offers.
// A caller then uses only those: a value may have an interface's methods
// and still not offer it, as a provider in another process has every
// method and offers what the process behind it serves. A value that lists
// no facets offers each interface it implements.
type FacetLister interface {
	Facets() []Facet
}

// facetTypes are the interfaces the facets name, in the order of the
// constants.
var facetTypes = []struct {
	facet Facet
	typ   reflect.Type
}{
	{FacetPreflighter, reflect.TypeFor[Preflighter]()},
	{FacetCapable, reflect.TypeFor[Capable]()},
	{FacetStatusReporter, reflect.TypeFor[StatusReporter]()},
	{FacetCloser, reflect.TypeFor[Closer]()},
	{FacetDisks, reflect.TypeFor[Disks]()},
	{FacetCacheDirectories, reflect.TypeFor[CacheDirectories]()},
	{FacetDiskData, reflect.TypeFor[DiskData]()},
	{FacetDiskExporter, reflect.TypeFor[DiskExporter]()},
	{FacetInstancesGone, reflect.TypeFor[InstancesGone]()},
	{FacetSnapshotter, reflect.TypeFor[Snapshotter]()},
	{FacetImageTracker, reflect.TypeFor[ImageTracker]()},
	{FacetSnapshotReferrer, reflect.TypeFor[SnapshotReferrer]()},
	{FacetCollector, reflect.TypeFor[Collector]()},
	{FacetForgetter, reflect.TypeFor[Forgetter]()},
	{FacetGuestConnector, reflect.TypeFor[GuestConnector]()},
	{FacetDisplayConnector, reflect.TypeFor[DisplayConnector]()},
	{FacetScreenCapturer, reflect.TypeFor[ScreenCapturer]()},
	{FacetGuestExecutions, reflect.TypeFor[GuestExecutions]()},
	{FacetDebugForwarder, reflect.TypeFor[DebugForwarder]()},
	{FacetPreviewForwarder, reflect.TypeFor[PreviewForwarder]()},
	{FacetConsoleLogKeeper, reflect.TypeFor[ConsoleLogKeeper]()},
	{FacetConsoleReader, reflect.TypeFor[ConsoleReader]()},
	{FacetMemoryReclaimer, reflect.TypeFor[MemoryReclaimer]()},
	{FacetMemoryHighSetter, reflect.TypeFor[MemoryHighSetter]()},
	{FacetProcessLimitReader, reflect.TypeFor[ProcessLimitReader]()},
	{FacetUsageReader, reflect.TypeFor[UsageReader]()},
	{FacetHostMemoryReader, reflect.TypeFor[HostMemoryReader]()},
	{FacetDiskCapturer, reflect.TypeFor[DiskCapturer]()},
}

// As returns v as the optional interface T when v offers it: v implements
// T and, when v is a FacetLister, lists T's facet. Callers use it in place
// of a type assertion to T.
func As[T any](v any) (T, bool) {
	t, ok := v.(T)
	if !ok {
		return t, false
	}
	if l, ok := v.(FacetLister); ok {
		var zero T
		f, known := facetOf(reflect.TypeFor[T]())
		if !known || !slices.Contains(l.Facets(), f) {
			return zero, false
		}
	}
	return t, true
}

// Has reports whether v offers the facet f, as As does for f's interface.
// An unknown facet is never offered.
func Has(v any, f Facet) bool {
	typ, known := typeOf(f)
	if !known || v == nil || !reflect.TypeOf(v).Implements(typ) {
		return false
	}
	if l, ok := v.(FacetLister); ok {
		return slices.Contains(l.Facets(), f)
	}
	return true
}

// Implemented returns the facets whose interfaces v implements, whatever
// it lists, in the order of the constants. A FacetLister's list is these,
// or fewer.
func Implemented(v any) []Facet {
	if v == nil {
		return nil
	}
	vt := reflect.TypeOf(v)
	var out []Facet
	for _, ft := range facetTypes {
		if vt.Implements(ft.typ) {
			out = append(out, ft.facet)
		}
	}
	return out
}

func facetOf(typ reflect.Type) (Facet, bool) {
	for _, ft := range facetTypes {
		if ft.typ == typ {
			return ft.facet, true
		}
	}
	return "", false
}

func typeOf(f Facet) (reflect.Type, bool) {
	for _, ft := range facetTypes {
		if ft.facet == f {
			return ft.typ, true
		}
	}
	return nil, false
}
