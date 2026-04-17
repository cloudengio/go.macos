// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:generate gobundle install ./macos-keychain-plugin.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"cloudeng.io/logging/ctxlog"
	"cloudeng.io/macos/keychain"
	"cloudeng.io/macos/keychain/plugin"
)

func main() {
	if len(os.Args) > 1 {
		possiblyHandleCommandLine(os.Args[1:])
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	ctx := ctxlog.WithLogger(context.Background(), logger)
	srv := plugin.NewServer(logger)
	cfg, req, resp := srv.ReadRequest(ctx, os.Stdin)
	if resp != nil {
		srv.SendResponse(ctx, os.Stdout, resp)
		return
	}
	resp = srv.HandleRequest(ctx, cfg, req)
	srv.SendResponse(ctx, os.Stdout, resp)
}

const usage = `Usage: [--help|delete keychain-type account service|read keychain-type account service]

macos-keychain-plugin is a plugin for the macOS keychain.
To install it 'run go generate' in the go/cmd/keychain-plugin directory
taking care to set up the appropriate Apple signing 
identity and provisioning profile environment variables required by
gobundle-app.yml.
`

func possiblyHandleCommandLine(args []string) {
	if len(args) == 1 && args[0] == "--help" {
		fmt.Print(usage)
		os.Exit(1)
	}
	if len(args) != 4 {
		return
	}
	switch args[0] {
	case "delete":
		deleteArgs(args)
	case "read":
		read(args)
	default:
		fmt.Print(usage)
		os.Exit(1)
	}
}

func parseArgs(args []string) (kt keychain.Type, account, service string, kc *keychain.T) {
	account = args[2]
	service = args[3]
	kt, err := keychain.ParseType(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse keychain type %q: %v\n", args[1], err)
		os.Exit(1)
	}
	return kt, account, service, keychain.New(kt, account)
}

func deleteArgs(args []string) {
	kt, account, service, kc := parseArgs(args)
	if err := kc.DeleteSecureNote(service); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete keychain item %q for account %q: %v\n", service, account, err)
		os.Exit(1)
	}
	fmt.Printf("Deleted keychain item %q (account %s) from %s keychain \n", service, account, kt)
	os.Exit(0)
}

func read(args []string) {
	kt, account, service, kc := parseArgs(args)
	contents, err := kc.ReadSecureNote(service)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read keychain item %q for account %q: %v\n", service, account, err)
		os.Exit(1)
	}
	fmt.Printf("%v %v: %v bytes from keychain: %s\n", service, account, len(contents), kt)
	os.Exit(0)
}
