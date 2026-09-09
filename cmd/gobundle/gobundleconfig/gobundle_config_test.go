// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package gobundleconfig_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"cloudeng.io/cmdutil/cmdyaml"
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

func TestDefaultConfigSearchPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	files := gobundleconfig.LocateConfigFiles("", "")
	_ = files

	t.Setenv("HOME", "")
	files = gobundleconfig.LocateConfigFiles("", "")
	_ = files
}

func setupConfigTestDir(t *testing.T) (tmpDir, homeDir string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir = t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	homeDir = filepath.Join(tmpDir, "fakehome")
	if err := os.MkdirAll(homeDir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)
	return tmpDir, homeDir
}

func TestLocateConfigFiles_None(t *testing.T) {
	setupConfigTestDir(t)
	files := gobundleconfig.LocateConfigFiles("", "")
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %v", files)
	}
}

func TestLocateConfigFiles_Defaults(t *testing.T) {
	tmpDir, _ := setupConfigTestDir(t)
	sharedPath := newConfigFile(t, tmpDir, "gobundle-shared.yaml", "identity: dev\n")
	appPath := newConfigFile(t, tmpDir, "gobundle-app.yaml", "info.plist:\n  CFBundleDisplayName: App\n")

	files := gobundleconfig.LocateConfigFiles("", "")
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %v", files)
	}
	if filepath.Base(files[0]) != "gobundle-shared.yaml" || filepath.Base(files[1]) != "gobundle-app.yaml" {
		t.Errorf("files = %v, want [%s, %s]", files, sharedPath, appPath)
	}
}

func TestLocateConfigFiles_Custom(t *testing.T) {
	tmpDir, _ := setupConfigTestDir(t)
	customShared := newConfigFile(t, tmpDir, "custom-shared.yaml", "identity: custom\n")
	customApp := newConfigFile(t, tmpDir, "custom-app.yaml", "info.plist:\n  CFBundleDisplayName: Custom\n")

	files := gobundleconfig.LocateConfigFiles(customShared, customApp)
	if len(files) != 2 || files[0] != customShared || files[1] != customApp {
		t.Errorf("custom files = %v, want [%s, %s]", files, customShared, customApp)
	}
}

func TestLocateConfigFiles_Env(t *testing.T) {
	tmpDir, _ := setupConfigTestDir(t)
	envShared := newConfigFile(t, tmpDir, "env-shared.yaml", "identity: env\n")
	envApp := newConfigFile(t, tmpDir, "env-app.yaml", "info.plist:\n  CFBundleDisplayName: Env\n")
	t.Setenv(gobundleconfig.SharedBundleEnvVar, envShared)
	t.Setenv(gobundleconfig.AppBundleEnvVar, envApp)

	files := gobundleconfig.LocateConfigFiles("", "")
	if len(files) != 2 || files[0] != envShared || files[1] != envApp {
		t.Errorf("env files = %v, want [%s, %s]", files, envShared, envApp)
	}
}

func TestLocateConfigFiles_Home(t *testing.T) {
	_, homeDir := setupConfigTestDir(t)
	t.Setenv(gobundleconfig.SharedBundleEnvVar, "")
	t.Setenv(gobundleconfig.AppBundleEnvVar, "")

	homeShared := newConfigFile(t, homeDir, "gobundle-shared.yaml", "identity: home\n")
	homeApp := newConfigFile(t, homeDir, "gobundle-app.yaml", "info.plist:\n  CFBundleDisplayName: Home\n")

	files := gobundleconfig.LocateConfigFiles("", "")
	if len(files) != 2 || files[0] != homeShared || files[1] != homeApp {
		t.Errorf("home files = %v, want [%s, %s]", files, homeShared, homeApp)
	}
}

func TestWithDefaultsForBinary(t *testing.T) {
	// 1. Empty Path gets set based on binary name
	var cfg gobundleconfig.T
	cfg = cfg.WithDefaultsForBinary("/usr/local/bin/sample")
	if got, want := cfg.Path, "sample.app"; got != want {
		t.Errorf("cfg.Path = %q, want %q", got, want)
	}
	if got, want := cfg.Info.CFBundleExecutable, "sample"; got != want {
		t.Errorf("CFBundleExecutable = %q, want %q", got, want)
	}
	if got, want := cfg.Info.CFBundleName, "sample"; got != want {
		t.Errorf("CFBundleName = %q, want %q", got, want)
	}
	if got, want := cfg.Info.CFBundlePackageType, "APPL"; got != want {
		t.Errorf("CFBundlePackageType = %q, want %q", got, want)
	}

	// 2. Existing Path is preserved
	cfgWithCustomPath := gobundleconfig.T{
		Path: "/tmp/custom.app",
	}
	cfgWithCustomPath = cfgWithCustomPath.WithDefaultsForBinary("/usr/local/bin/sample")
	if got, want := cfgWithCustomPath.Path, "/tmp/custom.app"; got != want {
		t.Errorf("custom cfg.Path = %q, want %q", got, want)
	}
	if got, want := cfgWithCustomPath.Info.CFBundleExecutable, "sample"; got != want {
		t.Errorf("CFBundleExecutable = %q, want %q", got, want)
	}
}

func TestParseConfigYAML(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	sharedData := `
identity: "Developer ID Application: Company (ABC123XYZ)"
codesign-args:
  - "--timestamp"
entitlements:
  com.apple.security.app-sandbox: true
permissions:
  executable: 0755
  macos_dir: 0750
notary:
  keychain_profile: AC_NOTARY
`
	appData := `
bundle: /tmp/output.app
info.plist:
  CFBundleIdentifier: com.company.sample
  CFBundleDisplayName: Sample App
profile: /path/to/profile.mobileprovision
icon: /path/to/icon.png
`

	sharedFile := newConfigFile(t, tmpDir, "gobundle-shared.yaml", sharedData)
	appFile := newConfigFile(t, tmpDir, "gobundle-app.yaml", appData)

	var cfg gobundleconfig.T
	parser := cmdyaml.NewParser(cmdyaml.WithStrictFields(true))
	if err := parser.ParseFiles(ctx, &cfg, sharedFile, appFile); err != nil {
		t.Fatalf("ParseFiles failed: %v", err)
	}

	if got, want := cfg.Identity, "Developer ID Application: Company (ABC123XYZ)"; got != want {
		t.Errorf("Identity = %q, want %q", got, want)
	}
	if !cfg.SigningConfig.Configured() {
		t.Error("expected SigningConfig to be Configured()")
	}
	if got, want := cfg.Permissions.ExecutableMode(), os.FileMode(0755); got != want {
		t.Errorf("ExecutableMode = %04o, want %04o", got, want)
	}
	if got, want := cfg.Permissions.MacOSDirMode(), os.FileMode(0750); got != want {
		t.Errorf("MacOSDirMode = %04o, want %04o", got, want)
	}
	if !cfg.NotaryConfig.Configured() {
		t.Error("expected NotaryConfig to be Configured()")
	}
	if got, want := cfg.KeychainProfile, "AC_NOTARY"; got != want {
		t.Errorf("KeychainProfile = %q, want %q", got, want)
	}
	if got, want := cfg.Path, "/tmp/output.app"; got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
	if got, want := cfg.Info.CFBundleIdentifier, "com.company.sample"; got != want {
		t.Errorf("CFBundleIdentifier = %q, want %q", got, want)
	}
	if got, want := cfg.ProvisioningProfile, "/path/to/profile.mobileprovision"; got != want {
		t.Errorf("ProvisioningProfile = %q, want %q", got, want)
	}
	if got, want := cfg.Icon, "/path/to/icon.png"; got != want {
		t.Errorf("Icon = %q, want %q", got, want)
	}
}
