// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package main

import (
	"context"
	"fmt"
	"os"

	"cloudeng.io/cmdutil/flags"
	"cloudeng.io/macos/keychain"
	"cloudeng.io/macos/keychain/plugin"
)

// AppleKeychainSetPropertyFlags are the flags for the 'apple-keychain
// set-property' command. The item to change is identified by the keychain
// holding it, the service named as the command's argument and
// --keychain-account; each --set-<attribute> flag supplies a new value for
// the attribute it names, those not specified being left as they are.
type AppleKeychainSetPropertyFlags struct {
	plugin.KeychainFlags
	Type          flags.Enum[keychain.WriteType] `subcmd:"keychain-type,icloud,'the keychain holding the item: data-protection-local, file or icloud'"`
	Account       string                         `subcmd:"set-account,,new account for the item"`
	Label         string                         `subcmd:"set-label,,new label for the item"`
	Description   string                         `subcmd:"set-description,,new description for the item"`
	Accessibility string                         `subcmd:"set-accessibility,,'new accessibility for the item; it applies to the data-protection-local and icloud keychains only'"`
	Direct        bool                           `subcmd:"keychain-direct,false,'change the item in-process instead of via the plugin; items in the data-protection-local and icloud keychains cannot be changed by a binary without the keychain-access-groups entitlement'"`
}

// properties returns the properties described by the flags.
func (fl *AppleKeychainSetPropertyFlags) properties() (keychain.Properties, error) {
	props := keychain.Properties{
		Account:       fl.Account,
		Label:         fl.Label,
		Description:   fl.Description,
		Accessibility: fl.Accessibility,
	}
	if props.Empty() {
		return keychain.Properties{}, fmt.Errorf("nothing to set: specify at least one of --set-account, --set-label, --set-description or --set-accessibility")
	}
	if len(props.Accessibility) > 0 {
		if _, err := keychain.ParseAccessibility(props.Accessibility); err != nil {
			return keychain.Properties{}, fmt.Errorf("--set-accessibility: %w", err)
		}
	}
	return props, nil
}

// SetAppleKeychainProperty changes the account, accessibility, label or
// description of a single keychain item. The item's contents are neither
// read nor written.
//
// The change is made by the keychain plugin when one can be found, and
// in-process otherwise, exactly as reading, writing and querying items are:
// only the plugin holds the keychain-access-groups entitlement, so an item
// in the data-protection or iCloud keychain cannot be changed in-process.
func (pluginCmd) SetAppleKeychainProperty(ctx context.Context, f any, args []string) error {
	fl := f.(*AppleKeychainSetPropertyFlags)
	props, err := fl.properties()
	if err != nil {
		return err
	}
	service := args[0]
	if len(service) == 0 {
		return fmt.Errorf("missing keychain item")
	}
	pluginBinary, err := fl.setProperties(ctx, service, props)
	if err != nil {
		return handleError(fmt.Errorf("failed to set the properties of %v in the %v keychain: %w%v",
			service, fl.Type.Value, err, sandboxHint(pluginBinary, err)))
	}
	if globals.Verbose {
		if len(pluginBinary) > 0 {
			fmt.Fprintf(os.Stderr, "changed %v in the %v keychain using plugin %v\n", service, fl.Type.Value, pluginBinary)
		} else {
			fmt.Fprintf(os.Stderr, "changed %v in the %v keychain directly, in-process\n", service, fl.Type.Value)
		}
	}
	return nil
}

// setProperties changes the item's properties, returning the path of the
// plugin that made the change, which is empty when it was made in-process.
func (fl *AppleKeychainSetPropertyFlags) setProperties(ctx context.Context, service string, props keychain.Properties) (string, error) {
	cfg := fl.WriteConfig(fl.Type.Value)
	cfg.Direct = fl.Direct
	return cfg.SetProperties(ctx, service, props)
}
