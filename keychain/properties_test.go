// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package keychain_test

import (
	"errors"
	"fmt"
	"io/fs"
	"regexp"
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

func TestSetProperties(t *testing.T) {
	service := fmt.Sprintf("cloudeng.io-test-props-%v", t.Name())
	account := "initial-account"
	secret := []byte("original-secret-payload")

	kc := keychain.New(keychain.KeychainFileBased, account)
	_ = kc.DeleteSecureNote(service)
	if err := kc.WriteSecureNote(service, secret); err != nil {
		t.Fatalf("failed to write test note: %v", err)
	}

	currentAccount := account
	t.Cleanup(func() {
		cleanupKC := keychain.New(keychain.KeychainFileBased, currentAccount)
		_ = cleanupKC.DeleteSecureNote(service)
	})

	re := regexp.MustCompile("^" + regexp.QuoteMeta(service) + "$")
	props := keychain.Properties{
		Label:       "updated-label",
		Description: "updated-description",
	}

	t.Run("update label and description", func(t *testing.T) {
		if err := kc.SetProperties(service, props); err != nil {
			t.Fatalf("SetProperties failed: %v", err)
		}
		data, err := kc.ReadSecureNote(service)
		if err != nil {
			t.Fatalf("ReadSecureNote failed: %v", err)
		}
		if string(data) != string(secret) {
			t.Errorf("secret data changed: got %q, want %q", data, secret)
		}
		attrs, err := kc.Query(keychain.Filter{Service: re})
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		if len(attrs) != 1 {
			t.Fatalf("got %d attributes, want 1", len(attrs))
		}
		if attrs[0].Label != "updated-label" || attrs[0].Description != "updated-description" {
			t.Errorf("attributes = %+v", attrs[0])
		}
	})

	t.Run("update account", func(t *testing.T) {
		newAccount := "updated-account"
		if err := kc.SetProperties(service, keychain.Properties{Account: newAccount}); err != nil {
			t.Fatalf("SetProperties updating account failed: %v", err)
		}
		currentAccount = newAccount

		newKC := keychain.New(keychain.KeychainFileBased, newAccount)
		attrs, err := newKC.Query(keychain.Filter{Service: re})
		if err != nil {
			t.Fatalf("Query with new account failed: %v", err)
		}
		if len(attrs) != 1 || attrs[0].Account != newAccount {
			t.Errorf("attributes with new account = %+v", attrs)
		}
	})

	t.Run("invalid arguments", func(t *testing.T) {
		if err := kc.SetProperties("", props); err == nil {
			t.Error("SetProperties with empty service expected error, got nil")
		}
		if err := kc.SetProperties("nonexistent-service", props); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("SetProperties nonexistent item error = %v, want fs.ErrNotExist", err)
		}
	})
}
