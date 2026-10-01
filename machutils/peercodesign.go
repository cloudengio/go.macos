// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package machutils

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation

#include <Security/Security.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>

// verifyPeerCodeSignature resolves pid to its currently running code via
// SecCodeCopyGuestWithAttributes and checks it against requirementStr using
// SecCodeCheckValidity, returning the resulting OSStatus: errSecSuccess (0)
// if, and only if, pid is still running the code it was when checked, and
// that code satisfies requirementStr.
static OSStatus verifyPeerCodeSignature(pid_t pid, const char *requirementStr) {
	CFNumberRef pidNum = CFNumberCreate(kCFAllocatorDefault, kCFNumberIntType, &pid);
	const void *keys[1] = { kSecGuestAttributePid };
	const void *values[1] = { pidNum };
	CFDictionaryRef attrs = CFDictionaryCreate(kCFAllocatorDefault, keys, values, 1,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFRelease(pidNum);

	SecCodeRef code = NULL;
	OSStatus status = SecCodeCopyGuestWithAttributes(NULL, attrs, kSecCSDefaultFlags, &code);
	CFRelease(attrs);
	if (status != errSecSuccess) {
		return status;
	}

	CFStringRef reqString = CFStringCreateWithCString(kCFAllocatorDefault, requirementStr, kCFStringEncodingUTF8);
	SecRequirementRef requirement = NULL;
	status = SecRequirementCreateWithString(reqString, kSecCSDefaultFlags, &requirement);
	CFRelease(reqString);
	if (status != errSecSuccess) {
		CFRelease(code);
		return status;
	}

	status = SecCodeCheckValidity(code, kSecCSDefaultFlags, requirement);
	CFRelease(requirement);
	CFRelease(code);
	return status;
}

// copySecErrorMessage returns the human readable description of an OSStatus
// from the Security framework, as a newly allocated C string the caller must
// free, or NULL if none is available.
static char *copySecErrorMessage(OSStatus status) {
	CFStringRef msg = SecCopyErrorMessageString(status, NULL);
	if (msg == NULL) {
		return NULL;
	}
	CFIndex length = CFStringGetLength(msg);
	CFIndex maxSize = CFStringGetMaximumSizeForEncoding(length, kCFStringEncodingUTF8) + 1;
	char *buf = malloc((size_t)maxSize);
	if (buf == NULL || !CFStringGetCString(msg, buf, maxSize, kCFStringEncodingUTF8)) {
		free(buf);
		buf = NULL;
	}
	CFRelease(msg);
	return buf;
}

// copyCFStringContents copies a CFStringRef's contents into a newly
// allocated C string the caller must free, or NULL if it could not be
// represented in UTF-8.
static char *copyCFStringContents(CFStringRef s) {
	CFIndex length = CFStringGetLength(s);
	CFIndex maxSize = CFStringGetMaximumSizeForEncoding(length, kCFStringEncodingUTF8) + 1;
	char *buf = malloc((size_t)maxSize);
	if (buf == NULL || !CFStringGetCString(s, buf, maxSize, kCFStringEncodingUTF8)) {
		free(buf);
		return NULL;
	}
	return buf;
}

// copySelfRequirementString returns this process's own designated code
// signing requirement, as a newly allocated C string the caller must free,
// or NULL if it could not be determined, e.g. the running binary is
// unsigned.
static char *copySelfRequirementString() {
	SecCodeRef self = NULL;
	if (SecCodeCopySelf(kSecCSDefaultFlags, &self) != errSecSuccess) {
		return NULL;
	}
	SecRequirementRef req = NULL;
	OSStatus status = SecCodeCopyDesignatedRequirement(self, kSecCSDefaultFlags, &req);
	CFRelease(self);
	if (status != errSecSuccess) {
		return NULL;
	}
	CFStringRef reqStr = NULL;
	status = SecRequirementCopyString(req, kSecCSDefaultFlags, &reqStr);
	CFRelease(req);
	if (status != errSecSuccess) {
		return NULL;
	}
	char *buf = copyCFStringContents(reqStr);
	CFRelease(reqStr);
	return buf;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"net"
	"unsafe"
)

// ErrPeerCodeSignatureInvalid is returned by VerifyPeerCodeSignature when the
// peer either cannot be identified, is no longer running, or does not
// satisfy the given requirement.
var ErrPeerCodeSignatureInvalid = errors.New("peer code signature is not valid or does not satisfy the requirement")

// VerifyPeerCodeSignature verifies that the process connected via conn, a
// Unix domain socket connection, is a currently valid, unmodified binary
// satisfying requirement: a code signing requirement string in the language
// codesign and Xcode use (see `man csreq`), typically naming a Team ID
// and/or bundle identifier, e.g.
//
//	anchor apple generic and certificate leaf[subject.OU] = "TEAMID" and identifier "ai.onyourbehalf.router-helper"
//
// Filesystem or socket permissions on their own only prove that the
// connecting process is allowed to reach the socket (e.g. membership of the
// same App Group container); they say nothing about which binary is on the
// other end of it. This is the additional check needed to prove that it is a
// specific, currently valid, signed binary, using the same mechanism
// SecCodeCheckValidity applies to the calling process, applied instead to
// the peer of a local socket connection, identified by PeerPID.
//
// There is an unavoidable, narrow race between reading the peer's PID and
// resolving it to a running code object: the peer could in principle exit
// and have its PID recycled by an unrelated process in between. This is the
// same assumption system software on macOS makes when authenticating local
// socket peers this way; the window is on the order of the time between two
// syscalls, not something a connecting process can reliably exploit.
//
// A caller should perform this check immediately after accepting conn and
// before reading or acting on anything it sends, closing conn without
// further use if it returns an error.
func VerifyPeerCodeSignature(conn *net.UnixConn, requirement string) error {
	pid, err := PeerPID(conn)
	if err != nil {
		return err
	}

	cReq := C.CString(requirement)
	defer C.free(unsafe.Pointer(cReq))

	status := C.verifyPeerCodeSignature(C.pid_t(pid), cReq)
	if status == C.errSecSuccess {
		return nil
	}
	if msg := C.copySecErrorMessage(status); msg != nil {
		defer C.free(unsafe.Pointer(msg))
		return fmt.Errorf("%w: pid %d: %s (OSStatus %d)", ErrPeerCodeSignatureInvalid, pid, C.GoString(msg), int(status))
	}
	return fmt.Errorf("%w: pid %d: OSStatus %d", ErrPeerCodeSignatureInvalid, pid, int(status))
}

// ErrNoSelfRequirement is returned by SelfRequirementString when the running
// binary's own designated code signing requirement cannot be determined,
// typically because it is unsigned.
var ErrNoSelfRequirement = errors.New("could not determine this process's own designated code signing requirement")

// SelfRequirementString returns the running binary's own designated code
// signing requirement, in the same requirement-string language
// VerifyPeerCodeSignature takes. One use for it: a process can hand its own
// requirement string to a child process it spawns, so that child can verify
// the parent back over the same connection, making the check mutual rather
// than one-directional.
func SelfRequirementString() (string, error) {
	cReq := C.copySelfRequirementString()
	if cReq == nil {
		return "", ErrNoSelfRequirement
	}
	defer C.free(unsafe.Pointer(cReq))
	return C.GoString(cReq), nil
}
