// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cloudeng.io/macos/buildtools"
	"gopkg.in/yaml.v3"
)

func TestSignerIdentityAndArgs(t *testing.T) {
	ctx := context.Background()
	dryRunner := buildtools.NewCommandRunner(buildtools.WithDryRun(true))

	// 1. Empty identity returns an ErrorStep
	signerNoID := buildtools.NewSigner("", nil, nil, nil)
	_, err := signerNoID.SignPath("App.app", "Contents/MacOS/app").Run(ctx, dryRunner)
	if err == nil {
		t.Fatalf("expected error for empty identity, got nil")
	}
	if !strings.Contains(err.Error(), "no identity specified") {
		t.Errorf("unexpected error: %v", err)
	}

	// 2. Default arguments and no entitlements
	signerDefault := buildtools.NewSigner("Developer ID Application: Test", nil, nil, nil)
	res, err := signerDefault.SignPath("App.app", "Contents/MacOS/app").Run(ctx, dryRunner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedArgs := []string{"--sign", "Developer ID Application: Test", "--options", "runtime", "--force", "--timestamp", "App.app/Contents/MacOS/app"}
	if strings.Join(res.Args(), " ") != strings.Join(expectedArgs, " ") {
		t.Errorf("expected args %v, got %v", expectedArgs, res.Args())
	}

	// 3. Custom arguments override defaults
	customArgs := []string{"--options", "library", "-f"}
	signerCustom := buildtools.NewSigner("TestID", nil, nil, customArgs)
	res, err = signerCustom.SignPath("App.app", "").Run(ctx, dryRunner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedCustom := []string{"--sign", "TestID", "--options", "library", "-f", "App.app"}
	if strings.Join(res.Args(), " ") != strings.Join(expectedCustom, " ") {
		t.Errorf("expected args %v, got %v", expectedCustom, res.Args())
	}
}

func TestSignerGlobalEntitlements(t *testing.T) {
	ctx := context.Background()
	dryRunner := buildtools.NewCommandRunner(buildtools.WithDryRun(true))

	var globalEnt buildtools.Entitlements
	entYAML := `
com.apple.security.get-task-allow: true
`
	if err := yaml.Unmarshal([]byte(entYAML), &globalEnt); err != nil {
		t.Fatalf("failed to unmarshal entitlements: %v", err)
	}
	signerWithEnt := buildtools.NewSigner("TestID", &globalEnt, nil, nil)
	res, err := signerWithEnt.SignPath("App.app", "Contents/MacOS/app").Run(ctx, dryRunner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	args := res.Args()
	var entitlementsPath string
	for i, arg := range args {
		if arg == "--entitlements" && i+1 < len(args) {
			entitlementsPath = args[i+1]
			break
		}
	}
	if entitlementsPath == "" {
		t.Fatalf("expected --entitlements flag in args: %v", args)
	}
	if _, statErr := os.Stat(entitlementsPath); !os.IsNotExist(statErr) {
		t.Errorf("expected temp entitlements file to be removed, but stat was: %v", statErr)
	}
}

func TestSignerPerFileEntitlements(t *testing.T) {
	ctx := context.Background()
	dryRunner := buildtools.NewCommandRunner(buildtools.WithDryRun(true))

	var globalEnt buildtools.Entitlements
	entYAML := `
com.apple.security.get-task-allow: true
`
	if err := yaml.Unmarshal([]byte(entYAML), &globalEnt); err != nil {
		t.Fatalf("failed to unmarshal entitlements: %v", err)
	}

	var pfEnt buildtools.PerFileEntitlements
	pfYAML := `
app:
  com.apple.security.network.client: true
Contents/MacOS/helper:
  com.apple.security.device.camera: true
`
	if err := yaml.Unmarshal([]byte(pfYAML), &pfEnt); err != nil {
		t.Fatalf("failed to unmarshal per-file entitlements: %v", err)
	}
	signerPF := buildtools.NewSigner("TestID", &globalEnt, &pfEnt, nil)

	// Base name match ("app")
	res, err := signerPF.SignPath("App.app", "Contents/MacOS/app").Run(ctx, dryRunner)
	if err != nil || !containsEntitlementsArg(res.Args()) {
		t.Fatalf("expected entitlements in args: %v, err: %v", res.Args(), err)
	}

	// Full path match ("Contents/MacOS/helper")
	res, err = signerPF.SignPath("App.app", "Contents/MacOS/helper").Run(ctx, dryRunner)
	if err != nil || !containsEntitlementsArg(res.Args()) {
		t.Fatalf("expected entitlements in args: %v, err: %v", res.Args(), err)
	}

	// Fallback to global entitlements for other file ("other")
	res, err = signerPF.SignPath("App.app", "Contents/MacOS/other").Run(ctx, dryRunner)
	if err != nil || !containsEntitlementsArg(res.Args()) {
		t.Fatalf("expected entitlements in args: %v, err: %v", res.Args(), err)
	}

	// No match and no global entitlements
	signerPFNoGlobal := buildtools.NewSigner("TestID", nil, &pfEnt, nil)
	res, err = signerPFNoGlobal.SignPath("App.app", "Contents/MacOS/other").Run(ctx, dryRunner)
	if err != nil || containsEntitlementsArg(res.Args()) {
		t.Fatalf("did not expect entitlements for unmatched file: %v, err: %v", res.Args(), err)
	}
}

func TestSignerVerifyPath(t *testing.T) {
	ctx := context.Background()
	dryRunner := buildtools.NewCommandRunner(buildtools.WithDryRun(true))

	signerDefault := buildtools.NewSigner("Developer ID Application: Test", nil, nil, nil)
	res, err := signerDefault.VerifyPath("App.app", "Contents/MacOS/app").Run(ctx, dryRunner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedVerify := []string{"--verify", "--strict", "App.app/Contents/MacOS/app"}
	if strings.Join(res.Args(), " ") != strings.Join(expectedVerify, " ") {
		t.Errorf("expected args %v, got %v", expectedVerify, res.Args())
	}
}

func containsEntitlementsArg(args []string) bool {
	for _, arg := range args {
		if arg == "--entitlements" {
			return true
		}
	}
	return false
}

func TestSignPathErrorOutput(t *testing.T) {
	ctx := context.Background()
	// Test codesign failure includes entitlements contents in the returned error
	var ent buildtools.Entitlements
	entYAML := `
custom-entitlement-key: custom-value
`
	if err := yaml.Unmarshal([]byte(entYAML), &ent); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Using non-dry-run CommandRunner against a non-existent file will cause codesign to fail
	realRunner := buildtools.NewCommandRunner()
	signer := buildtools.NewSigner("-", &ent, nil, nil)
	nonExistent := filepath.Join(t.TempDir(), "nonexistent.app")
	_, err := signer.SignPath(nonExistent, "").Run(ctx, realRunner)
	if err == nil {
		t.Fatalf("expected codesign error, got nil")
	}
	if !strings.Contains(err.Error(), "custom-entitlement-key") {
		t.Errorf("expected error to contain entitlements content, got: %v", err)
	}
}
