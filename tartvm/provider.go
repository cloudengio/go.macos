// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package tartvm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"cloudeng.io/errors"
	"cloudeng.io/vms"
	"cloudeng.io/vms/ociimage"
	"cloudeng.io/vms/vmspool"
)

// Constructor creates a new, uninitialized tart VM instance. Each call must
// return a distinct vms.Instance (typically via New with a unique name). ctx
// governs any work done to construct the instance. It returns an error if the
// instance could not be created.
type Constructor = func(ctx context.Context) (vms.Instance, error)

// Provider is a vmspool.Provider backed by tart. It delegates VM construction to
// a caller-supplied Constructor and implements List, Get and Delete directly via
// the tart CLI, so using tart with a vmspool.Pool only requires supplying the
// construction function.
type Provider struct {
	constructor Constructor
	prefix      string              // if set, only VMs whose name has this prefix are managed
	pool        string              // optional pool name used to tag VMInfo.Pool
	tartBinary  string              // optional tart binary path, defaults to "tart"
	image       ociimage.Reference  // optional image the constructor clones, reported by Details
	resources   vmspool.VMResources // optional resources the constructor creates VMs with, reported by DefaultResources
}

var (
	_ vmspool.Provider = (*Provider)(nil)
	_ ociimage.Puller  = (*Provider)(nil)
)

// ProviderName is the name a tart Provider reports, see Provider.Name.
const ProviderName = "tart"

// Name implements vmspool.Provider by naming the VM technology, which is what
// identifies the provider behind a VM in status output and in errors.
func (p *Provider) Name() string { return ProviderName }

// DefaultResources implements vmspool.Provider, reporting the resources the
// Provider's VMs are created with, see WithProviderResources. As with the
// image, a Provider cannot discover them for itself, and a zero figure means
// unknown: tart keeps the image's own value for anything the clone did not set,
// and Get reports what a VM actually has once it exists.
func (p *Provider) DefaultResources() vmspool.VMResources { return p.resources }

// providerDetails reports what a Provider's VMs are created from.
type providerDetails struct {
	image ociimage.Reference
}

// Image implements vmspool.ProviderDetails.
func (d providerDetails) Image() ociimage.Reference { return d.image }

// Details implements vmspool.Provider, reporting the image the Provider's VMs
// are cloned from, or the zero Reference if it was not configured with one: a
// Provider creates its VMs through a caller-supplied Constructor and so cannot
// discover the image for itself, see WithProviderImage.
func (p *Provider) Details() vmspool.ProviderDetails {
	return providerDetails{image: p.image}
}

// ProviderOption configures a Provider.
type ProviderOption func(*Provider)

// WithNamePrefix scopes List and Delete to VMs whose names start with prefix.
// Without it a Provider manages every local tart VM, which is rarely desirable
// when more than one pool shares the host.
func WithNamePrefix(prefix string) ProviderOption {
	return func(p *Provider) { p.prefix = prefix }
}

// WithPoolName sets the pool name reported in VMInfo.Pool for the Provider's VMs.
func WithPoolName(name string) ProviderOption {
	return func(p *Provider) { p.pool = name }
}

// WithProviderImage records the image the Provider's Constructor clones its VMs
// from, which Details then reports and which Provider.Pull pulls. The
// Constructor owns the creation of a VM, so this is what it was told to clone,
// stated for anything reporting on the pool.
func WithProviderImage(ref ociimage.Reference) ProviderOption {
	return func(p *Provider) { p.image = ref }
}

// WithProviderResources records the resources the Provider's Constructor
// creates its VMs with, which DefaultResources then reports, see WithResources
// for the option that applies them to a VM. Like WithProviderImage it only
// states what the Constructor was told to do.
func WithProviderResources(resources vmspool.VMResources) ProviderOption {
	return func(p *Provider) { p.resources = resources }
}

// WithProviderTartBinary sets the tart binary path to use for all Provider operations.
func WithProviderTartBinary(path string) ProviderOption {
	return func(p *Provider) { p.tartBinary = path }
}

// NewProvider returns a Provider that constructs VMs with constructor and
// implements the remaining vmspool.Provider methods via the tart CLI.
func NewProvider(constructor Constructor, opts ...ProviderOption) *Provider {
	p := &Provider{constructor: constructor}
	for _, opt := range opts {
		opt(p)
	}
	if p.tartBinary == "" {
		p.tartBinary = DefaultTartBinary
	}
	return p
}

// New implements vmspool.Provider.
func (p *Provider) New(ctx context.Context) (vms.Instance, error) {
	if p.constructor == nil {
		return nil, fmt.Errorf("no constructor was provided in call to tartvm.NewProvider")
	}
	return p.constructor(ctx)
}

// tartLocalSource is the value that "tart list" reports for locally created VMs,
// as opposed to OCI images that have been pulled.
const tartLocalSource = "local"

// List implements vmspool.Provider, returning the local tart VMs whose names
// match the configured prefix.
func (p *Provider) List(ctx context.Context) ([]vmspool.VMInfo, error) {
	all, err := ListAll(ctx, p.tartBinary)
	if err != nil {
		return nil, err
	}
	var out []vmspool.VMInfo
	for _, e := range all {
		if e.Source != tartLocalSource {
			continue
		}
		if p.prefix != "" && !strings.HasPrefix(e.Name, p.prefix) {
			continue
		}
		out = append(out, vmspool.VMInfo{
			Name:     e.Name,
			Pool:     p.pool,
			State:    e.State,
			Running:  e.Running,
			Accessed: e.Accessed,
		})
	}
	return out, nil
}

// Get implements vmspool.Provider, returning the resources allocated to a single
// VM via "tart get".
func (p *Provider) Get(ctx context.Context, vmName string) (vmspool.VMDetail, error) {
	out, err := runTartOut(ctx, p.tartBinary, "get", vmName, "--format", "json")
	if err != nil {
		return vmspool.VMDetail{}, err
	}
	var g struct {
		CPU     int    `json:"CPU"`
		Memory  int    `json:"Memory"` // MiB
		Disk    int    `json:"Disk"`   // GiB
		Running bool   `json:"Running"`
		State   string `json:"State"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out), &g); err != nil {
		return vmspool.VMDetail{}, fmt.Errorf("tart get %s: parse: %w", vmName, err)
	}
	return vmspool.VMDetail{
		VMInfo: vmspool.VMInfo{
			Name:    vmName,
			Pool:    p.pool,
			State:   g.State,
			Running: g.Running,
		},
		VMResources: vmspool.VMResources{
			DiskGiB:  g.Disk,
			NumCores: g.CPU,
			MemGiB:   g.Memory / 1024,
		},
	}, nil
}

// Delete implements vmspool.Provider, stopping (if running) and deleting every
// VM the Provider manages. Deletion continues past individual failures.
func (p *Provider) Delete(ctx context.Context, stopTimeout time.Duration) ([]string, error) {
	found, err := p.List(ctx)
	if err != nil {
		return nil, err
	}
	var errs errors.M
	deleted := make([]string, 0, len(found))
	for _, vm := range found {
		if vm.Running {
			args := []string{"stop", vm.Name}
			if stopTimeout > 0 {
				secs := int((stopTimeout + time.Second - 1) / time.Second)
				args = append(args, "--timeout", strconv.Itoa(secs))
			}
			if err := runTart(ctx, p.tartBinary, args...); err != nil {
				// Report the failure but still attempt the delete below; the VM
				// may have exited in the meantime.
				errs.Append(err)
			}
		}
		if err := runTart(ctx, p.tartBinary, "delete", vm.Name); err != nil {
			errs.Append(err)
			continue
		}
		deleted = append(deleted, vm.Name)
	}
	return deleted, errs.Err()
}

// Pull fetches ref into the local OCI cache with "tart pull", so that a
// subsequent clone uses the newly fetched copy rather than whatever was cached
// before. tartBinary may be empty, in which case DefaultTartBinary is used.
//
// Only a reference naming a registry can be pulled; a local image name has
// nothing to pull from, and tart reports that as an error. Use ociimage.Pull
// to skip such a reference rather than fail it.
func Pull(ctx context.Context, tartBinary string, ref ociimage.Reference, opts ociimage.PullOptions) error {
	if tartBinary == "" {
		tartBinary = DefaultTartBinary
	}
	args := []string{"pull", ref.String()}
	if opts.Insecure {
		args = append(args, "--insecure")
	}
	if opts.Concurrency > 0 {
		args = append(args, "--concurrency", strconv.Itoa(opts.Concurrency))
	}
	return runTart(ctx, tartBinary, args...)
}

// Pull implements ociimage.Puller with the Provider's own tart binary, see
// WithProviderTartBinary. ref is pulled whatever the Provider's own image is:
// the Provider reports what its Constructor clones but does not run it, so the
// caller that owns the Constructor says what to pull, see WithProviderImage.
func (p *Provider) Pull(ctx context.Context, ref ociimage.Reference, opts ociimage.PullOptions) error {
	return Pull(ctx, p.tartBinary, ref, opts)
}

// SetResources updates the resource configuration of a tart VM image using "tart
// set".
func SetResources(ctx context.Context, tartBinary, image string, resources vmspool.VMResources) error {
	if tartBinary == "" {
		tartBinary = DefaultTartBinary
	}
	args := append([]string{"set", image}, resourceFlags(resources)...)
	return runTart(ctx, tartBinary, args...)
}

func runTart(ctx context.Context, tartBinary string, args ...string) error {
	_, err := runTartOut(ctx, tartBinary, args...)
	return err
}

func runTartOut(ctx context.Context, tartBinary string, args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, tartBinary, args...) //nolint:gosec // G204: args are caller-controlled, not user input.
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, convertError(args, stderr.String(), err)
	}
	return stdout.Bytes(), nil
}

// resourcesConfigured reports whether any resource was asked for, ie. whether
// a clone needs a "tart set" at all.
func resourcesConfigured(r vmspool.VMResources) bool {
	return r.DiskGiB > 0 || r.NumCores > 0 || r.MemGiB > 0
}

// resourceFlags renders r as the flags of "tart set", omitting whatever was
// left at zero so that tart keeps the image's own value for it. tart takes the
// disk size in GiB and the memory in MiB.
func resourceFlags(r vmspool.VMResources) []string {
	var opts []string
	if r.DiskGiB > 0 {
		opts = append(opts, "--disk", strconv.Itoa(r.DiskGiB))
	}
	if r.NumCores > 0 {
		opts = append(opts, "--cpu", strconv.Itoa(r.NumCores))
	}
	if r.MemGiB > 0 {
		opts = append(opts, "--memory", strconv.Itoa(r.MemGiB*1024))
	}
	return opts
}
