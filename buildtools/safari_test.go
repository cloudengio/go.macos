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
	"howett.net/plist"
)

// newSafariExtension returns a bundle and an extension whose resources are a
// minimal web extension, ie. a directory containing a manifest.json.
func newSafariExtension(t *testing.T) (buildtools.AppBundle, buildtools.SafariWebExtension) {
	t.Helper()
	tmp := t.TempDir()
	bundle := buildtools.AppBundle{
		Path: filepath.Join(tmp, "Example.app"),
		Info: buildtools.InfoPlist{
			CFBundleIdentifier: "io.cloudeng.Example",
			CFBundleName:       "Example",
			CFBundleExecutable: "Example",
		},
	}
	if err := runHelperSteps(t, bundle.Create()); err != nil {
		t.Fatalf("create bundle: %v", err)
	}

	resources := filepath.Join(tmp, "extension")
	if err := os.MkdirAll(filepath.Join(resources, "images"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string]string{
		"manifest.json":     `{"manifest_version":3,"name":"Example","version":"1.0"}`,
		"background.js":     "// background\n",
		"images/icon48.png": "not-a-png",
	} {
		if err := os.WriteFile(filepath.Join(resources, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}

	handler := filepath.Join(tmp, "ExampleHandler")
	if err := os.WriteFile(handler, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil { //nolint:gosec // test fixture
		t.Fatal(err)
	}

	ext := buildtools.SafariWebExtension{
		Name:           "Example Extension",
		Executable:     handler,
		Resources:      resources,
		PrincipalClass: "ExampleExtension.SafariWebExtensionHandler",
		Info: buildtools.InfoPlist{
			CFBundleIdentifier: "io.cloudeng.Example.Extension",
			CFBundleName:       "Example Extension",
			CFBundleExecutable: "ExampleHandler",
		},
	}
	return bundle, ext
}

// TestAddSafariWebExtension verifies that the .appex is built in the bundle's
// PlugIns directory with the extension's resources and an Info.plist that
// identifies it as a web extension.
func TestAddSafariWebExtension(t *testing.T) {
	bundle, ext := newSafariExtension(t)
	if err := runHelperSteps(t, bundle.AddSafariWebExtension(ext)); err != nil {
		t.Fatalf("add extension: %v", err)
	}

	appex := bundle.SafariWebExtensionPath(ext)
	if got, want := appex, bundle.PlugIns("Example Extension.appex"); got != want {
		t.Errorf("path: got %v, want %v", got, want)
	}

	// The handler executable and the extension's own resources are present.
	// The resources are the contents of the directory, not the directory.
	for _, rel := range []string{
		filepath.Join("Contents", "MacOS", "ExampleHandler"),
		filepath.Join("Contents", "Resources", "manifest.json"),
		filepath.Join("Contents", "Resources", "background.js"),
		filepath.Join("Contents", "Resources", "images", "icon48.png"),
		filepath.Join("Contents", "Info.plist"),
	} {
		if _, err := os.Stat(filepath.Join(appex, rel)); err != nil {
			t.Errorf("%v: %v", rel, err)
		}
	}

	// The Info.plist identifies a web extension, not an app extension.
	data, err := os.ReadFile(filepath.Join(appex, "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if _, err := plist.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal Info.plist: %v", err)
	}
	nsext, ok := got["NSExtension"].(map[string]any)
	if !ok {
		t.Fatalf("NSExtension: got %T, want a dictionary", got["NSExtension"])
	}
	if got, want := nsext["NSExtensionPointIdentifier"], buildtools.SafariWebExtensionPointIdentifier; got != want {
		t.Errorf("NSExtensionPointIdentifier: got %v, want %v", got, want)
	}
	if got, want := nsext["NSExtensionPrincipalClass"], ext.PrincipalClass; got != want {
		t.Errorf("NSExtensionPrincipalClass: got %v, want %v", got, want)
	}
	if got, want := got["CFBundlePackageType"], "XPC!"; got != want {
		t.Errorf("CFBundlePackageType: got %v, want %v", got, want)
	}
}

// TestAddSafariWebExtensionRejectsAppExtensions verifies that the older Safari
// app extensions are refused rather than built as though they were web
// extensions.
func TestAddSafariWebExtensionRejectsAppExtensions(t *testing.T) {
	bundle, ext := newSafariExtension(t)
	ext.Info.NSExtension = &buildtools.NSExtensionPlist{
		NSExtensionPointIdentifier: "com.apple.Safari.extension",
	}
	err := runHelperSteps(t, bundle.AddSafariWebExtension(ext))
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("got %v, want it to report app extensions as unsupported", err)
	}

	// An unrelated extension point is refused too.
	ext.Info.NSExtension.NSExtensionPointIdentifier = "com.apple.share-services"
	if err := runHelperSteps(t, bundle.AddSafariWebExtension(ext)); err == nil {
		t.Error("expected an error for an unrelated extension point, got nil")
	}
}

// TestAddSafariWebExtensionErrors covers the misconfigurations that would
// otherwise produce an extension Safari cannot load.
func TestAddSafariWebExtensionErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		modify func(*buildtools.SafariWebExtension)
		want   string
	}{
		{"no name", func(e *buildtools.SafariWebExtension) { e.Name = "" }, "name not specified"},
		{"no executable", func(e *buildtools.SafariWebExtension) { e.Executable = "" }, "executable not specified"},
		{"no resources", func(e *buildtools.SafariWebExtension) { e.Resources = "" }, "resources directory not specified"},
		{"no principal class", func(e *buildtools.SafariWebExtension) { e.PrincipalClass = "" }, "principal class not specified"},
		{"no identifier", func(e *buildtools.SafariWebExtension) { e.Info.CFBundleIdentifier = "" }, "CFBundleIdentifier not specified"},
		{"identifier not prefixed by the app's", func(e *buildtools.SafariWebExtension) {
			e.Info.CFBundleIdentifier = "io.example.Other"
		}, "must be prefixed by"},
	} {
		bundle, ext := newSafariExtension(t)
		tc.modify(&ext)
		err := runHelperSteps(t, bundle.AddSafariWebExtension(ext))
		if err == nil {
			t.Errorf("%v: expected an error, got nil", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: got %v, want it to contain %q", tc.name, err, tc.want)
		}
	}
}

// TestAddSafariWebExtensionRequiresManifest verifies that resources without a
// manifest.json are refused: a manifest is what makes it a web extension.
func TestAddSafariWebExtensionRequiresManifest(t *testing.T) {
	bundle, ext := newSafariExtension(t)
	if err := os.Remove(filepath.Join(ext.Resources, "manifest.json")); err != nil {
		t.Fatal(err)
	}
	err := runHelperSteps(t, bundle.AddSafariWebExtension(ext))
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "manifest.json") {
		t.Errorf("got %v, want it to mention manifest.json", err)
	}
}

// TestSafariHasNoNativeMessagingManifest verifies that the JSON manifest based
// route is reported as inapplicable to Safari, pointing at the .appex instead.
func TestSafariHasNoNativeMessagingManifest(t *testing.T) {
	_, err := buildtools.NativeMessagingHostsDir(buildtools.Safari, buildtools.UserScope, "/Users/e")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "AddSafariWebExtension") {
		t.Errorf("got %v, want it to point at AddSafariWebExtension", err)
	}

	cfg := buildtools.NativeMessagingConfig{Name: "io.cloudeng.example"}
	if err := runHelperSteps(t, []buildtools.Step{cfg.Validate(buildtools.Safari)}); err == nil {
		t.Error("expected an error validating a safari manifest, got nil")
	}
}

// TestSignSafariWebExtension verifies that both the inner executable and the
// .appex bundle itself are signed in inside-out order.
func TestSignSafariWebExtension(t *testing.T) {
	bundle, ext := newSafariExtension(t)
	signer := buildtools.NewSigner("Developer ID", nil, nil, nil)
	steps := bundle.SignSafariWebExtension(signer, ext)
	if len(steps) != 2 {
		t.Fatalf("expected 2 signing steps (inner executable, then .appex), got %d", len(steps))
	}

	// Error case: empty extension name
	extNoName := ext
	extNoName.Name = ""
	errSteps := bundle.SignSafariWebExtension(signer, extNoName)
	if len(errSteps) != 1 {
		t.Fatalf("expected 1 error step, got %d", len(errSteps))
	}
	if err := runHelperSteps(t, errSteps); err == nil {
		t.Error("expected error for empty extension name, got nil")
	}
}

// TestAddSafariWebExtensionSteps verifies that resourceSteps run in place of
// the default verbatim copy of Resources, while the .appex is still created,
// signed identity established, and the handler executable copied in the same
// way as AddSafariWebExtension.
func TestAddSafariWebExtensionSteps(t *testing.T) {
	bundle, ext := newSafariExtension(t)
	appex := bundle.SafariWebExtensionPath(ext)
	marker := filepath.Join(appex, "Contents", "Resources", "custom.txt")

	custom := buildtools.WriteFile([]byte("custom resources"), 0600, marker)
	if err := runHelperSteps(t, bundle.AddSafariWebExtensionSteps(ext, custom)); err != nil {
		t.Fatalf("add extension: %v", err)
	}

	// The custom step ran...
	if data, err := os.ReadFile(marker); err != nil {
		t.Errorf("custom resource step did not run: %v", err)
	} else if string(data) != "custom resources" {
		t.Errorf("custom resource contents = %q", data)
	}
	// ...instead of the default verbatim copy.
	if _, err := os.Stat(filepath.Join(appex, "Contents", "Resources", "manifest.json")); err == nil {
		t.Error("manifest.json was copied verbatim; want only the custom resource step to have run")
	}
	// The rest is unaffected: .appex layout and handler executable.
	for _, rel := range []string{
		filepath.Join("Contents", "MacOS", "ExampleHandler"),
		filepath.Join("Contents", "Info.plist"),
	} {
		if _, err := os.Stat(filepath.Join(appex, rel)); err != nil {
			t.Errorf("%v: %v", rel, err)
		}
	}
}

// TestExtensionConfigResolveIdentifier verifies the "leading dot means
// relative to the containing app's identifier" shorthand.
func TestExtensionConfigResolveIdentifier(t *testing.T) {
	for _, tc := range []struct {
		identifier, containingAppID, want string
	}{
		{"io.example.Example.Extension", "io.example.Example", "io.example.Example.Extension"},
		{".Extension", "io.example.Example", "io.example.Example.Extension"},
		{"", "io.example.Example", ""}, // no leading dot: left as is, e.g. for validation to reject
	} {
		c := buildtools.ExtensionConfig{Identifier: tc.identifier}
		if got := c.ResolveIdentifier(tc.containingAppID); got != tc.want {
			t.Errorf("ResolveIdentifier(%q) with Identifier %q = %q, want %q",
				tc.containingAppID, tc.identifier, got, tc.want)
		}
	}
}

// TestExtensionConfigInfoPlist verifies the field mapping from ExtensionConfig
// to the base Info.plist it returns.
func TestExtensionConfigInfoPlist(t *testing.T) {
	c := buildtools.ExtensionConfig{
		Name:       "Example Extension",
		Identifier: ".Extension",
		Executable: "ExampleHandler",
	}
	info := c.InfoPlist("io.example.Example")
	if got, want := info.CFBundleIdentifier, "io.example.Example.Extension"; got != want {
		t.Errorf("CFBundleIdentifier = %q, want %q", got, want)
	}
	if got, want := info.CFBundleName, c.Name; got != want {
		t.Errorf("CFBundleName = %q, want %q", got, want)
	}
	if got, want := info.CFBundleDisplayName, c.Name; got != want {
		t.Errorf("CFBundleDisplayName = %q, want %q", got, want)
	}
	if got, want := info.CFBundleExecutable, c.Executable; got != want {
		t.Errorf("CFBundleExecutable = %q, want %q", got, want)
	}
}

// TestExtensionConfigSafariWebExtension verifies the field mapping from
// ExtensionConfig (plus a resolved executable and Info.plist) to
// SafariWebExtension.
func TestExtensionConfigSafariWebExtension(t *testing.T) {
	c := buildtools.ExtensionConfig{
		Name:           "Example Extension",
		Resources:      "/tmp/resources",
		PrincipalClass: "ExampleExtension.SafariWebExtensionHandler",
	}
	info := buildtools.InfoPlist{CFBundleIdentifier: "io.example.Example.Extension"}
	ext := c.SafariWebExtension("/tmp/bin/ExampleHandler", info)
	if got, want := ext.Name, c.Name; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
	if got, want := ext.Executable, "/tmp/bin/ExampleHandler"; got != want {
		t.Errorf("Executable = %q, want %q", got, want)
	}
	if got, want := ext.Resources, c.Resources; got != want {
		t.Errorf("Resources = %q, want %q", got, want)
	}
	if got, want := ext.PrincipalClass, c.PrincipalClass; got != want {
		t.Errorf("PrincipalClass = %q, want %q", got, want)
	}
	if got, want := ext.Info.CFBundleIdentifier, info.CFBundleIdentifier; got != want {
		t.Errorf("Info.CFBundleIdentifier = %q, want %q", got, want)
	}
}

// ExtensionConfig.Resolve (building the Swift package handler binary) has
// moved to the bundle builder that uses buildtools, mirroring AppBundle's
// Go binary being built by its own caller: see buildSwiftBinary and its
// tests in browser/cmd/extension-bundle-builder.
