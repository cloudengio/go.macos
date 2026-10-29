// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package macosutils_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"cloudeng.io/macos/macosutils"
	"howett.net/plist"
)

func TestBundleInfo(t *testing.T) {
	bundle := makeBundle(t, t.TempDir(), "Example.app")
	writeFile(t, filepath.Join(bundle, "Contents", "Info.plist"), `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
	<key>CFBundleIdentifier</key><string>com.example.app</string>
	<key>Empty</key><string></string>
	<key>Flag</key><true/>
</dict></plist>
`, 0600)
	info, err := macosutils.ReadBundleInfo(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Path(), filepath.Join(bundle, "Contents", "Info.plist"); got != want {
		t.Errorf("Path: %q, want %q", got, want)
	}
	if got, err := info.String("CFBundleIdentifier"); err != nil || got != "com.example.app" {
		t.Errorf("CFBundleIdentifier: %q, %v", got, err)
	}
	if got, err := info.String("Empty"); err != nil || got != "" {
		t.Errorf("Empty: %q, %v", got, err)
	}
	for _, key := range []string{"Flag", "Missing"} {
		if _, err := info.String(key); err == nil {
			t.Errorf("%v: expected an error", key)
		}
	}
	if v, ok := info.Value("Flag"); !ok || v != true {
		t.Errorf("Flag: %v, %v", v, ok)
	}
	if _, ok := info.Value("Missing"); ok {
		t.Error("Missing: expected no value")
	}
}

// TestReadBundleInfoFormats checks that a binary property list is read, and
// that a missing or malformed one is an error.
func TestReadBundleInfoFormats(t *testing.T) {
	if _, err := macosutils.ReadBundleInfo(filepath.Join(t.TempDir(), "None.app")); err == nil {
		t.Error("expected an error for a bundle with no Info.plist")
	}
	malformed := makeBundle(t, t.TempDir(), "Malformed.app")
	writeFile(t, filepath.Join(malformed, "Contents", "Info.plist"), "<plist><dict><key>x</key>", 0600)
	if _, err := macosutils.ReadBundleInfo(malformed); err == nil {
		t.Error("expected an error for a malformed Info.plist")
	}

	// A binary property list, as Xcode writes them.
	binary := makeBundle(t, t.TempDir(), "Binary.app")
	writeBinaryPlist(t, filepath.Join(binary, "Contents", "Info.plist"), map[string]any{"CFBundleName": "Binary"})
	info, err := macosutils.ReadBundleInfo(binary)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := info.String("CFBundleName"); err != nil || got != "Binary" {
		t.Errorf("binary plist: %q, %v", got, err)
	}
}

// enclosingEnv marks the test binary re-executed, from within a bundle, by
// TestEnclosingBundle, and carries the bundle that it should report.
const enclosingEnv = "CLOUDENG_MACOSUTILS_TEST_ENCLOSING"

// TestEnclosingBundle checks that a test binary, which is not in a bundle,
// has no enclosing bundle, and, by copying it into a bundle's Contents/MacOS
// and Contents/Helpers and re-executing it there, that it reports that
// bundle from both.
func TestEnclosingBundle(t *testing.T) {
	if os.Getenv(enclosingEnv) != "" {
		t.Skip("running inside the re-executed child")
	}
	if _, err := macosutils.EnclosingBundle(); !errors.Is(err, macosutils.ErrNotInBundle) {
		t.Errorf("got %v, want ErrNotInBundle", err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("cannot determine test executable: %v", err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Skipf("cannot read test executable %v: %v", exe, err)
	}
	bundle := makeBundle(t, t.TempDir(), "Host.app")
	for _, dir := range []string{"MacOS", "Helpers"} {
		hosted := filepath.Join(bundle, "Contents", dir, "host")
		if err := os.MkdirAll(filepath.Dir(hosted), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(hosted, data, 0700); err != nil { //nolint:gosec // G306, G703: the copy, in this test's temp directory, must be executable.
			t.Fatalf("copying test executable: %v", err)
		}
		cmd := exec.Command(hosted, "-test.run=TestEnclosingBundleChild", "-test.v")
		cmd.Env = append(os.Environ(), enclosingEnv+"="+bundle)
		out, err := cmd.CombinedOutput()
		if err != nil || !strings.Contains(string(out), "--- PASS: TestEnclosingBundleChild") {
			t.Errorf("from Contents/%v: %v\n%s", dir, err, out)
		}
	}
}

// TestEnclosingBundleChild runs only inside the process re-executed by
// TestEnclosingBundle.
func TestEnclosingBundleChild(t *testing.T) {
	want := os.Getenv(enclosingEnv)
	if want == "" {
		t.Skip("only runs as the child of TestEnclosingBundle")
	}
	// EnclosingBundle resolves symlinks: macOS temp directories are under a
	// symlinked /var.
	if resolved, err := filepath.EvalSymlinks(want); err == nil {
		want = resolved
	}
	if got, err := macosutils.EnclosingBundle(); err != nil || got != want {
		t.Errorf("EnclosingBundle = %q, %v, want %q", got, err, want)
	}
}

func writeBinaryPlist(t *testing.T, path string, v any) {
	t.Helper()
	data, err := plist.Marshal(v, plist.BinaryFormat)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, string(data), 0600)
}
