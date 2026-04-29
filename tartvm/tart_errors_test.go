// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package tartvm_test

import (
	"io"
	"strings"
	"testing"
	"time"

	"cloudeng.io/cicd"
	tartvm "cloudeng.io/macos/tartvm"
)

// assertActionError checks that err is non-nil and contains
// "action <action> not allowed in state <state>".
func assertActionError(t *testing.T, err error, action, state string) {
	t.Helper()
	want := "action " + action + " not allowed in state " + state
	if err == nil {
		t.Fatalf("expected error %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err.Error(), want)
	}
}

// TestInvalidOpsFromInitial verifies that Start, Stop, Suspend, and Delete are
// all rejected before any tart call is made — pure state-machine validation.
func TestInvalidOpsFromInitial(t *testing.T) {
	ctx := t.Context()
	inst := tartvm.New(ctx, "dummy-source", "dummy-name")

	_, stopErr := inst.Stop(ctx, time.Minute)
	assertActionError(t, stopErr, "Stop", "Initial")

	assertActionError(t, inst.Start(ctx, io.Discard, io.Discard), "Start", "Initial")
	assertActionError(t, inst.Suspend(ctx), "Suspend", "Initial")
	assertActionError(t, inst.Delete(ctx), "Delete", "Initial")
}

/*
// TestInvalidOpsFromStopped clones a VM and verifies that Suspend and Clone are
// rejected from Stopped state. Both errors fire before tart is called.
func TestInvalidOpsFromStopped(t *testing.T) {
	ctx := t.Context()
	inst := tartvm.New(ctx, imageLinux, vmName(t), tartvm.WithRunOptions(tartvm.DefaultLinuxRunOptions()...))
	cleanupVM(t, inst)

	if err := inst.Clone(ctx); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	assertActionError(t, inst.Suspend(ctx), "Suspend", "Stopped")
	assertActionError(t, inst.Clone(ctx), "Clone", "Stopped")
}
*/

// TestInvalidOpsFromRunningLinux starts a Linux VM and verifies that Clone,
// Start, and Delete are rejected from Running state.
func TestInvalidOpsFromRunningLinux(t *testing.T) {
	cicd.LongRunningTest(t, 1)
	testInvalidOpsFromRunning(t, imageLinux, tartvm.DefaultLinuxRunOptions()...)
}

// TestInvalidOpsFromRunningMacOS starts a macOS VM and verifies that Clone,
// Start, and Delete are rejected from Running state.
func TestInvalidOpsFromRunningMacOS(t *testing.T) {
	cicd.LongRunningTest(t, 1)
	testInvalidOpsFromRunning(t, imageMacOS, tartvm.DefaultMacOSRunOptions()...)
}

func testInvalidOpsFromRunning(t *testing.T, image string, runOptions ...string) {
	t.Helper()
	ctx := t.Context()
	inst := tartvm.New(ctx, image, vmName(t), tartvm.WithRunOptions(runOptions...))
	cleanupVM(t, inst)

	if err := inst.Clone(ctx); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	assertActionError(t, inst.Suspend(ctx), "Suspend", "Stopped")
	assertActionError(t, inst.Clone(ctx), "Clone", "Stopped")

	if err := inst.Start(ctx, io.Discard, io.Discard); err != nil {
		t.Fatalf("Start: %v", err)
	}

	assertActionError(t, inst.Clone(ctx), "Clone", "Running")
	assertActionError(t, inst.Start(ctx, io.Discard, io.Discard), "Start", "Running")
	assertActionError(t, inst.Delete(ctx), "Delete", "Running")

	// VM is still running
	if !inst.Suspendable() {
		return
	}

	if err := inst.Suspend(ctx); err != nil {
		t.Fatalf("Suspend: %v", err)
	}

	_, stopErr := inst.Stop(ctx, time.Minute)
	assertActionError(t, stopErr, "Stop", "Suspended")
	assertActionError(t, inst.Clone(ctx), "Clone", "Suspended")
}
