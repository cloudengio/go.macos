// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools

import "testing"

func TestWithDefaults(t *testing.T) {
	app := AppBundle{}
	got := BundleSpec{Icon: "icon.png"}.withDefaults(app, "/bin/ls").Info
	if got.CFBundleIconFile != "AppIcon.icns" || got.CFBundleExecutable != "ls" || got.CFBundleName != "ls" {
		t.Errorf("unexpected defaults: %+v", got)
	}
	if got := (BundleSpec{}).withDefaults(app, "/bin/ls").Info; got.CFBundleIconFile != "" {
		t.Errorf("icon file set without an icon: %q", got.CFBundleIconFile)
	}
	app.Info.CFBundleIconFile = "Mine.icns"
	if got := (BundleSpec{Icon: "icon.png"}).withDefaults(app, "/bin/ls").Info; got.CFBundleIconFile != "Mine.icns" {
		t.Errorf("explicit icon file overridden: %q", got.CFBundleIconFile)
	}
}
