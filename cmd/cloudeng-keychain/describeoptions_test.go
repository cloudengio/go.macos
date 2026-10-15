// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package main

import (
	"context"
	"strings"
	"testing"

	"cloudeng.io/file/filetestutil"
	"cloudeng.io/macos/keychain"
)

// describedOptions runs 'describe-options' and returns what it wrote.
func describedOptions(t *testing.T) string {
	t.Helper()
	out, err := filetestutil.CaptureStdout(func() error {
		return runCLI(context.Background(), "describe-options")
	})
	if err != nil {
		t.Fatalf("describe-options: %v", err)
	}
	return string(out)
}

// TestDescribeOptions verifies that every value of every enum that a flag
// accepts is listed. The expectations come from the enums themselves, so a
// value added to one but not to the listing fails the test.
func TestDescribeOptions(t *testing.T) {
	listed := describedOptions(t)
	for _, group := range []struct {
		name   string
		values []string
	}{
		{"keychain type", enumValues(keychain.Type(0))},
		{"keychain write type", enumValues(keychain.WriteType(0))},
		{"accessibility", enumValues(keychain.Accessibility(0))},
		{"output format", []string{formatText, formatJSON}},
		{"flag", []string{"--keychain-type", "--keychain-accessibility", "--set-accessibility", "--format"}},
	} {
		for _, value := range group.values {
			if !strings.Contains(listed, value) {
				t.Errorf("%v %q is not listed:\n%s", group.name, value, listed)
			}
		}
	}
}

// TestDescribeOptionsDefaults verifies that the defaults reported are those
// of the flags, read from the flags rather than restated, and that each
// keychain type is described and not merely named.
func TestDescribeOptionsDefaults(t *testing.T) {
	listed := describedOptions(t)
	for _, tc := range []struct {
		flags any
		name  string
	}{
		{&ReadFlags{}, "keychain-type"},
		{&WriteFlags{}, "keychain-type"},
		{&WriteFlags{}, "keychain-accessibility"},
		{&AppleKeychainQueryFlags{}, "format"},
	} {
		got := flagDefault(tc.flags, tc.name)
		if got == "unknown" || len(got) == 0 {
			t.Errorf("--%v: default could not be read from the flag", tc.name)
			continue
		}
		if !strings.Contains(listed, got) {
			t.Errorf("--%v: default %q is not listed:\n%s", tc.name, got, listed)
		}
	}
	for _, value := range enumValues(keychain.Type(0)) {
		if len(keychainTypeDescriptions[value]) == 0 {
			t.Errorf("keychain type %q has no description", value)
		}
	}
}
