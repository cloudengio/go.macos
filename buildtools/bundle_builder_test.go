// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cloudeng.io/macos/buildtools"
	"gopkg.in/yaml.v3"
)

func newBuilder(t *testing.T) buildtools.BundleBuilder {
	t.Helper()
	dir := t.TempDir()
	profile := filepath.Join(dir, "example.provisionprofile")
	if err := os.WriteFile(profile, []byte("profile"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := buildtools.BundleConfig{
		Bundle: filepath.Join(dir, "example.app"),
		Info: buildtools.InfoPlist{
			CFBundleIdentifier: "com.example.app",
			CFBundleName:       "example",
			CFBundleExecutable: "example",
		}.WithDefaults("example"),
		Signing:             buildtools.SigningConfig{Identity: "Developer ID Application: Example (TEAMID)"},
		Notary:              buildtools.NotaryConfig{KeychainProfile: "notary-profile"},
		Permissions:         buildtools.PermissionsConfig{Executable: 0755, MacOSDir: 0755},
		ProvisioningProfile: profile,
	}
	app := cfg.AppBundle()
	return buildtools.BundleBuilder{
		Config:   cfg,
		App:      app,
		Binary:   "/bin/echo",
		Contents: []buildtools.Step{app.CopyContents("/etc/hosts", "Resources", "hosts")},
	}
}

// commandLines returns the commands that the steps would run.
func commandLines(t *testing.T, b buildtools.BundleBuilder) []string {
	t.Helper()
	steps, err := b.Steps()
	if err != nil {
		t.Fatal(err)
	}
	results := buildtools.NewRunner().AddSteps(steps...).
		Run(t.Context(), buildtools.NewCommandRunner(buildtools.WithDryRun(true)))
	if err := results.Error(); err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, r := range results {
		lines = append(lines, r.CommandLine())
	}
	return lines
}

// indexOf returns the index of the first line that has the prefix, or -1.
func indexOf(lines []string, prefix string) int {
	for i, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return i
		}
	}
	return -1
}

// TestBundleBuilderOrder checks that everything that changes the contents of the
// bundle is done before it is signed, and that notarization follows signing.
func TestBundleBuilderOrder(t *testing.T) {
	b := newBuilder(t)
	b.Notarize = true
	lines := commandLines(t, b)

	positions := map[string]int{}
	for name, prefix := range map[string]string{
		"create":      "mkdir -p",
		"profile":     "cp " + b.Config.ProvisioningProfile,
		"plist":       "write Info.plist",
		"executable":  "cp /bin/echo",
		"contents":    "cp /etc/hosts",
		"permissions": "chmod",
		"sign":        "codesign",
		"zip":         "ditto",
		"notarize":    "xcrun notarytool",
		"staple":      "xcrun stapler",
	} {
		i := indexOf(lines, prefix)
		if i < 0 {
			t.Fatalf("no %q step (%q) in:\n%s", name, prefix, strings.Join(lines, "\n"))
		}
		positions[name] = i
	}
	order := []string{"create", "profile", "plist", "executable", "contents", "permissions", "sign", "zip", "notarize", "staple"}
	for i := 1; i < len(order); i++ {
		if positions[order[i-1]] >= positions[order[i]] {
			t.Errorf("%v (%d) should precede %v (%d):\n%s", order[i-1], positions[order[i-1]], order[i], positions[order[i]], strings.Join(lines, "\n"))
		}
	}
	// Both the executable and the bundle are signed.
	if n := strings.Count(strings.Join(lines, "\n"), "codesign"); n != 2 {
		t.Errorf("expected the executable and the bundle to be signed, got %d signings:\n%s", n, strings.Join(lines, "\n"))
	}
}

func TestBundleBuilderOptionalSteps(t *testing.T) {
	// Not notarized unless asked, and not signed without an identity.
	b := newBuilder(t)
	lines := commandLines(t, b)
	if indexOf(lines, "ditto") >= 0 || indexOf(lines, "xcrun") >= 0 {
		t.Errorf("unexpected notarization:\n%s", strings.Join(lines, "\n"))
	}
	if indexOf(lines, "codesign") < 0 {
		t.Errorf("expected signing:\n%s", strings.Join(lines, "\n"))
	}
	b.Config.Signing = buildtools.SigningConfig{}
	lines = commandLines(t, b)
	if indexOf(lines, "codesign") >= 0 {
		t.Errorf("unexpected signing:\n%s", strings.Join(lines, "\n"))
	}
	// Neither is the provisioning profile embedded if there is none.
	b.Config.ProvisioningProfile = ""
	lines = commandLines(t, b)
	if indexOf(lines, "cp ") < 0 || strings.Contains(strings.Join(lines, "\n"), "embedded.provisionprofile") {
		t.Errorf("unexpected provisioning profile:\n%s", strings.Join(lines, "\n"))
	}
}

func TestBundleBuilderValidation(t *testing.T) {
	var entitled buildtools.SigningConfig
	if err := yaml.Unmarshal([]byte("identity: \"Developer ID Application: Example\"\nentitlements:\n  keychain-access-groups: [TEAMID.com.example]\n"), &entitled); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		mod  func(*buildtools.BundleBuilder)
		want string
	}{
		{"notarize without credentials", func(b *buildtools.BundleBuilder) {
			b.Notarize, b.Config.Notary = true, buildtools.NotaryConfig{}
		}, "notary credentials"},
		{"notarize without a developer id", func(b *buildtools.BundleBuilder) {
			b.Notarize = true
			b.Config.Signing.Identity = "Apple Development: someone"
		}, "Developer ID"},
		{"notarize without signing", func(b *buildtools.BundleBuilder) {
			b.Notarize, b.Config.Signing = true, buildtools.SigningConfig{}
		}, "not signed"},
		{"restricted entitlements without a profile", func(b *buildtools.BundleBuilder) {
			b.Config.Signing, b.Config.ProvisioningProfile = entitled, ""
		}, "provisioning profile"},
		{"inaccessible profile", func(b *buildtools.BundleBuilder) {
			b.Config.ProvisioningProfile = filepath.Join(filepath.Dir(b.Config.Bundle), "absent")
		}, "not accessible"},
	} {
		b := newBuilder(t)
		tc.mod(&b)
		if _, err := b.Steps(); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: got %v, want an error containing %q", tc.name, err, tc.want)
		}
	}
	if _, err := newBuilder(t).Steps(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Entitlements that are not restricted need no provisioning profile.
	var sandboxed buildtools.SigningConfig
	if err := yaml.Unmarshal([]byte("identity: \"Developer ID Application: Example\"\nentitlements:\n  com.apple.security.app-sandbox: true\n"), &sandboxed); err != nil {
		t.Fatal(err)
	}
	b := newBuilder(t)
	b.Config.Signing, b.Config.ProvisioningProfile = sandboxed, ""
	if _, err := b.Steps(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBundleConfigYAML(t *testing.T) {
	const spec = `
bundle: build/example.app
info_plist:
  CFBundleIdentifier: com.example.app
signing:
  identity: "Developer ID Application: Example"
notary:
  keychain_profile: np
permissions:
  executable: 0755
provisioning_profile: example.provisionprofile
`
	var cfg buildtools.BundleConfig
	if err := yaml.Unmarshal([]byte(spec), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Bundle != "build/example.app" || cfg.Info.CFBundleIdentifier != "com.example.app" ||
		cfg.Signing.Identity != "Developer ID Application: Example" || cfg.Notary.KeychainProfile != "np" ||
		cfg.Permissions.ExecutableMode() != 0755 || cfg.ProvisioningProfile != "example.provisionprofile" {
		t.Errorf("got %+v", cfg)
	}
	if app := cfg.AppBundle(); app.Path != cfg.Bundle || app.Info.CFBundleIdentifier != cfg.Info.CFBundleIdentifier {
		t.Errorf("got %+v", app)
	}
}
