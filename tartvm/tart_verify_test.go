// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package tartvm

import (
	"context"
	"errors"
	"testing"

	"cloudeng.io/vms"
	"cloudeng.io/vms/ociimage"
)

const verifiedDigest = "sha256:0123456789abcdef"

var verifyPolicy = ociimage.Policy{Signers: []ociimage.Signer{{Key: "cosign.pub"}}}

// stubVerifier pins the reference to verifiedDigest, or fails with err.
type stubVerifier struct {
	err   error
	calls int
}

func (s *stubVerifier) Verify(_ context.Context, ref ociimage.Reference, _ ociimage.Policy) (ociimage.Verified, error) {
	s.calls++
	if s.err != nil {
		return ociimage.Verified{}, s.err
	}
	return ociimage.Verified{Reference: ref.WithDigest(verifiedDigest), Signatures: []ociimage.Signature{{Key: "cosign.pub"}}}, nil
}

// TestCloneVerifies checks that the digest that was verified is what is
// cloned, and that nothing is verified unless it was asked for.
func TestCloneVerifies(t *testing.T) {
	invocations := fakeTart(t, "", "", 0)
	v := &stubVerifier{}
	inst := New(t.Context(), "ghcr.io/cirruslabs/ubuntu:latest", "vm", WithVerification(v, verifyPolicy))
	if err := inst.Clone(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got, want := invocations(), []string{"clone ghcr.io/cirruslabs/ubuntu@" + verifiedDigest + " vm"}; len(got) != 1 || got[0] != want[0] {
		t.Errorf("got %v, want %v", got, want)
	}

	invocations = fakeTart(t, "", "", 0)
	v = &stubVerifier{}
	inst = New(t.Context(), "ghcr.io/cirruslabs/ubuntu:latest", "vm")
	if err := inst.Clone(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got, want := invocations(), []string{"clone ghcr.io/cirruslabs/ubuntu:latest vm"}; got[0] != want[0] || v.calls != 0 {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestCloneVerificationFails checks that nothing is cloned, and the instance
// is left as it was, for an image that is unsigned or cannot be verified.
func TestCloneVerificationFails(t *testing.T) {
	for name, tc := range map[string]struct {
		source string
		err    error
	}{
		"unsigned":    {"ghcr.io/cirruslabs/ubuntu:latest", errors.Join(ociimage.ErrNotVerified, ociimage.ErrUnsigned)},
		"untrusted":   {"ghcr.io/cirruslabs/ubuntu:latest", ociimage.ErrNotVerified},
		"local image": {"linux-ci", nil},
	} {
		invocations := fakeTart(t, "", "", 0)
		inst := New(t.Context(), tc.source, "vm", WithVerification(&stubVerifier{err: tc.err}, verifyPolicy))
		err := inst.Clone(t.Context())
		if !errors.Is(err, ociimage.ErrNotVerified) {
			t.Errorf("%v: got %v, want ErrNotVerified", name, err)
		}
		if tc.err != nil && !errors.Is(err, tc.err) {
			t.Errorf("%v: the cause was lost: %v", name, err)
		}
		if got := invocations(); len(got) != 0 {
			t.Errorf("%v: tart was run: %v", name, got)
		}
		if got := inst.State(t.Context()); got != vms.StateInitial {
			t.Errorf("%v: state %v", name, got)
		}
	}
}
