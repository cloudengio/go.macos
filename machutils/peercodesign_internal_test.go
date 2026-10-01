// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin && cgo

package machutils

import (
	"errors"
	"testing"
)

func TestVerifyPeerCodeSignatureRejectsMismatch(t *testing.T) {
	server, _ := unixConnPair(t)
	// No binary, signed or not, can satisfy an identifier that does not
	// exist: this is the security-critical direction to prove works, since
	// wrongly *accepting* an invalid peer, not wrongly rejecting a valid
	// one, is the dangerous failure mode.
	const impossible = `identifier "io.cloudeng.machutils.nonexistent-test-binary-xyz"`
	err := VerifyPeerCodeSignature(server, impossible)
	if err == nil {
		t.Fatal("expected an error for a requirement no binary can satisfy, got nil")
	}
	if !errors.Is(err, ErrPeerCodeSignatureInvalid) {
		t.Errorf("got %v, want it to wrap ErrPeerCodeSignatureInvalid", err)
	}
}

func TestVerifyPeerCodeSignatureAcceptsSelf(t *testing.T) {
	requirement, err := SelfRequirementString()
	if err != nil {
		t.Skipf("could not determine this test binary's own designated requirement: %v", err)
	}

	server, _ := unixConnPair(t)
	if err := VerifyPeerCodeSignature(server, requirement); err != nil {
		t.Errorf("VerifyPeerCodeSignature against this binary's own designated requirement (%q): %v", requirement, err)
	}
}

func TestVerifyPeerCodeSignature_NilConnection(t *testing.T) {
	if err := VerifyPeerCodeSignature(nil, `identifier "test"`); err == nil {
		t.Error("expected error for nil connection, got nil")
	} else if !errors.Is(err, ErrPeerCodeSignatureInvalid) {
		t.Errorf("got %v, want it to wrap ErrPeerCodeSignatureInvalid", err)
	}
}

func TestVerifyPeerCodeSignature_InvalidUTF8(t *testing.T) {
	server, _ := unixConnPair(t)
	invalidUTF8 := string([]byte{0xff, 0xfe, 0xfd})
	err := VerifyPeerCodeSignature(server, invalidUTF8)
	if err == nil {
		t.Fatal("expected error for invalid UTF-8, got nil")
	}
	if !errors.Is(err, ErrInvalidRequirement) {
		t.Errorf("got %v, want it to wrap ErrInvalidRequirement", err)
	}
}

func TestVerifyPeerCodeSignature_MalformedRequirement(t *testing.T) {
	server, _ := unixConnPair(t)
	err := VerifyPeerCodeSignature(server, "invalid requirement syntax !!!")
	if err == nil {
		t.Fatal("expected error for malformed requirement syntax, got nil")
	}
	if !errors.Is(err, ErrInvalidRequirement) {
		t.Errorf("got %v, want it to wrap ErrInvalidRequirement", err)
	}
}

func TestRequirement_Empty(t *testing.T) {
	if _, err := NewRequirement(""); err == nil {
		t.Error("expected error for empty requirement, got nil")
	} else if !errors.Is(err, ErrInvalidRequirement) {
		t.Errorf("got %v, want it to wrap ErrInvalidRequirement", err)
	}
}

func TestRequirement_Reuse(t *testing.T) {
	selfReq, err := SelfRequirementString()
	if err != nil {
		t.Skipf("could not determine this test binary's designated requirement: %v", err)
	}

	req, err := NewRequirement(selfReq)
	if err != nil {
		t.Fatalf("NewRequirement: %v", err)
	}
	defer req.Close()

	if req.String() != selfReq {
		t.Errorf("req.String() = %q, want %q", req.String(), selfReq)
	}

	// Verify on first pair
	server1, _ := unixConnPair(t)
	if err := req.Verify(server1); err != nil {
		t.Errorf("first verify failed: %v", err)
	}

	// Reuse on second pair
	server2, _ := unixConnPair(t)
	if err := req.Verify(server2); err != nil {
		t.Errorf("second verify failed: %v", err)
	}
}
