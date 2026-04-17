// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
)

func runCommand(ctx context.Context, binary string, args []string) error {
	cmd := exec.CommandContext(ctx, binary, args...) //nolint:gosec // G702 overly restritive for this use case.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func handleGoRun(ctx context.Context, args []string) {
	if slices.Contains(args, "-exec") {
		exit(1, "cannot use -exec with gosign\n")
	}
	if len(args) == 0 {
		rungoExit(ctx, "run")
	}
	extendedArgs := []string{"run",
		"-exec", os.Args[0] + " __runsign__"}
	extendedArgs = append(extendedArgs, args...)
	rungoExit(ctx, extendedArgs...)
}

func handleGoRunExec(ctx context.Context, merged []byte, binary string) error {
	tmpDir, err := os.MkdirTemp("", "gobundle-run")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %v", err)
	}
	cfg, err := configForGoBuild(binary, tmpDir, merged)
	if err != nil {
		return fmt.Errorf("error processing config for go run: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	b := newBundle(cfg)
	if err := b.createAndSign(ctx, binary); err != nil {
		return fmt.Errorf("error creating and signing bundle: %v", err)
	}
	return runCommand(ctx, b.ap.ExecutablePath(), os.Args[3:])
}
