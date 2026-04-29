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

	"cloudeng.io/cicd"
	tartvm "cloudeng.io/macos/tartvm"
	"cloudeng.io/os/executil"
	"cloudeng.io/vms"
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

func (c *tartConstructor) New() vms.Instance {
	n := c.counter.Add(1)
	name := fmt.Sprintf("testpool-%d-%d", time.Now().Unix()%100000, n)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{})).With("test", name, "source", c.source)
	opts := []tartvm.Option{tartvm.WithRunOptions(c.runOpts...), tartvm.WithLogger(logger)}
	return tartvm.New(context.Background(), c.source, name, opts...)
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
	all, err := tartvm.ListAll(ctx)
	if err != nil {
		return tartvm.ListEntry{}, false, fmt.Errorf("tart list: %v", err)
	}
	e, ok := all.Lookup(name)
	return e, ok, nil
}

func tartRequireState(ctx context.Context, inst vms.Instance, msg string, final vms.State, intermediate ...vms.State) error {

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := vms.WaitForState(ctx, inst, time.Millisecond, final, intermediate...); err != nil {
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
