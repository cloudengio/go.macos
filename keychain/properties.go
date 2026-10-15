// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package keychain

// Properties are the attributes of a keychain item that SetProperties can
// change. An empty field leaves the attribute it names as it is; there is no
// way to clear an attribute, only to replace it.
//
// Accessibility is named by one of the values that ParseAccessibility
// accepts. It applies only to items in the data-protection and iCloud
// keychains; an item in the file-based keychain has no such attribute, its
// access being governed by an access control list instead, and the keychain
// silently ignores an attempt to set one. SetProperties reports that as an
// error rather than appearing to have made a change that it has not.
type Properties struct {
	Account       string `json:"account,omitempty" yaml:"account,omitempty"`
	Label         string `json:"label,omitempty" yaml:"label,omitempty"`
	Description   string `json:"description,omitempty" yaml:"description,omitempty"`
	Accessibility string `json:"accessibility,omitempty" yaml:"accessibility,omitempty"`
}

// Empty returns true if no property is to be changed.
func (p Properties) Empty() bool {
	return len(p.Account) == 0 && len(p.Label) == 0 &&
		len(p.Description) == 0 && len(p.Accessibility) == 0
}
