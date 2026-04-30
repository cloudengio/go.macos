// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package plugin_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"cloudeng.io/cmdutil/flags"
	"cloudeng.io/macos/keychain"
	"cloudeng.io/macos/keychain/plugin"
	"cloudeng.io/security/keys/keychain/plugins"
)

var cwd string

func init() {
	var err error
	cwd, err = os.Getwd()
	if err != nil {
		panic("failed to get current working directory: " + err.Error())
	}
}

func TestPluginFlagsAndConfig(t *testing.T) {
	egp := filepath.Join(cwd, "testdata/example_plugin")
	args := []string{
		"--keychain-plugin=" + egp,
		"--keychain-use-app=not-there",
		"--keychain-type=data-protection",
		"--keychain-account=test-account",
		"--keychain-update-in-place=true",
		"--keychain-accessibility=when-unlocked",
	}
	var flagCfg plugin.WriteFlags
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	if err := flags.RegisterFlagsInStruct(fs, "subcmd", &flagCfg, nil, nil); err != nil {
		t.Fatalf("failed to register flags: %v", err)
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	cfg, err := flagCfg.Config()
	if err != nil {
		t.Fatalf("failed to get config from flags: %v", err)
	}
	if got, want := cfg.Binary, egp; got != want {
		t.Errorf("got Binary %q, want %q", got, want)
	}
	if got, want := cfg.Type, keychain.KeychainDataProtectionLocal; got != want {
		t.Errorf("got Type %v, want %v", got, want)
	}
	if got, want := cfg.Account, "test-account"; got != want {
		t.Errorf("got Account %q, want %q", got, want)
	}
	if got, want := cfg.UpdateInPlace, true; got != want {
		t.Errorf("got UpdateInPlace %v, want %v", got, want)
	}
	if got, want := cfg.Accessibility, keychain.AccessibleWhenUnlocked; got != want {
		t.Errorf("got Accessibility %v, want %v", got, want)
	}

	args = []string{
		"--keychain-plugin=" + egp + "-not-there",
		"--keychain-use-app=not-there",
		"--keychain-type=data-protection",
		"--keychain-account=test-account",
		"--keychain-update-in-place=true",
		"--keychain-accessibility=when-unlocked",
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	cfg, err = flagCfg.Config()
	if err == nil {
		t.Fatalf("expected an error for missing plugin binary, got nil")
	}

}

func TestPluginReadRequest(t *testing.T) {
	ctx := t.Context()
	cfg := plugin.Config{
		Binary:        "not-needed-since-we-run-the-server-directly",
		Type:          keychain.KeychainDataProtectionLocal,
		Account:       "test-account",
		UpdateInPlace: true,
		Accessibility: keychain.AccessibleWhenUnlocked,
	}

	logBuf := &strings.Builder{}
	logger := slog.New(slog.NewTextHandler(logBuf, nil))
	ps := plugin.NewServer(logger)

	req, err := plugin.NewRequest("test_key", cfg)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	req.ID = 123
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	rCfg, rReq, resp := ps.ReadRequest(ctx, bytes.NewReader(data))

	if got, want := rReq, req; !reflect.DeepEqual(got, want) {
		t.Errorf("got request %v, want %v", got, want)
	}

	if got, want := rCfg, &cfg; !reflect.DeepEqual(got, want) {
		t.Errorf("got config %v, want %v", got, want)
	}
	if resp != nil {
		t.Fatalf("expected nil response, got %v", resp)
	}

	logged := logBuf.String()
	if !strings.Contains(logged, "new request") {
		t.Errorf("expected log to contain 'new request', got %q", logged)
	}
	if !strings.Contains(logged, "id=123") {
		t.Errorf("expected log to contain 'account=test-account', got %q", logged)
	}
	if !strings.Contains(logged, "account=test-account") {
		t.Errorf("expected log to contain 'account=test-account', got %q", logged)
	}
	if !strings.Contains(logged, "key=test_key") {
		t.Errorf("expected log to contain 'key=test_key', got %q", logged)
	}
	if !strings.Contains(logged, "type=data-protection") {
		t.Errorf("expected log to contain 'type=data-protection', got %q", logged)
	}
	if !strings.Contains(logged, "accessibility=when-unlocked") {
		t.Errorf("expected log to contain 'accessibility=when-unlocked', got %q", logged)
	}
	if !strings.Contains(logged, "write=false") {
		t.Errorf("expected log to contain 'write=false', got %q", logged)
	}
	if !strings.Contains(logged, "update_in_place=true") {
		t.Errorf("expected log to contain 'update_in_place=true', got %q", logged)
	}

}

func TestSendResponse(t *testing.T) {
	ctx := t.Context()
	logBuf := &strings.Builder{}
	logger := slog.New(slog.NewTextHandler(logBuf, nil))
	ps := plugin.NewServer(logger)

	resp := plugins.Response{
		ID:       123,
		Contents: []byte("test contents"),
		Error: &plugins.Error{
			Message: "test error",
			Detail:  "error details",
		},
	}

	var output strings.Builder
	ps.SendResponse(ctx, &output, &resp)
	logged := logBuf.String()
	if !strings.Contains(logged, "sent response") {
		t.Errorf("expected log to contain 'sent response', got %q", logged)
	}
	if !strings.Contains(logged, "id=123") {
		t.Errorf("expected log to contain 'id=123', got %q", logged)
	}

}

func TestReadWriteTypes(t *testing.T) {
	var r plugin.ReadType
	if err := r.Set("all"); err != nil {
		t.Fatalf("failed to set read type: %v", err)
	}
	if got, want := r.String(), "all"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if err := r.Set("invalid"); err == nil {
		t.Errorf("expected an error for invalid type")
	}

	var w plugin.WriteType
	if err := w.Set("icloud"); err != nil {
		t.Fatalf("failed to set write type: %v", err)
	}
	if got, want := w.String(), "icloud"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if err := w.Set("all"); err == nil {
		t.Errorf("expected an error for 'all' with write type")
	}
	if err := w.Set("invalid"); err == nil {
		t.Errorf("expected an error for invalid type")
	}
}

func TestReadFlags(t *testing.T) {
	egp := filepath.Join(cwd, "testdata/example_plugin")
	args := []string{
		"--keychain-plugin=" + egp,
		"--keychain-use-app=not-there",
		"--keychain-type=all",
		"--keychain-account=test-account",
	}
	var flagCfg plugin.ReadFlags
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	if err := flags.RegisterFlagsInStruct(fs, "subcmd", &flagCfg, nil, nil); err != nil {
		t.Fatalf("failed to register flags: %v", err)
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	cfg, err := flagCfg.Config()
	if err != nil {
		t.Fatalf("failed to get config from flags: %v", err)
	}
	if got, want := cfg.Binary, filepath.Join(cwd, "testdata/example_plugin"); got != want {
		t.Errorf("got Binary %q, want %q", got, want)
	}
	if got, want := cfg.Type, keychain.KeychainAll; got != want {
		t.Errorf("got Type %v, want %v", got, want)
	}
	if got, want := cfg.Account, "test-account"; got != want {
		t.Errorf("got Account %q, want %q", got, want)
	}

	args = []string{
		"--keychain-plugin=" + egp + "-not-there",
		"--keychain-use-app=not-there",
		"--keychain-type=all",
		"--keychain-account=test-account",
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	cfg, err = flagCfg.Config()
	if err == nil {
		t.Fatalf("expected an error for missing plugin binary, got nil")
	}

}
