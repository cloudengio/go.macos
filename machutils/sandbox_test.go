// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package machutils_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"cloudeng.io/macos/machutils"
	"howett.net/plist"
)

func entitlementsPlist(body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>` +
		`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` +
		`<plist version="1.0"><dict>` + body + `</dict></plist>`
}

// signedCopy returns a copy of the test binary signed with the given
// entitlements, which is the only way to control what a process reports for
// itself: the entitlements come from the signature of the running executable.
func signedCopy(t *testing.T, entitlements string) string {
	t.Helper()
	codesign, err := exec.LookPath("codesign")
	if err != nil {
		t.Skip("codesign is not available in this environment")
	}
	dir := t.TempDir()
	data, err := os.ReadFile(testBinary)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "signed-test-bin")
	if err := os.WriteFile(bin, data, 0700); err != nil { //nolint:gosec // an executable copy
		t.Fatal(err)
	}
	plistPath := filepath.Join(dir, "entitlements.plist")
	if err := os.WriteFile(plistPath, []byte(entitlements), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(codesign, "-f", "-s", "-", "--entitlements", plistPath, bin).CombinedOutput()
	if err != nil {
		t.Fatalf("codesign failed: %v: %s", err, out)
	}
	return bin
}

// TestEntitlementsNone verifies the case of an executable signed without any
// entitlements, which the kernel reports as success with nothing copied
// rather than as an error. A Go test binary is such an executable.
func TestEntitlementsNone(t *testing.T) {
	entitlements, err := machutils.Entitlements()
	if err != nil {
		t.Fatal(err)
	}
	if entitlements != nil {
		t.Errorf("got %s, want no entitlements", entitlements)
	}
	sandboxed, err := machutils.IsSandboxed()
	if err != nil {
		t.Fatal(err)
	}
	if sandboxed {
		t.Error("the test binary reports itself as sandboxed")
	}
}

// TestSubprocessEntitlements verifies that entitlements recorded in a
// signature are read back from the kernel, which is the half of IsSandboxed
// that a table test cannot reach.
func TestSubprocessEntitlements(t *testing.T) {
	bin := signedCopy(t, entitlementsPlist(
		`<key>com.apple.security.get-task-allow</key><true/>`+
			`<key>com.apple.security.network.client</key><true/>`))

	stdout, stderr, err := runBinary(t, bin, "-entitlements")
	if err != nil {
		t.Fatalf("subprocess failed: %v (stderr: %s)", err, stderr)
	}
	// What comes back must be the property list that was signed in, not just
	// something containing the right words.
	var values map[string]any
	if _, err := plist.Unmarshal([]byte(stdout), &values); err != nil {
		t.Fatalf("the entitlements do not parse: %v: %q", err, stdout)
	}
	for _, key := range []string{"com.apple.security.get-task-allow", "com.apple.security.network.client"} {
		if enabled, ok := values[key].(bool); !ok || !enabled {
			t.Errorf("%v: got %v, want true", key, values[key])
		}
	}
	if _, ok := values[machutils.AppSandboxEntitlement]; ok {
		t.Errorf("%v is present but was not signed in", machutils.AppSandboxEntitlement)
	}

	// Entitlements that do not include the sandbox must not be reported as
	// sandboxed: having entitlements at all is not the test.
	stdout, stderr, err = runBinary(t, bin, "-sandboxed")
	if err != nil {
		t.Fatalf("subprocess failed: %v (stderr: %s)", err, stderr)
	}
	if got, want := strings.TrimSpace(stdout), "false"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestSubprocessSandboxEntitlement runs a binary signed with the sandbox
// entitlement. macOS refuses to run one outside a container, so it is normally
// killed before it reports anything; should a release run it, it must report
// itself as sandboxed. Either outcome shows that the entitlement cannot be
// carried by a process running unconfined, which is what makes its presence
// conclusive.
func TestSubprocessSandboxEntitlement(t *testing.T) {
	bin := signedCopy(t, entitlementsPlist(`<key>com.apple.security.app-sandbox</key><true/>`))
	stdout, stderr, err := runBinary(t, bin, "-sandboxed")
	if err != nil {
		t.Logf("the kernel refused to run a sandboxed binary with no container: %v (stderr: %s)", err, stderr)
		return
	}
	if got, want := strings.TrimSpace(stdout), "true"; got != want {
		t.Errorf("a process carrying %v reported %v, want %v", machutils.AppSandboxEntitlement, got, want)
	}
}
