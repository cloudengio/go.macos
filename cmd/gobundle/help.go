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

    bundle:               - path to the app bundle to create, if empty <binary>.app is used
    info_plist:           - a dictionary of fields that correspond to Info.plist entries.
    signing:
        identity:             - signing identity
        codesign-args:        - array of additional arguments to codesign
        entitlements:         - a dictionary of entitlements to embed in the app
        perfile_entitlements: - entitlements for individual files in the bundle
    provisioning_profile: - path to the provisioning profile to embed in the app bundle,
                            it can include environment variables. It is required
                            for entitlements that it authorizes, such as
                            keychain-access-groups.
    permissions:          - executable and macos_dir permissions for the app.
    icon:                 - a png file from which to create the icon of the app.
    notary:               - notarization credentials: keychain_profile (a notarytool
                            stored profile), or apple_id/team_id/password. Passwords
                            should use @keychain:<item> or @env:<VAR> to avoid exposing
                            secrets in process tables. Notarization is requested with
                            the --notarize flag, which is only applied by 'install'
                            (notarization is slow and only matters for distribution);
                            'build' and 'run' still embed the profile, so entitlements
                            work locally. It requires an identity and notary credentials.

For example:
    signing:
        identity: "Developer ID Application: You (Your Team ID)"
        entitlements:
            com.apple.security.app-sandbox: true
    provisioning_profile: $HOME/Downloads/example_app.provisionprofile
    notary:
        keychain_profile: my-notary-profile
    info_plist:
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
The files are named by the --config flag, which takes a comma separated
list, may be repeated, and replaces the defaults. Their type is determined by
their suffix: .yaml and .yml files are the configuration files, applied in
the order given, and a .env file is the environment variables file, of which
at most one is used. The defaults are gobundle-shared.yaml, gobundle-app.yaml
and gobundle-env.env.

Each file is searched for in the following locations, in order:
 1. The current directory
 2. The gobundle directory within the user's configuration directory, ie.
    $HOME/Library/Application Support/gobundle on macOS
 3. $HOME/.config/gobundle

In each directory a name is looked for as it is and with a leading '.', eg.
gobundle-shared.yaml and .gobundle-shared.yaml, where .yml may be used
instead of .yaml for a name that has no suffix. An absolute path is used
exactly as given.

If --config does not name any file of a type, the environment variable
GOBUNDLE_CONFIG, a comma separated list of YAML files, or GOBUNDLE_ENV_CONFIG,
the environment variables file, is used if it is set, and the defaults
otherwise.

The defaults are optional: those that are not found are ignored, and if
there is no environment variables file the values come from the shell
environment alone. A file that is named, by --config or by an environment
variable, and is not found is an error rather than being silently ignored.

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
