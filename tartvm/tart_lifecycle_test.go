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
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"cloudeng.io/cicd"
	tarvm "cloudeng.io/macos/tartvm"
	"cloudeng.io/vms"
	"cloudeng.io/vms/vmstestutil"
)

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
		if strings.HasPrefix(entry.Name, "testlifecycle") || strings.HasPrefix(entry.Name, "testpool") {
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
		if err := vms.CleanupVM(ctx, inst, time.Minute); err != nil {
			t.Logf("cleanup failed for VM %q: %v", inst.ID(), err)
		}
	})
}

func TestLifecycleMacOS(t *testing.T) {
	cicd.LongRunningTest(t, 1)
	tc := defaultInstanceConfig
	tc.Constructor = newMacOSConstructor()
	vmstestutil.TestInstanceLifecycle(t, tc)
}

// TestExecLinux verifies Exec runs commands inside a running Linux VM,
// captures stdout correctly, and returns an error for non-zero exit codes.
