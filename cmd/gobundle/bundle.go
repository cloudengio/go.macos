// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"cloudeng.io/macos/buildtools"
	"cloudeng.io/macos/cmd/gobundle/gobundleconfig"
)

type bundle struct {
	cfg        gobundleconfig.T
	stepRunner *buildtools.StepRunner
	ap         buildtools.AppBundle
}

func newBundle(cfg gobundleconfig.T) bundle {
	return bundle{
		cfg:        cfg,
		stepRunner: buildtools.NewRunner(buildtools.WithStepVerbose(verbose)),
		ap: buildtools.AppBundle{
			Path: cfg.Path,
			Info: cfg.Info,
		},
	}
}

func (b bundle) handleIcons() (func(), error) {
	if len(b.cfg.Icon) == 0 {
		return func() {}, nil
	}
	tempDir, err := os.MkdirTemp("", "gobundle-icon")
	if err != nil {
		return nil, err
	}
	iconDir := filepath.Join(tempDir, "AppIcon.iconset")
	if err := os.Mkdir(iconDir, 0700); err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, err
	}
	iconSet := buildtools.IconSet{
		Icon: b.cfg.Icon,
		Dir:  iconDir,
	}
	b.stepRunner.AddSteps(iconSet.CreateIconVariants(
		iconSet.Icon, iconDir)...)
	b.stepRunner.AddSteps(iconSet.CreateIcns())
	b.stepRunner.AddSteps(b.ap.CopyIcons(iconSet)...)
	return func() {
		os.RemoveAll(tempDir)
	}, nil
}

// createAndSign builds and signs the bundle. notarize requests notarization;
// since notarization is slow and only matters for bundles distributed to
// other Macs the --notarize flag is required to enable it.
// Local `build`/`run` still embed the provisioning profile, so entitlements
// are authorized without it.
func (b bundle) createAndSign(ctx context.Context, binary string, notarize bool) error {
	b.stepRunner.AddSteps(b.ap.Clean()...)
	b.stepRunner.AddSteps(b.ap.Create()...)
	if b.cfg.ProvisioningProfile != "" {
		profile := os.ExpandEnv(b.cfg.ProvisioningProfile)
		b.stepRunner.AddSteps(b.ap.CopyContents(profile, "embedded.provisionprofile"))
	}
	b.stepRunner.AddSteps(b.ap.WriteInfoPlist(),
		b.ap.CopyExecutable(binary))

	cleanup, err := b.handleIcons()
	if err != nil {
		return fmt.Errorf("error processing icons: %v", err)
	}
	defer cleanup()

	if b.cfg.SigningConfig.Configured() {
		signer := b.cfg.Signer()
		b.stepRunner.AddSteps(
			b.ap.SignExecutable(signer),
			b.ap.Sign(signer),
		)
	}

	// When requested (notarize: true, and only for `install`), notarize the
	// signed bundle with Apple's notary service and staple the ticket so
	// Gatekeeper accepts it on other Macs. It requires a signed bundle and notary
	// credentials.
	if notarize {
		if !b.cfg.NotaryConfig.Configured() {
			return fmt.Errorf("notarize is set but no notarization credentials are configured: set a 'notary' section in the config")
		}
		if err := b.cfg.ValidateSigning(b.cfg.SigningConfig); err != nil {
			return err
		}
		printf("Submitting %s to Apple notary service (waiting for response)...\n", filepath.Base(b.ap.Path))
		b.stepRunner.AddSteps(b.ap.Notarize(b.cfg.NotaryConfig)...)
	}

	b.stepRunner.AddSteps(b.ap.SetExecutablePermissions(binary, b.cfg.Permissions.ExecutableMode()))
	b.stepRunner.AddSteps(b.ap.SetMacOSDirPermissions(b.cfg.Permissions.MacOSDirMode()))

	var runnerOpts []buildtools.CommandRunnerOption
	if verbose {
		runnerOpts = append(runnerOpts,
			buildtools.WithStdout(os.Stdout),
			buildtools.WithStderr(os.Stderr))
	}
	results := b.stepRunner.Run(ctx, buildtools.NewCommandRunner(runnerOpts...))
	for _, r := range results {
		if r.Error() != nil {
			fmt.Printf("%s (%s)\noutput: %s\n", r.CommandLine(), r.Error(), r.Output())
			continue
		}
		if !verbose {
			printf("%s\n%s", r.CommandLine(), r.Output())
		}
	}
	return results.Error()
}
