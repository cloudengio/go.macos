// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin && cgo

package machutils

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation

#include <Security/Security.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>

// compileRequirement compiles requirementStr into a SecRequirementRef.
// Returns errSecSuccess on success and sets *outReq.
// Returns errSecCSReqInvalid if requirementStr is NULL, not UTF-8, or invalid syntax.
static OSStatus compileRequirement(const char *requirementStr, SecRequirementRef *outReq) {
	if (requirementStr == NULL || outReq == NULL) {
		return errSecCSReqInvalid;
	}
	CFStringRef reqString = CFStringCreateWithCString(kCFAllocatorDefault, requirementStr, kCFStringEncodingUTF8);
	if (reqString == NULL) {
		return errSecCSReqInvalid;
	}
	OSStatus status = SecRequirementCreateWithString(reqString, kSecCSDefaultFlags, outReq);
	CFRelease(reqString);
	return status;
}

// copyCFStringContents copies a CFStringRef's contents into a newly
// allocated C string the caller must free, or NULL if s is NULL or
// could not be represented in UTF-8.
static char *copyCFStringContents(CFStringRef s) {
	if (s == NULL) {
		return NULL;
	}
	CFIndex length = CFStringGetLength(s);
	CFIndex maxSize = CFStringGetMaximumSizeForEncoding(length, kCFStringEncodingUTF8) + 1;
	char *buf = malloc((size_t)maxSize);
	if (buf == NULL || !CFStringGetCString(s, buf, maxSize, kCFStringEncodingUTF8)) {
		free(buf);
		return NULL;
	}
	return buf;
}

// copySecErrorMessage returns the human readable description of an OSStatus
// from the Security framework, as a newly allocated C string the caller must
// free, or NULL if none is available.
static char *copySecErrorMessage(OSStatus status) {
	CFStringRef msg = SecCopyErrorMessageString(status, NULL);
	if (msg == NULL) {
		return NULL;
	}
	char *buf = copyCFStringContents(msg);
	CFRelease(msg);
	return buf;
}

// verifyGuestCode resolves the peer process to its running code object and
// validates it against requirement using SecCodeCheckValidityWithErrors.
// If auditToken is non-NULL and tokenLen > 0, it uses kSecGuestAttributeAudit,
// which matches the exact process instance including its kernel pidversion
// to prevent PID reuse. Otherwise, it falls back to kSecGuestAttributePid.
static OSStatus verifyGuestCode(const void *auditToken, size_t tokenLen, pid_t pid, SecRequirementRef requirement, char **outErrMsg) {
	CFDictionaryRef attrs = NULL;
	if (auditToken != NULL && tokenLen > 0) {
		CFDataRef tokenData = CFDataCreate(kCFAllocatorDefault, (const UInt8 *)auditToken, (CFIndex)tokenLen);
		if (tokenData != NULL) {
			const void *keys[1] = { kSecGuestAttributeAudit };
			const void *values[1] = { tokenData };
			attrs = CFDictionaryCreate(kCFAllocatorDefault, keys, values, 1,
				&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
			CFRelease(tokenData);
		}
	}
	if (attrs == NULL && pid > 0) {
		CFNumberRef pidNum = CFNumberCreate(kCFAllocatorDefault, kCFNumberIntType, &pid);
		if (pidNum != NULL) {
			const void *keys[1] = { kSecGuestAttributePid };
			const void *values[1] = { pidNum };
			attrs = CFDictionaryCreate(kCFAllocatorDefault, keys, values, 1,
				&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
			CFRelease(pidNum);
		}
	}
	if (attrs == NULL) {
		return errSecAllocate;
	}

	SecCodeRef code = NULL;
	OSStatus status = SecCodeCopyGuestWithAttributes(NULL, attrs, kSecCSDefaultFlags, &code);
	CFRelease(attrs);
	if (status != errSecSuccess) {
		return status;
	}

	CFErrorRef cfErr = NULL;
	SecCSFlags flags = kSecCSDefaultFlags | kSecCSConsiderExpiration;
	status = SecCodeCheckValidityWithErrors(code, flags, requirement, &cfErr);
	CFRelease(code);

	if (status != errSecSuccess && cfErr != NULL) {
		if (outErrMsg != NULL) {
			CFStringRef desc = CFErrorCopyDescription(cfErr);
			if (desc != NULL) {
				*outErrMsg = copyCFStringContents(desc);
				CFRelease(desc);
			}
		}
		CFRelease(cfErr);
	}
	return status;
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
	"runtime"
	"unsafe"
)

// ErrPeerCodeSignatureInvalid is returned by VerifyPeerCodeSignature or
// Requirement.Verify when the peer either cannot be identified, is no longer
// running, or does not satisfy the given requirement.
var ErrPeerCodeSignatureInvalid = errors.New("peer code signature is not valid or does not satisfy the requirement")

// ErrInvalidRequirement is returned when a code signing requirement string
// cannot be compiled, e.g. because of a syntax error or invalid encoding.
var ErrInvalidRequirement = errors.New("invalid code signing requirement")

// ErrNoSelfRequirement is returned by SelfRequirementString when the running
// binary's own designated code signing requirement cannot be determined,
// typically because it is unsigned.
var ErrNoSelfRequirement = errors.New("could not determine this process's own designated code signing requirement")

// Requirement represents a compiled macOS code signing requirement.
// It can be safely reused across multiple connections for efficient verification.
type Requirement struct {
	req C.SecRequirementRef
	str string
}

// NewRequirement parses and compiles a code signing requirement string.
// It returns ErrInvalidRequirement if the requirement string is invalid.
//
//nolint:gocritic // cgo pointer checks generate dupSubExpr in AST
func NewRequirement(requirement string) (*Requirement, error) {
	if requirement == "" {
		return nil, fmt.Errorf("%w: empty requirement string", ErrInvalidRequirement)
	}
	cReq := C.CString(requirement)
	defer C.free(unsafe.Pointer(cReq))

	var req C.SecRequirementRef
	status := C.compileRequirement(cReq, &req)
	if status != C.errSecSuccess {
		if msg := C.copySecErrorMessage(status); msg != nil {
			defer C.free(unsafe.Pointer(msg))
			return nil, fmt.Errorf("%w: %s (OSStatus %d)", ErrInvalidRequirement, C.GoString(msg), int(status))
		}
		return nil, fmt.Errorf("%w: OSStatus %d", ErrInvalidRequirement, int(status))
	}

	r := &Requirement{req: req, str: requirement}
	runtime.SetFinalizer(r, func(obj *Requirement) {
		obj.Close()
	})
	return r, nil
}

// Close releases the underlying SecRequirementRef.
func (r *Requirement) Close() {
	if uintptr(r.req) != 0 {
		C.CFRelease(C.CFTypeRef(r.req))
		r.req = 0
	}
}

// String returns the requirement string from which r was compiled.
func (r *Requirement) String() string {
	return r.str
}

// Verify verifies that the process connected via conn satisfies this requirement.
//
//nolint:gocritic // cgo pointer checks generate dupSubExpr in AST
func (r *Requirement) Verify(conn *net.UnixConn) error {
	if conn == nil {
		return fmt.Errorf("%w: nil connection", ErrPeerCodeSignatureInvalid)
	}
	if r == nil || uintptr(r.req) == 0 {
		return fmt.Errorf("%w: uninitialized requirement", ErrInvalidRequirement)
	}

	token, hasToken := peerAuditToken(conn)
	var pid int32
	var tokenPtr unsafe.Pointer
	var tokenLen C.size_t
	if hasToken {
		pid = token.pid()
		tokenPtr = unsafe.Pointer(&token)
		tokenLen = C.size_t(unsafe.Sizeof(token))
	} else {
		var err error
		pid, err = PeerPID(conn)
		if err != nil {
			return err
		}
	}

	var cErrMsg *C.char
	status := C.verifyGuestCode(tokenPtr, tokenLen, C.pid_t(pid), r.req, &cErrMsg)
	if status == C.errSecSuccess {
		return nil
	}

	var errMsg string
	if cErrMsg != nil {
		errMsg = C.GoString(cErrMsg)
		C.free(unsafe.Pointer(cErrMsg))
	} else if msg := C.copySecErrorMessage(status); msg != nil {
		errMsg = C.GoString(msg)
		C.free(unsafe.Pointer(msg))
	}

	if errMsg != "" {
		return fmt.Errorf("%w: pid %d: %s (OSStatus %d)", ErrPeerCodeSignatureInvalid, pid, errMsg, int(status))
	}
	return fmt.Errorf("%w: pid %d: OSStatus %d", ErrPeerCodeSignatureInvalid, pid, int(status))
}

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
// the peer of a local socket connection.
//
// On modern macOS systems, peer verification retrieves the peer's kernel audit
// token (via LOCAL_PEERTOKEN), which includes the task generation ID (pidversion).
// This binds verification directly to the connecting process instance, eliminating
// the PID recycling race condition. If audit tokens are unavailable, verification
// falls back to the peer PID.
//
// A caller should perform this check immediately after accepting conn and
// before reading or acting on anything it sends, closing conn without
// further use if it returns an error.
func VerifyPeerCodeSignature(conn *net.UnixConn, requirement string) error {
	req, err := NewRequirement(requirement)
	if err != nil {
		return err
	}
	defer req.Close()
	return req.Verify(conn)
}

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
