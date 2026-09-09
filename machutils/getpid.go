// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

// Package machutils provides low level utilities for interacting with the macOS
// kernel.
package machutils

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// ErrFailedToRetrieveParentUID is returned when the parent process UID
// cannot be retrieved from the kernel.
var ErrFailedToRetrieveParentUID = errors.New("failed to retrieve parent process UID from the kernel")

// GetParentUID retrieves the real user ID (RUID) of the parent process.
func GetParentUID() (uint32, error) {
	// 1. Get the parent's Process ID directly from the running Go environment
	ppid := os.Getppid()

	// 2. Query the kernel sysctl interface for the process info
	kproc, err := unix.SysctlKinfoProc("kern.proc.pid", ppid)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrFailedToRetrieveParentUID, err)
	}

	return kproc.Eproc.Pcred.P_ruid, nil
}

// GetExecutableInfo retrieves the owner UID and file info of the executable.
//
// The executable is identified by ExecutablePath rather than by os.Executable,
// so that what is examined is the binary the kernel is running rather than the
// path it was launched through. The two differ for a process started through a
// symbolic link, and for one in an App Sandbox the latter can name a path that
// does not exist.
func GetExecutableInfo() (uint32, os.FileInfo, error) {
	execPath, err := ExecutablePath()
	if err != nil {
		// The kernel has no path to give for an executable that has been
		// unlinked, so name the path the process was launched through: it is
		// all that is left to identify it by, and says nothing about where
		// the binary is now.
		if launched, lerr := os.Executable(); lerr == nil {
			return 0, nil, fmt.Errorf("%w (launched as %v)", err, launched)
		}
		return 0, nil, err
	}

	fileInfo, err := os.Stat(execPath)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to stat the executable %v: %w", execPath, err)
	}

	systat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, nil, errors.New("failed to retrieve file stat information")
	}

	return systat.Uid, fileInfo, nil
}

// EnsureParentProcessSafe checks that:
//  1. the parent process UID matches the executable's UID
//  2. the current process UID matches that of the parent
//  3. the executable is not group- or world-writable or executable
//  4. the current process is not running with elevated privileges (SUID/SGID)
//  5. the process has not been orphaned (reparented to launchd)
//
// 1 ensures that only the executable's owner can launch it,
// 2 ensures that the current process UID matches that of the executable owner,
// 3 ensures that the executable cannot be modified or run by other users,
// 4 ensures that the process has not escalated privileges, and
// 5 ensures that parent identity cannot be spoofed via orphaning.
func EnsureParentProcessSafe() error {
	if os.Getppid() == 1 {
		return errors.New("process has been orphaned; parent identity cannot be verified")
	}
	parentUID, err := GetParentUID()
	if err != nil {
		return err
	}
	execUID, fileInfo, err := GetExecutableInfo()
	if err != nil {
		return err
	}
	perms := fileInfo.Mode().Perm()
	if perms&022 != 0 {
		return errors.New("executable is group or world writable; potential privilege escalation detected")
	}
	if perms&011 != 0 {
		return errors.New("executable is group or world executable; potential privilege escalation detected")
	}
	if parentUID != execUID {
		return errors.New("parent process UID does not match executable UID; potential privilege escalation detected")
	}
	currentUID := uint32(os.Getuid())
	if currentUID != execUID {
		return errors.New("current process UID does not match executable UID; potential privilege escalation detected")
	}
	if uint32(os.Geteuid()) != currentUID || os.Getegid() != os.Getgid() {
		return errors.New("process is running with elevated privileges; potential privilege escalation detected")
	}

	if fileInfo.Mode()&(fs.ModeSetuid|fs.ModeSetgid) != 0 {
		return errors.New("executable has setuid or setgid bits set; potential privilege escalation detected")
	}
	return nil
}
