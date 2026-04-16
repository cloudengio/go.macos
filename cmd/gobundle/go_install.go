// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func handleGoInstall(ctx context.Context, merged []byte, args []string) error {
	_, rest := consumeBuildArgs(args)
	installDir, binary := deterimineInstallBinary(rest)
	if err := os.Remove(binary); err != nil && !os.IsNotExist(err) { //nolint:gosec // G703 overly restrictive for this use case.
		return fmt.Errorf("error removing original binary: %v", err)
	}
	if err := rungo(ctx, append([]string{"install"}, args...)); err != nil {
		return err
	}
	if _, err := os.Stat(binary); err != nil { //nolint:gosec // G703 overly restrictive for this use case.
		return fmt.Errorf("error finding expected binary: %v: %v", binary, err)
	}
	cfg, err := configForGoInstall(installDir, binary, merged)
	if err != nil {
		return fmt.Errorf("error processing config for go install: %v", err)
	}
	b := newBundle(cfg)
	if err := b.createAndSign(ctx, binary); err != nil {
		return err
	}
	if err := os.Remove(binary); err != nil { //nolint:gosec // G703 overly restrictive for this use case.
		return fmt.Errorf("error removing original binary: %v", err)
	}
	if err := os.Symlink(b.ap.ExecutablePath(), binary); err != nil {
		return fmt.Errorf("error creating symlink to signed binary: %v", err)
	}
	return nil
}

func deterimineInstallBinary(rest []string) (string, string) {
	// Executables are installed in the directory named by the GOBIN environment
	// variable, which defaults to $GOPATH/bin or $HOME/go/bin if the GOPATH
	// environment variable is not set. Executables in $GOROOT
	// are installed in $GOROOT/bin or $GOTOOLDIR instead of $GOBIN.
	binary := determineBuildBinary("", rest)
	binary = filepath.Base(binary)
	installDir := os.Getenv("GOBIN")
	if len(installDir) > 0 && isDir(installDir) {
		return installDir, filepath.Join(installDir, binary)
	}
	gopath := os.Getenv("GOPATH")
	if len(gopath) > 0 && isDir(filepath.Join(gopath, "bin")) {
		return filepath.Join(gopath, "bin"), filepath.Join(gopath, "bin", binary)
	}
	home := os.Getenv("HOME")
	if len(home) > 0 {
		return filepath.Join(home, "go", "bin"), filepath.Join(home, "go", "bin", binary)
	}
	return "", binary
}

func configForGoInstall(installDir, binary string, merged []byte) (config, error) {
	if len(installDir) == 0 {
		return config{}, fmt.Errorf("cannot determine install directory")
	}
	cfg, err := configFromMerged(merged, binary)
	if err != nil {
		return config{}, fmt.Errorf("error processing config for go build: %v", err)
	}
	if cfg.Path == "" {
		cfg.Path = filepath.Join(installDir, filepath.Base(binary)+".app")
	}
	return cfg, nil
}
