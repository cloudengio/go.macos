// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"cloudeng.io/macos/buildtools"
	"cloudeng.io/macos/cmd/gobundle/gobundleconfig"
)

func runCommand(ctx context.Context, binary string, args []string) error {
	cmd := exec.CommandContext(ctx, binary, args...) //nolint:gosec // G702 overly restritive for this use case.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func handleGoRun(ctx context.Context, cfg gobundleconfig.T, notarize bool, args []string) {
	if slices.Contains(args, "-exec") {
		exit(1, "cannot use -exec with gosign\n")
	}
	if len(args) == 0 {
		rungoExit(ctx, "run")
	}
	extendedArgs := []string{"run", "-exec", os.Args[0],
		args[0], fmt.Sprintf("--notarize=%v", notarize), signThenRunVerb}
	extendedArgs = append(extendedArgs, args[1:]...)
	env := buildtools.GoBuildEnvForMacOSVersion(cfg.Info.LSMinimumSystemVersion)
	runAndExit(func() error {
		return rungo(ctx, extendedArgs, env...)
	})
}

func handleGoRunExec(ctx context.Context, cfg gobundleconfig.T, notarize bool, binary string, args []string) error {
	tmpDir, err := os.MkdirTemp("", "gobundle-run")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	cfg.Path = filepath.Join(tmpDir, cfg.Info.CFBundleExecutable+".app")
	b := newBundle(cfg)
	if err := b.createAndSign(ctx, binary, notarize); err != nil {
		return fmt.Errorf("error creating and signing bundle: %v", err)
	}
	return runCommand(ctx, b.ap.ExecutablePath(), args)
}
