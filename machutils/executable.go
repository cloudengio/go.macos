// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package machutils

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ErrFailedToRetrieveExecutablePath is returned when the path of the running
// executable cannot be retrieved from the kernel.
var ErrFailedToRetrieveExecutablePath = errors.New("failed to retrieve the executable path from the kernel")

const (
	// procInfoCallPIDInfo is PROC_INFO_CALL_PIDINFO.
	procInfoCallPIDInfo = 2
	// procPIDPathInfo is PROC_PIDPATHINFO, the flavor that returns the path
	// of a process's executable.
	procPIDPathInfo = 11
	// procPIDPathInfoSize is PROC_PIDPATHINFO_SIZE and procPIDPathInfoMaxSize
	// is PROC_PIDPATHINFO_MAXSIZE. A buffer outside that range is rejected.
	procPIDPathInfoSize    = 1024
	procPIDPathInfoMaxSize = 4 * procPIDPathInfoSize
)

// ExecutablePath returns the path of the binary that this process is running,
// as the kernel records it for the executing file.
//
// It exists because os.Executable does not answer that question on macOS. What
// os.Executable reports is the path the process was launched with, made
// absolute against the working directory when it is relative. That names the
// symbolic link when a process is started through one, and for a process in an
// App Sandbox it can name a path that does not exist at all, the recorded path
// having been joined to the container's redirected home directory rather than
// to the directory the binary occupies. Asking the kernel avoids both: the
// path returned is that of the file being executed, with symbolic links
// already resolved.
func ExecutablePath() (string, error) {
	return executablePath(os.Getpid())
}

// executablePath makes the same proc_info call that libproc's proc_pidpath
// makes. It is called directly rather than through libSystem for the reason
// given on csops.
func executablePath(pid int) (string, error) {
	buf := make([]byte, procPIDPathInfoMaxSize)
	//nolint:staticcheck // SA1019: libSystem exposes no wrapper reachable without cgo.
	_, _, errno := unix.Syscall6(unix.SYS_PROC_INFO,
		procInfoCallPIDInfo, uintptr(pid), procPIDPathInfo, 0,
		uintptr(unsafe.Pointer(unsafe.SliceData(buf))), uintptr(len(buf)))
	if errno != 0 {
		return "", fmt.Errorf("%w: %v", ErrFailedToRetrieveExecutablePath, errno)
	}
	// Success is reported by returning zero, not the length: the kernel copies
	// out a NUL terminated path and libproc's wrapper measures it itself.
	end := bytes.IndexByte(buf, 0)
	if end <= 0 {
		return "", fmt.Errorf("%w: no path was returned for process %v", ErrFailedToRetrieveExecutablePath, pid)
	}
	return string(buf[:end]), nil
}
