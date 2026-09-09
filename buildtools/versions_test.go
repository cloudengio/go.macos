// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools_test

import (
	"reflect"
	"testing"

	"cloudeng.io/macos/buildtools"
)

func TestGoBuildEnvForMacOSVersion(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{
			input: "",
			want:  nil,
		},
		{
			input: "   ",
			want:  nil,
		},
		{
			input: "v",
			want:  nil,
		},
		{
			input: "15.0",
			want: []string{
				"MACOSX_DEPLOYMENT_TARGET=15.0",
				"CGO_CFLAGS=-mmacosx-version-min=15.0",
				"CGO_CXXFLAGS=-mmacosx-version-min=15.0",
				"CGO_LDFLAGS=-mmacosx-version-min=15.0",
			},
		},
		{
			input: "10.15",
			want: []string{
				"MACOSX_DEPLOYMENT_TARGET=10.15",
				"CGO_CFLAGS=-mmacosx-version-min=10.15",
				"CGO_CXXFLAGS=-mmacosx-version-min=10.15",
				"CGO_LDFLAGS=-mmacosx-version-min=10.15",
			},
		},
		{
			input: "v14.4.1",
			want: []string{
				"MACOSX_DEPLOYMENT_TARGET=14.4.1",
				"CGO_CFLAGS=-mmacosx-version-min=14.4.1",
				"CGO_CXXFLAGS=-mmacosx-version-min=14.4.1",
				"CGO_LDFLAGS=-mmacosx-version-min=14.4.1",
			},
		},
		{
			input: "  13.0  ",
			want: []string{
				"MACOSX_DEPLOYMENT_TARGET=13.0",
				"CGO_CFLAGS=-mmacosx-version-min=13.0",
				"CGO_CXXFLAGS=-mmacosx-version-min=13.0",
				"CGO_LDFLAGS=-mmacosx-version-min=13.0",
			},
		},
		{
			input: "  v12.0  ",
			want: []string{
				"MACOSX_DEPLOYMENT_TARGET=12.0",
				"CGO_CFLAGS=-mmacosx-version-min=12.0",
				"CGO_CXXFLAGS=-mmacosx-version-min=12.0",
				"CGO_LDFLAGS=-mmacosx-version-min=12.0",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := buildtools.GoBuildEnvForMacOSVersion(tc.input)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("GoBuildEnvForMacOSVersion(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
