// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package gobundleconfig_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"cloudeng.io/cmdutil/cmdconfig"
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

// mustLocateConfigFiles returns the configuration files located for the
// supplied names, failing the test if they cannot be located. The mapping
// LocateConfigFiles also returns is covered by TestLocateConfigFiles_Mapping
// and TestEnvVarMapping.
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

// mustLocateEnvConfigFile returns the located environment variables file,
// failing the test if it cannot be located.
// newFilesFlag returns a flag with the default files, and with the YAML
// files named by set, if there are any.
func newFilesFlag(t *testing.T, set ...string) *cmdconfig.FilesFlag {
	t.Helper()
	var ff cmdconfig.FilesFlag
	ff.SetDefaults(gobundleconfig.DefaultConfigFiles()...)
	if len(set) > 0 {
		if err := ff.Set(strings.Join(set, ",")); err != nil {
			t.Fatal(err)
		}
	}
	return &ff
}

func TestLoad_None(t *testing.T) {
	setupConfigTestDir(t)
	cfg, loc, err := gobundleconfig.Load(t.Context(), newFilesFlag(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(loc.YAML) != 0 || len(loc.Env) != 0 || cfg.Identity != "" {
		t.Errorf("got %+v, %+v", cfg, loc)
	}
}

func TestLoad_Defaults(t *testing.T) {
	tmpDir, _ := setupConfigTestDir(t)
	newConfigFile(t, tmpDir, "gobundle-shared.yaml", "identity: dev\n")
	newConfigFile(t, tmpDir, "gobundle-app.yaml", "info.plist:\n  CFBundleDisplayName: App\n")

	cfg, loc, err := gobundleconfig.Load(t.Context(), newFilesFlag(t))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := loc.YAML, []string{"gobundle-shared.yaml", "gobundle-app.yaml"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if cfg.Identity != "dev" || cfg.Info.CFBundleDisplayName != "App" {
		t.Errorf("got %+v", cfg)
	}
}

func TestLoad_Set(t *testing.T) {
	tmpDir, _ := setupConfigTestDir(t)
	newConfigFile(t, tmpDir, "gobundle-shared.yaml", "identity: default\n")
	custom := newConfigFile(t, tmpDir, "custom-shared.yaml", "identity: custom\n")
	customApp := newConfigFile(t, tmpDir, "custom-app.yaml", "info.plist:\n  CFBundleDisplayName: Custom\n")

	// The files that are set replace the defaults.
	cfg, loc, err := gobundleconfig.Load(t.Context(), newFilesFlag(t, custom, customApp))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := loc.YAML, []string{custom, customApp}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if cfg.Identity != "custom" || cfg.Info.CFBundleDisplayName != "Custom" {
		t.Errorf("got %+v", cfg)
	}

	// A file that is set and does not exist is an error.
	if _, _, err := gobundleconfig.Load(t.Context(), newFilesFlag(t, filepath.Join(tmpDir, "absent.yaml"))); err == nil {
		t.Error("expected an error")
	}
}

func TestLoad_EnvVar(t *testing.T) {
	tmpDir, _ := setupConfigTestDir(t)
	envShared := newConfigFile(t, tmpDir, "env-shared.yaml", "identity: env\n")
	envApp := newConfigFile(t, tmpDir, "env-app.yaml", "info.plist:\n  CFBundleDisplayName: Env\n")
	t.Setenv(gobundleconfig.ConfigBundleEnvVar, envShared+","+envApp)

	cfg, loc, err := gobundleconfig.Load(t.Context(), newFilesFlag(t))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := loc.YAML, []string{envShared, envApp}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if cfg.Identity != "env" {
		t.Errorf("got %+v", cfg)
	}

	// The command line takes precedence over the environment.
	other := newConfigFile(t, tmpDir, "other.yaml", "identity: other\n")
	cfg, _, err = gobundleconfig.Load(t.Context(), newFilesFlag(t, other))
	if err != nil || cfg.Identity != "other" {
		t.Errorf("got %+v, %v", cfg, err)
	}
}

// TestLoad_UserConfigDir verifies that the configuration files are found in
// the user's configuration directory. Note that this is the gobundle
// directory within it, eg. $HOME/Library/Application Support/gobundle, and
// not $HOME itself: the search path is cmdconfig.SearchPath, which has never
// included the home directory directly.
func TestLoad_UserConfigDir(t *testing.T) {
	_, home := setupConfigTestDir(t)
	if runtime.GOOS == "darwin" {
		// os.UserConfigDir is derived from $HOME on darwin.
		t.Setenv("HOME", home)
	}
	configDir, err := os.UserConfigDir()
	if err != nil || !strings.HasPrefix(configDir, home) {
		t.Skipf("os.UserConfigDir: %v, %v", configDir, err)
	}
	appDir := filepath.Join(configDir, gobundleconfig.ConfigDir)
	if err := os.MkdirAll(appDir, 0700); err != nil {
		t.Fatal(err)
	}
	shared := newConfigFile(t, appDir, "gobundle-shared.yaml", "identity: user-config-dir\n")
	app := newConfigFile(t, appDir, "gobundle-app.yaml", "info.plist:\n  CFBundleDisplayName: UserConfigDir\n")

	_, loc, err := gobundleconfig.Load(t.Context(), newFilesFlag(t))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := loc.YAML, []string{shared, app}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestLoad_EnvFile verifies that the values a configuration file is expanded
// from are those of the process environment and, for any it does not define,
// of the environment variables file, and which file that is.
func TestLoad_EnvFile(t *testing.T) {
	tmpDir, _ := setupConfigTestDir(t)
	newConfigFile(t, tmpDir, "gobundle-shared.yaml", "identity: ${LOAD_TEST_IDENTITY}\nprofile: ${LOAD_TEST_PROFILE}\n")
	newConfigFile(t, tmpDir, "gobundle-env.env", "LOAD_TEST_IDENTITY=from-file\nLOAD_TEST_PROFILE=$HOME/profile\n")

	cfg, loc, err := gobundleconfig.Load(t.Context(), newFilesFlag(t))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := loc.Env, []string{"gobundle-env.env"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if cfg.Identity != "from-file" || cfg.ProvisioningProfile != os.Getenv("HOME")+"/profile" {
		t.Errorf("got %+v", cfg)
	}

	// The process environment overrides the file.
	t.Setenv("LOAD_TEST_IDENTITY", "from-env")
	if cfg, _, err = gobundleconfig.Load(t.Context(), newFilesFlag(t)); err != nil || cfg.Identity != "from-env" {
		t.Errorf("got %+v, %v", cfg, err)
	}

	// A file named by the environment variable replaces the default, and one
	// that is not there is an error.
	newConfigFile(t, tmpDir, "other.env", "LOAD_TEST_IDENTITY=other\nLOAD_TEST_PROFILE=p\n")
	t.Setenv("LOAD_TEST_IDENTITY", "")
	t.Setenv(gobundleconfig.EnvBundleEnvVar, "other")
	if cfg, _, err = gobundleconfig.Load(t.Context(), newFilesFlag(t)); err != nil || cfg.Identity != "other" {
		t.Errorf("got %+v, %v", cfg, err)
	}
	t.Setenv(gobundleconfig.EnvBundleEnvVar, "absent")
	if _, _, err = gobundleconfig.Load(t.Context(), newFilesFlag(t)); err == nil {
		t.Error("expected an error for an environment variables file that was specified and is not there")
	}
}

func TestLoad_Extra(t *testing.T) {
	tmpDir, _ := setupConfigTestDir(t)
	newConfigFile(t, tmpDir, "gobundle-shared.yaml", "identity: shared\n")
	plugin := newConfigFile(t, tmpDir, "plugin.yaml", "info.plist:\n  CFBundleDisplayName: Plugin\n")

	cfg, loc, err := gobundleconfig.Load(t.Context(), newFilesFlag(t), plugin)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := loc.YAML, []string{"gobundle-shared.yaml", plugin}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if cfg.Identity != "shared" || cfg.Info.CFBundleDisplayName != "Plugin" {
		t.Errorf("got %+v", cfg)
	}
	// The additional files must exist.
	if _, _, err := gobundleconfig.Load(t.Context(), newFilesFlag(t), filepath.Join(tmpDir, "absent.yaml")); err == nil {
		t.Error("expected an error")
	}
}
