// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package keychain

import (
	"fmt"
	"regexp"
	"time"
)

// SecureNoteDescription is the description attribute used by
// WriteSecureNote for the items that it creates. Secure notes created by
// the macOS Keychain Access application use the same description and are
// hence matched by Filter.SecureNotesOnly also.
const SecureNoteDescription = "secure note"

// Attributes describes a single keychain item: the attributes that identify
// and describe it, but never its contents. Query never asks the keychain for
// item data and hence Attributes has no field to report it in.
type Attributes struct {
	Keychain    Type   `json:"keychain" yaml:"keychain"`
	Service     string `json:"service" yaml:"service"`
	Account     string `json:"account" yaml:"account"`
	Label       string `json:"label,omitempty" yaml:"label,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Comment     string `json:"comment,omitempty" yaml:"comment,omitempty"`
	AccessGroup string `json:"access_group,omitempty" yaml:"access_group,omitempty"`

	// Accessibility names when the item's contents can be decrypted, eg.
	// when-unlocked, as one of the values accepted by ParseAccessibility. It
	// is recorded for items in the data-protection and iCloud keychains; an
	// item in the file-based keychain, whose access is governed by an access
	// control list rather than by this attribute, reports an empty value.
	Accessibility string `json:"accessibility,omitempty" yaml:"accessibility,omitempty"`

	Created  time.Time `json:"created,omitzero" yaml:"created,omitempty"`
	Modified time.Time `json:"modified,omitzero" yaml:"modified,omitempty"`
}

// IsSecureNote returns true if the item's description identifies it as a
// secure note.
func (a Attributes) IsSecureNote() bool {
	return a.Description == SecureNoteDescription
}

// Filter selects keychain items by matching regular expressions against
// their attributes. All of the criteria that are set must match, ie. they
// are and'ed together, and a zero Filter matches every item. The
// expressions are unanchored and case sensitive, use ^, $ and (?i) as
// required.
type Filter struct {
	// Any matches an item if any one of its service, account, label,
	// description or comment attributes matches.
	Any         *regexp.Regexp
	Service     *regexp.Regexp
	Account     *regexp.Regexp
	Label       *regexp.Regexp
	Description *regexp.Regexp
	Comment     *regexp.Regexp

	// SecureNotesOnly restricts the results to secure notes, ie. to items
	// whose description is SecureNoteDescription.
	SecureNotesOnly bool
}

// FilterSpec is the uncompiled, and hence serializable, form of a Filter.
// A query that is serviced by an out-of-process keychain plugin is sent as a
// FilterSpec, the plugin compiling the expressions itself since a compiled
// Filter cannot cross a process boundary.
type FilterSpec struct {
	Any             string `json:"any,omitempty" yaml:"any,omitempty"`
	Service         string `json:"service,omitempty" yaml:"service,omitempty"`
	Account         string `json:"account,omitempty" yaml:"account,omitempty"`
	Label           string `json:"label,omitempty" yaml:"label,omitempty"`
	Description     string `json:"description,omitempty" yaml:"description,omitempty"`
	Comment         string `json:"comment,omitempty" yaml:"comment,omitempty"`
	SecureNotesOnly bool   `json:"secure_notes_only,omitempty" yaml:"secure_notes_only,omitempty"`
}

// Compile compiles the spec's regular expressions into a Filter. An
// expression that cannot be compiled is reported as an error that names the
// attribute it applies to, ie. one of any, service, account, label,
// description or comment; the names are those of the spec's fields,
// lowercased, so that a caller whose flags are named after them can report
// the error against the offending flag.
func (s FilterSpec) Compile() (Filter, error) {
	filter := Filter{SecureNotesOnly: s.SecureNotesOnly}
	for _, c := range []struct {
		attribute string
		expr      string
		re        **regexp.Regexp
	}{
		{"any", s.Any, &filter.Any},
		{"service", s.Service, &filter.Service},
		{"account", s.Account, &filter.Account},
		{"label", s.Label, &filter.Label},
		{"description", s.Description, &filter.Description},
		{"comment", s.Comment, &filter.Comment},
	} {
		if len(c.expr) == 0 {
			continue
		}
		re, err := regexp.Compile(c.expr)
		if err != nil {
			return Filter{}, fmt.Errorf("%v: invalid regular expression %q: %w", c.attribute, c.expr, err)
		}
		*c.re = re
	}
	return filter, nil
}

// Matches returns true if attrs satisfies every criterion set in the filter.
func (f Filter) Matches(attrs Attributes) bool {
	if f.SecureNotesOnly && !attrs.IsSecureNote() {
		return false
	}
	for _, c := range []struct {
		re    *regexp.Regexp
		value string
	}{
		{f.Service, attrs.Service},
		{f.Account, attrs.Account},
		{f.Label, attrs.Label},
		{f.Description, attrs.Description},
		{f.Comment, attrs.Comment},
	} {
		if c.re != nil && !c.re.MatchString(c.value) {
			return false
		}
	}
	if f.Any == nil {
		return true
	}
	for _, v := range []string{
		attrs.Service, attrs.Account, attrs.Label, attrs.Description, attrs.Comment,
	} {
		if f.Any.MatchString(v) {
			return true
		}
	}
	return false
}
