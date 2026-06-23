// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

// Usage: gobundle --help|run|build|install|... [options]
//
//	gobundle makes it easy (and transparent) to create go applications as macOS
//	bundles and hence to be signed with entitlements and with embedded provisioning
//	profiles. It is intended to ease the development flow whilst working with the
//	macOS security model (and without having to disable security and sandboxing on
//	development machines).
//
//	gobundle wraps the go command, and in particular 'go run', 'go build' and 'go
//	install' to build the go executable and to transparently build a macOS bundle
//	using the executable. gobundle run|build|install <args> invokes go run|build|install
//	with <args>. For run it uses go run's -exec hook to capture the created executable
//	and to build an app bundle in a temporary directly and to run the executable
//	from the bundle. For build and install it creates the app bundle, moves the
//	executable into the bundle and creates a soft link to the executable in the app
//	bundle at the location at which go build would have left the executable. Executing
//	the soft link is then equivalent to executing the executable in the bundle. For
//	build and install it uses the same heuristics as the go command to determine
//	the name and location of the executable.
//
//	gobundle is configured using environment variables and YAML configuration files
//	that use the types in cloudeng.io/macos/buildtools.
//	The config file format is as follows:
//
//	    identity:       - signing identity
//	    codesign-args:  - array of additional arguments to codesign
//	    bundle:         - path to the app bundle to create, if empty <binary>.app is used
//	    profile:        - path to the provisioning profile to embed in the app bundle,
//	                      it can include environment variables
//	    entitlements:   - a dictionary of entitlements to embed in the app
//	    info.plist:     - a dictionary of fields that correspond to info.Plist entries.
//
//	For example:
//	    identity: "Apple Development: You (Your Team ID)"
//	    entitlements:
//	        com.apple.security.app-sandbox: true
//	    profile: $HOME/Downloads/example_app.provisionprofile
//	    info.plist:
//	        CFBundleIdentifier: <your-team-id>com.you.example
//
//	Environment variables can be used in any string value and are expanded before use.
//	This makes it possible to hide sensitive information such as a signing identity
//	from checked in files.
//
//
//	To make managing shared configurations easier, two config files of the same
//	format are used. One is intended to be shared across multiple apps and the other
//	is intended to be app specific. The app specific file is merged with the shared
//	one, overriding any duplicate keys. A common convention is to have signing
//	information in the shared file and app specific information such as the bundle
//	identifier and the provisioning profile in the app specific file. The merged
//	file is written to the app bundle as Resources/gobundle.yml.
//	The shared file is searched for in the following locations:
//	 1. The file specified by the environment variable GOBUNDLE_SHARED_CONFIG
//	 2. The current directory for files named: .gobundle-shared.yaml
//	    or gobundle-shared.yml
//	 3. The user's home directory for files named: .gobundle-shared.yaml
//	    or gobundle-shared.yml
//
//	The app specific file is searched for in the following locations:
//	 1. The file specified by the environment variable GOBUNDLE_APP_CONFIG
//	 2. The current directory for files named: .gobundle-app.yml
//	    or gobundle-app.yml
//	 3. The user's home directory for files named: .gobundle-app.yaml
//	    or gobundle-app.yml
//
//	In all cases .yaml may be used instead of .yml.
//
//	In addition, setting GOBUNDLE_VERBOSE to any non-empty value will enable verbose logging.
//
//	Examples:
//	  gobundle build ./cmd/myapp
//	  gobundle run ./cmd/myapp
//	  gobundle install ./cmd/myapp
package main
