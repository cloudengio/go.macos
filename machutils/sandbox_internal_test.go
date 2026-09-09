// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package machutils

import (
	"strings"
	"testing"
)

func entitlementsPlist(body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>` +
		`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` +
		`<plist version="1.0"><dict>` + body + `</dict></plist>`
}

// TestHasAppSandboxEntitlement covers the decision made about an entitlements
// property list, which cannot be reached through IsSandboxed from a test: a
// process carrying the sandbox entitlement but no container is killed at exec,
// so no test binary can be made to report true.
func TestHasAppSandboxEntitlement(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want bool
	}{
		{"none at all", "", false},
		{"whitespace only", "  \n\t", false},
		{"sandboxed", entitlementsPlist(`<key>com.apple.security.app-sandbox</key><true/>`), true},
		{"explicitly not sandboxed", entitlementsPlist(`<key>com.apple.security.app-sandbox</key><false/>`), false},
		{"other entitlements only", entitlementsPlist(`<key>com.apple.security.get-task-allow</key><true/>`), false},
		{"empty list", entitlementsPlist(``), false},
		{"sandboxed alongside others",
			entitlementsPlist(`<key>com.apple.security.get-task-allow</key><true/>` +
				`<key>com.apple.security.app-sandbox</key><true/>` +
				`<key>com.apple.security.network.client</key><true/>`), true},
		// The entitlement is a boolean; a string that reads as one is not it.
		{"not a boolean", entitlementsPlist(`<key>com.apple.security.app-sandbox</key><string>true</string>`), false},
		// The blob is trailed by padding in a real signature.
		{"trailing padding", entitlementsPlist(`<key>com.apple.security.app-sandbox</key><true/>`) + " \x00", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := hasAppSandboxEntitlement([]byte(tc.in))
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestHasAppSandboxEntitlementMalformed verifies that a list which cannot be
// parsed is reported as an error rather than as not sandboxed, since assuming
// the latter would be assuming the safer state on no evidence.
func TestHasAppSandboxEntitlementMalformed(t *testing.T) {
	_, err := hasAppSandboxEntitlement([]byte("<plist><dict><key>unterminated"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if got, want := err.Error(), "parsing entitlements"; !strings.Contains(got, want) {
		t.Errorf("error %q does not contain %q", got, want)
	}
}
