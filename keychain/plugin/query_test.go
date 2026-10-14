// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package plugin_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"cloudeng.io/encoding/json/jsonmsgs"
	"cloudeng.io/macos/keychain"
	"cloudeng.io/macos/keychain/plugin"
	"cloudeng.io/security/keys/keychain/plugins"
)

// writeTestNote writes a secure note to the file-based keychain and arranges
// for it to be deleted when the test completes. The file-based keychain is
// used because, unlike the data-protection and iCloud keychains, it is
// accessible to an unsigned test binary.
func writeTestNote(t *testing.T, service, account string, data []byte) {
	t.Helper()
	kc := keychain.New(keychain.KeychainFileBased, account)
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

// handle runs a request through a plugin Server as the plugin binary does,
// ie. by encoding it, decoding it again and then handling it.
func handle(t *testing.T, cfg plugin.Config) *plugins.Response {
	t.Helper()
	req, err := plugin.NewRequest("query", cfg)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	var buf bytes.Buffer
	msgr := jsonmsgs.NewMessager(io.NopCloser(&buf), &buf)
	if err := plugins.WriteRequest(msgr, req); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}
	srv := plugin.NewServer()
	rCfg, rReq, resp := srv.ReadRequest(t.Context(), msgr)
	if resp != nil {
		t.Fatalf("ReadRequest: %v", resp.Error)
	}
	return srv.HandleRequest(t.Context(), rCfg, rReq)
}

// TestServerQuery verifies that a request carrying a query filter is
// serviced as a query, the matching items' attributes being returned as JSON
// in the response's contents.
func TestServerQuery(t *testing.T) {
	service := fmt.Sprintf("cloudeng.io-test-plugin-query-%v", t.Name())
	const contents = "plugin-query-contents"
	writeTestNote(t, service, "query-account", []byte(contents))

	resp := handle(t, plugin.Config{
		Type: keychain.KeychainFileBased,
		Op:   plugin.OpQuery,
		QueryFilter: keychain.FilterSpec{
			Service: "^" + service + "$",
		},
	})
	if resp.Error != nil {
		t.Fatalf("query: %v", resp.Error)
	}
	var attrs []keychain.Attributes
	if err := json.Unmarshal(resp.Contents, &attrs); err != nil {
		t.Fatalf("unmarshaling %q: %v", resp.Contents, err)
	}
	if got, want := len(attrs), 1; got != want {
		t.Fatalf("got %v items, want %v: %s", got, want, resp.Contents)
	}
	got := attrs[0]
	if got.Service != service || got.Account != "query-account" ||
		got.Keychain != keychain.KeychainFileBased ||
		got.Description != keychain.SecureNoteDescription {
		t.Errorf("attributes = %+v", got)
	}
	if got.Created.IsZero() || got.Modified.IsZero() {
		t.Errorf("created/modified not reported: %+v", got)
	}
	// A query must never return the contents of an item.
	if bytes.Contains(resp.Contents, []byte(contents)) {
		t.Errorf("the response contains the contents of a note: %s", resp.Contents)
	}

	// The account configured for the request restricts the query to it.
	resp = handle(t, plugin.Config{
		Type:        keychain.KeychainFileBased,
		Op:          plugin.OpQuery,
		Account:     "a-different-account",
		QueryFilter: keychain.FilterSpec{Service: "^" + service + "$"},
	})
	if resp.Error != nil {
		t.Fatalf("query: %v", resp.Error)
	}
	if err := json.Unmarshal(resp.Contents, &attrs); err != nil {
		t.Fatalf("unmarshaling %q: %v", resp.Contents, err)
	}
	if got, want := len(attrs), 0; got != want {
		t.Errorf("got %v items, want %v: %s", got, want, resp.Contents)
	}
}

// TestServerQueryErrors verifies that a filter the plugin cannot compile is
// reported as a plugin error rather than ignored.
func TestServerQueryErrors(t *testing.T) {
	resp := handle(t, plugin.Config{
		Type:        keychain.KeychainFileBased,
		Op:          plugin.OpQuery,
		QueryFilter: keychain.FilterSpec{Service: "[unclosed"},
	})
	if resp.Error == nil {
		t.Fatal("got nil error, want an error")
	}
	if got, want := resp.Error.Message, "invalid query filter"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := resp.Error.Detail, "service: invalid regular expression"; !strings.Contains(got, want) {
		t.Errorf("got %q, want it to contain %q", got, want)
	}
}

// TestConfigQueryDirect verifies that Config.Query falls back to querying the
// keychain in-process when no plugin binary can be found, reporting an empty
// plugin path, and that OnlyUsePlugin makes a missing plugin an error.
func TestConfigQueryDirect(t *testing.T) {
	service := fmt.Sprintf("cloudeng.io-test-plugin-query-%v", t.Name())
	writeTestNote(t, service, "direct-account", []byte("direct-contents"))

	cfg := plugin.Config{
		Binary:               "/nonexistent/cloudeng-keychain-plugin",
		KeychainBundle:       "/nonexistent/cloudeng-keychain.app",
		KeychainPluginBundle: "/nonexistent/cloudeng-keychain-plugin.app",
		Type:                 keychain.KeychainFileBased,
	}
	spec := keychain.FilterSpec{Service: "^" + service + "$"}
	attrs, binary, err := cfg.Query(t.Context(), spec)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(binary) != 0 {
		t.Errorf("plugin binary = %q, want it to be empty for a direct query", binary)
	}
	if got, want := len(attrs), 1; got != want {
		t.Fatalf("got %v items, want %v: %+v", got, want, attrs)
	}
	if got, want := attrs[0].Service, service; got != want {
		t.Errorf("service = %v, want %v", got, want)
	}

	// An invalid filter is reported before a plugin is looked for.
	if _, _, err := cfg.Query(t.Context(), keychain.FilterSpec{Label: "[unclosed"}); err == nil {
		t.Error("got nil error, want an error")
	}

	// OnlyUsePlugin requires a plugin: a missing one is an error rather than
	// a silent fallback to a query that cannot see the entitled keychains.
	cfg.OnlyUsePlugin = true
	if _, _, err := cfg.Query(t.Context(), spec); err == nil {
		t.Error("got nil error, want an error for a missing plugin")
	}
}
