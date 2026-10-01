// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package machutils

import (
	"errors"
	"fmt"
	"net"
	"unsafe"

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
	if conn == nil {
		return 0, fmt.Errorf("%w: nil connection", ErrFailedToRetrievePeerPID)
	}
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
	if pid <= 0 {
		return 0, fmt.Errorf("%w: invalid peer pid %d", ErrFailedToRetrievePeerPID, pid)
	}
	return int32(pid), nil
}

const localPeerToken = 0x006 // LOCAL_PEERTOKEN from <sys/un.h>

type auditToken [8]uint32

func (t auditToken) pid() int32 {
	return int32(t[5])
}

// peerAuditToken retrieves the peer's audit token from the Unix domain socket
// via LOCAL_PEERTOKEN. The audit token contains the peer's PID and PID version
// (generation count), enabling unique identification of the task without being
// vulnerable to PID recycling.
func peerAuditToken(conn *net.UnixConn) (auditToken, bool) {
	if conn == nil {
		return auditToken{}, false
	}
	raw, err := conn.SyscallConn()
	if err != nil {
		return auditToken{}, false
	}
	var token auditToken
	tokenLen := uint32(unsafe.Sizeof(token))
	var ok bool
	_ = raw.Control(func(fd uintptr) {
		//nolint:staticcheck // SA1019: direct syscall used for 32-byte LOCAL_PEERTOKEN buffer retrieval
		_, _, errno := unix.Syscall6(
			unix.SYS_GETSOCKOPT,
			fd,
			uintptr(unix.SOL_LOCAL),
			uintptr(localPeerToken),
			uintptr(unsafe.Pointer(&token)),
			uintptr(unsafe.Pointer(&tokenLen)),
			0,
		)
		if errno == 0 && tokenLen == uint32(unsafe.Sizeof(token)) {
			ok = true
		}
	})
	return token, ok
}
