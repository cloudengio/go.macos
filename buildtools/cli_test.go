// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools_test

import (
	"context"
	"errors"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"cloudeng.io/macos/buildtools"
	"gopkg.in/yaml.v3"
)

const cliConfig = `bundle: ./testing.app
signing:
    identity: "Apple Development: some id"
    entitlements:
      com.apple.security.app-sandbox: true
    perfile_entitlements:
      xpcHelper:
        com.apple.security.app-sandbox: false
      Contents/MacOS/xpcHelper:
        com.apple.security.app-sandbox: false
`

func TestCLIConfig(t *testing.T) {

	var cfg buildtools.Config
	if err := yaml.Unmarshal([]byte(cliConfig), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got, want := cfg.AppBundle, "./testing.app"; got != want {
		t.Fatalf("unexpected bundle, got %q, want %q", got, want)
	}
	if got, want := cfg.Signing.Identity, "Apple Development: some id"; got != want {
		t.Fatalf("unexpected signing identity, got %q, want %q", got, want)
	}
	if cfg.Signing.Entitlements == nil {
		t.Fatal("expected entitlements but got nil")
	}

	pl, err := cfg.Signing.Entitlements.MarshalIndent(" ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	expectedEntitlements := plistPreamble + ` <dict>
  <key>com.apple.security.app-sandbox</key>
  <true/>
 </dict>
</plist>`

	if got, want := string(pl), expectedEntitlements; got != want {
		t.Errorf("unexpected entitlements, got:\n%s\nwant to contain:\n%s", got, want)
	}

	efor := func(p string) []byte {
		data, err := getEntitlements(cfg.Signing.PerFileEntitlements, p)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}

	pl = efor("xpcHelper")

	expectedPerFileEntitlements := plistPreamble + ` <dict>
  <key>com.apple.security.app-sandbox</key>
  <false/>
 </dict>
</plist>`

	if got, want := string(pl), expectedPerFileEntitlements; got != want {
		t.Errorf("unexpected per file entitlements, got:\n%s\nwant to contain:\n%s", got, want)
	}

	pl = efor("Contents/MacOS/xpcHelper")

	if got, want := string(pl), expectedPerFileEntitlements; got != want {
		t.Errorf("unexpected per file entitlements, got:\n%s\nwant to contain:\n%s", got, want)
	}
}

type mockStep struct {
	exec string
	args []string
	err  error
}

func (m *mockStep) Run(_ context.Context, _ *buildtools.CommandRunner) (buildtools.StepResult, error) {
	return buildtools.NewStepResult(m.exec, m.args, []byte("mock output"), m.err), m.err
}

func TestStepRunner(t *testing.T) {
	ctx := t.Context()
	runner := buildtools.NewRunner(
		buildtools.WithStepTiming(true),
		buildtools.WithStepVerbose(true),
	)

	step1 := &mockStep{exec: "echo", args: []string{"hello"}}
	step2 := &mockStep{exec: "echo", args: []string{"world"}}
	runner.AddSteps(step1, step2)

	results := runner.Run(ctx, nil)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if err := results.Error(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if got, want := results[0].CommandLine(), "echo hello "; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStepRunnerFailure(t *testing.T) {
	ctx := t.Context()
	runner := buildtools.NewRunner(buildtools.WithStepVerbose(true))

	stepErr := errors.New("command failed")
	step1 := &mockStep{exec: "failing-cmd", args: []string{"arg1"}, err: stepErr}
	step2 := &mockStep{exec: "unreachable", args: []string{"arg2"}}
	runner.AddSteps(step1, step2)

	results := runner.Run(ctx, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result due to early termination, got %d", len(results))
	}
	if !errors.Is(results.Error(), stepErr) {
		t.Errorf("got error %v, want %v", results.Error(), stepErr)
	}
}

func TestCommonFlagsStepRunnerOptions(t *testing.T) {
	for _, tc := range []struct {
		name      string
		flags     buildtools.CommonFlags
		wantCount int
	}{
		{"default", buildtools.CommonFlags{}, 0},
		{"timing only", buildtools.CommonFlags{Timing: true}, 1},
		{"verbose only", buildtools.CommonFlags{Verbose: true}, 1},
		{"both timing and verbose", buildtools.CommonFlags{Timing: true, Verbose: true}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := tc.flags.StepRunnerOptions()
			if got := len(opts); got != tc.wantCount {
				t.Errorf("StepRunnerOptions() returned %d options, want %d", got, tc.wantCount)
			}
		})
	}
}

func TestPermissionsConfigYAML(t *testing.T) {
	cases := []struct {
		yaml        string
		wantExec    fs.FileMode
		wantDirMode fs.FileMode
	}{
		{
			yaml: `
executable: 0700
macos_dir: 0755
`,
			wantExec:    0700,
			wantDirMode: 0755,
		},
		{
			yaml: `
executable_permissions: "rwx------"
macos_dir_permissions: "rwxr-xr-x"
`,
			wantExec:    0700,
			wantDirMode: 0755,
		},
		{
			yaml: `
executable: "u=rwx,go="
macos_dir: "u=rwx,go=rx"
`,
			wantExec:    0700,
			wantDirMode: 0755,
		},
	}

	for _, tc := range cases {
		var cfg buildtools.PermissionsConfig
		if err := yaml.Unmarshal([]byte(tc.yaml), &cfg); err != nil {
			t.Errorf("yaml.Unmarshal failed for %q: %v", tc.yaml, err)
			continue
		}
		if got := cfg.ExecutableMode(); got != tc.wantExec {
			t.Errorf("ExecutableMode() = %04o, want %04o", got, tc.wantExec)
		}
		if got := cfg.MacOSDirMode(); got != tc.wantDirMode {
			t.Errorf("MacOSDirMode() = %04o, want %04o", got, tc.wantDirMode)
		}
	}
}

func TestConfigPermissionsYAML(t *testing.T) {
	// 1. Nested permissions block
	yamlNested := `
bundle: ./testing.app
permissions:
  executable: 0700
  macos_dir: 0755
`
	var cfg1 buildtools.Config
	if err := yaml.Unmarshal([]byte(yamlNested), &cfg1); err != nil {
		t.Fatalf("Unmarshal nested failed: %v", err)
	}
	if got, want := cfg1.ExecutableMode(), fs.FileMode(0700); got != want {
		t.Errorf("cfg1.ExecutableMode() = %04o, want %04o", got, want)
	}
	if got, want := cfg1.MacOSDirMode(), fs.FileMode(0755); got != want {
		t.Errorf("cfg1.MacOSDirMode() = %04o, want %04o", got, want)
	}

	// 2. Top-level permissions
	yamlTopLevel := `
bundle: ./testing.app
executable_permissions: "rwx------"
macos_dir_permissions: "rwxr-xr-x"
`
	var cfg2 buildtools.Config
	if err := yaml.Unmarshal([]byte(yamlTopLevel), &cfg2); err != nil {
		t.Fatalf("Unmarshal top-level failed: %v", err)
	}
	if got, want := cfg2.ExecutableMode(), fs.FileMode(0700); got != want {
		t.Errorf("cfg2.ExecutableMode() = %04o, want %04o", got, want)
	}
	if got, want := cfg2.MacOSDirMode(), fs.FileMode(0755); got != want {
		t.Errorf("cfg2.MacOSDirMode() = %04o, want %04o", got, want)
	}
}

func TestCommonFlagsAndRunnerOptions(t *testing.T) {
	tempDir := t.TempDir()
	cfgFile := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("bundle: ./myapp.app\n"), 0600); err != nil {
		t.Fatal(err)
	}

	flags := buildtools.CommonFlags{
		DryRun:     true,
		Timing:     true,
		ConfigFile: cfgFile,
		Verbose:    true,
	}

	// CommandRunnerOptions
	opts := flags.CommandRunnerOptions()
	if len(opts) != 2 {
		t.Errorf("expected 2 CommandRunner options (DryRun + Timing), got %d", len(opts))
	}
	runner := buildtools.NewCommandRunner(opts...)
	if !runner.DryRun() {
		t.Error("expected DryRun to be true on runner")
	}

	// ParseFile
	var parsed struct {
		Bundle string `yaml:"bundle"`
	}
	if err := flags.ParseFile(&parsed); err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	if parsed.Bundle != "./myapp.app" {
		t.Errorf("parsed.Bundle = %q, want ./myapp.app", parsed.Bundle)
	}

	// PrintResult
	res := buildtools.RunResult{
		buildtools.NewStepResult("echo", []string{"hi"}, []byte("hi\n"), nil),
	}
	if err := flags.PrintResult(parsed, res); err != nil {
		t.Errorf("PrintResult failed: %v", err)
	}
}

func TestRegisterFlagsOrDie(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	type myFlags struct {
		buildtools.CommonFlags
	}
	var f myFlags
	// Should not panic on valid struct
	buildtools.RegisterFlagsOrDie(&f, fs)

	// Should panic on non-struct
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected RegisterFlagsOrDie to panic on non-struct")
		}
	}()
	notAStruct := 42
	buildtools.RegisterFlagsOrDie(&notAStruct, fs)
}

func TestSigningConfigMethods(t *testing.T) {
	// 1. Configured
	scEmpty := buildtools.SigningConfig{}
	if scEmpty.Configured() {
		t.Error("empty SigningConfig should not be Configured()")
	}

	scIdentity := buildtools.SigningConfig{Identity: "Test Identity"}
	if !scIdentity.Configured() {
		t.Error("SigningConfig with Identity should be Configured()")
	}

	scNoIdentity := buildtools.SigningConfig{
		CodesignArguments:   []string{"--force"},
		Entitlements:        &buildtools.Entitlements{},
		PerFileEntitlements: &buildtools.PerFileEntitlements{},
	}
	if scNoIdentity.Configured() {
		t.Error("SigningConfig without Identity should not be Configured()")
	}

	// 2. Signer()
	signer := scIdentity.Signer()
	dryRunner := buildtools.NewCommandRunner(buildtools.WithDryRun(true))
	step := signer.SignPath("/tmp/test.app", "")
	res, err := step.Run(t.Context(), dryRunner)
	if err != nil {
		t.Fatalf("signer.SignPath failed: %v", err)
	}
	if res.Executable() != "codesign" {
		t.Errorf("executable = %q, want codesign", res.Executable())
	}
}
