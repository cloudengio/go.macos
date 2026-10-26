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
}

func newBundle(cfg gobundleconfig.T) bundle {
	return bundle{
		cfg:        cfg,
		stepRunner: buildtools.NewRunner(buildtools.WithStepVerbose(verbose)),
	}
}

// createAndSign builds and signs the bundle. notarize requests notarization;
// since notarization is slow and only matters for bundles distributed to
// other Macs the --notarize flag is required to enable it.
// Local `build`/`run` still embed the provisioning profile, so entitlements
// are authorized without it.
func (b bundle) createAndSign(ctx context.Context, binary string, notarize bool) error {
	steps, err := b.cfg.Steps(
		buildtools.WithNotarize(notarize),
		buildtools.WithBinary("", binary))
	if err != nil {
		return err
	}
	if notarize {
		printf("Submitting %s to Apple notary service (waiting for response)...\n", filepath.Base(b.cfg.Path))
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
