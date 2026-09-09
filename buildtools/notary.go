// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NotaryConfig holds the credentials used to submit a bundle to Apple's
// notarization service via `xcrun notarytool`. Prefer KeychainProfile so that
// credentials are not embedded in configuration files.
type NotaryConfig struct {
	// KeychainProfile is the name of a notarytool credentials profile created
	// with `xcrun notarytool store-credentials <name>`.
	KeychainProfile string `yaml:"keychain_profile"`
	// AppleID, TeamID and Password provide credentials directly when a keychain
	// profile is not used. To avoid exposing credentials in the process table,
	// Password should use `@keychain:<item>` or `@env:<VAR_NAME>` reference syntax.
	AppleID  string `yaml:"apple_id"`
	TeamID   string `yaml:"team_id"`
	Password string `yaml:"password"`
	// Arguments, if set, replaces the default authentication arguments entirely.
	Arguments []string `yaml:"arguments"`
}

// Configured reports whether any notarization credentials have been supplied,
// i.e. whether Notarize can run.
func (n NotaryConfig) Configured() bool {
	return len(n.Arguments) > 0 || n.KeychainProfile != "" ||
		(n.AppleID != "" && n.TeamID != "" && n.Password != "")
}

// authArgs returns the notarytool authentication arguments implied by the
// configuration, or an error if no usable credentials were supplied.
func (n NotaryConfig) authArgs() ([]string, error) {
	switch {
	case len(n.Arguments) > 0:
		return n.Arguments, nil
	case n.KeychainProfile != "":
		return []string{"--keychain-profile", n.KeychainProfile}, nil
	case n.AppleID != "" && n.TeamID != "" && n.Password != "":
		return []string{"--apple-id", n.AppleID, "--team-id", n.TeamID, "--password", n.Password}, nil
	}
	return nil, fmt.Errorf("notary: no credentials: set keychain_profile, or apple_id+team_id+password")
}

// ValidateSigning checks that the signing configuration is compatible
// with notarization. Notarization requires that the bundle is signed with a
// Developer ID identity, so this checks that the signing configuration is
// present and that the identity is not an Apple Development or Mac Developer
// identity.
func (n NotaryConfig) ValidateSigning(signing SigningConfig) error {
	if !n.Configured() {
		return nil
	}
	if !signing.Configured() || signing.Identity == "" || signing.Identity == "-" {
		return fmt.Errorf("notarize is set but the bundle is not signed: set an 'identity' in the config")
	}
	if strings.HasPrefix(signing.Identity, "Apple Development:") || strings.HasPrefix(signing.Identity, "Mac Developer:") {
		return fmt.Errorf("notarize requires a 'Developer ID Application' identity, but an Apple Development identity (%q) was configured", signing.Identity)
	}
	return nil
}

// ValidateSigningConfig is an alias for ValidateSigning.
func (n NotaryConfig) ValidateSigningConfig(signing SigningConfig) error {
	return n.ValidateSigning(signing)
}

// Notarize returns the steps that submit the bundle to Apple's notarization
// service and staple the resulting ticket into it. The bundle must already be
// signed with a Developer ID identity using the hardened runtime and a secure
// timestamp (Sign does this by default). The bundle is zipped with ditto for
// submission (notarytool does not accept a bare .app); the archive is removed
// once submission completes.
func (b AppBundle) Notarize(cfg NotaryConfig) []Step {
	archive, err := b.notaryArchivePath()
	if err != nil {
		return []Step{ErrorStep(err, "ditto")}
	}
	return []Step{
		b.zipForNotarization(archive),
		b.submitForNotarization(cfg, archive),
		b.Staple(),
	}
}

// notaryArchivePath creates a secure temporary zip file for notarytool submission.
func (b AppBundle) notaryArchivePath() (string, error) {
	f, err := os.CreateTemp("", filepath.Base(b.Path)+"-*.notarize.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary archive for notarization: %w", err)
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

// zipForNotarization zips the bundle into archive using ditto, which preserves
// the bundle structure and code-signing metadata.
func (b AppBundle) zipForNotarization(archive string) Step {
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		res, err := cmdRunner.Run(ctx, "ditto", "-c", "-k", "--keepParent", b.Path, archive)
		if err != nil {
			_ = os.Remove(archive)
			return res, fmt.Errorf("failed to zip bundle for notarization: %w", err)
		}
		return res, nil
	})
}

// submitForNotarization submits archive to notarytool and waits for the result,
// removing archive afterwards.
func (b AppBundle) submitForNotarization(cfg NotaryConfig, archive string) Step {
	auth, err := cfg.authArgs()
	if err != nil {
		return ErrorStep(err, "xcrun", "notarytool", "submit")
	}
	args := append([]string{"notarytool", "submit", archive}, auth...)
	args = append(args, "--wait")
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		defer os.Remove(archive) //nolint:errcheck
		result, err := cmdRunner.Run(ctx, "xcrun", args...)
		if err != nil {
			return result, fmt.Errorf("failed to notarize %q: %w", b.Path, err)
		}
		return result, nil
	})
}

// Staple returns a Step that staples a notarization ticket into the bundle so
// that Gatekeeper can validate it offline.
func (b AppBundle) Staple() Step {
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		return cmdRunner.Run(ctx, "xcrun", "stapler", "staple", b.Path)
	})
}
