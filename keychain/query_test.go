// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package keychain_test

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"cloudeng.io/macos/keychain"
)

func TestFilterMatches(t *testing.T) {
	item := keychain.Attributes{
		Service:     "cloudeng.io-service",
		Account:     "alice",
		Label:       "my-label",
		Description: keychain.SecureNoteDescription,
		Comment:     "a comment",
	}
	re := regexp.MustCompile

	for _, tc := range []struct {
		name   string
		filter keychain.Filter
		want   bool
	}{
		{"zero filter matches everything", keychain.Filter{}, true},
		{"service", keychain.Filter{Service: re("^cloudeng.io-")}, true},
		{"service mismatch", keychain.Filter{Service: re("^other")}, false},
		{"account", keychain.Filter{Account: re("ali")}, true},
		{"account mismatch", keychain.Filter{Account: re("^bob$")}, false},
		{"label", keychain.Filter{Label: re("label$")}, true},
		{"description", keychain.Filter{Description: re("secure")}, true},
		{"comment", keychain.Filter{Comment: re("comment")}, true},
		{"case insensitive", keychain.Filter{Account: re("(?i)ALICE")}, true},
		{"all criteria match", keychain.Filter{
			Service: re("service"), Account: re("alice"), Label: re("label"),
		}, true},
		{"criteria are anded", keychain.Filter{
			Service: re("service"), Account: re("bob"),
		}, false},
		{"any matches the service", keychain.Filter{Any: re("cloudeng.io")}, true},
		{"any matches the account", keychain.Filter{Any: re("alice")}, true},
		{"any matches the comment", keychain.Filter{Any: re("a comment")}, true},
		{"any matches nothing", keychain.Filter{Any: re("not-present")}, false},
		{"any is anded with the per attribute criteria", keychain.Filter{
			Any: re("alice"), Service: re("^other"),
		}, false},
		{"secure notes only", keychain.Filter{SecureNotesOnly: true}, true},
	} {
		if got := tc.filter.Matches(item); got != tc.want {
			t.Errorf("%v: got %v, want %v", tc.name, got, tc.want)
		}
	}

	// An item that is not a secure note.
	password := item
	password.Description = "application password"
	if got, want := (keychain.Filter{SecureNotesOnly: true}).Matches(password), false; got != want {
		t.Errorf("secure notes only matched a %q item: got %v, want %v", password.Description, got, want)
	}
	if got, want := (keychain.Filter{}).Matches(password), true; got != want {
		t.Errorf("zero filter: got %v, want %v", got, want)
	}
	if got, want := item.IsSecureNote(), true; got != want {
		t.Errorf("IsSecureNote: got %v, want %v", got, want)
	}
	if got, want := password.IsSecureNote(), false; got != want {
		t.Errorf("IsSecureNote: got %v, want %v", got, want)
	}
}

// TestAttributesHaveNoContents guards the contract that a query reports
// attributes but never item contents: Attributes must not acquire a field
// that can carry an item's data.
func TestAttributesHaveNoContents(t *testing.T) {
	for _, name := range []string{"Data", "Contents", "Note", "Secret", "Value"} {
		if _, ok := fieldByName(keychain.Attributes{}, name); ok {
			t.Errorf("keychain.Attributes has a %v field: a query must never return item contents", name)
		}
	}
}

func fieldByName(v any, name string) (any, bool) {
	rt := reflect.TypeOf(v)
	f, ok := rt.FieldByName(name)
	if !ok {
		return nil, false
	}
	return f, true
}

// TestFilterSpecCompile verifies that a spec compiles to the filter it
// describes, and that an expression that cannot be compiled is reported
// against the attribute it applies to.
func TestFilterSpecCompile(t *testing.T) {
	spec := keychain.FilterSpec{
		Any:             "any",
		Service:         "svc",
		Account:         "acct",
		Label:           "lbl",
		Description:     "desc",
		Comment:         "cmt",
		SecureNotesOnly: true,
	}
	filter, err := spec.Compile()
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	for _, tc := range []struct {
		name string
		re   *regexp.Regexp
		want string
	}{
		{"any", filter.Any, "any"},
		{"service", filter.Service, "svc"},
		{"account", filter.Account, "acct"},
		{"label", filter.Label, "lbl"},
		{"description", filter.Description, "desc"},
		{"comment", filter.Comment, "cmt"},
	} {
		if tc.re == nil {
			t.Errorf("%v: not compiled", tc.name)
			continue
		}
		if got := tc.re.String(); got != tc.want {
			t.Errorf("%v: got %v, want %v", tc.name, got, tc.want)
		}
	}
	if !filter.SecureNotesOnly {
		t.Error("SecureNotesOnly was not carried over")
	}

	// A zero spec compiles to a filter that matches everything.
	filter, err = keychain.FilterSpec{}.Compile()
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !filter.Matches(keychain.Attributes{}) {
		t.Error("a zero spec compiled to a filter that matches nothing")
	}

	// An invalid expression names its attribute, so that a caller whose
	// flags share those names can report which flag is at fault.
	for _, tc := range []struct {
		spec keychain.FilterSpec
		want string
	}{
		{keychain.FilterSpec{Any: "[["}, "any: invalid regular expression"},
		{keychain.FilterSpec{Service: "[["}, "service: invalid regular expression"},
		{keychain.FilterSpec{Account: "*"}, "account: invalid regular expression"},
		{keychain.FilterSpec{Label: "("}, "label: invalid regular expression"},
		{keychain.FilterSpec{Description: "("}, "description: invalid regular expression"},
		{keychain.FilterSpec{Comment: "("}, "comment: invalid regular expression"},
		{keychain.FilterSpec{Service: strings.Repeat("a", keychain.MaxRegexpLength+1)}, "service: regular expression exceeds maximum length"},
	} {
		_, err := tc.spec.Compile()
		if err == nil {
			t.Errorf("%+v: got nil error, want an error", tc.spec)
			continue
		}
		if got := err.Error(); !strings.Contains(got, tc.want) {
			t.Errorf("got %q, want it to contain %q", got, tc.want)
		}
	}
}

func BenchmarkFilterMatches(b *testing.B) {
	spec := keychain.FilterSpec{
		Any:         "alice",
		Service:     "svc",
		Account:     "acct",
		Label:       "lbl",
		Description: "desc",
		Comment:     "cmt",
	}
	filter, err := spec.Compile()
	if err != nil {
		b.Fatalf("Compile: %v", err)
	}
	attrs := keychain.Attributes{
		Service:     "svc-foo",
		Account:     "acct-bar",
		Label:       "lbl-baz",
		Description: "desc-qux",
		Comment:     "cmt-corge",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = filter.Matches(attrs)
	}
}
