// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package machutils

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
	"howett.net/plist"
)

// ErrFailedToRetrieveEntitlements is returned when the code signing
// entitlements of the running process cannot be retrieved from the kernel.
var ErrFailedToRetrieveEntitlements = errors.New("failed to retrieve code signing entitlements from the kernel")

// AppSandboxEntitlement is the entitlement that places a process in the macOS
// App Sandbox.
const AppSandboxEntitlement = "com.apple.security.app-sandbox"

const (
	// csOpsEntitlementsBlob is CS_OPS_ENTITLEMENTS_BLOB, the csops operation
	// that returns the entitlements recorded for a process when it was
	// signed, as a property list.
	csOpsEntitlementsBlob = 7
	// csMagicEmbeddedEntitlements is CSMAGIC_EMBEDDED_ENTITLEMENTS, the magic
	// number that blob begins with.
	csMagicEmbeddedEntitlements = 0xfade7171
	// entitlementsHeaderSize is the size of the blob's header, which is the
	// magic number followed by the length of the whole blob, both big endian.
	entitlementsHeaderSize = 8
	// maxEntitlementsSize bounds the allocation made for a blob, so that a
	// length which is not credible is rejected rather than acted upon.
	maxEntitlementsSize = 1 << 20
)

// csops invokes the csops system call for the given process. buf must not be
// empty: the kernel is told how much room it has, and reports ERANGE when
// that is not enough.
//
// The call is made directly rather than through libSystem, which Apple gives
// as the only supported way of reaching the kernel and which x/sys marks
// every darwin syscall number as deprecated for. libSystem exposes no wrapper
// for csops, so the alternative is cgo, and this package is otherwise free of
// it. The consequence is that a release which changes the syscall interface
// would break this, where a libSystem wrapper would not.
func csops(pid int, ops uint32, buf []byte) error {
	//nolint:staticcheck // SA1019: libSystem exposes no csops wrapper; see above.
	_, _, errno := unix.Syscall6(unix.SYS_CSOPS, uintptr(pid), uintptr(ops),
		uintptr(unsafe.Pointer(unsafe.SliceData(buf))), uintptr(len(buf)), 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

// Entitlements returns the code signing entitlements of the running
// executable, as the property list recorded in its signature, or nil if it
// was signed without any.
//
// Only the property list form of the entitlements is consulted. A signature
// that carries them solely in their DER form, which the kernel keeps
// separately, is reported here as having none.
func Entitlements() ([]byte, error) {
	pid := os.Getpid()
	header := make([]byte, entitlementsHeaderSize)
	err := csops(pid, csOpsEntitlementsBlob, header)
	if err == nil {
		// A process with no entitlements is not an error: the kernel reports
		// success and leaves the buffer as it was.
		return nil, nil
	}
	if !errors.Is(err, syscall.ERANGE) {
		return nil, fmt.Errorf("%w: %v", ErrFailedToRetrieveEntitlements, err)
	}
	// The buffer being too small is how the size of the blob is discovered:
	// the kernel writes it into the header it could not fill.
	size := binary.BigEndian.Uint32(header[4:entitlementsHeaderSize])
	if size <= entitlementsHeaderSize || size > maxEntitlementsSize {
		return nil, fmt.Errorf("%w: implausible blob size of %v bytes", ErrFailedToRetrieveEntitlements, size)
	}
	blob := make([]byte, size)
	if err := csops(pid, csOpsEntitlementsBlob, blob); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFailedToRetrieveEntitlements, err)
	}
	if magic := binary.BigEndian.Uint32(blob[0:4]); magic != csMagicEmbeddedEntitlements {
		return nil, fmt.Errorf("%w: unexpected blob magic 0x%08x", ErrFailedToRetrieveEntitlements, magic)
	}
	return blob[entitlementsHeaderSize:], nil
}

// IsSandboxed reports whether the running executable is confined by the macOS
// App Sandbox, which it determines from the com.apple.security.app-sandbox
// entitlement in its code signature.
//
// That entitlement is what places a process in the App Sandbox: it is applied
// by the kernel when the process is executed, and a process carrying it with
// no container to run in is killed rather than run unconfined. Its presence is
// therefore conclusive. App extensions, Safari web extensions among them, are
// always signed with it.
//
// It says nothing about the other ways a process can be confined on macOS. A
// profile applied by sandbox_init, or by being launched under sandbox-exec,
// leaves no trace in the code signature and is not reported here.
func IsSandboxed() (bool, error) {
	entitlements, err := Entitlements()
	if err != nil {
		return false, err
	}
	return hasAppSandboxEntitlement(entitlements)
}

// hasAppSandboxEntitlement reports whether an entitlements property list
// enables the App Sandbox. An absent or empty list is not an error: an
// executable signed without entitlements simply has none.
func hasAppSandboxEntitlement(entitlements []byte) (bool, error) {
	if len(bytes.TrimSpace(entitlements)) == 0 {
		return false, nil
	}
	var values map[string]any
	if _, err := plist.Unmarshal(entitlements, &values); err != nil {
		return false, fmt.Errorf("%w: parsing entitlements: %v", ErrFailedToRetrieveEntitlements, err)
	}
	enabled, ok := values[AppSandboxEntitlement].(bool)
	return ok && enabled, nil
}
