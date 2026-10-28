// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"time"
)

// OS is the guest operating system of an instance.
type OS string

// The guest operating systems Toshi runs.
const (
	Linux   OS = "linux"
	Windows OS = "windows"
	MacOS   OS = "macos"
)

// State is where an instance is in its lifecycle.
//
// The normal path is creating → ready → running → stopping → stopped.
// Destroy moves an instance from any state to destroyed, and a provider
// error moves it from any state to failed.
type State string

// The lifecycle states of an instance.
const (
	Creating  State = "creating"
	Ready     State = "ready"
	Running   State = "running"
	Stopping  State = "stopping"
	Stopped   State = "stopped"
	Failed    State = "failed"
	Destroyed State = "destroyed"
)

// DiskKind is what a Disk holds. It fixes how the guest finds the disk.
type DiskKind string

// The kinds of Disk.
const (
	// DiskCache is the job's workspace cache, mounted on /cache.
	DiskCache DiskKind = "cache"
	// DiskState is a state volume, read-write in one instance
	// at a time.
	DiskState DiskKind = "state"
)

// How a guest finds each kind of disk.
const (
	// CacheDiskSerial is the serial of the virtio disk that holds a VM's
	// workspace cache.
	CacheDiskSerial = "toshi-cache"
	// StateVolumeSerial is the serial of the virtio disk that holds a VM's
	// state volume.
	StateVolumeSerial = "toshi-volume"
	// StateVolumeMount is where a container has its state volume.
	StateVolumeMount = "/volume"
)

// Serial is the virtio serial a guest finds a disk of kind k by.
func (k DiskKind) Serial() string {
	if k == DiskState {
		return StateVolumeSerial
	}
	return CacheDiskSerial
}

// Disk is a host disk given to an instance: a disk file, or a directory
// for a provider whose disks are directories (the container provider). It
// reaches the instance in one of two ways, and which one a provider takes
// for a kind is a property of that kind in that provider:
//   - from boot, in Spec.Disks;
//   - hot-plugged into a running instance, by Disks.AttachDisk.
//
// The qemu provider hot-plugs a cache disk into its one cache port and
// attaches a state volume from boot; the container provider mounts both
// from Create.
type Disk struct {
	Kind DiskKind `json:"kind"`
	// Path is the disk's host path.
	Path string `json:"path"`
}

// validate checks d as an entry of Spec.Disks.
func (d Disk) validate() error {
	switch {
	case d.Kind != DiskCache && d.Kind != DiskState:
		return fmt.Errorf("%w: unknown disk kind %q", ErrInvalidSpec, d.Kind)
	case !filepath.IsAbs(d.Path) || filepath.Clean(d.Path) != d.Path:
		// A clean path, so that one volume has one lexical spelling for
		// the one-instance check. Symlinks are not resolved.
		return fmt.Errorf("%w: %s disk path must be a clean absolute path, got %q", ErrInvalidSpec, d.Kind, d.Path)
	}
	return nil
}

// Spec describes the VM to create.
type Spec struct {
	// Image names the guest image to boot. Its format is provider-specific.
	Image string
	// OS is the guest operating system. Required.
	OS OS
	// CPUs is the number of virtual CPUs. Must be greater than zero.
	CPUs int
	// MemoryMiB is the guest memory in MiB. Must be greater than zero.
	MemoryMiB int
	// DiskGiB is the size of the guest disk in GiB. Zero means the image's
	// own size.
	DiskGiB int
	// NestedKVM gives a Linux guest the host's hardware virtualization (svm
	// or vmx), so the guest has /dev/kvm. Off, the guest has none.
	// Only Linux guests may set it.
	NestedKVM bool `json:"nested_kvm,omitempty"`
	// DisplayInput gives a Windows guest a USB controller and a tablet, so a
	// display session can point and click on its screen. Off, the guest has
	// no USB device. Only Windows guests may set it.
	DisplayInput bool `json:"display_input,omitempty"`
	// CPUWeight, when set, is the cgroup v2 cpu.weight (1-10000) of the
	// instance's own cgroup, and MemoryMaxMiB its memory.max: a filler VM
	// runs at weight 1 under a memory cap. Providers that cannot
	// apply them refuse the spec.
	CPUWeight    int `json:"cpu_weight,omitempty"`
	MemoryMaxMiB int `json:"memory_max_mib,omitempty"`
	// CPUMaxPercent caps the instance's CPU time at this percent of one
	// CPU; 0 means no cap.
	CPUMaxPercent int `json:"cpu_max_percent,omitempty"`
	// ImageFile, when set, is the absolute path of a qcow2 overlay the
	// instance boots from in place of Image's base: an image version whose
	// backing file is named by Image's base file name alone. Only a provider
	// that implements ImageBooter accepts it; it must find that base file
	// beside ImageFile, as the one Image names, and refuses any other chain.
	// Others refuse the spec. An instance with an ImageFile is never
	// snapshotted, restored or captured.
	ImageFile string `json:"image_file,omitempty"`
	// ShareDir, when set, is a host directory a Linux guest mounts over 9p
	// with the tag "toshi-share".
	ShareDir string `json:"share_dir,omitempty"`
	// Disks are host disks for a Linux guest, at most one of each kind,
	// with clean absolute paths, attached from boot by the providers each
	// kind below names. Use Disk, SetDisk and RemoveDisk.
	//   - A DiskCache disk is the host directory of the job's workspace cache,
	//     which a provider that implements CacheDirectories mounts on
	//     /cache in place of a new empty /cache. The agent sets it only for such a provider; others
	//     ignore it.
	//   - A DiskState disk is a state volume the guest has read-write: a qcow2 file the qemu provider attaches as a virtio
	//     disk with serial StateVolumeSerial, or a directory the container
	//     provider bind-mounts on StateVolumeMount. One instance at a time:
	//     Create returns ErrInUse while another instance of the provider
	//     that is not destroyed has the same path. An instance with a state
	//     volume is never snapshotted or restored.
	Disks []Disk `json:"disks,omitempty"`
	// OpenSource says the instance runs an open-source filler job: the host
	// firewall matches its connections, so providers that can place it in a
	// subgroup of the VM cgroup do, the OpenSourceCgroupName one; the qemu
	// provider puts its cgroup there.
	OpenSource bool `json:"open_source,omitempty"`
	// Metadata is handed to the guest unchanged. Providers do not interpret
	// it, except a Snapshotter its SnapshotMetadataKey.
	Metadata map[string]string
	// Labels are for finding instances in List. They are not passed to the
	// guest.
	Labels map[string]string
	// Key, when set, is a caller-chosen name for the instance, opaque to the
	// provider. Among the provider's instances that are not destroyed, at
	// most one has a given Key in a given Workspace: Create with the Key of
	// such an instance creates nothing and returns a *KeyInUseError naming
	// it, which wraps ErrKeyInUse. A Failed instance keeps its key until it
	// is destroyed; Destroy frees it. An empty Key is never in use.
	Key string `json:"key,omitempty"`
	// Workspace is the workspace the instance belongs to, opaque to the
	// provider. It scopes Key: one Key in two workspaces names two
	// instances. It is kept on the instance's Spec, as Get and List show it.
	Workspace string `json:"workspace,omitempty"`
}

// OpenSourceCgroupName is the subgroup of the VM cgroup that an OpenSource
// instance runs in: the host's firewall matches its own cgroup level, so it
// must be a level of its own, and the agent refuses open-source jobs when it
// is missing.
const OpenSourceCgroupName = "tier"

// SameKey reports whether s and o name one instance by Key: both have the
// same non-empty Key in the same Workspace.
func (s Spec) SameKey(o Spec) bool {
	return s.Key != "" && s.Key == o.Key && s.Workspace == o.Workspace
}

// Validate reports whether s can be used to create an instance. The error
// wraps ErrInvalidSpec.
func (s Spec) Validate() error {
	switch {
	case s.OS != Linux && s.OS != Windows && s.OS != MacOS:
		return fmt.Errorf("%w: unknown OS %q", ErrInvalidSpec, s.OS)
	case s.CPUs <= 0:
		return fmt.Errorf("%w: CPUs must be greater than zero, got %d", ErrInvalidSpec, s.CPUs)
	case s.MemoryMiB <= 0:
		return fmt.Errorf("%w: MemoryMiB must be greater than zero, got %d", ErrInvalidSpec, s.MemoryMiB)
	case s.DiskGiB < 0:
		return fmt.Errorf("%w: DiskGiB must not be negative, got %d", ErrInvalidSpec, s.DiskGiB)
	case s.NestedKVM && s.OS != Linux:
		return fmt.Errorf("%w: NestedKVM is for Linux guests, got OS %q", ErrInvalidSpec, s.OS)
	case s.DisplayInput && s.OS != Windows:
		return fmt.Errorf("%w: DisplayInput is for Windows guests, got OS %q", ErrInvalidSpec, s.OS)
	case s.CPUWeight < 0 || s.CPUWeight > 10000:
		return fmt.Errorf("%w: CPUWeight must be within 1-10000, got %d", ErrInvalidSpec, s.CPUWeight)
	case s.MemoryMaxMiB < 0:
		return fmt.Errorf("%w: MemoryMaxMiB must not be negative, got %d", ErrInvalidSpec, s.MemoryMaxMiB)
	case s.ImageFile != "" && s.OS != Linux:
		return fmt.Errorf("%w: ImageFile is for Linux guests, got OS %q", ErrInvalidSpec, s.OS)
	case s.ImageFile != "" && (!filepath.IsAbs(s.ImageFile) || filepath.Clean(s.ImageFile) != s.ImageFile):
		return fmt.Errorf("%w: ImageFile must be a clean absolute path, got %q", ErrInvalidSpec, s.ImageFile)
	case s.ShareDir != "" && s.OS != Linux:
		return fmt.Errorf("%w: ShareDir is for Linux guests, got OS %q", ErrInvalidSpec, s.OS)
	case len(s.Disks) > 0 && s.OS != Linux:
		return fmt.Errorf("%w: Disks are for Linux guests, got OS %q", ErrInvalidSpec, s.OS)
	}
	seen := make(map[DiskKind]bool, len(s.Disks))
	for _, d := range s.Disks {
		if err := d.validate(); err != nil {
			return err
		}
		if seen[d.Kind] {
			return fmt.Errorf("%w: more than one %s disk", ErrInvalidSpec, d.Kind)
		}
		seen[d.Kind] = true
	}
	return nil
}

// Disk returns s's disk of kind k, if it has one.
func (s Spec) Disk(k DiskKind) (Disk, bool) {
	for _, d := range s.Disks {
		if d.Kind == k {
			return d, true
		}
	}
	return Disk{}, false
}

// SetDisk gives s the disk d in place of any disk of d's kind. It never
// writes to the slice s had, which a clone of s may share.
func (s *Spec) SetDisk(d Disk) {
	s.RemoveDisk(d.Kind)
	s.Disks = append(s.Disks, d)
}

// RemoveDisk takes s's disk of kind k away. It never writes to the slice
// s had.
func (s *Spec) RemoveDisk(k DiskKind) {
	var kept []Disk
	for _, d := range s.Disks {
		if d.Kind != k {
			kept = append(kept, d)
		}
	}
	s.Disks = kept
}

// Clone returns a copy of s that shares no maps or slices with it.
func (s Spec) Clone() Spec {
	s.Metadata = maps.Clone(s.Metadata)
	s.Labels = maps.Clone(s.Labels)
	s.Disks = slices.Clone(s.Disks)
	return s
}

// Instance is a snapshot of one VM at the moment a provider call returned.
// It is a value, not a handle: it does not change when the VM does. Call Get
// or Wait for a newer snapshot.
type Instance struct {
	// ID identifies the instance within its provider.
	ID string
	// Spec is the spec the instance was created with.
	Spec Spec
	// State is the lifecycle state when the snapshot was taken.
	State State
	// CreatedAt is when the provider accepted the Create or Restore call.
	CreatedAt time.Time
	// Error says what went wrong. It is set only when State is Failed.
	Error string
}

// Provider starts and stops runner VMs on one kind of backend: a local
// hypervisor or a cloud account. The scheduler and the host agent use only
// this interface.
//
// Methods are safe for concurrent use. Errors wrap the sentinel errors of
// this package where one applies, so callers test them with errors.Is.
type Provider interface {
	// Name identifies the provider, for logs and metrics.
	Name() string

	// Create validates spec and starts creating an instance. It returns the
	// instance in the Creating or Ready state; readiness is asynchronous, so
	// callers use Wait to see it become Ready. An invalid spec wraps
	// ErrInvalidSpec; a valid spec the provider cannot run wraps
	// ErrUnsupported. A spec whose Key is in use returns a *KeyInUseError
	// (see Spec.Key). Create checks the key and records the instance in one
	// step, so of concurrent Creates with one Key and Workspace exactly one
	// succeeds. When ctx ends before Create returns, the error wraps
	// ctx.Err() and nothing of the instance remains: no instance holds the
	// spec's Key and no resource was kept.
	Create(ctx context.Context, spec Spec) (Instance, error)

	// Get returns the current snapshot of an instance. A destroyed instance
	// is returned in the Destroyed state until the provider forgets it; an
	// id the provider does not know wraps ErrNotFound.
	Get(ctx context.Context, id string) (Instance, error)

	// List returns every instance the provider still knows, destroyed ones
	// included until they are forgotten, in no particular order.
	List(ctx context.Context) ([]Instance, error)

	// Start moves a Ready instance to Running. From any other state it
	// wraps ErrInvalidTransition.
	Start(ctx context.Context, id string) error

	// Stop moves a Running instance through Stopping to Stopped. From any
	// other state it wraps ErrInvalidTransition.
	Stop(ctx context.Context, id string) error

	// Destroy releases the instance and moves it to Destroyed from any
	// state, Failed included. Destroying a destroyed instance succeeds.
	Destroy(ctx context.Context, id string) error

	// Wait blocks until the instance is in state want and returns that
	// snapshot. It returns an error, with the latest snapshot, when:
	//   - the instance reaches Destroyed, or reaches Failed while want is not
	//     Destroyed, before reaching want;
	//   - want lies behind the current state on the normal path and can no
	//     longer be reached (the error wraps ErrInvalidTransition);
	//   - ctx ends (the error wraps ctx.Err()).
	// An unknown id wraps ErrNotFound.
	Wait(ctx context.Context, id string, want State) (Instance, error)
}

// Closer is implemented by providers that hold work of their own beyond
// their instances, such as connections or watchers. The agent calls Close
// once, after it has stopped, on a provider that implements it. Close stops
// that work and leaves the instances as they are; ctx bounds how long it
// may take.
type Closer interface {
	Close(ctx context.Context) error
}

// Disks is implemented by providers that keep workspace cache disks. A provider
// that can give a running VM a disk hot-plugs it with AttachDisk; one that also
// implements CacheDirectories mounts its disks at Create instead. Paths are
// files on the host, or directories for a CacheDirectories provider. The agent
// owns their lifecycle; the provider only knows their format.
type Disks interface {
	// CreateCacheBase creates a blank cache disk of sizeGiB at path.
	CreateCacheBase(ctx context.Context, path string, sizeGiB int) error

	// CreateCacheOverlay creates a copy-on-write disk at path whose
	// backing file is backing, which it never writes.
	CreateCacheOverlay(ctx context.Context, path, backing string) error

	// CacheRequiredBytes returns the file size a flattened copy of path
	// and its backing chain would need.
	CacheRequiredBytes(ctx context.Context, path string) (int64, error)

	// FlattenCache writes path and its backing chain to dst as one disk
	// without a backing file.
	FlattenCache(ctx context.Context, path, dst string) error

	// AttachDisk hot-plugs d into running instance id as a disk the guest
	// finds by d.Kind's serial. A kind the provider attaches only from
	// boot (Spec.Disks) returns ErrUnsupported: a state volume everywhere,
	// and every kind in a CacheDirectories provider.
	AttachDisk(ctx context.Context, id string, d Disk) error
}

// DiskData is implemented by a Disks provider that can tell whether a disk
// was ever written. A state volume's disk made blank for a first mount is
// removed when its job ends only while it holds no written data.
type DiskData interface {
	// DiskHasData reports whether the disk at path holds written data,
	// whatever it is. It reads path without writing it, once no VM uses
	// it; an error means it cannot tell.
	DiskHasData(ctx context.Context, path string) (bool, error)
}

// DiskExporter is implemented by a Disks provider that can write a disk as
// one standalone file, the bytes a state volume's saved version holds. A
// host that fetches the file attaches it as the volume's disk as it is.
type DiskExporter interface {
	// ExportDisk writes the disk at src to dst, a file it creates, once no
	// VM uses src: standalone and compressed, of the same virtual size. It
	// reads src without writing it.
	ExportDisk(ctx context.Context, src, dst string) error
}

// CacheDirectories is implemented by a Disks provider whose caches are
// directories, not disk files: the container provider. With it, CreateCacheBase
// makes an empty directory, CreateCacheOverlay a copy of the backing directory,
// CacheRequiredBytes returns the directory's bytes and FlattenCache renames
// path to dst. The agent hands the job's directory to Create as a DiskCache
// entry of Spec.Disks and never calls AttachDisk.
type CacheDirectories interface {
	Disks
	// CacheDirectories reports whether caches are directories now: false
	// when the provider cannot keep them on this host, which then keeps
	// no cache.
	CacheDirectories() bool
}

// InstancesGone is implemented by providers that can say whether an instance
// an earlier agent process left on this host is positively gone. The agent
// asks it once, when it starts, for each state volume claim a dead agent left
// behind; a provider that does not implement it cannot tell, and the agent
// then keeps those claims.
type InstancesGone interface {
	// StateRoot is the absolute, cleaned directory the provider keeps its
	// instance state in. A claim record stores it, with Name, as the
	// provider's identity.
	StateRoot() string

	// InstanceGone reports whether instance id is positively gone. gone is
	// true only when the provider's start-up scan read its state root and the
	// instance's directory is not there. certain is false when the provider
	// cannot tell, and the caller then keeps the claim.
	InstanceGone(id string) (gone, certain bool)

	// Unreaped reports how many instance directories the provider's start-up
	// scan left behind because it could not confirm them gone, and certain
	// is false when the scan did not read the state root. Zero with certain
	// true means the scan left nothing unconfirmed.
	Unreaped() (left int, certain bool)
}

// SnapshotMetadataKey is the Spec.Metadata key that holds the id of the
// snapshot an instance was restored from: the caller of Restore sets it, and
// the instance Restore returns has it. A guest in an instance that has it
// was restored with its memory and does not send ready again.
const SnapshotMetadataKey = "toshi.snapshot"

// Snapshotter is implemented by providers that can save and restore a VM.
// Callers find it with a type assertion on a Provider.
type Snapshotter interface {
	// Snapshot saves a Running or Stopped instance and returns an id for
	// Restore. From any other state it wraps ErrInvalidTransition.
	Snapshot(ctx context.Context, id string) (snapshotID string, err error)

	// Restore creates a new instance from a snapshot and returns it in the
	// Ready state. spec must be valid and use the snapshot's OS, otherwise
	// the error wraps ErrInvalidSpec; an unknown snapshot wraps ErrNotFound.
	// An ImageTracker refuses a snapshot whose base image has changed since
	// with an error that wraps ErrImageChanged.
	Restore(ctx context.Context, snapshotID string, spec Spec) (Instance, error)
}

// ImageTracker is implemented by Snapshotters that can tell whether the base
// image a snapshot came from is still the one on disk. Callers
// find it with a type assertion on a Provider. An identity is opaque: two
// are the same image only when they are equal strings.
type ImageTracker interface {
	// ImageIdentity names the base image spec boots from as it is on disk
	// now.
	ImageIdentity(ctx context.Context, spec Spec) (string, error)

	// SnapshotImageIdentity names the base image the snapshot was taken
	// from, or "" when the snapshot did not record one. An unknown snapshot
	// wraps ErrNotFound.
	SnapshotImageIdentity(ctx context.Context, snapshotID string) (string, error)
}

// SnapshotRefs is what the agent's warm pools hold of the snapshots: the
// snapshot each pool restores from, by runner size, and the snapshots the pools
// stopped using, with when they did.
type SnapshotRefs struct {
	Pools   map[string]string
	Dropped map[string]time.Time
}

// SnapshotReferrer is implemented by Snapshotters that collect the
// snapshots nothing uses. The agent calls SetSnapshotRefs each
// time a pool's snapshot changes. Callers find it with a type assertion on
// a Provider.
type SnapshotReferrer interface {
	SetSnapshotRefs(ctx context.Context, refs SnapshotRefs) error
}

// Collector is implemented by providers that keep instance and snapshot
// files on the host and can remove the ones nothing references.
// Callers find it with a type assertion on a Provider.
type Collector interface {
	// Collect removes what nothing has referenced for grace, at most
	// maxPerPass entries, and returns the bytes it gave back.
	Collect(ctx context.Context, grace time.Duration, maxPerPass int) (int64, error)
}

// DiskCapturer is implemented by providers that can save a stopped
// instance's root disk as an overlay on the base image it booted from.
// Callers find it with a type assertion on a Provider.
type DiskCapturer interface {
	// CaptureDisk writes the root disk of a Stopped instance to dst as a
	// compressed qcow2 overlay whose backing file is named by the base
	// image's file name alone, so a host that holds the same base can use
	// it. dst is replaced only once it is complete. From any other state
	// it wraps ErrInvalidTransition; an instance with a state volume, or a
	// base image whose SHA-256 the provider does not know, wraps
	// ErrUnsupported; a base image that changed on disk since the instance
	// booted wraps ErrImageChanged.
	CaptureDisk(ctx context.Context, id, dst string) (DiskCapture, error)
}

// DiskCapture is what a captured disk is an overlay on.
type DiskCapture struct {
	// Base is the base image's name, such as "ubuntu-24.04".
	Base string
	// BaseDigest is the base image's SHA-256 in hex.
	BaseDigest string
}

// ImageBooter is implemented by providers that boot an instance from an
// image version, Spec.ImageFile. Callers find it with a type assertion on a
// Provider.
type ImageBooter interface {
	// BaseImages lists the bases the provider boots that an image version
	// can be an overlay on, each with its SHA-256 as the provider knows it.
	// A base whose SHA-256 the provider does not know is left out.
	BaseImages() ([]BaseImage, error)
}

// BaseImage is a base an image version can be an overlay on.
type BaseImage struct {
	// Name is the base's name, such as "ubuntu-24.04", as Spec.Image
	// names it.
	Name string
	// SHA256 is the base file's SHA-256 in hex.
	SHA256 string
}

// Forgetter is implemented by providers that drop a destroyed instance from
// their bookkeeping. Callers find it with a type assertion on a Provider.
type Forgetter interface {
	// Forget removes a destroyed instance. After it, Get and Wait wrap
	// ErrNotFound and List leaves it out. An unknown id wraps ErrNotFound.
	// An instance that is not Destroyed wraps ErrInvalidTransition.
	Forget(id string) error
}
