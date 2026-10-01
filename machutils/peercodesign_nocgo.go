// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin && !cgo

package machutils

import (
	"errors"
	"net"
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

// ErrCgoDisabled is returned when peer code signature verification is attempted
// in an environment where cgo is disabled.
var ErrCgoDisabled = errors.New("cgo is required for peer code signature verification on darwin")

// Requirement represents a compiled macOS code signing requirement.
type Requirement struct{}

// NewRequirement returns an error when cgo is disabled.
func NewRequirement(_ string) (*Requirement, error) {
	return nil, ErrCgoDisabled
}

// Close is a no-op when cgo is disabled.
func (r *Requirement) Close() {}

// String returns empty string when cgo is disabled.
func (r *Requirement) String() string {
	return ""
}

// Verify returns an error when cgo is disabled.
func (r *Requirement) Verify(_ *net.UnixConn) error {
	return ErrCgoDisabled
}

// VerifyPeerCodeSignature returns an error when cgo is disabled.
func VerifyPeerCodeSignature(_ *net.UnixConn, _ string) error {
	return ErrCgoDisabled
}

// SelfRequirementString returns an error when cgo is disabled.
func SelfRequirementString() (string, error) {
	return "", ErrCgoDisabled
}
