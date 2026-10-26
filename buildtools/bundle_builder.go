// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools

import (
	"fmt"
	"os"
)

// BundleBuilder creates the steps that build and sign an application bundle
// as described by a BundleConfig.
type BundleBuilder struct {
	// Config is the configuration of the bundle.
	Config BundleConfig
	// App is the bundle to create. It is Config.AppBundle() unless the
	// caller has resolved defaults for its path or Info.plist, or both.
	App AppBundle
	// Binary is the executable to copy into the bundle.
	Binary string
	// Notarize requests that the signed bundle is notarized, which requires
	// notary credentials and a Developer ID signing identity.
	Notarize bool
	// Contents are further steps that add to the contents of the bundle, eg.
	// resources and icons. They are run after the executable is copied and
	// before the bundle is signed.
	Contents []Step
}

// Validate checks that the bundle can be built: that the configuration is
// consistent, that a provisioning profile that is configured can be read, and
// that notarization, if requested, is possible.
func (b BundleBuilder) Validate() error {
	if err := b.Config.Validate(); err != nil {
		return err
	}
	if b.Config.ProvisioningProfile != "" {
		if _, err := os.Stat(b.Config.ProvisioningProfile); err != nil {
			return fmt.Errorf("provisioning profile %q is not accessible: %w", b.Config.ProvisioningProfile, err)
		}
	}
	if b.Notarize {
		if !b.Config.Notary.Configured() {
			return fmt.Errorf("notarization requires notary credentials: set a 'notary' section in the configuration")
		}
		return b.Config.Notary.ValidateSigning(b.Config.Signing)
	}
	return nil
}

// Steps returns the steps to build the bundle after validating it, in the order
// they must be run: the bundle is created and its Info.plist, the executable,
// the provisioning profile and the Contents are added to it, the permissions
// are set, the bundle is signed, if there is a signing identity, and
// then notarized, if requested. Everything that changes the contents of the
// bundle precedes signing, which any later change would invalidate.
func (b BundleBuilder) Steps() ([]Step, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	var steps []Step
	steps = append(steps, b.App.Clean()...)
	steps = append(steps, b.App.Create()...)
	if b.Config.ProvisioningProfile != "" {
		steps = append(steps, b.App.InstallProvisioningProfile(b.Config.ProvisioningProfile))
	}
	steps = append(steps, b.App.WriteInfoPlist(), b.App.CopyExecutable(b.Binary))
	steps = append(steps, b.Contents...)
	steps = append(steps,
		b.App.SetExecutablePermissions(b.Binary, b.Config.Permissions.ExecutableMode()),
		b.App.SetMacOSDirPermissions(b.Config.Permissions.MacOSDirMode()))
	if b.Config.Signing.Configured() {
		signer := b.Config.Signing.Signer()
		steps = append(steps, b.App.SignExecutable(signer), b.App.Sign(signer))
	}
	if b.Notarize {
		steps = append(steps, b.App.Notarize(b.Config.Notary)...)
	}
	return steps, nil
}
