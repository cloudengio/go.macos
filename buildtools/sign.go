// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type Signer struct {
	identity            string
	entitlements        *Entitlements
	perFileEntitlements *PerFileEntitlements
	arguments           []string
}

// NewSigner creates a new signer.
// The most specific entitlements for a given path will be used.
// If no file specific entitlement exists, the global one (if any)
// is used.
func NewSigner(identity string, entitlements *Entitlements, perFileEntitlements *PerFileEntitlements, arguments []string) Signer {
	signer := Signer{
		identity:            identity,
		entitlements:        entitlements,
		perFileEntitlements: perFileEntitlements,
		arguments:           arguments,
	}
	return signer
}

func (s Signer) entitlementsFor(path string) (Entitlements, bool) {
	if len(path) > 0 && s.perFileEntitlements != nil {
		pf, ok := s.perFileEntitlements.For(path)
		if ok {
			return pf, true
		}
	}
	if s.entitlements != nil {
		return *s.entitlements, true
	}
	return Entitlements{}, false
}

func (s Signer) entitlementsFileFor(path string) (string, bool, error) {
	ent, ok := s.entitlementsFor(path)
	if !ok {
		return "", false, nil
	}
	data, err := ent.MarshalIndent("  ")
	if err != nil {
		return "", false, err
	}
	prefix := "entitlements.plist-"
	if base := filepath.Base(path); base != "" && base != "." {
		prefix = base + "-entitlements.plist-"
	}
	tmpFile, err := os.CreateTemp("", prefix)
	if err != nil {
		return "", false, err
	}
	if _, err := tmpFile.Write(data); err != nil {
		os.Remove(tmpFile.Name()) //nolint:errcheck
		return "", false, err
	}
	name := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		os.Remove(name) //nolint:errcheck
		return "", false, err
	}
	return tmpFile.Name(), true, nil
}

// SignPath returns a Step that signs the specified path within the
// specified bundle. If path is empty, the bundle itself is signed.
func (s Signer) SignPath(bundle, path string) Step {
	if s.identity == "" {
		return ErrorStep(fmt.Errorf("cannot sign path %q: no identity specified", path), "codesign")
	}
	args := []string{"--sign", s.identity}
	if len(s.arguments) == 0 {
		args = append(args, "--options", "runtime", "--force", "--timestamp")
	} else {
		args = append(args, s.arguments...)
	}
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		stepArgs := append([]string{}, args...)
		entitlementsFile, ok, err := s.entitlementsFileFor(path)
		if err != nil {
			return ErrorStep(fmt.Errorf("failed to create entitlements file for %q: %w", path, err), "codesign").Run(ctx, cmdRunner)
		}
		if ok {
			defer os.Remove(entitlementsFile) //nolint:errcheck
			stepArgs = append(stepArgs, "--entitlements", entitlementsFile)
		}
		stepArgs = append(stepArgs, filepath.Join(bundle, path))
		result, err := cmdRunner.Run(ctx, "codesign", stepArgs...)
		if err != nil {
			if entitlementsFile != "" {
				ent, nerr := os.ReadFile(entitlementsFile)
				if nerr == nil {
					err = fmt.Errorf("%w; entitlements: %s", err, string(ent))
				}
			}
			return result, fmt.Errorf("failed to sign %q: %w", path, err)
		}
		return result, nil
	})
}

// VerifyPath returns a Step that verifies the signature of the specified path within the
// specified bundle. If path is empty, the bundle itself is verified.
func (s Signer) VerifyPath(bundle, path string) Step {
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		return cmdRunner.Run(ctx, "codesign", "--verify", "--strict", filepath.Join(bundle, path))
	})
}
