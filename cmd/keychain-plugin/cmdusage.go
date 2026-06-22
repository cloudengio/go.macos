// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

// Usage: [--help|delete keychain-type account service|read keychain-type account service]
//
// macos-keychain-plugin is a plugin for the macOS keychain.
// To install it 'run go generate' in the go/cmd/keychain-plugin directory
// taking care to set up the appropriate Apple signing
// identity and provisioning profile environment variables required by
// gobundle-app.yml.
package main
