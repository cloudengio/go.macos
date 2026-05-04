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
	ps := plugin.NewServer(plugin.WithLogger(logger))

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
	if resp != nil {
		t.Fatalf("expected nil response, got %v (error: %v)", resp, resp.Error)
	}

	// Normalize expected request by round-tripping through JSON
	var expectedReq plugins.Request
	if err := json.Unmarshal(data, &expectedReq); err != nil {
		t.Fatalf("failed to unmarshal expected request: %v", err)
	}

	if got, want := rReq.ID, expectedReq.ID; got != want {
		t.Errorf("got request ID %v, want %v", got, want)
	}
	if got, want := rReq.Keyname, expectedReq.Keyname; got != want {
		t.Errorf("got request Keyname %v, want %v", got, want)
	}
	if got, want := rReq.Write, expectedReq.Write; got != want {
		t.Errorf("got request Write %v, want %v", got, want)
	}
	if !bytes.Equal(rReq.Contents, expectedReq.Contents) {
		t.Errorf("got request Contents %v, want %v", rReq.Contents, expectedReq.Contents)
	}

	// Normalize expected config by round-tripping through JSON to account for any
	// zero-values or type aliases that json.Marshal/Unmarshal might coerce.
	var expectedCfg plugin.Config
	cfgData, _ := json.Marshal(cfg)
	_ = json.Unmarshal(cfgData, &expectedCfg)

	if got, want := rCfg, &expectedCfg; !reflect.DeepEqual(got, want) {
		t.Errorf("got config %+v, want %+v", got, want)
	}

	logged := logBuf.String()
	checks := []string{
		"new request",
		"id=123",
		"test-account",
		"test_key",
		"data-protection",
		"when-unlocked",
		"write=false",
		"update_in_place=true",
	}
	for _, check := range checks {
		if !strings.Contains(logged, check) {
			t.Errorf("expected log to contain %q, got %q", check, logged)
		}
	}

}

func TestSendResponse(t *testing.T) {
	ctx := t.Context()
	logBuf := &strings.Builder{}
	logger := slog.New(slog.NewTextHandler(logBuf, nil))
	ps := plugin.NewServer(plugin.WithLogger(logger))

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
