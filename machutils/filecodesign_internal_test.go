// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin && cgo

package machutils

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func testExecutable(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe
}

func TestVerifyFileCodeSignatureAcceptsSelf(t *testing.T) {
	requirement, err := SelfRequirementString()
	if err != nil {
		t.Skipf("could not determine this test binary's own designated requirement: %v", err)
	}
	if err := VerifyFileCodeSignature(testExecutable(t), requirement); err != nil {
		t.Errorf("VerifyFileCodeSignature against this binary's own designated requirement (%q): %v", requirement, err)
	}
}

func TestVerifyFileCodeSignatureRejectsMismatch(t *testing.T) {
	const impossible = `identifier "io.cloudeng.machutils.nonexistent-test-binary-xyz"`
	err := VerifyFileCodeSignature(testExecutable(t), impossible)
	if !errors.Is(err, ErrFileCodeSignatureInvalid) {
		t.Errorf("got %v, want it to wrap ErrFileCodeSignatureInvalid", err)
	}
}

func TestVerifyFileCodeSignatureRejectsModified(t *testing.T) {
	requirement, err := SelfRequirementString()
	if err != nil {
		t.Skipf("could not determine this test binary's own designated requirement: %v", err)
	}
	data, err := os.ReadFile(testExecutable(t))
	if err != nil {
		t.Fatal(err)
	}
	// A copy is still valid, while a copy with a byte of its code changed,
	// one well past the headers, is not.
	dir := t.TempDir()
	same, modified := filepath.Join(dir, "same"), filepath.Join(dir, "modified")
	if err := os.WriteFile(same, data, 0o600); err != nil { //nolint:gosec // a path in t.TempDir
		t.Fatal(err)
	}
	if err := VerifyFileCodeSignature(same, requirement); err != nil {
		t.Errorf("unmodified copy: %v", err)
	}
	data[len(data)/2] ^= 0xff
	if err := os.WriteFile(modified, data, 0o600); err != nil { //nolint:gosec // a path in t.TempDir
		t.Fatal(err)
	}
	if err := VerifyFileCodeSignature(modified, requirement); !errors.Is(err, ErrFileCodeSignatureInvalid) {
		t.Errorf("modified copy: got %v, want it to wrap ErrFileCodeSignatureInvalid", err)
	}
}

func TestVerifyFileCodeSignatureRejectsUnsigned(t *testing.T) {
	dir := t.TempDir()
	unsigned := filepath.Join(dir, "unsigned.sh")
	if err := os.WriteFile(unsigned, []byte("#!/bin/sh\nexit 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFileCodeSignature(unsigned, `anchor apple generic`); !errors.Is(err, ErrFileCodeSignatureInvalid) {
		t.Errorf("unsigned: got %v, want it to wrap ErrFileCodeSignatureInvalid", err)
	}
	if err := VerifyFileCodeSignature(filepath.Join(dir, "missing"), `anchor apple generic`); !errors.Is(err, ErrFileCodeSignatureInvalid) {
		t.Errorf("missing: got %v, want it to wrap ErrFileCodeSignatureInvalid", err)
	}
	if err := VerifyFileCodeSignature("", `anchor apple generic`); !errors.Is(err, ErrFileCodeSignatureInvalid) {
		t.Errorf("empty path: got %v, want it to wrap ErrFileCodeSignatureInvalid", err)
	}
}

func TestVerifyFileCodeSignatureAppleBinary(t *testing.T) {
	ls, err := exec.LookPath("ls")
	if err != nil {
		t.Skip(err)
	}
	if err := VerifyFileCodeSignature(ls, `anchor apple`); err != nil {
		t.Errorf("%v: %v", ls, err)
	}
	if err := VerifyFileCodeSignature(ls, `anchor apple generic and certificate leaf[subject.OU] = "NOTATEAM00"`); !errors.Is(err, ErrFileCodeSignatureInvalid) {
		t.Errorf("%v with another team: got %v, want it to wrap ErrFileCodeSignatureInvalid", ls, err)
	}
}

func TestVerifyFileCodeSignatureInvalidRequirement(t *testing.T) {
	if err := VerifyFileCodeSignature(testExecutable(t), "invalid requirement syntax !!!"); !errors.Is(err, ErrInvalidRequirement) {
		t.Errorf("got %v, want it to wrap ErrInvalidRequirement", err)
	}
}
