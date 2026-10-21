// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package gobundleconfig_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

// mustLocateConfigFiles returns the configuration files located for the
// supplied names, failing the test if they cannot be located. The mapping
// LocateConfigFiles also returns is covered by TestLocateConfigFiles_Mapping
// and TestEnvVarMapping.
func mustLocateConfigFiles(t *testing.T, sharedConfig, appConfig string) []string {
	t.Helper()
	files, expand, err := gobundleconfig.LocateConfigFiles(sharedConfig, appConfig, "")
	if err != nil {
		t.Fatalf("LocateConfigFiles: %v", err)
	}
	if expand == nil {
		t.Fatal("LocateConfigFiles returned a nil mapping")
	}
	return files
}

func TestDefaultConfigSearchPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	files := mustLocateConfigFiles(t, "", "")
	_ = files

	t.Setenv("HOME", "")
	files = mustLocateConfigFiles(t, "", "")
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
	files := mustLocateConfigFiles(t, "", "")
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %v", files)
	}
}

func TestLocateConfigFiles_Defaults(t *testing.T) {
	tmpDir, _ := setupConfigTestDir(t)
	sharedPath := newConfigFile(t, tmpDir, "gobundle-shared.yaml", "identity: dev\n")
	appPath := newConfigFile(t, tmpDir, "gobundle-app.yaml", "info.plist:\n  CFBundleDisplayName: App\n")

	files := mustLocateConfigFiles(t, "", "")
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

	files := mustLocateConfigFiles(t, customShared, customApp)
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

	files := mustLocateConfigFiles(t, "", "")
	if len(files) != 2 || files[0] != envShared || files[1] != envApp {
		t.Errorf("env files = %v, want [%s, %s]", files, envShared, envApp)
	}
}

// TestLocateConfigFiles_UserConfigDir verifies that the configuration files
// are found in the user's configuration directory. Note that this is the
// gobundle directory within it, eg.
// $HOME/Library/Application Support/gobundle, and not $HOME itself: the
// search path is cmdutil.ConfigFilePath, which has never included the home
// directory directly.
func TestLocateConfigFiles_UserConfigDir(t *testing.T) {
	setupConfigTestDir(t)
	t.Setenv(gobundleconfig.SharedBundleEnvVar, "")
	t.Setenv(gobundleconfig.AppBundleEnvVar, "")

	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Skipf("os.UserConfigDir: %v", err)
	}
	appDir := filepath.Join(configDir, gobundleconfig.ConfigDir)
	if err := os.MkdirAll(appDir, 0700); err != nil {
		t.Fatal(err)
	}

	shared := newConfigFile(t, appDir, "gobundle-shared.yaml", "identity: user-config-dir\n")
	app := newConfigFile(t, appDir, "gobundle-app.yaml", "info.plist:\n  CFBundleDisplayName: UserConfigDir\n")

	files := mustLocateConfigFiles(t, "", "")
	if len(files) != 2 || files[0] != shared || files[1] != app {
		t.Errorf("files = %v, want [%s, %s]", files, shared, app)
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

// mustLocateEnvConfigFile returns the located environment variables file,
// failing the test if it cannot be located.
func mustLocateEnvConfigFile(t *testing.T, envConfig string) string {
	t.Helper()
	file, err := gobundleconfig.LocateEnvConfigFile(envConfig)
	if err != nil {
		t.Fatalf("LocateEnvConfigFile(%q): %v", envConfig, err)
	}
	return file
}

func TestLocateEnvConfigFile(t *testing.T) {
	tmpDir, homeDir := setupConfigTestDir(t)

	// The default name is optional: its absence is not an error.
	if got := mustLocateEnvConfigFile(t, ""); len(got) != 0 {
		t.Errorf("got %q, want no file", got)
	}

	// The default name in the current directory, which is the first entry in
	// the search path and so is returned relative to it. The file carries the
	// .env suffix, not a yaml one, because its contents are NAME=VALUE
	// definitions.
	newConfigFile(t, tmpDir, "gobundle-env.env", "IDENTITY=from-file\n")
	if got, want := mustLocateEnvConfigFile(t, ""), "gobundle-env.env"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// The dot-prefixed form is found too.
	newConfigFile(t, tmpDir, ".only-hidden.env", "IDENTITY=hidden\n")
	if got, want := mustLocateEnvConfigFile(t, "only-hidden"), ".only-hidden.env"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// A name supplied by the caller, with and without the suffix.
	newConfigFile(t, tmpDir, "custom.env", "IDENTITY=custom\n")
	if got, want := mustLocateEnvConfigFile(t, "custom"), "custom.env"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := mustLocateEnvConfigFile(t, "custom.env"), "custom.env"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// A yaml named file is not an environment variables file, and asking for
	// one by name that is not there is an error.
	newConfigFile(t, tmpDir, "yamlish.yaml", "IDENTITY=yaml\n")
	if _, err := gobundleconfig.LocateEnvConfigFile("yamlish"); err == nil {
		t.Error("got nil error, want an error for a name that was specified")
	}

	// The environment variable takes precedence over the caller's name, as
	// it does for the shared and app configuration files.
	newConfigFile(t, tmpDir, "from-env-var.env", "IDENTITY=env\n")
	t.Setenv(gobundleconfig.EnvBundleEnvVar, "from-env-var")
	if got, want := mustLocateEnvConfigFile(t, "custom"), "from-env-var.env"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// An absolute path is used as it is, so it may be named however the
	// caller likes.
	dotEnv := newConfigFile(t, homeDir, "named.env", "IDENTITY=dotenv\n")
	t.Setenv(gobundleconfig.EnvBundleEnvVar, dotEnv)
	if got := mustLocateEnvConfigFile(t, ""); got != dotEnv {
		t.Errorf("got %q, want %q", got, dotEnv)
	}
}

// TestLocateEnvConfigFile_Specified verifies that a file that was asked for
// by name and is not there is an error, whereas the default name is optional.
func TestLocateEnvConfigFile_Specified(t *testing.T) {
	tmpDir, _ := setupConfigTestDir(t)
	missingAbs := filepath.Join(tmpDir, "no-such-file.env")

	for _, tc := range []struct {
		name      string
		envConfig string
		fromEnv   string
		want      string // a substring of the expected error
	}{
		{"relative name from the caller", "nope", "", "nope.env not found in any of"},
		{"absolute name from the caller", missingAbs, "", missingAbs + " not found"},
		{"relative name from the environment", "", "nope", "nope.env not found in any of"},
		{"absolute name from the environment", "", missingAbs, missingAbs + " not found"},
		{"a directory", "", tmpDir, tmpDir + " not found"},
	} {
		t.Setenv(gobundleconfig.EnvBundleEnvVar, tc.fromEnv)
		file, err := gobundleconfig.LocateEnvConfigFile(tc.envConfig)
		if err == nil {
			t.Errorf("%v: got nil error, want an error", tc.name)
			continue
		}
		if len(file) != 0 {
			t.Errorf("%v: got %q, want no file alongside the error", tc.name, file)
		}
		if got := err.Error(); !strings.Contains(got, tc.want) {
			t.Errorf("%v: got %q, want it to contain %q", tc.name, got, tc.want)
		}
	}

	// The default name, in contrast, is optional.
	t.Setenv(gobundleconfig.EnvBundleEnvVar, "")
	if got := mustLocateEnvConfigFile(t, ""); len(got) != 0 {
		t.Errorf("got %q, want no file", got)
	}
}

// TestEnvVarMapping verifies that the values a configuration file is expanded
// with come from the environment variables file, that the shell environment
// overrides them, and that a file that cannot be read is an error.
func TestEnvVarMapping(t *testing.T) {
	tmpDir, _ := setupConfigTestDir(t)
	envFile := newConfigFile(t, tmpDir, "vars.env", `
# A comment.
export IDENTITY=identity-from-file
TEAM_ID="TEAM123"
OVERRIDDEN=file-value
EMPTY=
`)

	t.Setenv("OVERRIDDEN", "shell-value")
	t.Setenv("ONLY_IN_SHELL", "shell-only")

	mapping, err := gobundleconfig.EnvVarMapping(envFile)
	if err != nil {
		t.Fatalf("EnvVarMapping: %v", err)
	}
	for _, tc := range []struct {
		name, want string
	}{
		{"IDENTITY", "identity-from-file"},
		{"TEAM_ID", "TEAM123"},
		{"OVERRIDDEN", "shell-value"}, // the shell wins
		{"ONLY_IN_SHELL", "shell-only"},
		{"EMPTY", ""},
		{"NOT_DEFINED_ANYWHERE", ""},
	} {
		if got := mapping(tc.name); got != tc.want {
			t.Errorf("%v = %q, want %q", tc.name, got, tc.want)
		}
	}

	// With no file the process environment is used on its own.
	mapping, err = gobundleconfig.EnvVarMapping("")
	if err != nil {
		t.Fatalf("EnvVarMapping: %v", err)
	}
	if got, want := mapping("ONLY_IN_SHELL"), "shell-only"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := mapping("IDENTITY"); len(got) != 0 {
		t.Errorf("got %q, want no value without an environment variables file", got)
	}

	// A file that cannot be read is an error rather than being ignored.
	if _, err := gobundleconfig.EnvVarMapping(filepath.Join(tmpDir, "no-such-file.env")); err == nil {
		t.Error("got nil error, want an error for a file that does not exist")
	}
}

// TestLocateConfigFiles_Mapping verifies that the single call returns both
// the files to parse and a mapping that draws on the environment variables
// file, and that a file that cannot be read is reported as an error.
func TestLocateConfigFiles_Mapping(t *testing.T) {
	tmpDir, _ := setupConfigTestDir(t)
	newConfigFile(t, tmpDir, "gobundle-shared.yaml", "identity: ${IDENTITY}\n")
	newConfigFile(t, tmpDir, "gobundle-env.env", "IDENTITY=from-file\nOVERRIDDEN=file-value\n")
	t.Setenv("OVERRIDDEN", "shell-value")

	files, expand, err := gobundleconfig.LocateConfigFiles("", "", "")
	if err != nil {
		t.Fatalf("LocateConfigFiles: %v", err)
	}
	if got, want := len(files), 1; got != want {
		t.Fatalf("got %v files, want %v: %v", got, want, files)
	}
	// The environment variables file is not one of the files to parse: it is
	// not YAML.
	if got := filepath.Base(files[0]); got != "gobundle-shared.yaml" {
		t.Errorf("files = %v, want only the shared configuration file", files)
	}
	if got, want := expand("IDENTITY"), "from-file"; got != want {
		t.Errorf("IDENTITY = %q, want %q", got, want)
	}
	if got, want := expand("OVERRIDDEN"), "shell-value"; got != want {
		t.Errorf("OVERRIDDEN = %q, want %q: the shell overrides the file", got, want)
	}

	// An environment variables file that cannot be read is an error, and no
	// files or mapping are returned.
	t.Setenv(gobundleconfig.EnvBundleEnvVar, filepath.Join(tmpDir, "unreadable"))
	if err := os.WriteFile(filepath.Join(tmpDir, "unreadable"), []byte("NAME='unterminated\n"), 0600); err != nil {
		t.Fatal(err)
	}
	files, expand, err = gobundleconfig.LocateConfigFiles("", "", "")
	if err == nil {
		t.Error("got nil error, want an error for an unparseable environment variables file")
	}
	if files != nil || expand != nil {
		t.Errorf("files = %v, mapping non-nil = %v, want neither", files, expand != nil)
	}
}

// TestLocateConfigFiles_NoEnvFile verifies that an environment variables
// file that is not there is ignored, however it was named, leaving the
// process environment as the only source of values.
func TestLocateConfigFiles_NoEnvFile(t *testing.T) {
	tmpDir, _ := setupConfigTestDir(t)
	newConfigFile(t, tmpDir, "gobundle-shared.yaml", "identity: ${IDENTITY}\n")
	t.Setenv("IDENTITY", "from-the-shell")

	// The default name is absent, which is not an error: the process
	// environment is the only source of values.
	files, expand, err := gobundleconfig.LocateConfigFiles("", "", "")
	if err != nil {
		t.Fatalf("LocateConfigFiles: %v", err)
	}
	if got, want := len(files), 1; got != want {
		t.Fatalf("got %v files, want %v: %v", got, want, files)
	}
	if expand == nil {
		t.Fatal("nil mapping")
	}
	if got, want := expand("IDENTITY"), "from-the-shell"; got != want {
		t.Errorf("IDENTITY = %q, want %q", got, want)
	}
	if got := expand("NOT_DEFINED_ANYWHERE"); len(got) != 0 {
		t.Errorf("got %q, want no value", got)
	}

	// A file that was asked for by name and is not there is an error, and
	// neither files nor a mapping are returned.
	files, expand, err = gobundleconfig.LocateConfigFiles("", "", "nope")
	if err == nil {
		t.Error("got nil error, want an error for a specified file that is not there")
	}
	if files != nil || expand != nil {
		t.Errorf("files = %v, mapping non-nil = %v, want neither", files, expand != nil)
	}
}
