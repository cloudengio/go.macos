// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"

	"cloudeng.io/macos/machutils"
)

func main() {
	var (
		checkUID     = flag.Int64("check-uid", -1, "expected UID to check against")
		execUID      = flag.Bool("exec-uid", false, "print the UID and permissions of the running executable")
		deleteSelf   = flag.Bool("delete-self", false, "remove the executable before reading its UID, for error testing")
		ensureSafe   = flag.Bool("ensure-safe", false, "report whether the parent process is safe")
		orphan       = flag.String("orphan", "", "spawn a copy that reports its parent UID to the named file, then exit without waiting for it")
		report       = flag.String("report-parent-uid", "", "wait to be reparented, then write the parent PID and UID to the named file")
		orphanSafe   = flag.String("orphan-safe", "", "spawn a copy that tests EnsureParentProcessSafe after being orphaned, then exit without waiting for it")
		reportSafe   = flag.String("report-safe", "", "wait to be reparented, then write EnsureParentProcessSafe result to named file")
		sandboxed    = flag.Bool("sandboxed", false, "report whether this process is confined by the App Sandbox")
		entitlements = flag.Bool("entitlements", false, "print the code signing entitlements of this process")
		execPaths    = flag.Bool("exec-paths", false, "print the path this process was launched through and the path the kernel records for it")
	)
	flag.Parse()

	switch {
	case *execUID:
		reportExecutableOwner(*deleteSelf)
		return
	case *ensureSafe:
		if err := machutils.EnsureParentProcessSafe(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(4)
		}
		fmt.Fprintln(os.Stdout, "safe")
		return
	case *orphan != "":
		spawnOrphan(*orphan)
		return
	case *report != "":
		reportParentAfterReparenting(*report)
		return
	case *orphanSafe != "":
		spawnOrphanSafe(*orphanSafe)
		return
	case *reportSafe != "":
		reportSafeAfterReparenting(*reportSafe)
		return
	case *sandboxed:
		reportSandboxed()
		return
	case *entitlements:
		reportEntitlements()
		return
	case *execPaths:
		reportExecutablePaths()
		return
	}

	uid, err := machutils.GetParentUID()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if *checkUID >= 0 && int64(uid) != *checkUID {
		fmt.Fprintf(os.Stderr, "UID mismatch: expected %d, got %d\n", *checkUID, uid)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stdout, "%d\n", uid)
}

// reportExecutablePaths prints the path the process was launched through and
// the path the kernel records for the binary it is running, which differ when
// the process was started through a symbolic link.
func reportExecutablePaths() {
	launched, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(7)
	}
	actual, err := machutils.ExecutablePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(7)
	}
	fmt.Fprintf(os.Stdout, "%s\n%s\n", launched, actual)
}

// reportSandboxed prints whether this process is confined by the App Sandbox.
func reportSandboxed() {
	confined, err := machutils.IsSandboxed()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(5)
	}
	fmt.Fprintf(os.Stdout, "%v\n", confined)
}

// reportEntitlements prints the code signing entitlements of this process,
// which is nothing at all when it was signed without any.
func reportEntitlements() {
	entitlements, err := machutils.Entitlements()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(6)
	}
	fmt.Fprintf(os.Stdout, "%s", entitlements)
}

// reportExecutableOwner prints the UID and permissions of the running
// executable, having first removed it if deleteSelf is set, which leaves the
// executable's path with nothing to stat.
func reportExecutableOwner(deleteSelf bool) {
	if deleteSelf {
		path, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if err := os.Remove(path); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
	uid, fi, err := machutils.GetExecutableInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(3)
	}
	fmt.Fprintf(os.Stdout, "%d %o\n", uid, uint32(fi.Mode().Perm()))
}

// spawnOrphan starts a copy of this binary that will report its parent, then
// returns immediately without waiting for it, so that the copy is orphaned and
// the kernel reparents it.
func spawnOrphan(out string) {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	cmd := exec.Command(self, "-report-parent-uid", out) //nolint:gosec // self
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	// Deliberately no Wait: exiting here orphans the child.
}

// reportParentAfterReparenting waits for the kernel to reparent this process,
// which happens once the process that started it has exited, then writes the
// parent PID and the UID reported for it. The file is written atomically so
// that a reader never sees it half written.
func reportParentAfterReparenting(out string) {
	deadline := time.Now().Add(30 * time.Second)
	for os.Getppid() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	uid, err := machutils.GetParentUID()
	line := fmt.Sprintf("%d %d\n", os.Getppid(), uid)
	if err != nil {
		line = fmt.Sprintf("%d error %v\n", os.Getppid(), err)
	}
	tmp := out + ".tmp"
	if err := os.WriteFile(tmp, []byte(line), 0600); err != nil {
		os.Exit(1)
	}
	if err := os.Rename(tmp, out); err != nil {
		os.Exit(1)
	}
}

// spawnOrphanSafe starts a copy of this binary that will test EnsureParentProcessSafe
// after being orphaned, then exits immediately without waiting for it.
func spawnOrphanSafe(out string) {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	cmd := exec.Command(self, "-report-safe", out) //nolint:gosec // self
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	// Deliberately no Wait: exiting here orphans the child.
}

// reportSafeAfterReparenting waits for the process to be reparented, then calls
// EnsureParentProcessSafe and writes the outcome to out.
func reportSafeAfterReparenting(out string) {
	deadline := time.Now().Add(30 * time.Second)
	for os.Getppid() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	err := machutils.EnsureParentProcessSafe()
	line := "safe\n"
	if err != nil {
		line = fmt.Sprintf("error: %v\n", err)
	}
	tmp := out + ".tmp"
	if err := os.WriteFile(tmp, []byte(line), 0600); err != nil {
		os.Exit(1)
	}
	if err := os.Rename(tmp, out); err != nil {
		os.Exit(1)
	}
}
