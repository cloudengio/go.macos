// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package tartvm

import (
	"context"
	"testing"

	"cloudeng.io/vms/ociimage"
)

// remoteRef is a reference naming a registry, which is the only kind that can
// be pulled.
func remoteRef(t *testing.T, s string) ociimage.Reference {
	t.Helper()
	ref, err := ociimage.Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	if !ref.Remote() {
		t.Fatalf("%q names no registry, so it cannot be pulled", s)
	}
	return ref
}

// TestPullArgs covers the command line Pull builds: the reference is pulled
// whole, so that its registry and tag reach tart, and the options appear only
// when they were asked for, leaving tart's own defaults in place otherwise.
func TestPullArgs(t *testing.T) {
	for _, tc := range []struct {
		name string
		ref  string
		opts ociimage.PullOptions
		want string
	}{
		{"no options", "ghcr.io/cirruslabs/ubuntu:latest", ociimage.PullOptions{}, "pull ghcr.io/cirruslabs/ubuntu:latest"},
		{"insecure", "mm-1:5001/linux-ci:latest", ociimage.PullOptions{Insecure: true}, "pull mm-1:5001/linux-ci:latest --insecure"},
		{"concurrency", "mm-1:5001/linux-ci:latest", ociimage.PullOptions{Concurrency: 4}, "pull mm-1:5001/linux-ci:latest --concurrency 4"},
		{
			"both", "mm-1:5001/linux-ci:latest",
			ociimage.PullOptions{Insecure: true, Concurrency: 2},
			"pull mm-1:5001/linux-ci:latest --insecure --concurrency 2",
		},
		{
			// A digest is part of the reference, so it is pulled with it.
			"digest", "ghcr.io/cirruslabs/ubuntu@sha256:abc123",
			ociimage.PullOptions{},
			"pull ghcr.io/cirruslabs/ubuntu@sha256:abc123",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invocations := fakeTart(t, "", "", 0)
			if err := Pull(context.Background(), "", remoteRef(t, tc.ref), tc.opts); err != nil {
				t.Fatalf("Pull: %v", err)
			}
			got := invocations()
			if len(got) != 1 {
				t.Fatalf("tart invocations: got %v, want exactly one", got)
			}
			if got[0] != tc.want {
				t.Errorf("tart args: got %q, want %q", got[0], tc.want)
			}
		})
	}
}

// TestPullFails verifies that a failed pull is reported rather than passing
// for a pull that fetched nothing, which a caller relies on to fail the
// creation of a VM instead of cloning a stale image.
func TestPullFails(t *testing.T) {
	invocations := fakeTart(t, "", "unauthorized", 1)
	err := Pull(context.Background(), "", remoteRef(t, "ghcr.io/cirruslabs/ubuntu:latest"), ociimage.PullOptions{})
	if err == nil {
		t.Fatal("Pull: got nil error, want the stub's failure")
	}
	if len(invocations()) != 1 {
		t.Errorf("tart invocations: got %v, want exactly one", invocations())
	}
}

// TestProviderPull covers the Provider as an ociimage.Puller: it pulls with
// the tart binary it was configured with, and goes through ociimage.Pull as a
// caller that need not know which references can be pulled at all.
func TestProviderPull(t *testing.T) {
	var _ ociimage.Puller = (*Provider)(nil)

	ctx := context.Background()
	ref := remoteRef(t, "ghcr.io/cirruslabs/ubuntu:latest")
	opts := ociimage.PullOptions{Insecure: true}

	invocations := fakeTart(t, "", "", 0)
	p := NewProvider(nil, WithProviderImage(ref))
	// The provider reports the reference it was configured with, parsed, which
	// is what it was told its constructor clones.
	if got := p.Details().Image(); got != ref {
		t.Errorf("Details().Image(): got %+v, want %+v", got, ref)
	}
	if err := ociimage.Pull(ctx, p, ref, opts); err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if got, want := invocations(), []string{"pull ghcr.io/cirruslabs/ubuntu:latest --insecure"}; len(got) != 1 || got[0] != want[0] {
		t.Errorf("tart args: got %v, want %v", got, want)
	}

	// A local reference has nowhere to be pulled from, so ociimage.Pull does
	// not run tart at all rather than letting it fail.
	local, err := ociimage.Parse("linux-ci")
	if err != nil {
		t.Fatal(err)
	}
	notInvoked := fakeTart(t, "", "", 0)
	if err := ociimage.Pull(ctx, p, local, opts); err != nil {
		t.Errorf("Pull: got %v, want a local reference to be skipped", err)
	}
	if got := notInvoked(); got != nil {
		t.Errorf("tart invocations: got %v, want none", got)
	}
}
