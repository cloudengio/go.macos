// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cloudeng.io/macos/keychain/plugin"
	"cloudeng.io/os/executil"
)

func runCmdNoError(t *testing.T, name string, args ...string) string {
	t.Helper()
	out := bytes.NewBuffer(make([]byte, 0, 1024))
	cmd := exec.CommandContext(t.Context(), name, args...) //nolint:gosec // G702 overly restritive for a test.
	if testing.Verbose() {
		cmd.Stdout = io.MultiWriter(out, os.Stdout)
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = out
	}
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to run %v %v: %v\n%s\n", name, args, err, out.String())
	}
	return out.String()
}

func TestDockerBuildRun(t *testing.T) {
	ctx := context.Background()
	if os.Getenv("SKIP_DOCKER_TESTS") != "" {
		t.Skip("skipping docker tests")
	}
	tmpDir := t.TempDir()

	// Assumes keychain, macos-keychain-plugin and docker desktop
	// are installed.

	endpointCmdPath, err := executil.GoBuild(ctx, filepath.Join(tmpDir, "docker-entrypoint"), ".")
	if err != nil {
		t.Fatalf("failed to build docker-entrypoint command: %v", err)
	}

	runCmdNoError(t, "docker", "build", "-t", "ep-test", ".")

	// Setup Keychain
	serviceName := fmt.Sprintf("ep-test-service-%d", time.Now().UnixNano())
	account := os.Getenv("USER")

	// Data to store
	keyID := "test-key"
	tokenVal := "test-value-secret" //nolint:gosec // G101 false positive for test data.
	keyData := []map[string]string{
		{
			"key_id": keyID,
			"token":  tokenVal,
			"user":   "test-user",
		},
	}
	jsonData, _ := json.Marshal(keyData)
	tempFile := filepath.Join(tmpDir, "keychain-data.json")
	if err := os.WriteFile(tempFile, jsonData, 0600); err != nil {
		t.Fatalf("failed to write keychain data to temp file: %v", err)
	}

	runCmdNoError(t, "keychain",
		"write", "--keychain-type=file", "--name="+serviceName, tempFile)

	defer func() {
		pluginBinary, err := plugin.LocatePluginBinary(
			filepath.Join("/", "Applications", "macos-keychain-plugin.app"),
			"macos-keychain-plugin")
		if err != nil {
			t.Fatalf("failed to find plugin binary: %v", err)
		}
		out := runCmdNoError(t, pluginBinary,
			"delete", "file", account, serviceName)
		t.Log("delete output:", out)
	}()

	// args: docker run ...
	// We want to run: keyctl print $(keyctl search @s user test-key)
	// command inside container:
	shellCmd := fmt.Sprintf("keyctl print $(keyctl search @s user %s)", keyID)
	args := []string{
		"run", "--keychain-item=" + serviceName,
		"--",
		"--rm", "ep-test", "sh", "-c", shellCmd}

	out := runCmdNoError(t, endpointCmdPath, args...)
	if !strings.Contains(out, tokenVal) {
		t.Errorf("expected output to contain %q, got:\n%s", tokenVal, out)
	}
}
