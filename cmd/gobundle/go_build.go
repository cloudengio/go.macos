// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cloudeng.io/macos/buildtools"
	"cloudeng.io/macos/cmd/gobundle/gobundleconfig"
)

func handleGoBuild(ctx context.Context, cfg gobundleconfig.T, notarize bool, binary string, args []string) error {
	dashO, _ := consumeBuildArgs(args)
	if dashO != "" && isDir(dashO) {
		cfg.Path = filepath.Join(dashO, cfg.Info.CFBundleExecutable+".app")
	}
	if cfg.Path == "" {
		cfg.Path = cfg.Info.CFBundleExecutable + ".app"
	}
	b := newBundle(cfg)

	if fi, err := os.Stat(b.ap.ExecutablePath()); err == nil && fi.Mode().IsRegular() {
		if err := os.Remove(b.ap.ExecutablePath()); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("error removing existing signed binary: %v", err)
		}
	}

	env := buildtools.GoBuildEnvForMacOSVersion(cfg.Info.LSMinimumSystemVersion)
	if err := rungo(ctx, append([]string{"build"}, args...), env...); err != nil {
		return err
	}
	if _, err := os.Stat(binary); err != nil {
		return fmt.Errorf("error finding expected binary: %v: %v", binary, err)
	}
	if err := b.createAndSign(ctx, binary, notarize); err != nil {
		return err
	}
	if err := os.Remove(binary); err != nil {
		return fmt.Errorf("error removing original binary: %v", err)
	}
	if err := os.Symlink(b.ap.ExecutablePath(), binary); err != nil {
		return fmt.Errorf("error creating symlink to signed binary: %v", err)
	}
	printf("Created symlink: %s -> %s\n", binary, b.ap.ExecutablePath())
	return nil
}

var buildArgs = map[string]int{
	"-C":             1,
	"-a":             0,
	"-n":             0,
	"-p":             1,
	"-race":          0,
	"-msan":          0,
	"-asan":          0,
	"-cover":         1,
	"-coverpkg":      1,
	"-covermode":     1,
	"-v":             0,
	"-work":          0,
	"-x":             0,
	"-asmflags":      1,
	"-buildmode":     1,
	"-buildvcs":      0,
	"-compiler":      1,
	"-gccgoflags":    1,
	"-gcflags":       1,
	"-installsuffix": 1,
	"-json":          0,
	"-ldflags":       1,
	"-linkshared":    0,
	"-mod":           1,
	"-modcacherw":    0,
	"-modfile":       1,
	"-overlay":       1,
	"-pgo":           1,
	"-pkgdir":        1,
	"-tags":          1,
	"-trimpath":      0,
	"-toolexec":      1,
	"-o":             -1,
}

func getBuildBinaryAbs(args []string) string {
	dashO, rest := consumeBuildArgs(args)
	return determineBuildBinary(dashO, rest)
}

func consumeBuildArgs(args []string) (string, []string) {
	dashO := ""
	for i := 0; i < len(args); i++ {
		n, ok := buildArgs[args[i]]
		if !ok {
			return dashO, args[i:]
		}
		if n == -1 { // -o
			if i+1 < len(args) {
				dashO = args[i+1]
			}
			i++
			continue
		}
		i += n
	}
	return dashO, nil
}

func withDir(dir, path string) string {
	if len(dir) > 0 {
		return filepath.Join(dir, filepath.Base(path))
	}
	return filepath.Base(path)
}

func determineBuildBinary(dashO string, rest []string) string {
	dir := ""
	if len(dashO) > 0 {
		if !isDir(dashO) {
			// -o <file> trumps all other options
			return dashO
		}
		dir = dashO
	}
	if len(rest) == 0 || len(rest) == 1 && rest[0] == "." {
		pwd, _ := os.Getwd()
		return withDir(dir, pwd)
	}
	firstgo := strings.TrimSuffix(rest[0], ".go")
	return withDir(dir, firstgo)
}

func isDir(path string) bool {
	info, err := os.Stat(path) //nolint:gosec // G703 overly restrictive for this use case.
	if err != nil {
		return false
	}
	return info.IsDir()
}
