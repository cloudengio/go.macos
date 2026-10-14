// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package keychain_test

import (
	"strings"
	"testing"

	"cloudeng.io/macos/keychain"
)

func TestPropertiesEmpty(t *testing.T) {
	if !(keychain.Properties{}).Empty() {
		t.Error("a zero Properties is not empty")
	}
	for _, props := range []keychain.Properties{
		{Account: "a"}, {Label: "l"}, {Description: "d"}, {Accessibility: "when-unlocked"},
	} {
		if props.Empty() {
			t.Errorf("%+v: reported as empty", props)
		}
	}
}

// TestPropertiesValidate covers what a change to an item's attributes is
// refused for. SetProperties and the in-memory keychain that stands in for
// it both validate through this, so both refuse the same things.
func TestPropertiesValidate(t *testing.T) {
	for _, tc := range []struct {
		name  string
		typ   keychain.Type
		props keychain.Properties
		want  string
	}{
		{"nothing to set", keychain.KeychainICloud, keychain.Properties{}, "no properties to set"},
		{
			"invalid accessibility", keychain.KeychainICloud,
			keychain.Properties{Accessibility: "whenever"}, "accessibility: invalid",
		},
		{
			// The file-based keychain has no accessibility attribute and
			// ignores one, so setting it must be refused rather than appear
			// to have worked.
			"accessibility on a file-based item", keychain.KeychainFileBased,
			keychain.Properties{Accessibility: "when-unlocked"},
			"accessibility cannot be set for an item in the file-based keychain",
		},
		{
			"keychain type all", keychain.KeychainAll,
			keychain.Properties{Label: "x"}, "cannot set properties in keychain of type 'all'",
		},
	} {
		err := tc.props.Validate(tc.typ)
		if err == nil {
			t.Errorf("%v: got nil error, want an error", tc.name)
			continue
		}
		if got := err.Error(); !strings.Contains(got, tc.want) {
			t.Errorf("%v: got %q, want it to contain %q", tc.name, got, tc.want)
		}
	}

	// What is accepted: each attribute on its own, and an accessibility for
	// the keychains that have the attribute.
	for _, tc := range []struct {
		typ   keychain.Type
		props keychain.Properties
	}{
		{keychain.KeychainFileBased, keychain.Properties{Account: "a"}},
		{keychain.KeychainFileBased, keychain.Properties{Label: "l"}},
		{keychain.KeychainFileBased, keychain.Properties{Description: "d"}},
		{keychain.KeychainICloud, keychain.Properties{Accessibility: "after-first-unlock"}},
		{keychain.KeychainDataProtectionLocal, keychain.Properties{Accessibility: "when-unlocked"}},
		{keychain.KeychainICloud, keychain.Properties{Account: "a", Label: "l", Description: "d", Accessibility: "always"}},
	} {
		if err := tc.props.Validate(tc.typ); err != nil {
			t.Errorf("%v %+v: %v", tc.typ, tc.props, err)
		}
	}
}
