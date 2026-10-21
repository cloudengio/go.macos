// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"strings"

	"cloudeng.io/text/linewrap"
)

const usage = `Usage: gobundle --help|run|build|install|... [options]`

const para1 = `gobundle makes it easy (and transparent) to create go applications
as macOS bundles and hence to be signed with entitlements and
with embedded provisioning profiles.
It is intended to ease the development flow whilst working with the
macOS security model (and without having
to disable security and sandboxing on development machines).
`

const para2 = `
gobundle wraps the go command, and in particular 'go run', 'go build'
and 'go install' to build the go executable and to
transparently build a macOS bundle using the executable.
gobundle run|build|install <args> invokes go run|build|install with <args>.
For run it uses go run's -exec hook to capture the created executable
and to build an app bundle in a temporary directly and to run the
executable from the bundle.
For build and install it creates the app bundle,
moves the executable into the bundle and
creates a soft link to the executable in the app bundle at the location
at which go build would have left the executable.
Executing the soft link is then equivalent to executing the executable in the bundle.
For build and install it uses the same heuristics as the go command to determine the name and location of the executable.
`

const para3 = `
gobundle is configured using environment variables and YAML configuration files that use the types in cloudeng.io/macos/buildtools.
`

const para4 = `
The config file format is as follows:

    identity:       - signing identity
    codesign-args:  - array of additional arguments to codesign
    bundle:         - path to the app bundle to create, if empty <binary>.app is used
    profile:        - path to the provisioning profile to embed in the app bundle,
                      it can include environment variables
    entitlements:   - a dictionary of entitlements to embed in the app
    info.plist:     - a dictionary of fields that correspond to info.Plist entries.
    notarize:       - if true, notarize and staple the signed bundle so it opens
                      on other Macs. Only applied by 'install' (notarization is
                      slow and only matters for distribution); 'build' and 'run'
                      still embed the profile, so entitlements work locally.
                      Requires an identity and notary credentials.
    notary:         - notarization credentials: keychain_profile (a notarytool
                      stored profile), or apple_id/team_id/password. Passwords
                      should use @keychain:<item> or @env:<VAR> to avoid exposing
                      secrets in process tables.

For example:
    identity: "Developer ID Application: You (Your Team ID)"
    entitlements:
        com.apple.security.app-sandbox: true
    profile: $HOME/Downloads/example_app.provisionprofile
    notarize: true
    notary:
        keychain_profile: my-notary-profile
    info.plist:
        CFBundleIdentifier: <your-team-id>com.you.example

Environment variables can be used in any string value and are expanded before use.
This makes it possible to hide sensitive information such as a signing identity
from checked in files.
The values they are expanded with come from the shell environment and, for
any variable the shell does not define, from a file of NAME=VALUE
definitions (see the environment variables file below).
`

const para5 = `
To make managing shared configurations easier, two config files of the
same format are used.
One is intended to be shared across multiple apps and the other is intended
to be app specific.
The app specific file is merged with the shared one, overriding any
duplicate keys.
A common convention is to have signing information in the shared file
and app specific information such as the
bundle identifier and the provisioning profile in the app specific file.
The merged file is written to the app bundle as Resources/gobundle.yml.
`

const para5a = `
A third file supplies the values that variable references in those two
configuration files are expanded with. It is not YAML: it contains
NAME=VALUE definitions in the style of a .env file, one per line, with
optional quoting, 'export' prefixes and # comments.
Its purpose is to keep settings such as a signing identity or a team
identifier in one place, out of the checked in configuration files, without
having to set them in every shell. The shell environment takes precedence
over it, so a definition can be overridden for a single invocation.
`

const para6 = ` 
Each file is searched for in the following locations, in order:
 1. The file named by its environment variable, ie. GOBUNDLE_SHARED_CONFIG,
    GOBUNDLE_APP_CONFIG or GOBUNDLE_ENV_CONFIG
 2. The current directory
 3. The gobundle directory within the user's configuration directory, ie.
    $HOME/Library/Application Support/gobundle on macOS
 4. $HOME/.config/gobundle

The names looked for in each directory are, for the shared file,
gobundle-shared.yaml and .gobundle-shared.yaml, and for the app specific
file, gobundle-app.yaml and .gobundle-app.yaml, where .yml may be used
instead of .yaml. The environment variables file is named gobundle-env.env
or .gobundle-env.env, with a .env suffix rather than a yaml one because its
contents are NAME=VALUE definitions.

A name given by one of the environment variables, or by --shared-config,
--app-config or --env-config, replaces the gobundle part of the names above;
an absolute path is used exactly as given.

The environment variables file is optional: if none is specified and
gobundle-env.env is not found, values come from the shell environment alone.
Asking for one by name that is not there, with --env-config or
GOBUNDLE_ENV_CONFIG, is an error rather than being silently ignored.

The environment variables file is named with a .yaml or .yml extension
because that is what the configuration file search looks for, even though
its contents are NAME=VALUE definitions rather than YAML. A file named
otherwise can be used by giving its absolute path to --env-config or
GOBUNDLE_ENV_CONFIG.

In addition, setting GOBUNDLE_VERBOSE to any non-empty value will enable verbose logging.

Examples:
  gobundle build ./cmd/myapp
  gobundle run ./cmd/myapp
  gobundle install ./cmd/myapp
`

func printHelpAndExit() {
	out := strings.Builder{}
	out.WriteString(usage + "\n\n")
	out.WriteString(linewrap.Block(2, 80, para1))
	out.WriteString(linewrap.Block(2, 80, para2))
	out.WriteString(linewrap.Block(2, 80, para3))
	out.WriteString(linewrap.Verbatim(2, para4))
	out.WriteString(linewrap.Block(2, 80, para5))
	out.WriteString(linewrap.Block(2, 80, para5a))
	out.WriteString(linewrap.Verbatim(2, para6))
	fmt.Fprintln(os.Stderr, out.String())
	os.Exit(1)
}
