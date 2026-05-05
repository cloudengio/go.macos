// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package keychain_test

import (
	"encoding/json"
	"testing"

	"cloudeng.io/macos/keychain"
	"gopkg.in/yaml.v3"
)

var canonicalTypes = []struct {
	typ keychain.Type
	str string
}{
	{keychain.KeychainFileBased, "file"},
	{keychain.KeychainDataProtectionLocal, "data-protection-local"},
	{keychain.KeychainICloud, "icloud"},
	{keychain.KeychainAll, "all"},
}

func TestTypeJSONRoundtrip(t *testing.T) {
	for _, tc := range canonicalTypes {
		data, err := json.Marshal(tc.typ)
		if err != nil {
			t.Errorf("Marshal(%v): %v", tc.str, err)
			continue
		}
		var got keychain.Type
		if err := json.Unmarshal(data, &got); err != nil {
			t.Errorf("Unmarshal(%v): %v", string(data), err)
			continue
		}
		if got != tc.typ {
			t.Errorf("roundtrip %v: got %v, want %v", tc.str, got, tc.typ)
		}
	}

	var got keychain.Type
	if err := json.Unmarshal([]byte(`"invalid"`), &got); err == nil {
		t.Error("expected error unmarshalling invalid JSON type")
	}
	if err := json.Unmarshal([]byte(`123`), &got); err == nil {
		t.Error("expected error unmarshalling non-string JSON type")
	}
}

func TestTypeYAMLRoundtrip(t *testing.T) {
	for _, tc := range canonicalTypes {
		data, err := yaml.Marshal(tc.typ)
		if err != nil {
			t.Errorf("Marshal(%v): %v", tc.str, err)
			continue
		}
		var got keychain.Type
		if err := yaml.Unmarshal(data, &got); err != nil {
			t.Errorf("Unmarshal(%v): %v", string(data), err)
			continue
		}
		if got != tc.typ {
			t.Errorf("roundtrip %v: got %v, want %v", tc.str, got, tc.typ)
		}
	}

	var got keychain.Type
	if err := yaml.Unmarshal([]byte("invalid\n"), &got); err == nil {
		t.Error("expected error unmarshalling invalid YAML type")
	}
}

func TestTypeTextRoundtrip(t *testing.T) {
	for _, tc := range canonicalTypes {
		text, err := tc.typ.MarshalText()
		if err != nil {
			t.Errorf("MarshalText(%v): %v", tc.str, err)
			continue
		}
		if string(text) != tc.str {
			t.Errorf("MarshalText(%v): got %q, want %q", tc.typ, string(text), tc.str)
		}
		var got keychain.Type
		if err := got.UnmarshalText(text); err != nil {
			t.Errorf("UnmarshalText(%v): %v", string(text), err)
			continue
		}
		if got != tc.typ {
			t.Errorf("roundtrip %v: got %v, want %v", tc.str, got, tc.typ)
		}
	}

	var got keychain.Type
	if err := got.UnmarshalText([]byte("invalid")); err == nil {
		t.Error("expected error unmarshalling invalid text type")
	}
}
