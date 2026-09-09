// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools_test

import (
	"strings"
	"testing"

	"cloudeng.io/macos/buildtools"
)

func TestFileOneOf(t *testing.T) {
	for _, tc := range []struct {
		name string
		file buildtools.File
		want string
	}{
		{
			name: "local only",
			file: buildtools.File{Src: "src", DstLocal: "local"},
			want: "local",
		},
		{
			name: "system only",
			file: buildtools.File{Src: "src", DstSystem: "system"},
			want: "system",
		},
		{
			name: "both set",
			file: buildtools.File{Src: "src", DstLocal: "local", DstSystem: "system"},
			want: "",
		},
		{
			name: "neither set",
			file: buildtools.File{Src: "src"},
			want: "src",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.file.OneOf(); got != tc.want {
				t.Errorf("OneOf() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFileRewriteHOME(t *testing.T) {
	file := buildtools.File{
		Src:       "$HOME/src/file",
		DstLocal:  "${HOME}/local/file",
		DstSystem: "/Library/Application Support/app",
	}
	rewritten := file.RewriteHOME()
	if got, want := rewritten.Src, "${TARGET_HOME}/src/file"; got != want {
		t.Errorf("Src = %q, want %q", got, want)
	}
	if got, want := rewritten.DstLocal, "${TARGET_HOME}/local/file"; got != want {
		t.Errorf("DstLocal = %q, want %q", got, want)
	}
	if got, want := rewritten.DstSystem, "/Library/Application Support/app"; got != want {
		t.Errorf("DstSystem = %q, want %q", got, want)
	}
}

func TestBashScript(t *testing.T) {
	scr := buildtools.NewBashScript(buildtools.BashInstallPreamble)
	scr.Append("# Custom comment\n")

	f := buildtools.File{
		Src:       "src/path",
		DstSystem: "/system/path",
		DstLocal:  "/local/path",
	}
	scr.InstallFile(true, f, "/tmp/manifest.txt")
	// Second call should not duplicate installer_copy definition
	scr.InstallFile(false, f, "")

	scr.CreateInstallManifest(true, buildtools.File{DstSystem: "/system/manifest.txt"})
	scr.CreateInstallManifest(false, buildtools.File{Src: "src/manifest.txt", DstLocal: "/local/manifest.txt"})

	out := string(scr.Bytes())
	if !strings.Contains(out, buildtools.BashInstallPreamble) {
		t.Error("script missing preamble")
	}
	if !strings.Contains(out, "# Custom comment") {
		t.Error("script missing appended comment")
	}
	// installer_copy function should only appear once
	if strings.Count(out, "function installer_copy") != 1 {
		t.Errorf("expected function installer_copy to appear exactly once, appeared %d times",
			strings.Count(out, "function installer_copy"))
	}
	if !strings.Contains(out, `installer_copy "$target" true "src/path" "/system/path" "/local/path" "/tmp/manifest.txt"`) {
		t.Error("script missing first InstallFile command")
	}
	if !strings.Contains(out, `installer_copy "$target" false "src/path" "/system/path" "/local/path" ""`) {
		t.Error("script missing second InstallFile command")
	}
	if !strings.Contains(out, "/dev/null") {
		t.Error("expected /dev/null for empty manifest source in CreateInstallManifest")
	}
}
