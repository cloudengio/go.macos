// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package macosutils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"howett.net/plist"
)

// ErrNotInBundle is returned by EnclosingBundle for an executable that is not
// in an app bundle.
var ErrNotInBundle = errors.New("not in an app bundle")

// EnclosingBundle returns the app bundle that contains the running
// executable, either as its main executable, in Contents/MacOS, or as a
// helper, in Contents/Helpers, which is where a bundle keeps the other
// executables that it runs, such as native messaging hosts and daemons. It
// returns ErrNotInBundle if the executable is in neither.
//
// It inherits the caveat documented on ProcessInBundle for a sandboxed
// process launched with a relative path.
func EnclosingBundle() (string, error) {
	exe, err := ExecutablePath()
	if err != nil {
		return "", err
	}
	for _, dir := range []string{"MacOS", "Helpers"} {
		if bundle, ok := InBundle(exe, "Contents", dir); ok {
			return bundle, nil
		}
	}
	return "", ErrNotInBundle
}

// BundleInfo is the Info.plist of an app bundle, read and parsed once, so
// that any number of its values can be looked up.
type BundleInfo struct {
	path   string
	values map[string]any
}

// ReadBundleInfo reads and parses the Contents/Info.plist of the app bundle,
// or other bundle, such as an .appex, at bundle. Text and binary property
// lists are both read.
func ReadBundleInfo(bundle string) (*BundleInfo, error) {
	path := filepath.Join(bundle, "Contents", "Info.plist")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var values map[string]any
	if _, err := plist.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("parse %v: %w", path, err)
	}
	return &BundleInfo{path: path, values: values}, nil
}

// Path returns the path of the Info.plist.
func (b *BundleInfo) Path() string {
	return b.path
}

// Value returns the value of key, and whether there is one.
func (b *BundleInfo) Value(key string) (any, bool) {
	v, ok := b.values[key]
	return v, ok
}

// String returns the value of key, which must be a string. It is an error
// for the key to be absent or not a string; an empty string is returned as
// is.
func (b *BundleInfo) String(key string) (string, error) {
	v, ok := b.values[key]
	if !ok {
		return "", fmt.Errorf("%v has no %v", b.path, key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%v in %v is a %T, not a string", key, b.path, v)
	}
	return s, nil
}
