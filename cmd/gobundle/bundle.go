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
			Path: cfg.Bundle,
			Info: cfg.Info,
		},
	}
}

// iconSteps returns the steps that create the icon of the app, and a function
// to remove the temporary directory that they use.
func (b bundle) iconSteps() ([]buildtools.Step, func(), error) {
	if len(b.cfg.Icon) == 0 {
		return nil, func() {}, nil
	}
	tempDir, err := os.MkdirTemp("", "gobundle-icon")
	if err != nil {
		return nil, nil, err
	}
	iconDir := filepath.Join(tempDir, "AppIcon.iconset")
	if err := os.Mkdir(iconDir, 0700); err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, nil, err
	}
	iconSet := buildtools.IconSet{
		Icon: b.cfg.Icon,
		Dir:  iconDir,
	}
	steps := iconSet.CreateIconVariants(iconSet.Icon, iconDir)
	steps = append(steps, iconSet.CreateIcns())
	steps = append(steps, b.ap.CopyIcons(iconSet)...)
	return steps, func() {
		os.RemoveAll(tempDir)
	}, nil
}

// createAndSign builds and signs the bundle. notarize requests notarization;
// since notarization is slow and only matters for bundles distributed to
// other Macs the --notarize flag is required to enable it.
// Local `build`/`run` still embed the provisioning profile, so entitlements
// are authorized without it.
func (b bundle) createAndSign(ctx context.Context, binary string, notarize bool) error {
	icons, cleanup, err := b.iconSteps()
	if err != nil {
		return fmt.Errorf("error processing icons: %v", err)
	}
	defer cleanup()

	steps, err := buildtools.BundleBuilder{
		Config:   b.cfg.BundleConfig,
		App:      b.ap,
		Binary:   binary,
		Notarize: notarize,
		Contents: icons,
	}.Steps()
	if err != nil {
		return err
	}
	if notarize {
		printf("Submitting %s to Apple notary service (waiting for response)...\n", filepath.Base(b.ap.Path))
	}
	b.stepRunner.AddSteps(steps...)

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
