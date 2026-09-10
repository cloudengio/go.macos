// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package tartvm

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"cloudeng.io/cmdutil/cmdtypes"
	"cloudeng.io/vms"
)

// fakeTartFailingSubcommand puts a stub tart on PATH that succeeds for every
// subcommand except failSubcommand, which writes stderr and exits with
// exitCode. It is used to exercise the paths where one tart command in a
// multi-command operation, such as Clone with resources, fails.
func fakeTartFailingSubcommand(t *testing.T, failSubcommand, stderr string, exitCode int) func() []string {
	t.Helper()
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> " + strconv.Quote(calls) + "\n" +
		"if [ \"$1\" = " + strconv.Quote(failSubcommand) + " ]; then\n" +
		"  printf '%s' " + strconv.Quote(stderr) + " >&2\n" +
		"  exit " + strconv.Itoa(exitCode) + "\n" +
		"fi\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "tart"), []byte(script), 0700); err != nil { //nolint:gosec // test stub must be executable
		t.Fatalf("writing tart stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func() []string {
		buf, err := os.ReadFile(calls)
		if err != nil {
			return nil // never invoked
		}
		return strings.Split(strings.TrimSpace(string(buf)), "\n")
	}
}

// fakeTartBlockingSubcommand puts a stub tart on PATH that succeeds for every
// subcommand, but blocks in blockSubcommand until the returned release
// function is called. It allows a test to observe the state of an instance
// whilst one of the tart commands issued by a multi-command operation, such as
// Clone with resources, is still running. The returned wait function blocks
// until blockSubcommand has been reached.
func fakeTartBlockingSubcommand(t *testing.T, blockSubcommand string) (wait, release func()) {
	t.Helper()
	dir := t.TempDir()
	started := filepath.Join(dir, "started")
	proceed := filepath.Join(dir, "proceed")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = " + strconv.Quote(blockSubcommand) + " ]; then\n" +
		"  : > " + strconv.Quote(started) + "\n" +
		"  while [ ! -f " + strconv.Quote(proceed) + " ]; do sleep 0.01; done\n" +
		"fi\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "tart"), []byte(script), 0700); err != nil { //nolint:gosec // test stub must be executable
		t.Fatalf("writing tart stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	wait = func() {
		for i := 0; i < 3000; i++ {
			if _, err := os.Stat(started); err == nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Errorf("timed out waiting for tart %v to be run", blockSubcommand)
	}
	release = func() {
		if err := os.WriteFile(proceed, nil, 0600); err != nil {
			t.Errorf("releasing tart stub: %v", err)
		}
	}
	return wait, release
}

func testResources() ResourceConfig {
	return ResourceConfig{
		Disk:   50 * cmdtypes.GB,
		NumCPU: 4,
		Mem:    8192 * cmdtypes.MiB,
	}
}

// TestCloneNoResources verifies that Clone runs 'tart clone' alone when no
// resources are configured.
func TestCloneNoResources(t *testing.T) {
	invocations := fakeTart(t, "", "", 0)
	inst := New(t.Context(), "test-source", "test-vm")

	if err := inst.Clone(t.Context()); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if got, want := inst.State(t.Context()), vms.StateStopped; got != want {
		t.Errorf("state after clone: got %v, want %v", got, want)
	}
	got := invocations()
	if want := []string{"clone test-source test-vm"}; len(got) != 1 || got[0] != want[0] {
		t.Errorf("tart invocations: got %v, want %v", got, want)
	}
}

// TestCloneSetsResources verifies that Clone runs 'tart set' after the clone
// when resources are configured and only then transitions to Stopped.
func TestCloneSetsResources(t *testing.T) {
	invocations := fakeTart(t, "", "", 0)
	inst := New(t.Context(), "test-source", "test-vm", WithResources(testResources()))

	if err := inst.Clone(t.Context()); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if got, want := inst.State(t.Context()), vms.StateStopped; got != want {
		t.Errorf("state after clone: got %v, want %v", got, want)
	}
	got := invocations()
	want := []string{
		"clone test-source test-vm",
		"set test-vm --disk 50 --cpu 4 --memory 8192",
	}
	if len(got) != len(want) {
		t.Fatalf("tart invocations: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("tart invocation %v: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestCloneSetResourcesFails verifies that a failure to set resources after a
// successful clone is reported and leaves the instance in Stopped, ie. in a
// state from which the cloned, but misconfigured, VM can be deleted.
func TestCloneSetResourcesFails(t *testing.T) {
	invocations := fakeTartFailingSubcommand(t, "set", "tart set failed", 1)
	inst := New(t.Context(), "test-source", "test-vm", WithResources(testResources()))

	err := inst.Clone(t.Context())
	if err == nil {
		t.Fatal("Clone: got nil error, want an error")
	}
	if got, want := err.Error(), "setting resources for test-vm"; !strings.Contains(got, want) {
		t.Errorf("Clone: got %q, want it to contain %q", got, want)
	}
	if got, want := inst.State(t.Context()), vms.StateStopped; got != want {
		t.Fatalf("state after a failed set: got %v, want %v", got, want)
	}
	if got, want := len(invocations()), 2; got != want {
		t.Errorf("tart invocations: got %v, want %v", got, want)
	}
	// The cloned VM must still be deletable.
	if err := inst.Delete(t.Context()); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got, want := inst.State(t.Context()), vms.StateDeleted; got != want {
		t.Errorf("state after delete: got %v, want %v", got, want)
	}
}

// TestCloneFails verifies that a failed clone leaves the instance in its
// previous state and that 'tart set' is not run.
func TestCloneFails(t *testing.T) {
	invocations := fakeTartFailingSubcommand(t, "clone", "tart clone failed", 1)
	inst := New(t.Context(), "test-source", "test-vm", WithResources(testResources()))

	if err := inst.Clone(t.Context()); err == nil {
		t.Fatal("Clone: got nil error, want an error")
	}
	if got, want := inst.State(t.Context()), vms.StateInitial; got != want {
		t.Errorf("state after a failed clone: got %v, want %v", got, want)
	}
	if got, want := len(invocations()), 1; got != want {
		t.Errorf("tart invocations: got %v, want %v", got, want)
	}
}

// TestCloneStateWhilstSettingResources verifies that an instance is not
// reported as Stopped, ie. as ready to be started, until its resources have
// been set, and that it is Stopped, rather than in an intermediate state,
// whilst 'tart set' is running.
func TestCloneStateWhilstSettingResources(t *testing.T) {
	wait, release := fakeTartBlockingSubcommand(t, "set")
	inst := New(t.Context(), "test-source", "test-vm", WithResources(testResources()))

	errCh := make(chan error, 1)
	go func() {
		errCh <- inst.Clone(t.Context())
	}()

	wait()
	// The clone has completed but the resources have not been set yet, so the
	// instance must not claim to be ready to be started.
	if got, want := inst.State(t.Context()), vms.StateStopped; got != want {
		t.Errorf("state whilst setting resources: got %v, want %v", got, want)
	}
	release()

	if err := <-errCh; err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if got, want := inst.State(t.Context()), vms.StateStopped; got != want {
		t.Errorf("state after clone: got %v, want %v", got, want)
	}
}
