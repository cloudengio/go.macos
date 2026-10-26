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

const (
	devID    = "Developer ID Application: Example (TEAMID)"
	extKey   = "PlugIns/Example.appex"
	frameKey = extKey + "/Contents/Frameworks/Lib.framework"
)

// newBundle returns a bundle with a nested extension, that has a framework
// within it, a provisioning profile, and the options to build it.
func newBundle(t *testing.T) (buildtools.Bundle, []buildtools.BuildOption) {
	t.Helper()
	dir := t.TempDir()
	profile := filepath.Join(dir, "example.provisionprofile")
	if err := os.WriteFile(profile, []byte("profile"), 0600); err != nil {
		t.Fatal(err)
	}
	info := func(id, exe string) buildtools.InfoPlist {
		return buildtools.InfoPlist{CFBundleIdentifier: id, CFBundleName: exe, CFBundleExecutable: exe}.WithDefaults(exe)
	}
	b := buildtools.Bundle{
		Path:   filepath.Join(dir, "example.app"),
		Notary: buildtools.NotaryConfig{KeychainProfile: "notary-profile"},
		BundleSpec: buildtools.BundleSpec{
			Info:                info("com.example.app", "example"),
			Signing:             buildtools.SigningConfig{Identity: devID, CodesignArguments: []string{"--deep-not"}},
			Permissions:         buildtools.PermissionsConfig{Executable: 0755, MacOSDir: 0755},
			ProvisioningProfile: profile,
			SubBundles: []buildtools.SubBundle{{
				Path: extKey,
				BundleSpec: buildtools.BundleSpec{
					Info: info(".Extension", "ext"),
					SubBundles: []buildtools.SubBundle{{
						Path:       "Contents/Frameworks/Lib.framework",
						BundleSpec: buildtools.BundleSpec{Info: info(".Lib", "lib")},
					}},
				},
			}},
		},
	}
	// The path of a nested bundle is relative to its parent's Contents.
	b.SubBundles[0].Path = "PlugIns/Example.appex"
	b.SubBundles[0].SubBundles[0].Path = "Frameworks/Lib.framework"
	app := b.AppBundle()
	opts := []buildtools.BuildOption{
		buildtools.WithBinary("", "/bin/echo"),
		buildtools.WithBinary(extKey, "/bin/ls"),
		buildtools.WithBinary(frameKey, "/bin/cat"),
		buildtools.WithContents(extKey, b.SubBundles[0].AppBundle(app).CopyContents("/etc/hosts", "Resources", "hosts")),
	}
	return b, opts
}

// commandLines returns the commands that the steps for the bundle would run.
func commandLines(t *testing.T, b buildtools.Bundle, opts []buildtools.BuildOption) []string {
	t.Helper()
	steps, err := b.Steps(opts...)
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

// indexOf returns the index of the first line that starts with prefix and
// contains substr, or -1.
func indexOf(lines []string, prefix, substr string) int {
	for i, l := range lines {
		if strings.HasPrefix(l, prefix) && strings.Contains(l, substr) {
			return i
		}
	}
	return -1
}

func count(lines []string, prefix string) int {
	n := 0
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			n++
		}
	}
	return n
}

// checkPopulated checks that the executables and contents are added to every
// bundle, and that each is created before the bundles nested in it.
func checkPopulated(t *testing.T, b buildtools.Bundle, lines []string) {
	t.Helper()
	all := strings.Join(lines, "\n")
	root := b.Path
	ext := filepath.Join(root, "Contents", "PlugIns", "Example.appex")
	lib := filepath.Join(ext, "Contents", "Frameworks", "Lib.framework")
	// Every bundle's executable is copied, and the contents are added.
	for _, want := range []string{
		"cp /bin/echo " + filepath.Join(root, "Contents", "MacOS", "example"),
		"cp /bin/ls " + filepath.Join(ext, "Contents", "MacOS", "ext"),
		"cp /bin/cat " + filepath.Join(lib, "Contents", "MacOS", "lib"),
		"cp /etc/hosts " + filepath.Join(ext, "Contents", "Resources", "hosts"),
		"cp " + b.ProvisioningProfile + " " + filepath.Join(root, "Contents", "embedded.provisionprofile"),
	} {
		if indexOf(lines, want, "") < 0 {
			t.Errorf("missing %q in:\n%s", want, all)
		}
	}

	// Every bundle is created before the ones nested in it.
	rootIdx, extIdx, libIdx := indexOf(lines, "mkdir -p "+root, ""), indexOf(lines, "mkdir -p "+ext, ""), indexOf(lines, "mkdir -p "+lib, "")
	if rootIdx < 0 || rootIdx > extIdx || extIdx > libIdx {
		t.Errorf("bundles must be created outermost first (%d, %d, %d):\n%s", rootIdx, extIdx, libIdx, all)
	}

}

// TestBundleStepsOrder checks that the contents of every bundle are added
// before any is signed, that the bundles are signed from the inside out, and
// that only the top-level bundle is notarized, after it has been signed.
func TestBundleStepsOrder(t *testing.T) {
	b, opts := newBundle(t)
	opts = append(opts, buildtools.WithNotarize(true))
	lines := commandLines(t, b, opts)
	all := strings.Join(lines, "\n")

	root := b.Path
	ext := filepath.Join(root, "Contents", "PlugIns", "Example.appex")
	lib := filepath.Join(ext, "Contents", "Frameworks", "Lib.framework")

	checkPopulated(t, b, lines)

	// Each bundle is signed, its executable and then the bundle, after
	// the bundles that it contains, and nothing changes a bundle after signing.
	if n := count(lines, "codesign"); n != 6 {
		t.Errorf("expected the executable and the bundle of each of the three to be signed, got %d:\n%s", n, all)
	}
	firstSign := indexOf(lines, "codesign", "")
	libSign, extSign, rootSign := signIndex(lines, lib), signIndex(lines, ext), signIndex(lines, root)
	if firstSign < 0 || firstSign > libSign || libSign > extSign || extSign > rootSign {
		t.Errorf("signing must be from the inside out (%d, %d, %d, %d):\n%s", firstSign, libSign, extSign, rootSign, all)
	}
	for _, prefix := range []string{"cp ", "write ", "chmod", "mkdir"} {
		if last := lastIndexOf(lines, prefix); last > firstSign {
			t.Errorf("a %q step (%d) follows signing (%d):\n%s", prefix, last, firstSign, all)
		}
	}

	// Only the top-level bundle is notarized, after signing.
	if count(lines, "ditto") != 1 || count(lines, "xcrun notarytool") != 1 || count(lines, "xcrun stapler") != 1 {
		t.Errorf("only the top-level bundle is notarized:\n%s", all)
	}
	if indexOf(lines, "ditto", "") < rootSign {
		t.Errorf("notarization must follow signing:\n%s", all)
	}
}

func lastIndexOf(lines []string, prefix string) int {
	last := -1
	for i, l := range lines {
		if strings.HasPrefix(l, prefix) {
			last = i
		}
	}
	return last
}

// signIndex returns the index of the codesign command that signs path, ie. that
// ends with it, or -1.
func signIndex(lines []string, path string) int {
	for i, l := range lines {
		if strings.HasPrefix(l, "codesign") && strings.HasSuffix(strings.TrimSpace(l), path) {
			return i
		}
	}
	return -1
}

func TestBundleSigningInheritance(t *testing.T) {
	b, opts := newBundle(t)
	lines := commandLines(t, b, opts)
	// The nested bundles are signed with the identity and arguments of the top-level bundle.
	for _, l := range lines {
		if strings.HasPrefix(l, "codesign") && (!strings.Contains(l, devID) || !strings.Contains(l, "--deep-not")) {
			t.Errorf("not signed with the inherited identity and arguments: %v", l)
		}
	}

	// ... unless they have their own.
	b.SubBundles[0].Signing = buildtools.SigningConfig{Identity: "Other Identity", CodesignArguments: []string{"--other"}}
	lines = commandLines(t, b, opts)
	for _, l := range lines {
		if !strings.HasPrefix(l, "codesign") {
			continue
		}
		if strings.Contains(l, "Example.appex") {
			// The extension and the framework within it, which inherits from it.
			if !strings.Contains(l, "Other Identity") || !strings.Contains(l, "--other") {
				t.Errorf("expected the identity of the extension: %v", l)
			}
		} else if !strings.Contains(l, devID) {
			t.Errorf("expected the identity of the top-level bundle: %v", l)
		}
	}

	// No identity anywhere means nothing is signed.
	b, opts = newBundle(t)
	b.Signing = buildtools.SigningConfig{}
	b.ProvisioningProfile = ""
	if lines = commandLines(t, b, opts); count(lines, "codesign") != 0 {
		t.Errorf("unexpected signing:\n%s", strings.Join(lines, "\n"))
	}
}

func TestBundleOptionalSteps(t *testing.T) {
	b, opts := newBundle(t)
	if lines := commandLines(t, b, opts); count(lines, "ditto") != 0 {
		t.Errorf("not notarized unless asked:\n%s", strings.Join(lines, "\n"))
	}
	b.ProvisioningProfile = ""
	lines := commandLines(t, b, opts)
	if strings.Contains(strings.Join(lines, "\n"), "embedded.provisionprofile") {
		t.Errorf("unexpected provisioning profile:\n%s", strings.Join(lines, "\n"))
	}
}

func TestBundleIcon(t *testing.T) {
	b, opts := newBundle(t)
	icon := filepath.Join(filepath.Dir(b.Path), "icon.png")
	if err := os.WriteFile(icon, []byte("not really a png"), 0600); err != nil {
		t.Fatal(err)
	}
	b.Icon = icon
	lines := commandLines(t, b, opts)
	iconIdx, sign := indexOf(lines, "icon ", icon), indexOf(lines, "codesign", "")
	if iconIdx < 0 || sign < 0 || iconIdx > sign {
		t.Errorf("the icon (%d) must be created before signing (%d):\n%s", iconIdx, sign, strings.Join(lines, "\n"))
	}
}

func TestBundleIconMissing(t *testing.T) {
	b, opts := newBundle(t)
	b.SubBundles[0].Icon = filepath.Join(t.TempDir(), "missing.png")
	if _, err := b.Steps(opts...); err == nil || !strings.Contains(err.Error(), "icon") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBundleUnresolvedIdentifier(t *testing.T) {
	b, opts := newBundle(t)
	b.Info.CFBundleIdentifier = ""
	if _, err := b.Steps(opts...); err == nil || !strings.Contains(err.Error(), "starts with a '.'") {
		t.Errorf("unexpected error: %v", err)
	}
	b.Info.CFBundleIdentifier = ".app"
	if _, err := b.Steps(opts...); err == nil || !strings.Contains(err.Error(), "starts with a '.'") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBundleValidation(t *testing.T) {
	var restricted buildtools.SigningConfig
	if err := yaml.Unmarshal([]byte("entitlements:\n  keychain-access-groups: [TEAMID.com.example]\n"), &restricted); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		mod  func(*buildtools.Bundle, *[]buildtools.BuildOption)
		want string
	}{
		{"no path", func(b *buildtools.Bundle, _ *[]buildtools.BuildOption) { b.Path = "" }, "path of the bundle"},
		{"absolute nested path", func(b *buildtools.Bundle, _ *[]buildtools.BuildOption) { b.SubBundles[0].Path = "/PlugIns/x" }, "relative"},
		{"nested path that leaves", func(b *buildtools.Bundle, _ *[]buildtools.BuildOption) { b.SubBundles[0].Path = "../x.appex" }, "stay within"},
		{"no nested path", func(b *buildtools.Bundle, _ *[]buildtools.BuildOption) { b.SubBundles[0].Path = "" }, "relative"},
		{"duplicate identifier", func(b *buildtools.Bundle, _ *[]buildtools.BuildOption) {
			b.SubBundles[0].Info.CFBundleIdentifier = b.Info.CFBundleIdentifier
		}, "same identifier"},
		{"nested restricted entitlements without a profile", func(b *buildtools.Bundle, _ *[]buildtools.BuildOption) {
			s := restricted
			b.SubBundles[0].Signing = s
		}, "provisioning profile"},
		{"missing executable", func(_ *buildtools.Bundle, o *[]buildtools.BuildOption) { *o = (*o)[:2] }, "no executable"}, // drops the framework binary
		{"inaccessible nested profile", func(b *buildtools.Bundle, _ *[]buildtools.BuildOption) {
			b.SubBundles[0].ProvisioningProfile = filepath.Join(filepath.Dir(b.Path), "absent")
		}, "not accessible"},
		{"notarize without credentials", func(b *buildtools.Bundle, o *[]buildtools.BuildOption) {
			*o, b.Notary = append(*o, buildtools.WithNotarize(true)), buildtools.NotaryConfig{}
		}, "notary credentials"},
		{"notarize without a developer id", func(b *buildtools.Bundle, o *[]buildtools.BuildOption) {
			*o = append(*o, buildtools.WithNotarize(true))
			b.Signing.Identity = "Apple Development: someone"
		}, "Developer ID"},
		{"notarize without signing", func(b *buildtools.Bundle, o *[]buildtools.BuildOption) {
			*o, b.Signing = append(*o, buildtools.WithNotarize(true)), buildtools.SigningConfig{}
		}, "not signed"},
	} {
		b, opts := newBundle(t)
		tc.mod(&b, &opts)
		if _, err := b.Steps(opts...); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: got %v, want an error containing %q", tc.name, err, tc.want)
		}
	}
	b, opts := newBundle(t)
	if _, err := b.Steps(opts...); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Restricted entitlements are fine with a profile, and others need none.
	b.SubBundles[0].Signing = restricted
	b.SubBundles[0].ProvisioningProfile = b.ProvisioningProfile
	if _, err := b.Steps(opts...); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	var sandboxed buildtools.SigningConfig
	if err := yaml.Unmarshal([]byte("entitlements:\n  com.apple.security.app-sandbox: true\n"), &sandboxed); err != nil {
		t.Fatal(err)
	}
	b.SubBundles[0].Signing, b.SubBundles[0].ProvisioningProfile = sandboxed, ""
	if _, err := b.Steps(opts...); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveIdentifier(t *testing.T) {
	if got, want := buildtools.ResolveIdentifier("com.example.app", ".Extension"), "com.example.app.Extension"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := buildtools.ResolveIdentifier("com.example.app", "com.other"), "com.other"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	b, _ := newBundle(t)
	app := b.AppBundle()
	ext := b.SubBundles[0].AppBundle(app)
	lib := b.SubBundles[0].SubBundles[0].AppBundle(ext)
	if ext.Info.CFBundleIdentifier != "com.example.app.Extension" || lib.Info.CFBundleIdentifier != "com.example.app.Extension.Lib" {
		t.Errorf("got %q, %q", ext.Info.CFBundleIdentifier, lib.Info.CFBundleIdentifier)
	}
	if want := filepath.Join(app.Path, "Contents", "PlugIns", "Example.appex"); ext.Path != want {
		t.Errorf("got %q, want %q", ext.Path, want)
	}
}

func TestBundleYAML(t *testing.T) {
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
icon: icon.png
sub_bundles:
  - path: PlugIns/Example.appex
    info_plist:
      CFBundleIdentifier: .Extension
    signing:
      entitlements:
        com.apple.security.app-sandbox: true
    sub_bundles:
      - path: Frameworks/Lib.framework
        info_plist:
          CFBundleIdentifier: .Lib
`
	var b buildtools.Bundle
	if err := yaml.Unmarshal([]byte(spec), &b); err != nil {
		t.Fatal(err)
	}
	if b.Path != "build/example.app" || b.Info.CFBundleIdentifier != "com.example.app" ||
		b.Signing.Identity != "Developer ID Application: Example" || b.Notary.KeychainProfile != "np" ||
		b.Permissions.ExecutableMode() != 0755 || b.ProvisioningProfile != "example.provisionprofile" || b.Icon != "icon.png" {
		t.Errorf("got %+v", b)
	}
	if len(b.SubBundles) != 1 || b.SubBundles[0].Path != "PlugIns/Example.appex" || b.SubBundles[0].Signing.Entitlements == nil ||
		len(b.SubBundles[0].SubBundles) != 1 || b.SubBundles[0].SubBundles[0].Info.CFBundleIdentifier != ".Lib" {
		t.Errorf("got %+v", b.SubBundles)
	}
}

func TestBundleExecutableDefaults(t *testing.T) {
	b, opts := newBundle(t)
	ext := &b.SubBundles[0]
	ext.Info.CFBundleExecutable, ext.Info.CFBundleName = "", ""
	lines := commandLines(t, b, opts)
	// The executable is named after its binary, /bin/ls, when copied and signed.
	want := filepath.Join(b.Path, "Contents", extKey, "Contents", "MacOS", "ls")
	if indexOf(lines, "", want) < 0 {
		t.Errorf("no command uses %v: %v", want, lines)
	}
	if got := count(lines, "codesign"); got < 1 {
		t.Errorf("no signing commands: %v", lines)
	}
}
