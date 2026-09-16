// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

//go:generate go run bundle_builder.go

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"cloudeng.io/cmdutil/flags"
	"cloudeng.io/cmdutil/keys/keyscmd"
	"cloudeng.io/cmdutil/subcmd"
	"cloudeng.io/file"
	"cloudeng.io/macos/keychain"
	"cloudeng.io/macos/keychain/plugin"
	"cloudeng.io/security/keys/keychain/plugins"
)

var dummyAccessibility flags.Enum[keychain.Accessibility]
var dummyType flags.Enum[keychain.Type]
var dummyWriteType flags.Enum[keychain.WriteType]

type globalFlags struct {
	Verbose bool `subcmd:"verbose,false,set to enable verbose logging"`
}

var globals globalFlags

var cmdSpec = fmt.Sprintf(`name: cloudeng-keychain
summary: provide access to local keychains across multiple operating systems
commands:
  - name: read
    summary: |-
       read an item from the keychain writing to filename, if filename is - the item will be written to stdout. Valid values for flags are as follows:
         --keychain-accessibility: %s
         --keychain-type: %s
    arguments:
      - <filename>
  - name: write
    summary: |-
      write an item read from <filename> to the keychain, if filename is - the item will be read from stdin. Valid values for flags are as follows:
         --keychain-accessibility: %s
         --keychain-type: %s
    arguments:
      - <filename>
  {{range subcmdExtension "key-info"}}
  {{.}}{{end}}
`, dummyAccessibility.AllowedValues(), dummyType.AllowedValues(),
	dummyAccessibility.AllowedValues(), dummyWriteType.AllowedValues())

func cli() *subcmd.CommandSetYAML {
	var pluginCmd pluginCmd
	ext := keyscmd.NewKeyInfoExtension("key-info", func(cmd *subcmd.CommandSetYAML) error {
		cmd.Set("key-info", "create").MustRunner(pluginCmd.Create, &CreateKeyInfoFlags{})
		cmd.Set("key-info", "list").MustRunner(pluginCmd.ListKeys, &ListKeysFlags{})
		cmd.Set("key-info", "get").MustRunner(pluginCmd.GetKeyInfo, &GetKeyInfoFlags{})
		cmd.Set("key-info", "set").MustRunner(pluginCmd.SetKeyInfo, &SetKeyInfoFlags{})
		cmd.Set("key-info", "delete").MustRunner(pluginCmd.DeleteKeyInfo, &DeleteKeyInfoFlags{})
		return nil
	})
	cmd, expanded, err := subcmd.FromYAMLTemplate(cmdSpec, ext)
	if err != nil {
		panic(fmt.Errorf("failed to parse command expanded spec %s: %w", expanded, err))
	}

	cmd.WithGlobalFlags(subcmd.NewFlagSet().MustRegisterFlagStruct(&globals, nil, nil))

	if err := cmd.AddExtensions(); err != nil {
		panic(fmt.Errorf("failed to add extensions: %w", err))
	}
	cmd.Set("read").MustRunner(pluginCmd.Read, &ReadFlags{})
	cmd.Set("write").MustRunner(pluginCmd.Write, &WriteFlags{})
	return cmd
}

func main() {
	ctx := context.Background()
	subcmd.Dispatch(ctx, cli())
}

type pluginCmd struct{}

type KeychainItemFlag struct {
	KeyChainItem string `subcmd:"keychain-item,,keychain item to use instead of the filename"`
}

type ReadFlags struct {
	plugin.ReadFlags
	KeychainItemFlag
}

type WriteFlags struct {
	plugin.WriteFlags
	KeychainItemFlag
}

func (pluginCmd) getReadFS(ctx context.Context, fl plugin.ReadFlags, item string) (file.ReadWriteFileFS, error) {
	if len(item) == 0 {
		return nil, fmt.Errorf("missing keychain item")
	}
	if fs, ok := file.ReadWriteFSFromContext(ctx); ok {
		return fs, nil
	}
	cfg, err := fl.Config()
	if err != nil {
		return nil, fmt.Errorf("failed to get config from flags: %w", err)
	}
	fs, err := cfg.FS(false)
	if err != nil {
		return nil, err
	}
	if globals.Verbose {
		if _, ok := fs.(*keychain.T); ok {
			fmt.Fprintf(os.Stderr, "accessing keychain directly\n")
		}
		if pfs, ok := fs.(*plugins.FS); ok {
			fmt.Fprintf(os.Stderr, "using plugin at: %s\n", pfs.PluginPath())
		}
	}
	return fs, nil
}

func (pluginCmd) getWriteFS(ctx context.Context, fl plugin.WriteFlags, item string) (file.ReadWriteFileFS, error) {
	if len(item) == 0 {
		return nil, fmt.Errorf("missing keychain item")
	}
	if fs, ok := file.ReadWriteFSFromContext(ctx); ok {
		return fs, nil
	}
	cfg, err := fl.Config()
	if err != nil {
		return nil, fmt.Errorf("failed to get config from flags: %w", err)
	}
	fs, err := cfg.FS(true)
	if err != nil {
		return nil, err
	}
	if globals.Verbose {
		if _, ok := fs.(*keychain.T); ok {
			fmt.Fprintf(os.Stderr, "accessing keychain directly\n")
		}
		if pfs, ok := fs.(*plugins.FS); ok {
			fmt.Fprintf(os.Stderr, "using plugin at: %s\n", pfs.PluginPath())
		}
	}
	return fs, nil
}

func (c pluginCmd) Read(ctx context.Context, f any, args []string) error {
	fl := f.(*ReadFlags)
	fs, err := c.getReadFS(ctx, fl.ReadFlags, fl.KeyChainItem)
	if err != nil {
		return err
	}
	err = keyscmd.SafeWriteToLocal(ctx, fs, fl.KeyChainItem, args[0], 0600)
	return handleError(err)
}

func (c pluginCmd) Write(ctx context.Context, f any, args []string) error {
	fl := f.(*WriteFlags)
	fs, err := c.getWriteFS(ctx, fl.WriteFlags, fl.KeyChainItem)
	if err != nil {
		return err
	}
	err = keyscmd.ReadFromLocal(ctx, args[0], fs, fl.KeyChainItem, 0600)
	return handleError(err)
}

type CreateKeyInfoFlags struct {
	keyscmd.SecretConfigFlags
}

func (pluginCmd) Create(ctx context.Context, f any, args []string) error {
	fv := f.(*CreateKeyInfoFlags)
	cfg := fv.SecretConfig()
	ki, err := cfg.New()
	if err != nil {
		return fmt.Errorf("failed to generate new key: %w", err)
	}
	return keyscmd.SafeWriteKeyInfoJSON(ctx, ki, args[0], 0600)
}

type ListKeysFlags struct {
	plugin.ReadFlags
	KeychainItemFlag
}

func (c pluginCmd) ListKeys(ctx context.Context, f any, _ []string) error {
	fl := f.(*ListKeysFlags)
	fs, err := c.getReadFS(ctx, fl.ReadFlags, fl.KeyChainItem)
	if err != nil {
		return err
	}
	kr := keyscmd.NewKeyReader(fs)
	ki, err := kr.GetKeys(ctx, fl.KeyChainItem)
	if err != nil {
		return handleError(fmt.Errorf("failed to get key info from %s: %w", fl.KeyChainItem, err))
	}
	for _, k := range ki {
		fmt.Printf("%s - %s\n", k.String(), k.Token().LastN(4))
		k.Token().Clear()
	}
	return nil
}

type GetKeyInfoFlags struct {
	plugin.ReadFlags
	KeychainItemFlag
	keyscmd.KeySpecFlags
}

func (c pluginCmd) GetKeyInfo(ctx context.Context, f any, args []string) error {
	fl := f.(*GetKeyInfoFlags)
	fs, err := c.getReadFS(ctx, fl.ReadFlags, fl.KeyChainItem)
	if err != nil {
		return err
	}
	kr := keyscmd.NewKeyReader(fs)
	ki, err := kr.GetKey(ctx, fl.KeyChainItem, fl.KeySpec())
	if err != nil {
		return handleError(fmt.Errorf("failed to get key info from %s: %w", fl.KeyChainItem, err))
	}
	err = keyscmd.SafeWriteKeyInfoJSON(ctx, ki, args[0], 0600)
	return handleError(err)
}

type SetKeyInfoFlags struct {
	plugin.WriteFlags
	KeychainItemFlag
}

func (c pluginCmd) SetKeyInfo(ctx context.Context, f any, args []string) error {
	fl := f.(*SetKeyInfoFlags)
	fs, err := c.getWriteFS(ctx, fl.WriteFlags, fl.KeyChainItem)
	if err != nil {
		return err
	}
	ki, err := keyscmd.ReadKeyInfoFromLocalJSON(ctx, args[0])
	if err != nil {
		return handleError(fmt.Errorf("failed to read key info from %s: %w", args[0], err))
	}
	kw := keyscmd.NewKeyWriter(fs)
	if err := kw.SetKeys(ctx, fl.KeyChainItem, fl.UpdateInPlace, ki); err != nil {
		return handleError(fmt.Errorf("failed to write key info to %s: %w", fl.KeyChainItem, err))
	}
	return nil
}

type DeleteKeyInfoFlags struct {
	plugin.WriteFlags
	KeychainItemFlag
	keyscmd.KeySpecFlags
}

func (c pluginCmd) DeleteKeyInfo(ctx context.Context, f any, _ []string) error {
	fl := f.(*DeleteKeyInfoFlags)
	fs, err := c.getWriteFS(ctx, fl.WriteFlags, fl.KeyChainItem)
	if err != nil {
		return err
	}
	kw := keyscmd.NewKeyWriter(fs)
	if err := kw.DeleteKey(ctx, fl.KeyChainItem, fl.KeySpec()); err != nil {
		return handleError(fmt.Errorf("failed to delete key info from %s: %w", fl.KeyChainItem, err))
	}
	return nil
}

func handleError(err error) error {
	if err == nil {
		return nil
	}
	pluginErr := plugins.AsError(err)
	if pluginErr == nil {
		return err
	}
	name := filepath.Base(os.Args[0])
	fmt.Fprintf(os.Stderr, "%s: plugin error: %s: %s\n", name, pluginErr.Message, pluginErr.Detail)
	if pluginErr.Stderr != "" {
		fmt.Fprintf(os.Stderr, "%s: plugin stderr: %s\n", name, pluginErr.Stderr)
	}
	return err
}
