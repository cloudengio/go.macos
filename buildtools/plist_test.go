// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"cloudeng.io/macos/buildtools"
	"gopkg.in/yaml.v3"
	"howett.net/plist"
)

const plistWithXPCYAML = `
CFBundleIdentifier: io.cloudeng.TestApp
CFBundleName: TestApp
CFBundleVersion: 1.0.0
CFBundleShortVersionString: 1.0
CFBundleExecutable: TestExecutable
CFBundleIconFile: AppIcon
CFBundlePackageType: APPL
LSMinimumSystemVersion: "15.0"
CFBundleDisplayName: Swift UI Example
SomethingNew: SomeValue
XPCService:
  ServiceName: io.cloudeng.TestService
  ServiceType: Application
  ProcessType: Interactive
  ProgramArguments:
    - TestExecutable
    - --arg1
`

func TestInfoPlist(t *testing.T) {
	var info buildtools.InfoPlist
	if err := yaml.Unmarshal([]byte(plistWithXPCYAML), &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got, want := info.CFBundleIdentifier, "io.cloudeng.TestApp"; got != want {
		t.Fatalf("unexpected CFBundleIdentifier, got %q, want %q", got, want)
	}
	if got, want := info.CFBundleExecutable, "TestExecutable"; got != want {
		t.Fatalf("unexpected CFBundleExecutable, got %q, want %q", got, want)
	}
	if got, want := info.CFBundleIconFile, "AppIcon"; got != want {
		t.Fatalf("unexpected CFBundleIconFile, got %q, want %q", got, want)
	}
	if got, want := info.XPCService.ServiceName, "io.cloudeng.TestService"; got != want {
		t.Fatalf("unexpected XPCService.ServiceName, got %q, want %q", got, want)
	}

	data, err := plist.MarshalIndent(info, plist.XMLFormat, "\t")
	if err != nil {
		t.Fatalf("failed to marshal info plist: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("marshaled data is empty")
	}

	// Simple validation that it contains expected values
	str := string(data)
	expected := []string{
		"<key>CFBundleIdentifier</key>",
		"<string>io.cloudeng.TestApp</string>",
		"<key>CFBundleName</key>",
		"<string>TestApp</string>",
		"<key>XPCService</key>",
		"<key>ProcessType</key>",
		"<string>Interactive</string>",
		"<key>ProgramArguments</key>",
		"<array>",
		"<string>TestExecutable</string>",
		"<string>--arg1</string>",
		"</array>",
	}
	for _, e := range expected {
		if !strings.Contains(str, e) {
			t.Errorf("expected marshaled data to contain %q but it doesn't", e)
		}
	}
}

func TestStepExecution(t *testing.T) {
	// Create a test context
	ctx := context.Background()
	tmpDir := t.TempDir()
	runner := buildtools.NewCommandRunner()

	// Create a simple step that just checks if a file exists
	step := buildtools.StepFunc(func(_ context.Context, _ *buildtools.CommandRunner) (buildtools.StepResult, error) {
		_, err := os.Stat(tmpDir)
		return buildtools.NewStepResult("stat", []string{tmpDir}, nil, err), err
	})

	// Execute the step
	result, err := step.Run(ctx, runner)
	if err != nil {
		t.Fatalf("step execution failed: %v", err)
	}

	if result.Executable() != "stat" {
		t.Fatal("step execution returned unexpected executable")
	}
}

func TestCommandRunnerStdoutStderr(t *testing.T) {
	ctx := context.Background()
	var stdoutBuf, stderrBuf strings.Builder
	runner := buildtools.NewCommandRunner(
		buildtools.WithStdout(&stdoutBuf),
		buildtools.WithStderr(&stderrBuf),
	)

	res, err := runner.Run(ctx, "echo", "hello world")
	if err != nil {
		t.Fatalf("echo failed: %v", err)
	}

	if got, want := strings.TrimSpace(res.Output()), "hello world"; got != want {
		t.Errorf("res.Output() = %q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(stdoutBuf.String()), "hello world"; got != want {
		t.Errorf("stdoutBuf = %q, want %q", got, want)
	}
}

func TestInfoPlistWithDefaults(t *testing.T) {
	// 1. Defaults on empty InfoPlist
	empty := buildtools.InfoPlist{}
	def := empty.WithDefaults("/usr/local/bin/my-app")
	if got, want := def.CFBundleExecutable, "my-app"; got != want {
		t.Errorf("CFBundleExecutable = %q, want %q", got, want)
	}
	if got, want := def.CFBundleName, "my-app"; got != want {
		t.Errorf("CFBundleName = %q, want %q", got, want)
	}
	if got, want := def.CFBundleDisplayName, "my-app"; got != want {
		t.Errorf("CFBundleDisplayName = %q, want %q", got, want)
	}
	if got, want := def.CFBundleIdentifier, "com.example.my-app"; got != want {
		t.Errorf("CFBundleIdentifier = %q, want %q", got, want)
	}
	if got, want := def.CFBundlePackageType, "APPL"; got != want {
		t.Errorf("CFBundlePackageType = %q, want %q", got, want)
	}
	if got, want := def.LSMinimumSystemVersion, "10.15"; got != want {
		t.Errorf("LSMinimumSystemVersion = %q, want %q", got, want)
	}
	if got, want := def.CFBundleVersion, "0.0.0"; got != want {
		t.Errorf("CFBundleVersion = %q, want %q", got, want)
	}

	// 2. Preserves existing values
	custom := buildtools.InfoPlist{
		CFBundleIdentifier:     "com.custom.id",
		CFBundleName:           "CustomName",
		CFBundleExecutable:     "custom_bin",
		CFBundlePackageType:    "XPC!",
		LSMinimumSystemVersion: "15.0",
		CFBundleDisplayName:    "CustomDisplay",
		CFBundleVersion:        "1.2.3",
	}
	res := custom.WithDefaults("other")
	if res.CFBundleIdentifier != "com.custom.id" || res.CFBundleName != "CustomName" ||
		res.CFBundleExecutable != "custom_bin" || res.CFBundlePackageType != "XPC!" ||
		res.LSMinimumSystemVersion != "15.0" || res.CFBundleDisplayName != "CustomDisplay" ||
		res.CFBundleVersion != "1.2.3" {
		t.Errorf("WithDefaults did not preserve existing fields: %+v", res)
	}
}

func TestInfoPlistValidations(t *testing.T) {
	// Valid InfoPlist
	ipl := buildtools.InfoPlist{}.WithDefaults("app")
	if err := ipl.Validate(); err != nil {
		t.Errorf("expected valid InfoPlist, got %v", err)
	}

	// Missing field
	badIPL := ipl
	badIPL.CFBundleIdentifier = ""
	if err := badIPL.Validate(); err == nil {
		t.Error("expected missing CFBundleIdentifier to fail Validate")
	}

	// XPCService validation
	iplWithXPC := ipl
	iplWithXPC.XPCService = &buildtools.XPCServicePlist{ServiceName: "com.test.service"}
	if err := iplWithXPC.Validate(); err != nil {
		t.Errorf("valid XPCService failed: %v", err)
	}
	iplWithBadXPC := ipl
	iplWithBadXPC.XPCService = &buildtools.XPCServicePlist{}
	if err := iplWithBadXPC.Validate(); err == nil {
		t.Error("expected empty ServiceName to fail Validate")
	}

	// NSExtension validation
	iplWithExt := ipl
	iplWithExt.NSExtension = &buildtools.NSExtensionPlist{
		NSExtensionPointIdentifier: "com.apple.Safari.web-extension",
		NSExtensionPrincipalClass:  "Handler",
	}
	if err := iplWithExt.Validate(); err != nil {
		t.Errorf("valid NSExtension failed: %v", err)
	}
	iplWithBadExt := ipl
	iplWithBadExt.NSExtension = &buildtools.NSExtensionPlist{}
	if err := iplWithBadExt.Validate(); err == nil {
		t.Error("expected empty NSExtension to fail Validate")
	}
}

func TestPlistMarshalYAMLAndPlist(t *testing.T) {
	ipl := buildtools.InfoPlist{
		CFBundleShortVersionString: "1.0",
		XPCService:                 &buildtools.XPCServicePlist{ServiceName: "xpc"},
		NSExtension:                &buildtools.NSExtensionPlist{NSExtensionPointIdentifier: "id", NSExtensionPrincipalClass: "cls"},
	}.WithDefaults("app")

	if _, err := ipl.MarshalPlist(); err != nil {
		t.Errorf("InfoPlist.MarshalPlist failed: %v", err)
	}
	if _, err := ipl.MarshalYAML(); err != nil {
		t.Errorf("InfoPlist.MarshalYAML failed: %v", err)
	}

	xpc := buildtools.XPCServicePlist{ServiceName: "xpc"}
	if _, err := xpc.MarshalPlist(); err != nil {
		t.Errorf("XPCServicePlist.MarshalPlist failed: %v", err)
	}
	if _, err := xpc.MarshalYAML(); err != nil {
		t.Errorf("XPCServicePlist.MarshalYAML failed: %v", err)
	}

	ext := buildtools.NSExtensionPlist{NSExtensionPointIdentifier: "id", NSExtensionPrincipalClass: "cls"}
	if _, err := ext.MarshalPlist(); err != nil {
		t.Errorf("NSExtensionPlist.MarshalPlist failed: %v", err)
	}
	if _, err := ext.MarshalYAML(); err != nil {
		t.Errorf("NSExtensionPlist.MarshalYAML failed: %v", err)
	}

	lap := buildtools.LaunchAgentPlist{
		Label:            "label",
		ProgramArguments: []string{"/bin/echo"},
		KeepAlive:        true,
		RunAtLoad:        true,
	}
	if _, err := lap.MarshalPlist(); err != nil {
		t.Errorf("LaunchAgentPlist.MarshalPlist failed: %v", err)
	}
	if _, err := lap.MarshalYAML(); err != nil {
		t.Errorf("LaunchAgentPlist.MarshalYAML failed: %v", err)
	}
}
