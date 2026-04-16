// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"os"

	"cloudeng.io/macos/buildtools"
)

type bundle struct {
	cfg        config
	stepRunner *buildtools.StepRunner
	ap         buildtools.AppBundle
}

func newBundle(cfg config) bundle {
	return bundle{
		cfg:        cfg,
		stepRunner: buildtools.NewRunner(),
		ap: buildtools.AppBundle{
			Path: cfg.Path,
			Info: cfg.Info,
		},
	}
}

func (b bundle) createAndSign(ctx context.Context, binary string) error {
	b.stepRunner.AddSteps(b.ap.Clean())
	b.stepRunner.AddSteps(b.ap.Create()...)
	if b.cfg.ProvisioningProfile != "" {
		profile := os.ExpandEnv(b.cfg.ProvisioningProfile)
		b.stepRunner.AddSteps(b.ap.CopyContents(profile, "embedded.provisionprofile"))
	}
	b.stepRunner.AddSteps(b.ap.WriteInfoPlist(),
		b.ap.CopyExecutable(binary))

	if len(b.cfg.Icon) > 0 {
		tempDir, err := os.MkdirTemp("", "gobundle-icon")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tempDir)
		tempDir = "./icons"
		iconSet := buildtools.IconSet{
			Icon: b.cfg.Icon,
			Dir:  tempDir,
		}
		b.stepRunner.AddSteps(iconSet.CreateIconVariants(
			b.cfg.Icon,
			tempDir)...)
		b.stepRunner.AddSteps(iconSet.CreateIcns())
		b.stepRunner.AddSteps(b.ap.CopyContents(iconSet.IconSetFile(), "Contents/Resources/"+"AppIcon.icns"))
	}

	if b.cfg.Identity != "" {
		signer := b.cfg.Signer()
		b.stepRunner.AddSteps(
			b.ap.SignExecutable(signer),
			b.ap.Sign(signer),
		)
	}
	results := b.stepRunner.Run(ctx, buildtools.NewCommandRunner())
	for _, r := range results {
		if r.Error() != nil {
			fmt.Printf("%s (%s)\noutput: %s\n", r.CommandLine(), r.Error(), r.Output())
			continue
		}
		printf("%s\n%s", r.CommandLine(), r.Output())
	}
	return results.Error()
}
