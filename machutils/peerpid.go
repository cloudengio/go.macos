// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package machutils

import (
	"errors"
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// ErrFailedToRetrievePeerPID is returned when the process ID of the peer of
// a Unix domain socket connection cannot be retrieved from the kernel.
var ErrFailedToRetrievePeerPID = errors.New("failed to retrieve peer process ID from the kernel")

// PeerPID returns the process ID of the process on the other end of conn, a
// Unix domain socket connection, via the LOCAL_PEEREPID socket option: a
// public, documented macOS extension to getsockopt (see unix(4)), unlike the
// csops interface the rest of this package otherwise resorts to, so no cgo
// is needed here.
//
// The PID alone proves nothing about which binary is running as that
// process; pair it with VerifyPeerCodeSignature to check that.
func PeerPID(conn *net.UnixConn) (int32, error) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrFailedToRetrievePeerPID, err)
	}
	var pid int
	var sockErr error
	if err := raw.Control(func(fd uintptr) {
		pid, sockErr = unix.GetsockoptInt(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEEREPID)
	}); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrFailedToRetrievePeerPID, err)
	}
	if sockErr != nil {
		return 0, fmt.Errorf("%w: %v", ErrFailedToRetrievePeerPID, sockErr)
	}
	return int32(pid), nil
}
