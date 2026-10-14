// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package plugin_test

import (
	"strings"
	"testing"

	"cloudeng.io/macos/keychain"
	"cloudeng.io/macos/keychain/plugin"
	"cloudeng.io/security/keys/keychain/plugins"
)

// TestOpOnTheWire verifies that the operation a request asks for survives
// the round trip through a request's plugin-specific payload, including when
// every parameter of that operation is empty. The payload is encoded with
// encoding/json/v2, whose omitzero and omitempty drop a struct that is
// entirely zero, so an operation inferred from the presence of such a struct
// is lost: a query with no criteria, which matches every item, would arrive
// as a request to read the item named by the keyname.
func TestOpOnTheWire(t *testing.T) {
	base := plugin.Config{
		Type:      keychain.KeychainAll,
		WriteType: keychain.KeychainICloud,
		Account:   "alice",
	}
	withOp := func(op plugin.Op, apply func(*plugin.Config)) plugin.Config {
		cfg := base
		cfg.Op = op
		if apply != nil {
			apply(&cfg)
		}
		return cfg
	}

	for _, tc := range []struct {
		name  string
		cfg   plugin.Config
		write bool
	}{
		{"query with no criteria", withOp(plugin.OpQuery, nil), false},
		{"query with a filter", withOp(plugin.OpQuery, func(c *plugin.Config) {
			c.QueryFilter = keychain.FilterSpec{Service: "^svc$", SecureNotesOnly: true}
		}), false},
		{"query with every criterion", withOp(plugin.OpQuery, func(c *plugin.Config) {
			c.QueryFilter = keychain.FilterSpec{
				Any: "a", Service: "s", Account: "ac", Label: "l",
				Description: "d", Comment: "c", SecureNotesOnly: true,
			}
		}), false},
		{"set properties with nothing to set", withOp(plugin.OpSetProperties, nil), true},
		{"set properties", withOp(plugin.OpSetProperties, func(c *plugin.Config) {
			c.Properties = keychain.Properties{
				Account: "bob", Label: "l", Description: "d", Accessibility: "when-unlocked",
			}
		}), true},
		{"read an item's contents", base, false},
		{"write an item's contents", base, true},
	} {
		var req plugins.Request
		var err error
		if tc.write {
			req, err = plugins.NewWriteRequest("an-item", []byte("contents"), tc.cfg)
		} else {
			req, err = plugins.NewRequest("an-item", tc.cfg)
		}
		if err != nil {
			t.Errorf("%v: creating the request: %v", tc.name, err)
			continue
		}
		var got plugin.Config
		if err := req.UnmarshalPluginSpecific(&got); err != nil {
			t.Errorf("%v: decoding the request: %v", tc.name, err)
			continue
		}
		if got, want := got.Op, tc.cfg.Op; got != want {
			t.Errorf("%v: op = %q, want %q (payload %s)", tc.name, got, want, req.PluginSpecific)
		}
		if got, want := got.QueryFilter, tc.cfg.QueryFilter; got != want {
			t.Errorf("%v: filter = %+v, want %+v", tc.name, got, want)
		}
		if got, want := got.Properties, tc.cfg.Properties; got != want {
			t.Errorf("%v: properties = %+v, want %+v", tc.name, got, want)
		}

		// A request that names no operation must carry no op member at all,
		// so that a plugin built before Op existed sees what it has always
		// seen for a read or a write of an item's contents.
		hasMember := strings.Contains(string(req.PluginSpecific), `"op"`)
		if want := tc.cfg.Op != plugin.OpAccess; hasMember != want {
			t.Errorf("%v: op member present = %v, want %v: %s", tc.name, hasMember, want, req.PluginSpecific)
		}
	}
}

// TestHandleRequestUnsupportedOp verifies that an operation the plugin does
// not implement is reported as such, rather than serviced as a read or write
// of an item's contents, which is how the loss of an operation on the wire
// first showed itself.
func TestHandleRequestUnsupportedOp(t *testing.T) {
	resp := handle(t, plugin.Config{
		Type: keychain.KeychainFileBased,
		Op:   plugin.Op("no-such-operation"),
	})
	if resp.Error == nil {
		t.Fatal("got nil error, want an error")
	}
	if got, want := resp.Error.Message, "unsupported operation"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := resp.Error.Detail, "no-such-operation"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
