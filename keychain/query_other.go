// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build !darwin

package keychain

import "errors"

// Query returns the attributes of every item in the requested keychain(s)
// that matches filter, for all accounts.
func Query(_ Type, _ Filter, _ ...Option) ([]Attributes, error) {
	return nil, errors.New("not implemented on this platform")
}

// Query returns the attributes of the items in the keychain that match
// filter.
func (T) Query(_ Filter) ([]Attributes, error) {
	return nil, errors.New("not implemented on this platform")
}
