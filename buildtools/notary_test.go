// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools_test

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"cloudeng.io/macos/buildtools"
)

func TestNotaryConfigConfigured(t *testing.T) {
	tests := []struct {
		name string
		cfg  buildtools.NotaryConfig
		want bool
	}{
		{
			name: "empty",
			cfg:  buildtools.NotaryConfig{},
			want: false,
		},
		{
			name: "keychain profile",
			cfg: buildtools.NotaryConfig{
				KeychainProfile: "my-profile",
			},
			want: true,
		},
		{
			name: "apple id credentials",
			cfg: buildtools.NotaryConfig{
				AppleID:  "user@example.com",
				TeamID:   "TEAM123456",
				Password: "@keychain:notary-pass",
			},
			want: true,
		},
		{
			name: "partial credentials - missing team_id",
			cfg: buildtools.NotaryConfig{
				AppleID:  "user@example.com",
				Password: "password",
			},
			want: false,
		},
		{
			name: "partial credentials - missing apple_id",
			cfg: buildtools.NotaryConfig{
				TeamID:   "TEAM123456",
				Password: "password",
			},
			want: false,
		},
		{
			name: "partial credentials - missing password",
			cfg: buildtools.NotaryConfig{
				AppleID: "user@example.com",
				TeamID:  "TEAM123456",
			},
			want: false,
		},
		{
			name: "arguments",
			cfg: buildtools.NotaryConfig{
				Arguments: []string{"--keychain-profile", "custom"},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.Configured(); got != tc.want {
				t.Errorf("Configured() = %v, want %v", got, tc.want)
			}
		})
	}
}

func assertContains(t *testing.T, list []string, items ...string) {
	t.Helper()
	for _, item := range items {
		if !slices.Contains(list, item) {
			t.Errorf("list %v does not contain expected item %q", list, item)
		}
	}
}

func TestAppBundleNotarizeKeychainProfile(t *testing.T) {
	ctx := context.Background()
	runner := buildtools.NewCommandRunner(buildtools.WithDryRun(true))
	bundle := buildtools.AppBundle{
		Path: filepath.Join("/tmp", "TestApp.app"),
	}

	cfg := buildtools.NotaryConfig{
		KeychainProfile: "my-profile",
	}
	steps := bundle.Notarize(cfg)
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(steps))
	}

	stepRunner := buildtools.NewRunner().AddSteps(steps...)
	results := stepRunner.Run(ctx, runner)
	if err := results.Error(); err != nil {
		t.Fatalf("unexpected error during dry run: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if got, want := results[0].Executable(), "ditto"; got != want {
		t.Errorf("step 0 executable = %q, want %q", got, want)
	}
	assertContains(t, results[0].Args(), "--keepParent", bundle.Path)

	if got, want := results[1].Executable(), "xcrun"; got != want {
		t.Errorf("step 1 executable = %q, want %q", got, want)
	}
	assertContains(t, results[1].Args(), "notarytool", "submit", "--keychain-profile", "my-profile", "--wait")

	if got, want := results[2].Executable(), "xcrun"; got != want {
		t.Errorf("step 2 executable = %q, want %q", got, want)
	}
	assertContains(t, results[2].Args(), "stapler", "staple", bundle.Path)
}

func TestAppBundleNotarizeAppleID(t *testing.T) {
	ctx := context.Background()
	runner := buildtools.NewCommandRunner(buildtools.WithDryRun(true))
	bundle := buildtools.AppBundle{
		Path: filepath.Join("/tmp", "TestApp.app"),
	}

	appleIDCfg := buildtools.NotaryConfig{ //nolint:gosec // test credential reference
		AppleID:  "dev@example.com",
		TeamID:   "TEAM123",
		Password: "@env:APP_SPECIFIC_PASSWORD",
	}
	steps := bundle.Notarize(appleIDCfg)
	stepRunner := buildtools.NewRunner().AddSteps(steps...)
	results := stepRunner.Run(ctx, runner)
	if err := results.Error(); err != nil {
		t.Fatalf("unexpected error during dry run: %v", err)
	}
	assertContains(t, results[1].Args(), "--apple-id", "dev@example.com", "--team-id", "TEAM123", "--password", "@env:APP_SPECIFIC_PASSWORD")
}

func TestAppBundleNotarizeMissingCredentials(t *testing.T) {
	ctx := context.Background()
	runner := buildtools.NewCommandRunner(buildtools.WithDryRun(true))
	bundle := buildtools.AppBundle{
		Path: filepath.Join("/tmp", "TestApp.app"),
	}

	emptyCfg := buildtools.NotaryConfig{}
	steps := bundle.Notarize(emptyCfg)
	stepRunner := buildtools.NewRunner().AddSteps(steps...)
	results := stepRunner.Run(ctx, runner)
	if results.Error() == nil {
		t.Errorf("expected error when notary credentials are empty, got nil")
	}
}

func TestAppBundleStapleDryRun(t *testing.T) {
	ctx := context.Background()
	runner := buildtools.NewCommandRunner(buildtools.WithDryRun(true))
	bundle := buildtools.AppBundle{
		Path: filepath.Join("/tmp", "TestApp.app"),
	}

	step := bundle.Staple()
	res, err := step.Run(ctx, runner)
	if err != nil {
		t.Fatalf("unexpected error during staple dry run: %v", err)
	}
	if got, want := res.Executable(), "xcrun"; got != want {
		t.Errorf("executable = %q, want %q", got, want)
	}
	args := res.Args()
	if !slices.Contains(args, "stapler") || !slices.Contains(args, "staple") || !slices.Contains(args, bundle.Path) {
		t.Errorf("staple args %v do not contain expected stapler command", args)
	}
}

func TestNotaryValidateSigningConfig(t *testing.T) {
	configuredNotary := buildtools.NotaryConfig{KeychainProfile: "profile"}
	unconfiguredNotary := buildtools.NotaryConfig{}

	// 1. Unconfigured notary is always valid (no notarization requested)
	if err := unconfiguredNotary.ValidateSigningConfig(buildtools.SigningConfig{}); err != nil {
		t.Errorf("unconfigured notary returned error: %v", err)
	}

	// 2. Signing not configured
	if err := configuredNotary.ValidateSigningConfig(buildtools.SigningConfig{}); err == nil {
		t.Error("expected error when signing is not configured")
	}

	// 3. Empty identity
	if err := configuredNotary.ValidateSigningConfig(buildtools.SigningConfig{CodesignArguments: []string{"--force"}}); err == nil {
		t.Error("expected error when identity is empty")
	}

	// 4. Ad-hoc identity "-"
	if err := configuredNotary.ValidateSigningConfig(buildtools.SigningConfig{Identity: "-"}); err == nil {
		t.Error("expected error for ad-hoc identity '-'")
	}

	// 5. Apple Development identity
	if err := configuredNotary.ValidateSigningConfig(buildtools.SigningConfig{Identity: "Apple Development: Dev (123)"}); err == nil {
		t.Error("expected error for Apple Development identity")
	}

	// 6. Mac Developer identity
	if err := configuredNotary.ValidateSigningConfig(buildtools.SigningConfig{Identity: "Mac Developer: Dev (123)"}); err == nil {
		t.Error("expected error for Mac Developer identity")
	}

	// 7. Developer ID Application identity (valid)
	if err := configuredNotary.ValidateSigningConfig(buildtools.SigningConfig{Identity: "Developer ID Application: Company (123)"}); err != nil {
		t.Errorf("expected Developer ID identity to succeed, got: %v", err)
	}
}
