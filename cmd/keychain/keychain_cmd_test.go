// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package main_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"cloudeng.io/macos/keychain/plugin"
	"cloudeng.io/os/executil"
	"cloudeng.io/security/keys/keychain/plugins"
)

func runCmd(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // G702 overly restritive for a test.
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runCmdNoError(ctx context.Context, t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := runCmd(ctx, name, args...)
	if err != nil {
		t.Fatalf("failed to run %s %v: %v\n%s\n", name, args, err, out)
	}
	t.Logf("command: %v %v\n%s\n", name, args, out)
	return out
}

func TestKeychainCommand(t *testing.T) {
	if os.Getenv("SKIP_KEYCHAIN_PLUGIN_TESTS") != "" {
		t.Skip("skipping keychain plugin tests")
	}
	ctx := context.Background()
	tmpDir := t.TempDir()

	// The account is the user's login name.
	account := os.Getenv("USER")

	// Build the keychain command binary
	keychainCmdPath, err := executil.GoBuild(ctx, filepath.Join(tmpDir, "keychain"), ".")
	if err != nil {
		t.Fatalf("failed to build keychain command: %v", err)
	}

	keychainTypes := []string{"file", "data-protection", "icloud"}

	for _, kt := range keychainTypes {
		t.Run(kt, func(t *testing.T) {
			// Generate random key name and value
			keyName := fmt.Sprintf("test-key-%s-%s", kt, randomString(8))
			value := randomString(32)

			// Create a temporary file with the value
			valueFile := filepath.Join(tmpDir, keyName+".txt")
			if err := os.WriteFile(valueFile, []byte(value), 0600); err != nil {
				t.Fatalf("failed to write value file: %v", err)
			}

			// Write to keychain
			// keychain write --keychain-plugin=<plugin> --keychain-type=<type> --name=<keyName> <valueFile>
			runCmdNoError(ctx, t, keychainCmdPath,
				"write", "--keychain-type="+kt, "--name="+keyName, valueFile)

			runCmdNoError(ctx, t, "security", "dump-keychain", "/Users/runner/work/_temp/keychain-ci-testing.keychain-db")

			// Read from keychain
			// keychain read --keychain-plugin=<plugin> --keychain-type=<type> <keyName>
			out := runCmdNoError(ctx, t, keychainCmdPath,
				"read", "--keychain-type="+kt, "--output=-", keyName)
			if got := out; len(got) == 0 || !strings.Contains(got, value) {
				t.Errorf("read value mismatch for %s: got %q, want %q", kt, got, value)
			}

			pluginBinary, err := plugin.LocatePluginBinary(
				filepath.Join("/", "Applications", "macos-keychain-plugin.app"),
				"macos-keychain-plugin")
			if err != nil {
				t.Fatalf("failed to find plugin binary: %v", err)
			}
			// Delete from keychain, use the plugin directly to do so.
			t.Logf("deleting keychain type %v item %q for account %q", kt, keyName, account)
			out = runCmdNoError(ctx, t, pluginBinary,
				"delete", kt, account, keyName)
			t.Log("delete output:", out)

			_, err = runCmd(ctx, keychainCmdPath,
				"read", "--keychain-type="+kt, "--output=-", keyName)
			if err == nil {
				t.Errorf("expected error when reading deleted keychain item %q for account %q, got none", keyName, account)
			}
			if !errors.Is(err, plugins.ErrKeyNotFound) {
				t.Logf("keychain item %q for account %q not found as expected", keyName, account)
			}
		})
	}
}

func randomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
