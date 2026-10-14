// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"text/tabwriter"

	"cloudeng.io/cmdutil/flags"
	"cloudeng.io/macos/keychain"
	"cloudeng.io/macos/keychain/plugin"
)

// DescribeOptionsFlags are the flags for the 'describe-options' command,
// which takes none.
type DescribeOptionsFlags struct{}

// keychainTypeDescriptions describes what each keychain is, keyed by the
// value that names it. The values themselves come from the enum, so a
// keychain that is added without being described here still appears.
var keychainTypeDescriptions = map[string]string{
	"file":                  "the legacy, local only, file based keychain; its items are governed by an access control list",
	"data-protection-local": "local, integrated with the system's secure enclave; requires a signed binary holding the keychain-access-groups entitlement",
	"icloud":                "synchronized across devices; requires a signed binary holding the keychain-access-groups entitlement",
	"all":                   "search every keychain; it can be used for reading and querying but not for writing",
}

// DescribeOptions lists the values accepted by the flags that take one of a
// fixed set, ie. the keychain types and accessibilities, along with the
// default that each flag takes when it is not specified.
func (pluginCmd) DescribeOptions(_ context.Context, _ any, _ []string) error {
	return describeOptions(os.Stdout)
}

func describeOptions(out io.Writer) error {
	fmt.Fprintf(out, "keychain types\n")
	tw := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
	for _, value := range enumValues(keychain.Type(0)) {
		fmt.Fprintf(tw, "  %s\t%s\n", value, keychainTypeDescriptions[value])
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(out, "\n  used by --keychain-type, which defaults to %v when reading or querying, and to %v when writing or changing an item.\n",
		flagDefault(&plugin.ReadFlags{}, "keychain-type"),
		flagDefault(&plugin.WriteFlags{}, "keychain-type"))

	fmt.Fprintf(out, "\naccessibility, ie. when an item's contents can be decrypted\n")
	for _, value := range enumValues(keychain.Accessibility(0)) {
		fmt.Fprintf(out, "  %s\n", value)
	}
	fmt.Fprintf(out, "\n  used by --keychain-accessibility when writing an item, which defaults to %v, and by\n",
		flagDefault(&plugin.WriteFlags{}, "keychain-accessibility"))
	fmt.Fprintf(out, "  --set-accessibility, which leaves the item's accessibility alone unless it is specified.\n")
	fmt.Fprintf(out, "  It applies to the %v and %v keychains only; an item in the %v keychain has no such\n",
		keychain.KeychainDataProtectionLocal, keychain.KeychainICloud, keychain.KeychainFileBased)
	fmt.Fprintf(out, "  attribute, its access being governed by an access control list instead.\n")

	fmt.Fprintf(out, "\noutput formats\n")
	for _, value := range []string{formatText, formatJSON} {
		fmt.Fprintf(out, "  %s\n", value)
	}
	fmt.Fprintf(out, "\n  used by the --format flag of apple-keychain query, which defaults to %v.\n",
		flagDefault(&AppleKeychainQueryFlags{}, "format"))
	return nil
}

// enumValues returns the values that name each member of an enum, sorted,
// taken from the enum itself so that they cannot disagree with what the
// flags accept.
func enumValues[T interface{ EnumValues() map[string]T }](zero T) []string {
	values := slices.Collect(maps.Keys(zero.EnumValues()))
	slices.Sort(values)
	return values
}

// flagDefault returns the default value of the named flag of fl, which must
// be a pointer to a struct whose fields carry subcmd tags. It is read from
// the flag itself so that it cannot disagree with what the commands do.
func flagDefault(fl any, name string) string {
	fs := flag.NewFlagSet("describe-options", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := flags.RegisterFlagsInStruct(fs, "subcmd", fl, nil, nil); err != nil {
		return "unknown"
	}
	f := fs.Lookup(name)
	if f == nil {
		return "unknown"
	}
	if len(f.DefValue) == 0 {
		return "an empty value"
	}
	return f.DefValue
}
