// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin && !cgo

package machutils_test

import (
	"errors"
	"testing"

	"cloudeng.io/macos/machutils"
)

func TestNoCgoStubs(t *testing.T) {
	if _, err := machutils.NewRequirement(`identifier "test"`); !errors.Is(err, machutils.ErrCgoDisabled) {
		t.Errorf("NewRequirement got %v, want ErrCgoDisabled", err)
	}
	if err := machutils.VerifyPeerCodeSignature(nil, `identifier "test"`); !errors.Is(err, machutils.ErrCgoDisabled) {
		t.Errorf("VerifyPeerCodeSignature got %v, want ErrCgoDisabled", err)
	}
	if _, err := machutils.SelfRequirementString(); !errors.Is(err, machutils.ErrCgoDisabled) {
		t.Errorf("SelfRequirementString got %v, want ErrCgoDisabled", err)
	}
}
