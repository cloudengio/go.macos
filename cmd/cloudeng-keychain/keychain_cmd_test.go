// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"cloudeng.io/cmdutil/keys"
	"cloudeng.io/cmdutil/keys/keyscmd"
	"cloudeng.io/file"
	"cloudeng.io/file/filetestutil"
	"cloudeng.io/macos/keychain/plugin"
	"cloudeng.io/security/keys/keychain/keychaintestutil"
	"cloudeng.io/security/keys/keychain/plugins"
)

func runCLI(ctx context.Context, args ...string) error {
	cmd := cli()
	return cmd.DispatchWithArgs(ctx, "cloudeng-keychain", args...)
}

// mustRunCLI runs the CLI and fails the test if it returns an error.
func mustRunCLI(ctx context.Context, t *testing.T, args ...string) {
	t.Helper()
	if err := runCLI(ctx, args...); err != nil {
		t.Fatalf("%v: %v", strings.Join(args, " "), err)
	}
}

// mustReadKeyInfo reads a key info from a local JSON file, failing the test if
// it cannot be read.
func mustReadKeyInfo(ctx context.Context, t *testing.T, filename string) keys.Info {
	t.Helper()
	ki, err := keyscmd.ReadKeyInfoFromLocalJSON(ctx, filename)
	if err != nil {
		t.Fatalf("reading keyinfo %v: %v", filename, err)
	}
	return ki
}

// wantKeyInfo checks that ki identifies the expected user and id.
func wantKeyInfo(t *testing.T, ki keys.Info, user, id string) {
	t.Helper()
	if got, want := ki.User, user; got != want {
		t.Errorf("User = %q, want %q", got, want)
	}
	if got, want := ki.ID, id; got != want {
		t.Errorf("ID = %q, want %q", got, want)
	}
}

// wantContains checks that out contains every one of subs.
func wantContains(t *testing.T, out string, subs ...string) {
	t.Helper()
	for _, sub := range subs {
		if !strings.Contains(out, sub) {
			t.Errorf("output missing %q: %s", sub, out)
		}
	}
}

func TestReadWriteWithInjectedFS(t *testing.T) {
	ctx := context.Background()
	p := keychaintestutil.New()
	inMemFS := keychaintestutil.NewFS(p, true)
	ctx = file.ContextWithReadWriteFS(ctx, inMemFS)

	tmpDir := t.TempDir()
	dataFile := filepath.Join(tmpDir, "secret.txt")
	if err := os.WriteFile(dataFile, []byte("super-secret-payload"), 0600); err != nil {
		t.Fatalf("writing secret file: %v", err)
	}

	// 1. Write file to keychain item
	if err := runCLI(ctx, "write", "--keychain-item=test-item", dataFile); err != nil {
		t.Fatalf("runCLI write: %v", err)
	}

	// Verify key in in-memory plugin
	stored, ok := p.Get("test-item")
	if !ok || string(stored) != "super-secret-payload" {
		t.Fatalf("p.Get = %q, %v, want super-secret-payload, true", stored, ok)
	}

	// 2. Read from keychain item to a new file
	outFile := filepath.Join(tmpDir, "read-back.txt")
	if err := runCLI(ctx, "read", "--keychain-item=test-item", outFile); err != nil {
		t.Fatalf("runCLI read: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading read-back file: %v", err)
	}
	if string(data) != "super-secret-payload" {
		t.Errorf("read data = %q, want super-secret-payload", string(data))
	}
}

func TestReadWritePipes(t *testing.T) {
	ctx := context.Background()
	p := keychaintestutil.New()
	inMemFS := keychaintestutil.NewFS(p, true)
	ctx = file.ContextWithReadWriteFS(ctx, inMemFS)

	// 1. Write from stdin
	err := filetestutil.FeedStdin("pipe-secret-data", func() error {
		return runCLI(ctx, "write", "--keychain-item=pipe-item", "-")
	})
	if err != nil {
		t.Fatalf("write from stdin: %v", err)
	}

	stored, ok := p.Get("pipe-item")
	if !ok || string(stored) != "pipe-secret-data" {
		t.Fatalf("p.Get(pipe-item) = %q, %v, want pipe-secret-data, true", stored, ok)
	}

	// 2. Read to stdout
	out, err := filetestutil.CaptureStdout(func() error {
		return runCLI(ctx, "read", "--keychain-item=pipe-item", "-")
	})
	if err != nil {
		t.Fatalf("read to stdout: %v", err)
	}
	if string(out) != "pipe-secret-data" {
		t.Errorf("read stdout = %q, want pipe-secret-data", out)
	}
}

func TestKeyInfoLifecycle(t *testing.T) {
	ctx := context.Background()
	p := keychaintestutil.New()
	inMemFS := keychaintestutil.NewFS(p, true)
	ctx = file.ContextWithReadWriteFS(ctx, inMemFS)

	tmpDir := t.TempDir()
	kiFile1 := filepath.Join(tmpDir, "key1.json")
	kiFile2 := filepath.Join(tmpDir, "key2.json")

	// 1. Create KeyInfo files using key-info create
	mustRunCLI(ctx, t, "key-info", "create", "--user=alice", "--id=token-1", "--size=16", "--format=hex", kiFile1)
	mustRunCLI(ctx, t, "key-info", "create", "--user=bob", "--id=token-2", "--size=16", "--format=hex", kiFile2)

	// Verify KeyInfo created
	ki1 := mustReadKeyInfo(ctx, t, kiFile1)
	wantKeyInfo(t, ki1, "alice", "token-1")
	// 16 random bytes rendered as hex.
	if got, want := len(ki1.Token().Value()), 32; got != want {
		t.Fatalf("ki1 token length = %d, want %d", got, want)
	}

	// 2. Set KeyInfos into keychain item
	mustRunCLI(ctx, t, "key-info", "set", "--keychain-item=my-keys", kiFile1)
	mustRunCLI(ctx, t, "key-info", "set", "--keychain-item=my-keys", kiFile2)

	// 3. List keys in keychain item
	listOut, err := filetestutil.CaptureStdout(func() error {
		return runCLI(ctx, "key-info", "list", "--keychain-item=my-keys")
	})
	if err != nil {
		t.Fatalf("key-info list: %v", err)
	}
	wantContains(t, string(listOut), "token-1[alice]", "token-2[bob]")

	// 4. Get a specific KeyInfo
	getOutFile := filepath.Join(tmpDir, "got-alice.json")
	mustRunCLI(ctx, t, "key-info", "get", "--keychain-item=my-keys", "--key-user=alice", "--key-id=token-1", getOutFile)
	gotKi := mustReadKeyInfo(ctx, t, getOutFile)
	wantKeyInfo(t, gotKi, "alice", "token-1")
	if got, want := gotKi.Token().Value(), ki1.Token().Value(); !bytes.Equal(got, want) {
		t.Errorf("got token %q, want %q", got, want)
	}

	// 5. Delete alice / token-1
	mustRunCLI(ctx, t, "key-info", "delete", "--keychain-item=my-keys", "--key-user=alice", "--key-id=token-1")

	// Verify alice is deleted and bob remains
	delGetOut := filepath.Join(tmpDir, "del-check.json")
	if err := runCLI(ctx, "key-info", "get", "--keychain-item=my-keys", "--key-user=alice", "--key-id=token-1", delGetOut); err == nil {
		t.Errorf("expected error getting deleted key, got nil")
	}

	getBobOut := filepath.Join(tmpDir, "got-bob.json")
	mustRunCLI(ctx, t, "key-info", "get", "--keychain-item=my-keys", "--key-user=bob", "--key-id=token-2", getBobOut)
}

func TestErrors(t *testing.T) {
	ctx := context.Background()
	p := keychaintestutil.New()
	inMemFS := keychaintestutil.NewFS(p, true)
	ctx = file.ContextWithReadWriteFS(ctx, inMemFS)

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "file.txt")
	_ = os.WriteFile(tmpFile, []byte("data"), 0600)

	// 1. Missing keychain item on read
	if err := runCLI(ctx, "read", tmpFile); err == nil || !strings.Contains(err.Error(), "missing keychain item") {
		t.Errorf("expected missing keychain item error, got: %v", err)
	}

	// 2. Missing keychain item on write
	if err := runCLI(ctx, "write", tmpFile); err == nil || !strings.Contains(err.Error(), "missing keychain item") {
		t.Errorf("expected missing keychain item error, got: %v", err)
	}

	// 3. Read non-existent item
	if err := runCLI(ctx, "read", "--keychain-item=does-not-exist", tmpFile); err == nil {
		t.Errorf("expected error reading non-existent item, got nil")
	}

	// 4. Injected error on read
	p.SetError("error-item", &plugins.Error{Message: "injected failure", Detail: "detail"})
	if err := runCLI(ctx, "read", "--keychain-item=error-item", tmpFile); err == nil {
		t.Errorf("expected error on injected failure, got nil")
	}

	// 5. Injected error on delete
	p.SetError("del-error", &plugins.Error{Message: "cannot delete", Detail: "detail"})
	if err := runCLI(ctx, "key-info", "delete", "--keychain-item=del-error", "--key-user=u", "--key-id=k"); err == nil {
		t.Errorf("expected error on injected delete error, got nil")
	}
}

func TestKeyInfoCreateWithManualJSON(t *testing.T) {
	ctx := context.Background()
	p := keychaintestutil.New()
	inMemFS := keychaintestutil.NewFS(p, true)
	ctx = file.ContextWithReadWriteFS(ctx, inMemFS)

	tmpDir := t.TempDir()
	customFile := filepath.Join(tmpDir, "custom.json")

	ki := keys.NewInfo("service-user", "api-key-1", []byte("my-static-token"))
	if err := keyscmd.SafeWriteKeyInfoJSON(ctx, ki, customFile, 0600); err != nil {
		t.Fatalf("SafeWriteKeyInfoJSON: %v", err)
	}

	// Set custom key into keychain
	if err := runCLI(ctx, "key-info", "set", "--keychain-item=service-store", customFile); err != nil {
		t.Fatalf("key-info set: %v", err)
	}

	// Get custom key back
	readBackFile := filepath.Join(tmpDir, "readback.json")
	if err := runCLI(ctx, "key-info", "get", "--keychain-item=service-store", "--key-user=service-user", "--key-id=api-key-1", readBackFile); err != nil {
		t.Fatalf("key-info get: %v", err)
	}

	gotKi, err := keyscmd.ReadKeyInfoFromLocalJSON(ctx, readBackFile)
	if err != nil {
		t.Fatalf("ReadKeyInfoFromLocalJSON: %v", err)
	}
	if gotKi.ID != "api-key-1" || gotKi.User != "service-user" || string(gotKi.Token().Value()) != "my-static-token" {
		t.Errorf("gotKi mismatch: %+v, token: %s", gotKi, string(gotKi.Token().Value()))
	}
}

func TestHandleErrorHelper(t *testing.T) {
	if err := handleError(nil); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}

	genericErr := errors.New("plain error")
	if err := handleError(genericErr); !errors.Is(err, genericErr) {
		t.Errorf("expected genericErr, got %v", err)
	}

	pluginErrWithStderr := &plugins.Error{
		Message: "plugin failed",
		Detail:  "some detail",
		Stderr:  "plugin stderr text",
	}
	out, err := filetestutil.CaptureStderr(func() error {
		return handleError(pluginErrWithStderr)
	})
	if !errors.Is(err, pluginErrWithStderr) {
		t.Errorf("expected pluginErrWithStderr, got %v", err)
	}
	if !strings.Contains(string(out), "plugin failed") || !strings.Contains(string(out), "plugin stderr text") {
		t.Errorf("stderr missing expected error text: %s", out)
	}
}

func TestDeleteKeyInfoUsesWriteFlags(t *testing.T) {
	// Verify DeleteKeyInfoFlags embeds plugin.WriteFlags (not ReadFlags).
	var fl DeleteKeyInfoFlags
	wf := fl.WriteFlags
	if got, want := reflect.TypeOf(wf), reflect.TypeFor[plugin.WriteFlags](); got != want {
		t.Fatalf("DeleteKeyInfoFlags.WriteFlags type = %v, want %v", got, want)
	}

	ctx := context.Background()
	p := keychaintestutil.New()
	inMemFS := keychaintestutil.NewFS(p, true)
	ctx = file.ContextWithReadWriteFS(ctx, inMemFS)

	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "k.json")
	mustRunCLI(ctx, t, "key-info", "create", "--user=alice", "--id=id-1", keyFile)
	mustRunCLI(ctx, t, "key-info", "set", "--keychain-item=items", keyFile)

	// Delete key
	mustRunCLI(ctx, t, "key-info", "delete", "--keychain-item=items", "--key-user=alice", "--key-id=id-1")
}
