// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package tartvm_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"cloudeng.io/algo/ratecontrol"
	"cloudeng.io/cicd"
	tartvm "cloudeng.io/macos/tartvm"
	"cloudeng.io/os/executil"
	"cloudeng.io/vms"
	"cloudeng.io/vms/ociimage"
	"cloudeng.io/vms/vmspool"
	"cloudeng.io/vms/vmstestutil"
)

// tartConstructor implements vmspool.Constructor for tart-backed VMs.
// Each New() call returns a distinct Instance with a unique name derived
// from a timestamp and an atomic counter so concurrent pool replenishment
// never reuses a name.
type tartConstructor struct {
	source  string
	counter atomic.Int64
	runOpts []string
}

func (c *tartConstructor) New(ctx context.Context) (vms.Instance, error) {
	n := c.counter.Add(1)
	name := fmt.Sprintf("testpool-%d-%d", time.Now().Unix()%100000, n)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{})).With("test", name, "source", c.source)
	opts := []tartvm.Option{tartvm.WithRunOptions(c.runOpts...), tartvm.WithLogger(logger)}
	return tartvm.New(ctx, c.source, name, opts...), nil
}

// Name and Details identify the provider to the pool and to the VMs it hands
// out; the image is the one this constructor clones.
func (c *tartConstructor) Name() string { return tartvm.ProviderName }

type constructorDetails struct {
	image ociimage.Reference
}

func (d constructorDetails) Image() ociimage.Reference { return d.image }

func (c *tartConstructor) Details() vmspool.ProviderDetails {
	ref, err := ociimage.Parse(c.source)
	if err != nil {
		// The source is this harness's own, so one that cannot be parsed is a
		// mistake in the test rather than something Details can report.
		panic(err)
	}
	return constructorDetails{image: ref}
}

// DefaultResources satisfies vmspool.Provider: this constructor sets no
// resources, so its VMs keep whatever their image was built with.
func (c *tartConstructor) DefaultResources() vmspool.VMResources {
	return vmspool.VMResources{}
}

// List, Get and Delete satisfy vmspool.Provider. The pool/instance test harness
// does not exercise them, so they are minimal stubs.
func (c *tartConstructor) List(context.Context) ([]vmspool.VMInfo, error) { return nil, nil }

func (c *tartConstructor) Get(_ context.Context, name string) (vmspool.VMDetail, error) {
	return vmspool.VMDetail{VMInfo: vmspool.VMInfo{Name: name}}, nil
}

func (c *tartConstructor) Delete(context.Context, time.Duration) ([]string, error) {
	return nil, nil
}

func newlinuxConstructor() *tartConstructor {
	return &tartConstructor{
		source:  imageLinux,
		runOpts: tartvm.DefaultLinuxRunOptions(),
	}
}

func newMacOSConstructor() *tartConstructor {
	return &tartConstructor{
		source:  imageMacOS,
		runOpts: tartvm.DefaultMacOSRunOptions(),
	}
}

func rwc(id string) io.Writer {
	return executil.NewLabelingWriter(os.Stderr, []byte(id+": "), '\n')
}

//go:generate astest --import "cloudeng.io/cicd" --match='^TestPool' --preamble=cicd.LongRunningTest(t,1);cfg=poolConfig.Get(t.Name()) --pkg-path cloudeng.io/vms/vmstestutil ./tartpool_test.go
//go:generate astest --import "cloudeng.io/cicd" --match='^TestInstance' --preamble=cicd.LongRunningTest(t,1);cfg=instanceConfig.Get(t.Name()) --pkg-path cloudeng.io/vms/vmstestutil ./tartinstance_test.go

var poolConfig = cicd.ConfigManager[vmstestutil.PoolTestConfig]{}

var defaultPoolConfig = vmstestutil.PoolTestConfig{
	Constructor:      newlinuxConstructor(),
	PoolSize:         2,
	ExecCmd:          "echo",
	ExecArgs:         []string{"hello"},
	ExecStdoutOutput: "hello\n",

	StdoutRWC: rwc,
	StderrRWC: rwc,

	Timeout:          15 * time.Minute,
	StagingBehaviour: vmspool.StagingBehaviourStopped,
}

var instanceConfig = cicd.ConfigManager[vmstestutil.InstanceTestConfig]{}

var defaultInstanceConfig = vmstestutil.InstanceTestConfig{
	Constructor: newlinuxConstructor(),

	Timeout: 15 * time.Minute,

	ExecCmd:    "echo",
	ExecArgs:   []string{"hello"},
	ExecStdout: "hello\n",
	ExecStderr: "",

	RequireUnderlyingState: tartRequireState,
}

func init() {
	poolConfig.SetDefault(defaultPoolConfig)
	instanceConfig.SetDefault(defaultInstanceConfig)
}

// tartLookup calls "tart list --format json" and returns the entry for name,
// or (zero, false) if the VM is not present.
func tartLookup(ctx context.Context, name string) (tartvm.ListEntry, bool, error) {
	all, err := tartvm.ListAll(ctx, "tart")
	if err != nil {
		return tartvm.ListEntry{}, false, fmt.Errorf("tart list: %v", err)
	}
	e, ok := all.Lookup(name)
	return e, ok, nil
}

func tartRequireState(ctx context.Context, inst vms.Instance, msg string, final vms.State, intermediate ...vms.State) error {

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	backoff := ratecontrol.NewExponentialBackoff(time.Millisecond, 20)
	if err := vms.WaitForState(ctx, inst, backoff, final, intermediate...); err != nil {
		return fmt.Errorf("++: %s: waiting for VMS state %v: %v", msg, final, err)
	}
	if final == vms.StateInitial {
		return nil
	}

	// Cross-check against tart list for states tart can distinguish.
	ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	entry, found, err := tartLookup(ctx, inst.ID())
	if err != nil {
		return err
	}

	wantState := func(state string) error {
		if !found {
			return fmt.Errorf("++: %s: tart list: VM %q not found", msg, inst.ID())
		} else if entry.State != state {
			return fmt.Errorf("++: %s: tart list: VM %q state=%q  want state=%q (%+v)", msg, inst.ID(), entry.State, state, entry)
		}
		return nil
	}

	switch final {
	case vms.StateDeleted:
		if found {
			return fmt.Errorf("++: %s: tart list: VM %q still present after delete", msg, inst.ID())
		}
	case vms.StateRunning:
		return wantState("running")
	case vms.StateSuspended:
		return wantState("suspended")
	case vms.StateStopped:
		return wantState("stopped")
	}
	return nil
}
