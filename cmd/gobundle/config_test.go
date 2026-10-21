// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cloudeng.io/cmdutil/cmdyaml"
	"cloudeng.io/macos/buildtools"
	"cloudeng.io/macos/cmd/gobundle/gobundleconfig"
)

func newConfigFile(t *testing.T, dir, name, data string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	return path
}

// loadConfig loads the configuration the way main does, ie. including the
// environment variables file that variable references are expanded from.
func loadConfig(t *testing.T, sharedFile, appFile, binary string) gobundleconfig.T {
	t.Helper()
	files, expand, err := gobundleconfig.LocateConfigFiles(sharedFile, appFile, "")
	if err != nil {
		t.Fatalf("failed to locate config files: %v", err)
	}
	var cfg gobundleconfig.T
	parser := cmdyaml.NewParser(cmdyaml.WithStrictFields(true), cmdyaml.WithExpandMapping(expand))
	if err := parser.ParseFiles(t.Context(), &cfg, files...); err != nil {
		t.Fatalf("failed to parse files %v: %v", files, err)
	}
	if len(binary) > 0 {
		cfg = cfg.WithDefaultsForBinary(binary)
	}
	return cfg
}

func TestLoadAndMergeConfigs(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	}()

	sharedConfig := `
identity: shared-identity
entitlements:
  com.apple.security.app-sandbox: true
`
	appConfig := `
info.plist:
  CFBundleIdentifier: com.shared.bundle
  CFBundleDisplayName: My App
`

	newConfigFile(t, tmpDir, "gobundle-shared.yaml", sharedConfig)
	newConfigFile(t, tmpDir, "gobundle-app.yaml", appConfig)

	cfg := loadConfig(t, "", "", "binary")

	if got, want := cfg.Identity, "shared-identity"; got != want {
		t.Errorf("Identity = %q, want %q", got, want)
	}
	if cfg.Entitlements == nil {
		t.Fatalf("expected Entitlements to be non-nil")
	}
	entData, err := cfg.Entitlements.MarshalIndent("")
	if err != nil || !strings.Contains(string(entData), "com.apple.security.app-sandbox") {
		t.Errorf("expected AppSandbox in Entitlements, got: %s", string(entData))
	}
	if got, want := cfg.Info.CFBundleIdentifier, "com.shared.bundle"; got != want {
		t.Errorf("CFBundleIdentifier = %q, want %q", got, want)
	}
	if got, want := cfg.Info.CFBundleDisplayName, "My App"; got != want {
		t.Errorf("CFBundleDisplayName = %q, want %q", got, want)
	}
	if got, want := cfg.Info.CFBundleExecutable, "binary"; got != want {
		t.Errorf("CFBundleExecutable = %q, want %q", got, want)
	}
}

func TestExpandEnv(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	}()

	t.Setenv("TEST_IDENTITY", "test-identity")
	t.Setenv("TEST_BUNDLE_ID", "com.test.bundle")

	sharedConfig := `
identity: ${TEST_IDENTITY}
entitlements:
  com.apple.security.app-sandbox: true
`
	appConfig := `
info.plist:
  CFBundleIdentifier: ${TEST_BUNDLE_ID}
  CFBundleDisplayName: My App
`
	newConfigFile(t, tmpDir, "gobundle-shared.yaml", sharedConfig)
	newConfigFile(t, tmpDir, "gobundle-app.yaml", appConfig)

	cfg := loadConfig(t, "", "", "binary")

	if got, want := cfg.Identity, "test-identity"; got != want {
		t.Errorf("Identity = %q, want %q", got, want)
	}
	if got, want := cfg.Info.CFBundleIdentifier, "com.test.bundle"; got != want {
		t.Errorf("CFBundleIdentifier = %q, want %q", got, want)
	}
}

func TestLoadAndMergeConfigsNotary(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	}()

	sharedConfig := `
identity: "Developer ID Application: Example Inc (TEAM123)"
notary:
  keychain_profile: my-notary-profile
`
	appConfig := `
info.plist:
  CFBundleIdentifier: com.shared.bundle
  CFBundleDisplayName: My App
`

	newConfigFile(t, tmpDir, "gobundle-shared.yaml", sharedConfig)
	newConfigFile(t, tmpDir, "gobundle-app.yaml", appConfig)

	cfg := loadConfig(t, "", "", "binary")
	if got, want := cfg.Identity, "Developer ID Application: Example Inc (TEAM123)"; got != want {
		t.Errorf("Identity = %q, want %q", got, want)
	}
	if got, want := cfg.KeychainProfile, "my-notary-profile"; got != want {
		t.Errorf("Notary.KeychainProfile = %q, want %q", got, want)
	}
	if !cfg.NotaryConfig.Configured() {
		t.Errorf("expected Notary to be Configured()")
	}
}

func TestExpandEnvNotary(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	}()

	t.Setenv("NOTARY_PROFILE", "env-notary-profile")

	sharedConfig := `
notary:
  keychain_profile: ${NOTARY_PROFILE}
`
	appConfig := `
info.plist:
  CFBundleIdentifier: com.env.bundle
`
	newConfigFile(t, tmpDir, "gobundle-shared.yaml", sharedConfig)
	newConfigFile(t, tmpDir, "gobundle-app.yaml", appConfig)

	cfg := loadConfig(t, "", "", "binary")
	if got, want := cfg.KeychainProfile, "env-notary-profile"; got != want {
		t.Errorf("Notary.KeychainProfile = %q, want %q", got, want)
	}
}

func TestPermissionsConfig(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	}()

	sharedConfig := `
permissions:
  executable: 0755
  macos_dir: 0750
`
	appConfig := `
info.plist:
  CFBundleIdentifier: com.perm.bundle
`
	newConfigFile(t, tmpDir, "gobundle-shared.yaml", sharedConfig)
	newConfigFile(t, tmpDir, "gobundle-app.yaml", appConfig)

	cfg := loadConfig(t, "", "", "binary")
	if got, want := cfg.Permissions.ExecutableMode(), os.FileMode(0755); got != want {
		t.Errorf("ExecutableMode() = %04o, want %04o", got, want)
	}
	if got, want := cfg.Permissions.MacOSDirMode(), os.FileMode(0750); got != want {
		t.Errorf("MacOSDirMode() = %04o, want %04o", got, want)
	}
}

func TestCreateAndSignNotarizeValidation(t *testing.T) {
	ctx := t.Context()

	// 1. Notarize requested but no identity
	b := newBundle(gobundleconfig.T{
		NotaryConfig: buildtools.NotaryConfig{
			KeychainProfile: "my-profile",
		},
	})
	err := b.createAndSign(ctx, "testbin", true)
	if err == nil || err.Error() != "notarize is set but the bundle is not signed: set an 'identity' in the config" {
		t.Errorf("expected missing identity error, got: %v", err)
	}

	// 2. Notarize requested with Apple Development identity
	b = newBundle(gobundleconfig.T{
		SigningConfig: buildtools.SigningConfig{
			Identity: "Apple Development: Developer (TEAM123)",
		},
		NotaryConfig: buildtools.NotaryConfig{
			KeychainProfile: "my-profile",
		},
	})
	err = b.createAndSign(ctx, "testbin", true)
	if err == nil || !strings.Contains(err.Error(), "requires a 'Developer ID Application' identity") {
		t.Errorf("expected Developer ID identity requirement error, got: %v", err)
	}

	// 3. Notarize requested with no notary credentials
	b = newBundle(gobundleconfig.T{
		SigningConfig: buildtools.SigningConfig{
			Identity: "Developer ID Application: Developer (TEAM123)",
		},
	})
	err = b.createAndSign(ctx, "testbin", true)
	if err == nil || !strings.Contains(err.Error(), "no notarization credentials are configured") {
		t.Errorf("expected unconfigured notary error, got: %v", err)
	}
}

// TestExpandEnvFromEnvFile verifies that variable references in the
// configuration files are expanded from the environment variables file, and
// that the shell environment overrides the values it defines.
func TestExpandEnvFromEnvFile(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	}()

	// IDENTITY and TEAM_ID come from the file; PROFILE is defined by both
	// and so must come from the shell.
	newConfigFile(t, tmpDir, "gobundle-env.env", `
export IDENTITY="Developer ID Application: Example Inc (TEAM123)"
TEAM_ID=TEAM123 # the team the identity belongs to
PROFILE=file-profile
`)
	// The identity is quoted because expansion is textual: the value
	// contains a colon, which would otherwise not be valid YAML once
	// substituted.
	newConfigFile(t, tmpDir, "gobundle-shared.yaml", `
identity: "${IDENTITY}"
profile: ${PROFILE}
`)
	newConfigFile(t, tmpDir, "gobundle-app.yaml", `
info.plist:
  CFBundleIdentifier: ${TEAM_ID}.com.example.app
`)

	t.Setenv("PROFILE", "shell-profile")

	cfg := loadConfig(t, "", "", "binary")
	if got, want := cfg.Identity, "Developer ID Application: Example Inc (TEAM123)"; got != want {
		t.Errorf("Identity = %q, want %q", got, want)
	}
	if got, want := cfg.Info.CFBundleIdentifier, "TEAM123.com.example.app"; got != want {
		t.Errorf("CFBundleIdentifier = %q, want %q", got, want)
	}
	if got, want := cfg.ProvisioningProfile, "shell-profile"; got != want {
		t.Errorf("ProvisioningProfile = %q, want %q: the shell must override the environment variables file", got, want)
	}

	// Without the file, a reference the shell does not define is left as it
	// is by the parser rather than being expanded to an empty value.
	if err := os.Remove(filepath.Join(tmpDir, "gobundle-env.env")); err != nil {
		t.Fatal(err)
	}
	cfg = loadConfig(t, "", "", "binary")
	if got, want := cfg.Identity, "${IDENTITY}"; got != want {
		t.Errorf("Identity = %q, want %q", got, want)
	}
	if got, want := cfg.ProvisioningProfile, "shell-profile"; got != want {
		t.Errorf("ProvisioningProfile = %q, want %q", got, want)
	}
}
