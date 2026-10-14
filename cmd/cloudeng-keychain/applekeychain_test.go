// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"cloudeng.io/file/filetestutil"
	"cloudeng.io/macos/keychain"
)

// The apple-keychain commands access the keychain directly, in-process, and
// so, unlike the read/write and key-info commands, cannot be tested against
// an injected in-memory keychain. These tests therefore write secure notes to
// the local, file-based keychain and remove them when the test completes.
func writeTestNote(t *testing.T, service, account string, data []byte) {
	t.Helper()
	kc := keychain.New(keychain.KeychainFileBased, account)
	// Remove any note left behind by an interrupted run.
	_ = kc.DeleteSecureNote(service)
	if err := kc.WriteSecureNote(service, data); err != nil {
		t.Fatalf("failed to write secure note %v: %v", service, err)
	}
	t.Cleanup(func() {
		if err := kc.DeleteSecureNote(service); err != nil {
			t.Logf("failed to delete secure note %v: %v", service, err)
		}
	})
}

// directFlags force a query to be made in-process rather than by a plugin. A
// plugin is located by searching /Applications and $PATH for the app bundles
// that contain one as well as for the plugin binary itself, so a test that
// must not pick up an installed plugin has to name all three as paths that
// do not exist.
var directFlags = []string{
	"--keychain-plugin=/nonexistent/cloudeng-keychain-plugin",
	"--keychain-app-bundle=/nonexistent/cloudeng-keychain.app",
	"--keychain-plugin-bundle=/nonexistent/cloudeng-keychain-plugin.app",
}

// mustQueryCLI runs 'apple-keychain query', in-process, and returns its
// stdout.
func mustQueryCLI(ctx context.Context, t *testing.T, args ...string) string {
	t.Helper()
	return mustQueryCLIWithFlags(ctx, t, append(slices.Clone(directFlags), args...)...)
}

// mustQueryCLIWithFlags runs 'apple-keychain query' with exactly the flags
// supplied and returns its stdout.
func mustQueryCLIWithFlags(ctx context.Context, t *testing.T, args ...string) string {
	t.Helper()
	args = append([]string{"apple-keychain", "query"}, args...)
	out, err := filetestutil.CaptureStdout(func() error {
		return runCLI(ctx, args...)
	})
	if err != nil {
		t.Fatalf("%v: %v", strings.Join(args, " "), err)
	}
	return string(out)
}

// buildPlugin builds the keychain plugin binary, returning its path. The
// binary is not signed and so holds no entitlements: it can search the
// file-based keychain, which is all that these tests require of it. Its
// permissions must deny group and world access or the plugin will refuse to
// run (see machutils.EnsureParentProcessSafe).
func buildPlugin(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "cloudeng-keychain-plugin")
	cmd := exec.Command("go", "build", "-o", binary, "./cloudeng-keychain-plugin")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building the plugin: %v: %s", err, out)
	}
	if err := os.Chmod(binary, 0700); err != nil {
		t.Fatalf("chmod %v: %v", binary, err)
	}
	return binary
}

func TestAppleKeychainQuery(t *testing.T) {
	ctx := context.Background()
	prefix := fmt.Sprintf("cloudeng.io-test-cli-%v", t.Name())
	alice := prefix + "-alice"
	bob := prefix + "-bob"
	const aliceContents = "alice-note-contents"
	writeTestNote(t, alice, "alice-account", []byte(aliceContents))
	writeTestNote(t, bob, "bob-account", []byte("bob-note-contents"))

	service := "--service=^" + regexp.QuoteMeta(prefix)

	// 1. The default, one line per item, output.
	out := mustQueryCLI(ctx, t, "--keychain-type=file", service)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if got, want := len(lines), 3; got != want { // header + two items
		t.Fatalf("got %v lines, want %v: %s", got, want, out)
	}
	wantContains(t, lines[0], "KEYCHAIN", "SERVICE", "ACCOUNT", "LABEL", "DESCRIPTION",
		"ACCESSIBILITY", "CREATED", "MODIFIED")
	// Results are sorted by service, so alice precedes bob.
	wantContains(t, lines[1], "file", alice, "alice-account", keychain.SecureNoteDescription)
	wantContains(t, lines[2], "file", bob, "bob-account")

	// The contents of a note must never be displayed.
	if strings.Contains(out, aliceContents) {
		t.Errorf("query output contains the contents of a note: %s", out)
	}

	// 2. Restricting the search by account.
	out = mustQueryCLI(ctx, t, "--keychain-type=file", service, "--account=^bob-")
	if got, want := len(strings.Split(strings.TrimSpace(out), "\n")), 2; got != want {
		t.Fatalf("got %v lines, want %v: %s", got, want, out)
	}
	wantContains(t, out, bob)
	if strings.Contains(out, alice) {
		t.Errorf("--account=^bob- matched %v: %s", alice, out)
	}

	// 3. The <regexp> argument matches any attribute.
	out = mustQueryCLI(ctx, t, "--keychain-type=file", service, "alice-account")
	wantContains(t, out, alice)
	if strings.Contains(out, bob) {
		t.Errorf("<regexp> alice-account matched %v: %s", bob, out)
	}

	// 4. A query that matches nothing writes nothing at all.
	if out := mustQueryCLI(ctx, t, "--keychain-type=file", service, "--account=^carol"); len(out) != 0 {
		t.Errorf("got %q, want no output", out)
	}

	// 5. --limit caps the number of items displayed.
	out = mustQueryCLI(ctx, t, "--keychain-type=file", service, "--limit=1")
	if got, want := len(strings.Split(strings.TrimSpace(out), "\n")), 2; got != want {
		t.Errorf("got %v lines, want %v: %s", got, want, out)
	}
}

func TestAppleKeychainQueryJSON(t *testing.T) {
	ctx := context.Background()
	service := fmt.Sprintf("cloudeng.io-test-cli-%v", t.Name())
	const contents = "json-note-contents"
	writeTestNote(t, service, "json-account", []byte(contents))

	out := mustQueryCLI(ctx, t, "--keychain-type=file", "--format=json",
		"--service=^"+regexp.QuoteMeta(service)+"$")
	var attrs []keychain.Attributes
	if err := json.Unmarshal([]byte(out), &attrs); err != nil {
		t.Fatalf("unmarshaling %q: %v", out, err)
	}
	if got, want := len(attrs), 1; got != want {
		t.Fatalf("got %v items, want %v: %s", got, want, out)
	}
	got := attrs[0]
	if got.Service != service || got.Account != "json-account" ||
		got.Keychain != keychain.KeychainFileBased ||
		got.Description != keychain.SecureNoteDescription {
		t.Errorf("attributes = %+v", got)
	}
	if got.Created.IsZero() || got.Modified.IsZero() {
		t.Errorf("created/modified not reported: %+v", got)
	}
	if strings.Contains(out, contents) {
		t.Errorf("JSON output contains the contents of a note: %s", out)
	}

	// A query that matches nothing is an empty JSON array rather than null.
	out = mustQueryCLI(ctx, t, "--keychain-type=file", "--format=json", "--service=^no-such-service$")
	if got, want := strings.TrimSpace(out), "[]"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAppleKeychainQueryLong(t *testing.T) {
	ctx := context.Background()
	service := fmt.Sprintf("cloudeng.io-test-cli-%v", t.Name())
	writeTestNote(t, service, "long-account", []byte("long-note-contents"))

	out := mustQueryCLI(ctx, t, "--keychain-type=file", "--long",
		"--service=^"+regexp.QuoteMeta(service)+"$")
	wantContains(t, out,
		"keychain:", "service:", "account:", "label:", "description:",
		"comment:", "access-group:", "accessibility:", "created:", "modified:",
		service, "long-account", keychain.SecureNoteDescription)
	if strings.Contains(out, "long-note-contents") {
		t.Errorf("--long output contains the contents of a note: %s", out)
	}
}

// TestAppleKeychainQuerySecureNotes verifies that --secure-notes restricts
// the results to items whose description identifies them as secure notes.
func TestAppleKeychainQuerySecureNotes(t *testing.T) {
	ctx := context.Background()
	service := fmt.Sprintf("cloudeng.io-test-cli-%v", t.Name())
	writeTestNote(t, service, "notes-account", []byte("notes-contents"))
	anchored := "--service=^" + regexp.QuoteMeta(service) + "$"

	out := mustQueryCLI(ctx, t, "--keychain-type=file", "--secure-notes", anchored)
	wantContains(t, out, service, keychain.SecureNoteDescription)

	// The same item is excluded by a description that it does not have.
	if out := mustQueryCLI(ctx, t, "--keychain-type=file", anchored,
		"--description=^application password$"); len(out) != 0 {
		t.Errorf("got %q, want no output", out)
	}
}

// TestAppleKeychainQueryErrors verifies that invalid flags are reported
// before the keychain is queried.
func TestAppleKeychainQueryErrors(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--service=[unclosed"}, "--service: invalid regular expression"},
		{[]string{"--account=*"}, "--account: invalid regular expression"},
		{[]string{"--label=(unclosed"}, "--label: invalid regular expression"},
		{[]string{"--description=(?P<"}, "--description: invalid regular expression"},
		{[]string{"--comment=[["}, "--comment: invalid regular expression"},
		{[]string{"[unclosed"}, `invalid regular expression "[unclosed"`},
		{[]string{"--format=xml"}, `--format: unsupported format "xml"`},
		{[]string{"--limit=-1"}, "--limit: must not be negative"},
		{[]string{"--keychain-type=no-such-keychain"}, "invalid value"},
	} {
		args := append([]string{"apple-keychain", "query"}, tc.args...)
		err := runCLI(ctx, args...)
		if err == nil {
			t.Errorf("%v: got nil error, want an error", strings.Join(args, " "))
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: got %v, want it to contain %q", strings.Join(args, " "), err, tc.want)
		}
	}
}

// TestAppleKeychainQueryViaPlugin verifies that the query is serviced by the
// plugin when one is found, ie. by the only binary that can be entitled to
// search the data-protection and iCloud keychains. The plugin built here is
// unsigned, so the item it is asked to find is one in the file-based
// keychain.
func TestAppleKeychainQueryViaPlugin(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the plugin binary")
	}
	ctx := context.Background()
	service := fmt.Sprintf("cloudeng.io-test-cli-%v", t.Name())
	const contents = "plugin-note-contents"
	writeTestNote(t, service, "plugin-account", []byte(contents))

	binary := buildPlugin(t)
	anchored := "--service=^" + regexp.QuoteMeta(service) + "$"
	args := []string{
		"--keychain-plugin=" + binary,
		"--keychain-app-bundle=/nonexistent/cloudeng-keychain.app",
		"--keychain-plugin-bundle=/nonexistent/cloudeng-keychain-plugin.app",
		"--keychain-type=file",
		anchored,
	}

	out := mustQueryCLIWithFlags(ctx, t, args...)
	wantContains(t, out, service, "plugin-account", keychain.SecureNoteDescription)
	if strings.Contains(out, contents) {
		t.Errorf("query output contains the contents of a note: %s", out)
	}

	// The same query as JSON, to verify that the attributes survive the
	// round trip through the plugin rather than merely being printed.
	out = mustQueryCLIWithFlags(ctx, t, append(slices.Clone(args), "--format=json")...)
	var attrs []keychain.Attributes
	if err := json.Unmarshal([]byte(out), &attrs); err != nil {
		t.Fatalf("unmarshaling %q: %v", out, err)
	}
	if got, want := len(attrs), 1; got != want {
		t.Fatalf("got %v items, want %v: %s", got, want, out)
	}
	if got := attrs[0]; got.Service != service || got.Account != "plugin-account" ||
		got.Keychain != keychain.KeychainFileBased || got.Created.IsZero() {
		t.Errorf("attributes = %+v", got)
	}

	// --verbose reports which plugin serviced the query, and the query makes
	// no complaint about missing entitlements since the plugin, not this
	// binary, is the one that needs them. The query is for an item that does
	// not exist so that it writes nothing to stdout.
	verbose := []string{
		"--verbose", "apple-keychain", "query",
		"--keychain-plugin=" + binary,
		"--keychain-app-bundle=/nonexistent/cloudeng-keychain.app",
		"--keychain-plugin-bundle=/nonexistent/cloudeng-keychain-plugin.app",
		"--keychain-type=all",
		"--service=^no-such-service$",
	}
	stderr, err := filetestutil.CaptureStderr(func() error {
		return runCLI(ctx, verbose...)
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	wantContains(t, string(stderr), "using plugin", binary)
	if strings.Contains(string(stderr), "warning:") {
		t.Errorf("warned about entitlements for a query made by a plugin: %s", stderr)
	}
}

// TestAppleKeychainQueryUnentitledWarning verifies that an in-process query
// of the keychains that require the keychain-access-groups entitlement says
// that it cannot see them, rather than reporting an empty result that looks
// like an empty keychain. The test binary is unsigned and so holds no
// entitlements.
func TestAppleKeychainQueryUnentitledWarning(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		keychainType string
		want         bool
	}{
		{"icloud", true},
		{"data-protection-local", true},
		{"all", true},
		{"file", false},
	} {
		args := append(slices.Clone(directFlags),
			"--keychain-type="+tc.keychainType, "--service=^no-such-service$")
		stderr, err := filetestutil.CaptureStderr(func() error {
			return runCLI(ctx, append([]string{"apple-keychain", "query"}, args...)...)
		})
		if err != nil {
			t.Fatalf("%v: %v", tc.keychainType, err)
		}
		got := strings.Contains(string(stderr), "keychain-access-groups")
		if got != tc.want {
			t.Errorf("%v: warned = %v, want %v: %s", tc.keychainType, got, tc.want, stderr)
		}
	}
}

// TestAppleKeychainQueryDirect verifies that --keychain-direct queries the
// keychain in-process, bypassing any plugin that is installed on this
// machine, and that it still says when a keychain it cannot see was asked
// for. Being able to avoid the plugin matters for a sandboxed binary, which
// cannot execute a plugin that is not inside its own bundle.
func TestAppleKeychainQueryDirect(t *testing.T) {
	ctx := context.Background()
	service := fmt.Sprintf("cloudeng.io-test-cli-%v", t.Name())
	const contents = "direct-note-contents"
	writeTestNote(t, service, "direct-account", []byte(contents))
	anchored := "--service=^" + regexp.QuoteMeta(service) + "$"

	// No plugin flags at all: the query must not consult an installed
	// plugin, whether or not one is present.
	out := mustQueryCLIWithFlags(ctx, t, "--keychain-direct", "--keychain-type=file", anchored)
	wantContains(t, out, service, "direct-account", keychain.SecureNoteDescription)
	if strings.Contains(out, contents) {
		t.Errorf("query output contains the contents of a note: %s", out)
	}

	// The keychains that need an entitlement this binary does not have are
	// still reported as invisible.
	stderr, err := filetestutil.CaptureStderr(func() error {
		return runCLI(ctx, "apple-keychain", "query", "--keychain-direct",
			"--keychain-type=icloud", "--service=^no-such-service$")
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	wantContains(t, string(stderr), "keychain-access-groups")
}
