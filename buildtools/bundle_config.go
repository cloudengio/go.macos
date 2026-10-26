// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools

import "fmt"

// BundleConfig is the configuration common to the tools that build a macOS
// application bundle, to be read from YAML and embedded, inline, in the
// configuration of each of them:
//
//	bundle: build/example.app
//	info_plist:
//	  CFBundleIdentifier: com.example.app
//	signing:
//	  identity: "Developer ID Application: Example (TEAMID)"
//	  entitlements:
//	    keychain-access-groups: [TEAMID.com.example.app]
//	notary:
//	  keychain_profile: example-notary
//	permissions:
//	  executable: 0755
//	provisioning_profile: example.provisionprofile
//
// See BundleBuilder for creating the bundle from it.
type BundleConfig struct {
	// Bundle is the path of the application bundle to create.
	Bundle string `yaml:"bundle"`
	// Info holds the fields of the bundle's Info.plist.
	Info InfoPlist `yaml:"info_plist"`
	// Signing holds the identity, entitlements and arguments for code signing.
	// The bundle is not signed if there is no identity.
	Signing SigningConfig `yaml:"signing"`
	// Notary holds the credentials used to notarize the signed bundle with
	// Apple's notarization service, when that is asked for.
	Notary NotaryConfig `yaml:"notary"`
	// Permissions are the permissions for the executable and for the MacOS
	// directory of the bundle.
	Permissions PermissionsConfig `yaml:"permissions,omitempty"`
	// ProvisioningProfile is the path of the profile to embed in the bundle as
	// Contents/embedded.provisionprofile. It is required for entitlements that
	// need to be authorized by it, eg. keychain access groups.
	ProvisioningProfile string `yaml:"provisioning_profile"`
}

// AppBundle returns the bundle described by the configuration.
func (c BundleConfig) AppBundle() AppBundle {
	return AppBundle{Path: c.Bundle, Info: c.Info}
}

// Validate checks that the configuration is consistent: entitlements that are
// restricted, see Entitlements.Restricted, are only authorized by an embedded
// provisioning profile.
func (c BundleConfig) Validate() error {
	if c.Signing.Configured() && c.ProvisioningProfile == "" && c.Signing.Entitlements.Restricted() {
		return fmt.Errorf("signing with restricted entitlements requires a provisioning profile: set provisioning_profile in the configuration")
	}
	return nil
}
