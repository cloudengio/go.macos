// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cloudeng.io/macos/buildtools"
)

// runHelperSteps executes steps against the filesystem, returning the first
// error rather than failing, so that the error paths can be tested.
func runHelperSteps(t *testing.T, steps []buildtools.Step) error {
	t.Helper()
	runner := buildtools.NewCommandRunner()
	for _, step := range steps {
		if _, err := step.Run(context.Background(), runner); err != nil {
			return err
		}
	}
	return nil
}

func newHelperBundle(t *testing.T) (buildtools.AppBundle, string) {
	t.Helper()
	tmp := t.TempDir()
	bundle := buildtools.AppBundle{
		Path: filepath.Join(tmp, "Example.app"),
		Info: buildtools.InfoPlist{
			CFBundleIdentifier: "io.cloudeng.Example",
			CFBundleName:       "Example",
			CFBundleExecutable: "Example",
		},
	}
	if err := runHelperSteps(t, bundle.Create()); err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	helper := filepath.Join(tmp, "example-helper")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil { //nolint:gosec // test fixture
		t.Fatalf("write helper: %v", err)
	}
	return bundle, helper
}

// TestNativeMessagingHostsDir verifies the directories that each browser
// searches for manifests.
func TestNativeMessagingHostsDir(t *testing.T) {
	const home = "/Users/example"
	for _, tc := range []struct {
		browser buildtools.BrowserType
		scope   buildtools.NativeMessagingScope
		want    string
	}{
		{buildtools.Chrome, buildtools.UserScope,
			home + "/Library/Application Support/Google/Chrome/NativeMessagingHosts"},
		{buildtools.Chrome, buildtools.SystemScope,
			"/Library/Google/Chrome/NativeMessagingHosts"},
		{buildtools.Edge, buildtools.UserScope,
			home + "/Library/Application Support/Microsoft Edge/NativeMessagingHosts"},
		{buildtools.Edge, buildtools.SystemScope,
			"/Library/Microsoft/Edge/NativeMessagingHosts"},
		{buildtools.Firefox, buildtools.UserScope,
			home + "/Library/Application Support/Mozilla/NativeMessagingHosts"},
		{buildtools.Firefox, buildtools.SystemScope,
			"/Library/Application Support/Mozilla/NativeMessagingHosts"},
	} {
		got, err := buildtools.NativeMessagingHostsDir(tc.browser, tc.scope, home)
		if err != nil {
			t.Errorf("%v/%v: %v", tc.browser, tc.scope, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%v/%v: got %v, want %v", tc.browser, tc.scope, got, tc.want)
		}
	}
}

// TestNativeMessagingHostsDirErrors verifies that the cases with no manifest
// directory are reported rather than returning a misleading path.
func TestNativeMessagingHostsDirErrors(t *testing.T) {
	// Safari has no manifest directory: an extension talks to its container.
	if _, err := buildtools.NativeMessagingHostsDir(buildtools.Safari, buildtools.UserScope, "/Users/e"); err == nil {
		t.Error("expected an error for safari, got nil")
	} else if !strings.Contains(err.Error(), "safari") {
		t.Errorf("got %v, want it to mention safari", err)
	}
	// A user scoped directory cannot be found without a home directory.
	if _, err := buildtools.NativeMessagingHostsDir(buildtools.Chrome, buildtools.UserScope, ""); err == nil {
		t.Error("expected an error for an empty home directory, got nil")
	}
	// A system scoped directory does not need one.
	if _, err := buildtools.NativeMessagingHostsDir(buildtools.Chrome, buildtools.SystemScope, ""); err != nil {
		t.Errorf("unexpected error for system scope: %v", err)
	}
}

// TestAddNativeMessagingHelper verifies that the helper is placed in the
// bundle and that a manifest is written for each browser.
func TestAddNativeMessagingHelper(t *testing.T) {
	bundle, helper := newHelperBundle(t)
	h := buildtools.NativeMessagingHelper{
		Executable:          helper,
		InstalledBundlePath: "/Applications/Example.app",
		Config: buildtools.NativeMessagingConfig{
			Name:        "io.cloudeng.example",
			Description: "Example native helper",
			AllowedOrigins: []string{
				"chrome-extension://abcdefghijklmnopabcdefghijklmnop/",
			},
			AllowedExtensions: []string{"example@cloudeng.io"},
		},
	}

	steps := bundle.AddNativeMessagingHelper(h, buildtools.Chrome, buildtools.Firefox)
	if err := runHelperSteps(t, steps); err != nil {
		t.Fatalf("add helper: %v", err)
	}

	// The helper is in Contents/Helpers.
	helperPath := bundle.Helpers("example-helper")
	if _, err := os.Stat(helperPath); err != nil {
		t.Errorf("helper not installed: %v", err)
	}

	// A manifest is written for each browser, naming the installed location
	// of the helper rather than where the bundle was built.
	wantPath := "/Applications/Example.app/Contents/Helpers/example-helper"
	for _, browser := range []buildtools.BrowserType{buildtools.Chrome, buildtools.Firefox} {
		verifyBrowserManifest(t, browser, bundle.NativeMessagingManifestPath(h, browser), wantPath, h)
	}
}

func verifyBrowserManifest(t *testing.T, browser buildtools.BrowserType, manifestPath, wantPath string, h buildtools.NativeMessagingHelper) {
	t.Helper()
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Errorf("%v: %v", browser, err)
		return
	}
	var got buildtools.NativeMessagingConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Errorf("%v: %v", browser, err)
		return
	}
	if got.Path != wantPath {
		t.Errorf("%v: path: got %v, want %v", browser, got.Path, wantPath)
	}
	if got.Type != "stdio" {
		t.Errorf("%v: type: got %v, want stdio", browser, got.Type)
	}
	if got.Name != h.Config.Name {
		t.Errorf("%v: name: got %v, want %v", browser, got.Name, h.Config.Name)
	}
	switch browser {
	case buildtools.Chrome:
		verifyChromeManifest(t, data, got, h.Config.AllowedOrigins)
	case buildtools.Firefox:
		verifyFirefoxManifest(t, data, got, h.Config.AllowedExtensions)
	}
}

func verifyChromeManifest(t *testing.T, data []byte, got buildtools.NativeMessagingConfig, wantOrigins []string) {
	t.Helper()
	if len(got.AllowedOrigins) != len(wantOrigins) || got.AllowedOrigins[0] != wantOrigins[0] {
		t.Errorf("chrome: allowed_origins: got %v, want %v", got.AllowedOrigins, wantOrigins)
	}
	if len(got.AllowedExtensions) != 0 {
		t.Errorf("chrome: allowed_extensions should be empty, got %v", got.AllowedExtensions)
	}
	if strings.Contains(string(data), "allowed_extensions") {
		t.Errorf("chrome: manifest JSON should not contain allowed_extensions: %s", string(data))
	}
}

func verifyFirefoxManifest(t *testing.T, data []byte, got buildtools.NativeMessagingConfig, wantExtensions []string) {
	t.Helper()
	if len(got.AllowedExtensions) != len(wantExtensions) || got.AllowedExtensions[0] != wantExtensions[0] {
		t.Errorf("firefox: allowed_extensions: got %v, want %v", got.AllowedExtensions, wantExtensions)
	}
	if len(got.AllowedOrigins) != 0 {
		t.Errorf("firefox: allowed_origins should be empty, got %v", got.AllowedOrigins)
	}
	if strings.Contains(string(data), "allowed_origins") {
		t.Errorf("firefox: manifest JSON should not contain allowed_origins: %s", string(data))
	}
}

// TestNativeMessagingHelperPathDefaultsToBundle verifies that a helper with no
// installed location named uses the bundle's own path, which is what is wanted
// when the bundle is run from where it is built.
func TestNativeMessagingHelperPathDefaultsToBundle(t *testing.T) {
	bundle, helper := newHelperBundle(t)
	h := buildtools.NativeMessagingHelper{Executable: helper}
	want := filepath.Join(bundle.Path, "Contents", "Helpers", "example-helper")
	if got := bundle.NativeMessagingHelperPath(h); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// An explicit name overrides the executable's base name.
	h.Name = "renamed"
	want = filepath.Join(bundle.Path, "Contents", "Helpers", "renamed")
	if got := bundle.NativeMessagingHelperPath(h); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestAddNativeMessagingHelperErrors verifies that a misconfigured helper is
// reported rather than producing a bundle a browser cannot use.
func TestAddNativeMessagingHelperErrors(t *testing.T) {
	bundle, helper := newHelperBundle(t)
	valid := buildtools.NativeMessagingConfig{
		Name:              "io.cloudeng.example",
		AllowedOrigins:    []string{"chrome-extension://abcdefghijklmnopabcdefghijklmnop/"},
		AllowedExtensions: []string{"example@cloudeng.io"},
	}

	t.Run("no executable", func(t *testing.T) {
		h := buildtools.NativeMessagingHelper{Config: valid}
		if err := runHelperSteps(t, bundle.AddNativeMessagingHelper(h, buildtools.Chrome)); err == nil {
			t.Error("expected an error, got nil")
		}
	})

	t.Run("no browsers", func(t *testing.T) {
		h := buildtools.NativeMessagingHelper{Executable: helper, Config: valid}
		if err := runHelperSteps(t, bundle.AddNativeMessagingHelper(h)); err == nil {
			t.Error("expected an error, got nil")
		}
	})

	t.Run("invalid name for chrome", func(t *testing.T) {
		cfg := valid
		cfg.Name = "Not Valid"
		h := buildtools.NativeMessagingHelper{Executable: helper, Config: cfg}
		if err := runHelperSteps(t, bundle.AddNativeMessagingHelper(h, buildtools.Chrome)); err == nil {
			t.Error("expected an error, got nil")
		}
	})

	t.Run("chrome without allowed origins", func(t *testing.T) {
		cfg := valid
		cfg.AllowedOrigins = nil
		h := buildtools.NativeMessagingHelper{Executable: helper, Config: cfg}
		err := runHelperSteps(t, bundle.AddNativeMessagingHelper(h, buildtools.Chrome))
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "allowed_origins") {
			t.Errorf("got %v, want it to mention allowed_origins", err)
		}
	})

	t.Run("firefox without allowed extensions", func(t *testing.T) {
		cfg := valid
		cfg.AllowedExtensions = nil
		h := buildtools.NativeMessagingHelper{Executable: helper, Config: cfg}
		err := runHelperSteps(t, bundle.AddNativeMessagingHelper(h, buildtools.Firefox))
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "allowed_extensions") {
			t.Errorf("got %v, want it to mention allowed_extensions", err)
		}
	})

	t.Run("relative installed path", func(t *testing.T) {
		h := buildtools.NativeMessagingHelper{
			Executable:          helper,
			InstalledBundlePath: "relative/path/Example.app",
			Config:              valid,
		}
		err := runHelperSteps(t, bundle.AddNativeMessagingHelper(h, buildtools.Chrome))
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "absolute path") {
			t.Errorf("got %v, want it to mention absolute path", err)
		}
	})

	t.Run("safari", func(t *testing.T) {
		h := buildtools.NativeMessagingHelper{Executable: helper, Config: valid}
		if err := runHelperSteps(t, bundle.AddNativeMessagingHelper(h, buildtools.Safari)); err == nil {
			t.Error("expected an error for safari, got nil")
		}
	})
}

// TestValidateFirefox covers the name rules that differ from Chrome's.
func TestValidateFirefox(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  buildtools.NativeMessagingConfig
		ok   bool
	}{
		{"valid", buildtools.NativeMessagingConfig{Name: "io.cloudeng.example",
			AllowedExtensions: []string{"e@cloudeng.io"}}, true},
		{"underscores and digits", buildtools.NativeMessagingConfig{Name: "helper_2",
			AllowedExtensions: []string{"e@cloudeng.io"}}, true},
		{"empty", buildtools.NativeMessagingConfig{AllowedExtensions: []string{"e"}}, false},
		{"leading dot", buildtools.NativeMessagingConfig{Name: ".example",
			AllowedExtensions: []string{"e"}}, false},
		{"consecutive dots", buildtools.NativeMessagingConfig{Name: "a..b",
			AllowedExtensions: []string{"e"}}, false},
		{"spaces", buildtools.NativeMessagingConfig{Name: "not valid",
			AllowedExtensions: []string{"e"}}, false},
		{"no extensions", buildtools.NativeMessagingConfig{Name: "io.cloudeng.example"}, false},
		{"relative path", buildtools.NativeMessagingConfig{Name: "io.cloudeng.example",
			AllowedExtensions: []string{"e@cloudeng.io"}, Path: "relative/path"}, false},
		{"absolute path", buildtools.NativeMessagingConfig{Name: "io.cloudeng.example",
			AllowedExtensions: []string{"e@cloudeng.io"}, Path: "/absolute/path"}, true},
	} {
		err := tc.cfg.ValidateFirefox()
		if ok := err == nil; ok != tc.ok {
			t.Errorf("%v: got err=%v, want ok=%v", tc.name, err, tc.ok)
		}
	}
}

// TestValidateChrome covers the validation rules for Chrome/Edge.
func TestValidateChrome(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  buildtools.NativeMessagingConfig
		ok   bool
	}{
		{"valid", buildtools.NativeMessagingConfig{Name: "io.cloudeng.example",
			AllowedOrigins: []string{"chrome-extension://abcdefghijklmnopabcdefghijklmnop/"}}, true},
		{"underscores and digits", buildtools.NativeMessagingConfig{Name: "helper_2",
			AllowedOrigins: []string{"chrome-extension://abcdefghijklmnopabcdefghijklmnop/"}}, true},
		{"empty name", buildtools.NativeMessagingConfig{AllowedOrigins: []string{"chrome-extension://a/"}}, false},
		{"leading dot", buildtools.NativeMessagingConfig{Name: ".example",
			AllowedOrigins: []string{"chrome-extension://a/"}}, false},
		{"trailing dot", buildtools.NativeMessagingConfig{Name: "example.",
			AllowedOrigins: []string{"chrome-extension://a/"}}, false},
		{"consecutive dots", buildtools.NativeMessagingConfig{Name: "a..b",
			AllowedOrigins: []string{"chrome-extension://a/"}}, false},
		{"spaces", buildtools.NativeMessagingConfig{Name: "not valid",
			AllowedOrigins: []string{"chrome-extension://a/"}}, false},
		{"no origins", buildtools.NativeMessagingConfig{Name: "io.cloudeng.example"}, false},
		{"relative path", buildtools.NativeMessagingConfig{Name: "io.cloudeng.example",
			AllowedOrigins: []string{"chrome-extension://a/"}, Path: "relative/path"}, false},
		{"absolute path", buildtools.NativeMessagingConfig{Name: "io.cloudeng.example",
			AllowedOrigins: []string{"chrome-extension://a/"}, Path: "/absolute/path"}, true},
	} {
		err := tc.cfg.ValidateChrome()
		if ok := err == nil; ok != tc.ok {
			t.Errorf("%v: got err=%v, want ok=%v", tc.name, err, tc.ok)
		}
	}
}

// TestAddHelperExecutable verifies that an additional executable can be added
// alongside a native messaging helper, and that it remains executable.
func TestAddHelperExecutable(t *testing.T) {
	bundle, helper := newHelperBundle(t)

	// A second binary, such as a tool the helper invokes.
	tool := filepath.Join(t.TempDir(), "converter")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil { //nolint:gosec // test fixture
		t.Fatal(err)
	}

	steps := bundle.AddHelperExecutable(helper)
	steps = append(steps, bundle.AddHelperExecutable(tool, "convert")...)
	if err := runHelperSteps(t, steps); err != nil {
		t.Fatalf("add helpers: %v", err)
	}

	for _, tc := range []struct{ name, want string }{
		{"example-helper", "default name is the base name of the source"},
		{"convert", "an explicit name overrides it"},
	} {
		info, err := os.Stat(bundle.Helpers(tc.name))
		if err != nil {
			t.Errorf("%v (%v): %v", tc.name, tc.want, err)
			continue
		}
		// cp preserves the mode, so the copy is still executable.
		if info.Mode().Perm()&0100 == 0 {
			t.Errorf("%v: mode %v is not executable", tc.name, info.Mode().Perm())
		}
	}

	if err := runHelperSteps(t, bundle.AddHelperExecutable("")); err == nil {
		t.Error("expected an error for an unspecified executable, got nil")
	}
}

func TestNativeMessagingScopeString(t *testing.T) {
	for _, tc := range []struct {
		s    buildtools.NativeMessagingScope
		want string
	}{
		{buildtools.UserScope, "user"},
		{buildtools.SystemScope, "system"},
		{buildtools.NativeMessagingScope(42), "unknown"},
	} {
		if got := tc.s.String(); got != tc.want {
			t.Errorf("Scope(%d).String() = %q, want %q", int(tc.s), got, tc.want)
		}
	}
}

func TestNativeMessagingHostsDirAll(t *testing.T) {
	home := "/Users/testuser"

	// Edge
	edgeUser, err := buildtools.NativeMessagingHostsDir(buildtools.Edge, buildtools.UserScope, home)
	if err != nil || !strings.Contains(edgeUser, "Microsoft Edge") {
		t.Errorf("Edge user dir: %v, err: %v", edgeUser, err)
	}
	edgeSys, err := buildtools.NativeMessagingHostsDir(buildtools.Edge, buildtools.SystemScope, "")
	if err != nil || !strings.Contains(edgeSys, "Microsoft/Edge") {
		t.Errorf("Edge system dir: %v, err: %v", edgeSys, err)
	}

	// Chrome system
	chromeSys, err := buildtools.NativeMessagingHostsDir(buildtools.Chrome, buildtools.SystemScope, "")
	if err != nil || !strings.Contains(chromeSys, "Google/Chrome") {
		t.Errorf("Chrome system dir: %v, err: %v", chromeSys, err)
	}

	// Firefox system
	ffSys, err := buildtools.NativeMessagingHostsDir(buildtools.Firefox, buildtools.SystemScope, "")
	if err != nil || !strings.Contains(ffSys, "Mozilla") {
		t.Errorf("Firefox system dir: %v, err: %v", ffSys, err)
	}

	// Error cases
	if _, err := buildtools.NativeMessagingHostsDir(buildtools.Chrome, buildtools.UserScope, ""); err == nil {
		t.Error("expected error for empty home on UserScope")
	}
	if _, err := buildtools.NativeMessagingHostsDir(buildtools.Chrome, buildtools.NativeMessagingScope(99), home); err == nil {
		t.Error("expected error for invalid scope")
	}
	if _, err := buildtools.NativeMessagingHostsDir(buildtools.Safari, buildtools.UserScope, home); err == nil {
		t.Error("expected error for Safari")
	}
	if _, err := buildtools.NativeMessagingHostsDir(buildtools.BrowserType(99), buildtools.UserScope, home); err == nil {
		t.Error("expected error for unknown browser")
	}
}

func TestNativeMessagingHelperSigningAndInstall(t *testing.T) {
	bundle, helper := newHelperBundle(t)
	signer := buildtools.NewSigner("Developer ID Application: Test", nil, nil, nil)
	dryRunner := buildtools.NewCommandRunner(buildtools.WithDryRun(true))
	ctx := t.Context()

	h := buildtools.NativeMessagingHelper{
		Executable: helper,
		Name:       "my-helper",
		Config: buildtools.NativeMessagingConfig{
			Name:           "com.example.helper",
			AllowedOrigins: []string{"chrome-extension://abcdefghijklmnopabcdefghijklmnop/"},
		},
	}

	// SignHelper
	step := bundle.SignHelper(signer, "my-helper")
	res, err := step.Run(ctx, dryRunner)
	if err != nil {
		t.Fatalf("SignHelper failed: %v", err)
	}
	if res.Executable() != "codesign" {
		t.Errorf("SignHelper executable = %q, want codesign", res.Executable())
	}

	// SignHelper empty name error
	if _, err := bundle.SignHelper(signer, "").Run(ctx, dryRunner); err == nil {
		t.Error("expected error for empty name in SignHelper")
	}

	// SignNativeMessagingHelper
	signStep := bundle.SignNativeMessagingHelper(signer, h)
	res, err = signStep.Run(ctx, dryRunner)
	if err != nil {
		t.Fatalf("SignNativeMessagingHelper failed: %v", err)
	}
	if res.Executable() != "codesign" {
		t.Errorf("SignNativeMessagingHelper executable = %q, want codesign", res.Executable())
	}

	// InstallNativeMessagingManifest (dry-run)
	installSteps := bundle.InstallNativeMessagingManifest(h, buildtools.Chrome, buildtools.UserScope)
	for i, s := range installSteps {
		if _, err := s.Run(ctx, dryRunner); err != nil {
			t.Fatalf("installStep %d failed: %v", i, err)
		}
	}

	// InstallNativeMessagingManifest rejected for Safari
	safariSteps := bundle.InstallNativeMessagingManifest(h, buildtools.Safari, buildtools.UserScope)
	if _, err := safariSteps[0].Run(ctx, dryRunner); err == nil {
		t.Error("expected error for Safari in InstallNativeMessagingManifest")
	}
}
