// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package keychain_test

import (
	"fmt"
	"testing"

	"cloudeng.io/file"
	"cloudeng.io/macos/keychain"
)

func TestType(t *testing.T) {
	for i, tc := range []struct {
		in   string
		want keychain.Type
	}{
		{"file", keychain.KeychainFileBased},
		{"default", keychain.KeychainFileBased},
		{"data-protection-local", keychain.KeychainDataProtectionLocal},
		{"data-protection", keychain.KeychainDataProtectionLocal},
		{"local", keychain.KeychainDataProtectionLocal},
		{"icloud", keychain.KeychainICloud},
		{"all", keychain.KeychainAll},
		{"", keychain.KeychainAll},
	} {
		got, err := keychain.ParseType(tc.in)
		if err != nil {
			t.Errorf("%v: failed to parse %v: %v", i, tc.in, err)
		}
		if got != tc.want {
			t.Errorf("%v: got %v, want %v", i, got, tc.want)
		}
		// The string representation may not be the same as the input.
		// For example, "default" becomes "file".
		var wantStr string
		switch tc.want {
		case keychain.KeychainFileBased:
			wantStr = "file"
		case keychain.KeychainDataProtectionLocal:
			wantStr = "data-protection-local"
		case keychain.KeychainICloud:
			wantStr = "icloud"
		case keychain.KeychainAll:
			wantStr = "all"
		}
		if got.String() != wantStr {
			t.Errorf("%v: got %v, want %v", i, got.String(), wantStr)
		}
	}
	_, err := keychain.ParseType("invalid")
	if err == nil {
		t.Errorf("expected error for invalid type")
	}
}

func TestAccessibility(t *testing.T) {
	for i, tc := range []struct {
		in   string
		want keychain.Accessibility
	}{
		{"default", keychain.AccessibleDefault},
		{"when-unlocked", keychain.AccessibleWhenUnlocked},
		{"after-first-unlock", keychain.AccessibleAfterFirstUnlock},
		{"always", keychain.AccessibleAlways},
		{"when-passcode-set-this-device-only", keychain.AccessibleWhenPasscodeSetThisDeviceOnly},
		{"when-unlocked-this-device-only", keychain.AccessibleWhenUnlockedThisDeviceOnly},
		{"after-first-unlock-this-device-only", keychain.AccessibleAfterFirstUnlockThisDeviceOnly},
		{"always-this-device-only", keychain.AccessibleAccessibleAlwaysThisDeviceOnly},
	} {
		got, err := keychain.ParseAccessibility(tc.in)
		if err != nil {
			t.Errorf("%v: failed to parse %v: %v", i, tc.in, err)
		}
		if got != tc.want {
			t.Errorf("%v: got %v, want %v", i, got, tc.want)
		}
		if got.String() != tc.in {
			t.Errorf("%v: got %v, want %v", i, got.String(), tc.in)
		}
	}
	_, err := keychain.ParseAccessibility("invalid")
	if err == nil {
		t.Errorf("expected error for invalid accessibility")
	}
}

func TestReadWriteSecureNote(t *testing.T) {
	service := fmt.Sprintf("cloudeng.io-test-service-%v", t.Name())
	account := "test-account"
	data := []byte("test-data")

	kc := keychain.New(keychain.KeychainFileBased, account)
	// Cleanup before test
	_ = kc.DeleteSecureNote(service)

	if err := kc.WriteSecureNote(service, data); err != nil {
		t.Fatalf("failed to write secure note: %v", err)
	}

	readData, err := kc.ReadSecureNote(service)
	if err != nil {
		t.Fatalf("failed to read secure note: %v", err)
	}

	if string(readData) != string(data) {
		t.Errorf("got %v, want %v", string(readData), string(data))
	}

	if err := kc.DeleteSecureNote(service); err != nil {
		t.Fatalf("failed to delete secure note: %v", err)
	}
}

func TestUpdateSecureNote(t *testing.T) {
	service := fmt.Sprintf("cloudeng.io-test-service-%v", t.Name())
	account := "test-account"
	data1 := []byte("test-data-1")
	data2 := []byte("test-data-2")

	kc := keychain.New(keychain.KeychainFileBased, account, keychain.WithUpdateInPlace(true))
	// Cleanup before test
	_ = kc.DeleteSecureNote(service)

	if err := kc.WriteSecureNote(service, data1); err != nil {
		t.Fatalf("failed to write secure note: %v", err)
	}

	// This should update the existing note.
	if err := kc.WriteSecureNote(service, data2); err != nil {
		t.Fatalf("failed to update secure note: %v", err)
	}

	readData, err := kc.ReadSecureNote(service)
	if err != nil {
		t.Fatalf("failed to read secure note: %v", err)
	}

	if string(readData) != string(data2) {
		t.Errorf("got %v, want %v", string(readData), string(data2))
	}

	if err := kc.DeleteSecureNote(service); err != nil {
		t.Fatalf("failed to delete secure note: %v", err)
	}
}

func TestWriteDataProtectionReadAll(t *testing.T) {
	service := fmt.Sprintf("cloudeng.io-test-service-%v", t.Name())
	account := "test-account"
	data := []byte("test-data-for-all")

	// Write to data filebased keychain.
	kcWrite := keychain.New(keychain.KeychainFileBased, account)
	// Cleanup before test
	_ = kcWrite.DeleteSecureNote(service)

	if err := kcWrite.WriteSecureNote(service, data); err != nil {
		t.Fatalf("failed to write secure note to data protection keychain: %v", err)
	}

	// Read from 'all' keychains.
	kcRead := keychain.New(keychain.KeychainAll, account)
	readData, err := kcRead.ReadSecureNote(service)
	if err != nil {
		t.Fatalf("failed to read secure note using 'all' type: %v", err)
	}

	if string(readData) != string(data) {
		t.Errorf("got %q, want %q", string(readData), string(data))
	}

	// Cleanup after test
	if err := kcWrite.DeleteSecureNote(service); err != nil {
		t.Fatalf("failed to delete secure note: %v", err)
	}
}

// TestReadWriteTypes verifies that WithReadWriteTypes overrides the type used
// for read and write/delete operations independently of the base type.
// The scenario: a KeychainAll-typed keychain searches all keychains on read,
// but DeleteSecureNote normally fails ("cannot delete from keychain of type 'all'").
// WithReadWriteTypes(..., KeychainFileBased) sets the write type so that delete
// targets the file-based keychain and succeeds.
func TestReadWriteTypes(t *testing.T) {
	service := fmt.Sprintf("cloudeng.io-test-service-%v", t.Name())
	account := "test-account"
	data := []byte("test-data")

	// Write to the file-based keychain.
	kcFile := keychain.New(keychain.KeychainFileBased, account)
	_ = kcFile.DeleteSecureNote(service)
	if err := kcFile.WriteSecureNote(service, data); err != nil {
		t.Fatalf("WriteSecureNote: %v", err)
	}

	// Verify that deleting from a plain KeychainAll keychain fails.
	kcAll := keychain.New(keychain.KeychainAll, account)
	if err := kcAll.DeleteSecureNote(service); err == nil {
		t.Error("expected error when deleting from KeychainAll without a write-type override")
	}

	// WithReadWriteTypes allows the same keychain to read across all keychains
	// while deleting/updating only the file-based keychain.
	kcRW := keychain.New(keychain.KeychainAll, account,
		keychain.WithWriteType(keychain.KeychainFileBased))

	readData, err := kcRW.ReadSecureNote(service)
	if err != nil {
		t.Fatalf("ReadSecureNote: %v", err)
	}
	if string(readData) != string(data) {
		t.Errorf("got %q, want %q", string(readData), string(data))
	}

	if err := kcRW.DeleteSecureNote(service); err != nil {
		t.Fatalf("DeleteSecureNote with write-type override: %v", err)
	}
}

func TestFS(*testing.T) {
	var _ file.ReadFileFS = keychain.New(keychain.KeychainFileBased, "test-account")
	var _ file.WriteFileFS = keychain.New(keychain.KeychainFileBased, "test-account")
}
