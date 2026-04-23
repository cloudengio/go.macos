// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package tartvm_test

// Lifecycle tests for the tart package. These tests create real VMs and walk
// them through their state transitions. They are are configured as
// level 1 long-running tests as per cloudeng.io/cicd and are enabled
// by setting CLOUDENG_LONG_RUNNING_TESTS=1 or by referring to the test
// names directly.

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"cloudeng.io/cicd"
	"cloudeng.io/logging/ctxlog"
	tarvm "cloudeng.io/macos/tartvm"
	"cloudeng.io/vms"
)

// tartLookup calls "tart list --format json" and returns the entry for name,
// or (zero, false) if the VM is not present.
func tartLookup(ctx context.Context, t *testing.T, name string) (tarvm.ListEntry, bool) {
	t.Helper()
	all, err := tarvm.ListAll(ctx)
	if err != nil {
		t.Fatalf("tart list: %v", err)
	}
	return all.Lookup(name)
}

const (
	imageLinux = "ghcr.io/cirruslabs/ubuntu:latest"
	imageMacOS = "ghcr.io/cirruslabs/macos-tahoe-base:latest"
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	if runtime.GOOS != "darwin" {
		fmt.Fprintln(os.Stderr, "tart is only supported on macOS; skipping tests")
		os.Exit(0)
	}
	if _, err := exec.LookPath("tart"); err != nil {
		fmt.Fprintln(os.Stderr, "tart CLI not found in PATH; skipping tests")
		os.Exit(1)
	}
	images, err := tarvm.ListAll(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tart list failed: %v; skipping tests\n", err)
		os.Exit(0)
	}
	for _, image := range []string{imageLinux, imageMacOS} {
		if _, ok := images.Lookup(image); !ok {
			fmt.Fprintf(os.Stderr, "tart image %q not found; skipping tests\n", image)
			fmt.Fprintf(os.Stderr, "run `tart pull %s` to pull the required images before running the tests.\n", image)
			os.Exit(0)
		}
	}
	code := m.Run()
	all, _ := tarvm.ListAll(ctx)
	for _, entry := range all {
		if strings.HasPrefix(entry.Name, "testlifecycle") {
			cleanup(ctx, entry)
		}
	}
	os.Exit(code)
}

func run(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "tart", args...).CombinedOutput() // #nosec G204
}

func cleanup(ctx context.Context, entry tarvm.ListEntry) {
	if entry.State == "running" {
		if out, err := run(ctx, "stop", entry.Name); err != nil {
			fmt.Fprintf(os.Stderr, "failed to stop tart VM %q: %v\nOutput: %s\n", entry.Name, err, out)
		}
	}
	if out, err := run(ctx, "delete", entry.Name); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete tart VM %q: %v\nOutput: %s\n", entry.Name, err, out)
	}
}

// vmName returns a short, unique VM name derived from the test name.
func vmName(t *testing.T) string {
	t.Helper()
	safe := strings.NewReplacer("/", "-", " ", "-", "_", "-").Replace(t.Name())
	safe = strings.ToLower(safe)
	// tart VM names must be short; trim and add a short timestamp suffix.
	if len(safe) > 24 {
		safe = safe[:24]
	}
	return fmt.Sprintf("%s-%d", safe, time.Now().Unix()%100000)
}

// cleanupVM stops (if running) and deletes the VM at test teardown.
func cleanupVM(t *testing.T, inst *tarvm.Instance) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := vms.CleanupVM(ctx, inst); err != nil {
			t.Logf("cleanup failed for VM %q: %v", inst.Name(), err)
		}
	})
}

func logStep(t *testing.T, format string, args ...any) (func(), io.Writer) {
	t.Helper()
	msg := fmt.Sprintf(format, args...)
	t.Logf("→ %s", msg)
	start := time.Now()
	return func() { t.Logf("  ✓ %s (%.1fs)", msg, time.Since(start).Seconds()) }, &lineWrapper{prefix: fmt.Sprintf("→→ %s", msg)}
}

func requireState(t *testing.T, inst *tarvm.Instance, msg string, final vms.State, intermediate ...vms.State) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	if err := vms.WaitForState(ctx, inst, time.Millisecond, final, intermediate...); err != nil {
		t.Fatalf("++: %s: waiting for VMS state %v: %v", msg, final, err)
	}
	if final == vms.StateInitial {
		return
	}

	// Cross-check against tart list for states tart can distinguish.
	ctx, cancel = context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	entry, found := tartLookup(ctx, t, inst.Name())

	wantState := func(state string) {
		if !found {
			t.Fatalf("++: %s: tart list: VM %q not found", msg, inst.Name())
		} else if entry.State != state {
			t.Fatalf("++: %s: tart list: VM %q state=%q  want state=%q (%+v)", msg, inst.Name(), entry.State, state, entry)
		}
	}

	switch final {
	case vms.StateDeleted:
		if found {
			t.Fatalf("++: %s: tart list: VM %q still present after delete", msg, inst.Name())
		}
	case vms.StateRunning:
		wantState("running")
	case vms.StateSuspended:
		wantState("suspended")
	case vms.StateStopped:
		wantState("stopped")
	}
}

type lineWrapper struct {
	prefix string
	mu     sync.Mutex
}

func (w *lineWrapper) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	lines := strings.SplitSeq(string(p), "\n")
	for line := range lines {
		if line != "" {
			fmt.Printf("%s:   %s\n", w.prefix, line)
		}
	}
	return len(p), nil
}

// runLifecycle walks a VM through:
// Initial → Clone → Stopped →
// Start → Running → Stop → Stopped → Stop (idempotent) →
// Start → Running → [Suspend → Suspended → Suspend (idempotent) → Start → Running →]
// Stop → Stopped → Delete → Deleted
func runLifecycle(t *testing.T, source string, runOptions ...string) {
	ctx := t.Context()
	ctx = ctxlog.WithLogger(ctx, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{})).With("test", t.Name(), "source", source))
	name := vmName(t)

	inst := tarvm.New(ctx, source, name, tarvm.WithRunOptions(runOptions...))
	cleanupVM(t, inst)

	requireState(t, inst, "initial", vms.StateInitial, vms.StateInitial)

	done, _ := logStep(t, "clone %s → %s", source, name)
	checkErr := func(action string, err error) {
		if err != nil {
			t.Fatalf("%s: %v", action, err)
		}
	}

	err := inst.Clone(ctx)
	checkErr("Clone", err)
	done()
	requireState(t, inst, "clone", vms.StateStopped, vms.StateCloning, vms.StateInitial)

	done, lr := logStep(t, "run")
	err = inst.Start(ctx, lr, lr)
	checkErr("Run", err)
	done()
	requireState(t, inst, "run",
		vms.StateRunning,
		vms.StateStopped, vms.StateStarting)

	done, _ = logStep(t, "stop")
	runErr, stopErr := inst.Stop(ctx, time.Minute)
	checkErr("Stop", runErr)
	checkErr("Stop", stopErr)
	done()
	requireState(t, inst, "stop",
		vms.StateStopped,
		vms.StateRunning, vms.StateStopping)

	done, _ = logStep(t, "stop again (idempotency)")
	runErr, stopErr = inst.Stop(ctx, time.Minute)
	checkErr("Stop (idempotent)", runErr)
	checkErr("Stop (idempotent)", stopErr)
	done()
	requireState(t, inst, "stop idempotent", vms.StateStopped)

	time.Sleep(time.Second)
	done, lr = logStep(t, "run again from stopped")
	err = inst.Start(ctx, lr, lr)
	checkErr("Start (second)", err)
	done()
	requireState(t, inst, "run again from stopped",
		vms.StateRunning,
		vms.StateRunning, vms.StateStopped)

	if inst.Suspendable() {
		done, _ = logStep(t, "suspend")
		err = inst.Suspend(ctx)
		checkErr("Suspend", err)
		done()
		requireState(t, inst, "suspend",
			vms.StateSuspended,
			vms.StateRunning, vms.StateSuspending)

		done, _ = logStep(t, "suspend again (idempotency)")
		err = inst.Suspend(ctx)
		checkErr("Suspend (idempotent)", err)
		done()
		requireState(t, inst, "suspend idempotent", vms.StateSuspended)

		done, lr = logStep(t, "run again from suspended")
		err = inst.Start(ctx, lr, lr)
		checkErr("Start (from suspended)", err)
		done()
		requireState(t, inst, "run again from suspended",
			vms.StateRunning,
			vms.StateSuspended, vms.StateStarting)
	}

	done, _ = logStep(t, "stop before delete")
	runErr, stopErr = inst.Stop(ctx, time.Minute)
	checkErr("Stop (before delete)", runErr)
	checkErr("Stop (before delete)", stopErr)
	done()
	requireState(t, inst, "stop before delete",
		vms.StateStopped,
		vms.StateRunning, vms.StateStopping)

	done, _ = logStep(t, "delete")
	err = inst.Delete(ctx)
	checkErr("Delete", err)
	done()
	requireState(t, inst, "delete",
		vms.StateDeleted,
		vms.StateDeleting)
}

func TestLifecycleLinux(t *testing.T) {
	cicd.LongRunningTest(t, 1)
	runLifecycle(t, imageLinux, tarvm.DefaultLinuxRunOptions()...)
}

func TestLifecycleMacOS(t *testing.T) {
	cicd.LongRunningTest(t, 1)
	runLifecycle(t, imageMacOS, tarvm.DefaultMacOSRunOptions()...)
}

// TestExecLinux verifies Exec runs commands inside a running Linux VM,
// captures stdout correctly, and returns an error for non-zero exit codes.
func TestExecLinux(t *testing.T) {
	cicd.LongRunningTest(t, 1)
	ctx := t.Context()
	inst := tarvm.New(ctx, imageLinux, vmName(t), tarvm.WithRunOptions(tarvm.DefaultLinuxRunOptions()...))
	cleanupVM(t, inst)

	// Exec must fail before the VM is running.
	if err := inst.Exec(ctx, io.Discard, io.Discard, "echo", "hello"); err == nil {
		t.Fatal("Exec on Initial VM: expected error, got nil")
	}

	if err := inst.Clone(ctx); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if err := inst.Start(ctx, io.Discard, io.Discard); err != nil {
		t.Fatalf("Start: %v", err)
	}

	var out strings.Builder
	if err := inst.Exec(ctx, &out, io.Discard, "echo", "hello from tart"); err != nil {
		t.Fatalf("Exec echo: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "hello from tart" {
		t.Errorf("Exec echo: got %q, want %q", got, "hello from tart")
	}

	// A failing command must return a non-nil error.
	if err := inst.Exec(ctx, io.Discard, io.Discard, "false"); err == nil {
		t.Error("Exec false: expected non-zero exit error, got nil")
	}
}
