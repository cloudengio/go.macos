// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools

import (
	"strings"
)

// GoBuildEnvForMacOSVersion returns the environment variables needed for
// go build and cgo to target the specified macOS version, as typically
// specified by LSMinimumSystemVersion in an Info.plist.
//
// The returned environment variables include:
//   - MACOSX_DEPLOYMENT_TARGET=<version>
//   - CGO_CFLAGS=-mmacosx-version-min=<version>
//   - CGO_CXXFLAGS=-mmacosx-version-min=<version>
//   - CGO_LDFLAGS=-mmacosx-version-min=<version>
//
// If version is empty after trimming whitespace and any leading 'v' prefix,
// nil is returned.
func GoBuildEnvForMacOSVersion(version string) []string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	if len(version) == 0 {
		return nil
	}
	return []string{
		"MACOSX_DEPLOYMENT_TARGET=" + version,
		"CGO_CFLAGS=-mmacosx-version-min=" + version,
		"CGO_CXXFLAGS=-mmacosx-version-min=" + version,
		"CGO_LDFLAGS=-mmacosx-version-min=" + version,
	}
}
