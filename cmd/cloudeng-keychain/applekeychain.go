// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"cloudeng.io/macos/keychain"
	"cloudeng.io/macos/keychain/plugin"
	"cloudeng.io/macos/machutils"
	"howett.net/plist"
)

// AppleKeychainQueryFlags are the flags for the 'apple-keychain query'
// command. The service, account, label, description and comment flags are
// regular expressions that are matched against the attribute of the same
// name; all of the criteria supplied must match. The embedded
// plugin.ReadFlags determine which keychain is searched and how the plugin
// that searches it is located; --keychain-account restricts the search to a
// single account, as an exact match made by the keychain itself, whereas
// --account matches accounts by regular expression.
type AppleKeychainQueryFlags struct {
	plugin.ReadFlags
	Service     string `subcmd:"service,,regular expression that an item's service must match"`
	Account     string `subcmd:"account,,regular expression that an item's account must match"`
	Label       string `subcmd:"label,,regular expression that an item's label must match"`
	Description string `subcmd:"description,,regular expression that an item's description must match"`
	Comment     string `subcmd:"comment,,regular expression that an item's comment must match"`
	SecureNotes bool   `subcmd:"secure-notes,false,'restrict the results to secure notes, ie. items whose description is secure note'"`
	Direct      bool   `subcmd:"keychain-direct,false,'query the keychain in-process instead of via the plugin; only the file-based keychain is visible to a binary without the keychain-access-groups entitlement, but unlike the plugin it can be used from a sandboxed binary, which cannot execute a plugin outside its own bundle'"`
	Limit       int    `subcmd:"limit,0,'maximum number of items to display, zero for all of them'"`
	Format      string `subcmd:"format,text,'output format: text or json'"`
	Long        bool   `subcmd:"long,false,'display every attribute of each item, one item per block, rather than one item per line'"`
}

const (
	formatText = "text"
	formatJSON = "json"
)

// filterSpec builds the keychain.FilterSpec described by the flags and the
// optional regular expression argument, which is matched against any of an
// item's attributes. The expressions are compiled here as well as by whoever
// services the query so that an invalid one is reported against the flag it
// came from, and before a plugin is run.
func (fl *AppleKeychainQueryFlags) filterSpec(args []string) (keychain.FilterSpec, error) {
	spec := keychain.FilterSpec{
		Service:         fl.Service,
		Account:         fl.Account,
		Label:           fl.Label,
		Description:     fl.Description,
		Comment:         fl.Comment,
		SecureNotesOnly: fl.SecureNotes,
	}
	// The argument is compiled before the spec is, so that the error for a
	// bad argument names no flag whereas every error from Compile names the
	// flag that its attribute is also the name of.
	if len(args) == 1 && len(args[0]) > 0 {
		if _, err := regexp.Compile(args[0]); err != nil {
			return keychain.FilterSpec{}, fmt.Errorf("invalid regular expression %q: %w", args[0], err)
		}
		spec.Any = args[0]
	}
	if _, err := spec.Compile(); err != nil {
		return keychain.FilterSpec{}, fmt.Errorf("--%w", err)
	}
	return spec, nil
}

func (fl *AppleKeychainQueryFlags) validate() error {
	switch fl.Format {
	case formatText, formatJSON:
	default:
		return fmt.Errorf("--format: unsupported format %q: use %v or %v", fl.Format, formatText, formatJSON)
	}
	if fl.Limit < 0 {
		return fmt.Errorf("--limit: must not be negative: %v", fl.Limit)
	}
	return nil
}

// QueryAppleKeychain searches the macOS keychain for the items whose
// attributes match the requested criteria and displays those attributes.
// Item contents are never read.
//
// The query is serviced by the keychain plugin when one can be found, and
// in-process otherwise, exactly as reading and writing items is. Only the
// plugin holds the keychain-access-groups entitlement, so items in the
// data-protection and iCloud keychains, including those written by this
// command, are invisible to an in-process query; warnUnentitled says so
// rather than leaving the empty result to be puzzled over.
func (pluginCmd) QueryAppleKeychain(ctx context.Context, f any, args []string) error {
	fl := f.(*AppleKeychainQueryFlags)
	if err := fl.validate(); err != nil {
		return err
	}
	spec, err := fl.filterSpec(args)
	if err != nil {
		return err
	}
	attrs, pluginBinary, err := fl.query(ctx, spec)
	if err != nil {
		return handleError(fmt.Errorf("failed to query the %v keychain: %w%v",
			fl.Type.Value, err, sandboxHint(pluginBinary, err)))
	}
	if globals.Verbose {
		if len(pluginBinary) > 0 {
			fmt.Fprintf(os.Stderr, "queried the %v keychain using plugin %v\n", fl.Type.Value, pluginBinary)
		} else {
			fmt.Fprintf(os.Stderr, "queried the %v keychain directly, in-process\n", fl.Type.Value)
		}
	}
	if len(pluginBinary) == 0 {
		warnUnentitled(os.Stderr, fl.Type.Value)
	}
	if fl.Limit > 0 && len(attrs) > fl.Limit {
		attrs = attrs[:fl.Limit]
	}
	return writeAttributes(os.Stdout, attrs, fl.Format, fl.Long)
}

// query runs the query, returning the matching items and the path of the
// plugin that serviced the query, which is empty when the keychain was
// queried in-process.
func (fl *AppleKeychainQueryFlags) query(ctx context.Context, spec keychain.FilterSpec) ([]keychain.Attributes, string, error) {
	cfg, err := fl.Config()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get config from flags: %w", err)
	}
	// An unspecified account searches every account, rather than defaulting
	// to the current user as it does when reading or writing a single item.
	cfg.Account = fl.KeychainFlags.Account
	cfg.Direct = fl.Direct
	return cfg.Query(ctx, spec)
}

// sandboxHint returns an explanation to append to the error from a failed
// query that was to be serviced by a plugin, when the reason it failed is
// that this binary is in the App Sandbox: a sandboxed binary cannot execute
// a plugin that lies outside its own bundle, and reports only that the
// operation is not permitted, or kills the plugin outright.
func sandboxHint(pluginBinary string, err error) string {
	if len(pluginBinary) == 0 || err == nil {
		return ""
	}
	sandboxed, serr := machutils.IsSandboxed()
	if serr != nil || !sandboxed {
		return ""
	}
	return fmt.Sprintf("\n\tthis binary is running in the App Sandbox (%v), which cannot run a plugin that is not within its own bundle;"+
		" either run a build that bundles the plugin, or use --keychain-direct to query the keychain in-process",
		machutils.AppSandboxEntitlement)
}

// entitledKeychains are the keychains that can only be searched by a binary
// holding the keychain-access-groups entitlement.
var entitledKeychains = []keychain.Type{
	keychain.KeychainDataProtectionLocal,
	keychain.KeychainICloud,
}

// warnUnentitled writes a warning to out if an in-process query of typ can
// see nothing because the running binary carries no keychain-access-groups
// entitlement. Whether the entitlement is present is reported by the kernel
// for this process; if that cannot be determined, no warning is written
// since the query itself has not failed.
func warnUnentitled(out io.Writer, typ keychain.Type) {
	if typ != keychain.KeychainAll && !slices.Contains(entitledKeychains, typ) {
		return
	}
	entitlements, err := machutils.Entitlements()
	if err != nil {
		return
	}
	var values map[string]any
	if len(entitlements) > 0 {
		if _, err := plist.Unmarshal(entitlements, &values); err != nil {
			return
		}
	}
	if _, ok := values[keychainAccessGroups]; ok {
		return
	}
	names := make([]string, 0, len(entitledKeychains))
	for _, t := range entitledKeychains {
		names = append(names, t.String())
	}
	fmt.Fprintf(out, "warning: the %v keychains require the %v entitlement, which this binary does not have, so their items are not visible here; install the signed plugin, or name it with --keychain-plugin, to search them\n",
		strings.Join(names, " and "), keychainAccessGroups)
}

// keychainAccessGroups is the entitlement that a binary must hold to access
// items in the data-protection and iCloud keychains.
const keychainAccessGroups = "keychain-access-groups"

func writeAttributes(out io.Writer, attrs []keychain.Attributes, format string, long bool) error {
	switch {
	case format == formatJSON:
		if attrs == nil {
			attrs = []keychain.Attributes{}
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(attrs)
	case long:
		return writeAttributeBlocks(out, attrs)
	}
	return writeAttributeTable(out, attrs)
}

// writeAttributeTable writes one item per line. Nothing at all is written if
// there are no items to display so that the output remains easy to consume
// from a script.
func writeAttributeTable(out io.Writer, attrs []keychain.Attributes) error {
	if len(attrs) == 0 {
		return nil
	}
	tw := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "KEYCHAIN\tSERVICE\tACCOUNT\tLABEL\tDESCRIPTION\tACCESSIBILITY\tCREATED\tMODIFIED")
	for _, a := range attrs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			a.Keychain, a.Service, a.Account, a.Label, a.Description,
			a.Accessibility, formatTime(a.Created), formatTime(a.Modified))
	}
	return tw.Flush()
}

// writeAttributeBlocks writes every attribute of each item, one item per
// block.
func writeAttributeBlocks(out io.Writer, attrs []keychain.Attributes) error {
	for i, a := range attrs {
		if i > 0 {
			fmt.Fprintln(out)
		}
		tw := tabwriter.NewWriter(out, 0, 8, 1, ' ', 0)
		for _, kv := range [][2]string{
			{"keychain", a.Keychain.String()},
			{"service", a.Service},
			{"account", a.Account},
			{"label", a.Label},
			{"description", a.Description},
			{"comment", a.Comment},
			{"access-group", a.AccessGroup},
			{"accessibility", a.Accessibility},
			{"created", formatTime(a.Created)},
			{"modified", formatTime(a.Modified)},
		} {
			fmt.Fprintf(tw, "%s:\t%s\n", kv[0], kv[1])
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}
	return nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format(time.RFC3339)
}
