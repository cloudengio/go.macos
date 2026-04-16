// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"cloudeng.io/cmdutil/keys"
	"cloudeng.io/macos/keychain/plugin"
	"cloudeng.io/security/keys/keychain/plugins"
)

type RunFlags struct {
	plugin.ReadFlags
	KeychainItem string `subcmd:"keychain-item,,'keychain item to read, the item should be in cloudeng.io/cmdutil/keys format'"`
}

func (dc dockerCmds) run(ctx context.Context, f any, args []string) error {
	fl := f.(*RunFlags)
	stdout := bytes.NewBuffer(make([]byte, 0, 1024))
	ims := keys.NewInMemoryKeyStore()
	if len(fl.KeychainItem) != 0 {
		cfg := fl.Config()
		fs := plugins.NewFS(cfg.Binary, cfg)
		if err := ims.ReadYAML(ctx, fs, fl.KeychainItem); err != nil {
			return err
		}
	}
	profile, err := dc.writeSeccompToTempFile()
	if err != nil {
		return err
	}
	defer func() {
		if err := os.Remove(profile); err != nil {
			fmt.Fprintln(os.Stderr, "failed to remove seccomp profile:", err)
		}
	}()
	dockerArgs := []string{"run", "-i", "--security-opt", "seccomp=" + profile}
	for _, a := range args {
		switch a {
		case "run", "-i", "--interactive":
			continue
		default:
			dockerArgs = append(dockerArgs, a)
		}
	}
	if err := writeIMS(stdout, ims); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	cmd.Stdin = stdout
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker run: %s %w", strings.Join(cmd.Args, " "), err)
	}
	return nil
}

type EntryFlags struct{}

func (dockerCmds) entry(context.Context, any, []string) error {
	return fmt.Errorf("entry not implemented on macos")
}
