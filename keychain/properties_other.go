// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build !darwin

package keychain

import "errors"

// Validate returns an error indicating that setting properties is not
// implemented on this platform.
func (p Properties) Validate(_ Type) error {
	return errors.New("not implemented on this platform")
}

// SetProperties returns an error indicating that setting properties is not
// implemented on this platform.
func (T) SetProperties(_ string, _ Properties) error {
	return errors.New("not implemented on this platform")
}
