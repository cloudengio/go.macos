// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"cloudeng.io/cmdutil/keys"
	"cloudeng.io/linux/keyrings"
	"github.com/cloudengio/keyctl"
)

type RunFlags struct{}

func (dockerCmds) run(_ context.Context, _ any, _ []string) error {
	return fmt.Errorf("run not implemented on linux")
}

type EntryFlags struct{}

func (dc dockerCmds) entry(ctx context.Context, _ any, args []string) error {
	ims, err := readIMS(os.Stdin)
	if err != nil {
		return err
	}
	if ims != nil {
		if err := dc.writeKeys(ctx, ims); err != nil {
			return err
		}
	}
	binary, err := exec.LookPath(args[0])
	if err != nil {
		return err
	}
	argstr := strings.Builder{}
	for _, a := range args {
		fmt.Fprintf(&argstr, "%q ", a)
	}
	return syscall.Exec(binary, args, os.Environ())
}

func (dc dockerCmds) writeKeys(ctx context.Context, ims *keys.InMemoryKeyStore) error {
	kr, err := keyctl.SessionKeyring()
	if err != nil {
		return fmt.Errorf("failed to get session keyring: %v", err)
	}
	kfs, err := keyrings.New(keyrings.WithKeyring(kr))
	if err != nil {
		return fmt.Errorf("failed to create keyring: %v", err)
	}
	for _, owner := range ims.KeyOwners() {
		ki, ok := ims.Get(owner.User, owner.ID)
		if !ok {
			return fmt.Errorf("key %q not found for user %q", owner.ID, owner.User)
		}
		token := ki.Token()
		if err := kfs.WriteFileCtx(ctx, token.ID, token.Value(), 0600); err != nil {
			return fmt.Errorf("failed to write key %q for user %q: %v", owner.ID, owner.User, err)
		}
		fmt.Printf("docker-entrypoint: key written: %v\n", token.ID)
	}
	return nil
}
