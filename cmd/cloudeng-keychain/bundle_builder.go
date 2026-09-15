// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build ignore

// Command builder builds the keychain app bundle: the keychain client as the
// bundle's main executable, wrapping the macos-keychain-plugin as a nested,
// entitled helper bundle.
//
//	keychain.app/
//	  Contents/MacOS/keychain                          <- client, no entitlements
//	  Contents/MacOS/macos-keychain-plugin.app/      <- nested, entitled + profile
//	    Contents/MacOS/macos-keychain-plugin
//	    Contents/embedded.provisionprofile
//
// The client needs no entitlements or provisioning profile. The plugin holds the
// keychain-access-groups + app-sandbox entitlements, which — being
// provisioning-profile restricted — are only AMFI-authorized for a bundle's own
// main executable; hence the plugin is a nested bundle with its own profile.
//
// The nested plugin's signing identity, entitlements, provisioning profile and
// notary credentials are read from keychain-plugin/gobundle-app.yml, the same
// config used to build the plugin standalone with gobundle, so there is a single
// source of truth.
//
// Run via `go generate` (see keychain_cmd.go) or directly:
//
//	go run bundle_builder.go [-output keychain.app] [-softlink] [-install] [-notarize]
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"cloudeng.io/cmdutil/cmdyaml"
	"cloudeng.io/cmdutil/flags"
	"cloudeng.io/macos/buildtools"
	"cloudeng.io/macos/cmd/gobundle/gobundleconfig"
	"cloudeng.io/macos/keychain/plugin"
	"cloudeng.io/os/executil"
)

const (
	clientExecutable = "cloudeng-keychain"
	pluginPackage    = "./cloudeng-keychain-plugin"
	pluginConfigYML  = "cloudeng-keychain-plugin/gobundle-app.yml"
	// pluginExecutable must match keychain/plugin.DefaultPluginBinary so the
	// client can locate the plugin inside the bundle.
	pluginExecutable = plugin.DefaultPluginBinary
	// nestedDir is where the plugin bundle lives within the outer app's Contents.
	nestedDir = "MacOS"
)

type builderFlags struct {
	Output   string `flags:"output,cloudeng-keychain.app,output app bundle path"`
	Softlink bool   `flags:"softlink,true,create a softlink to the executable in the bundle"`
	Install  bool   `flags:"install,false,install the app bundle to GOBIN or PATH"`
	Notarize bool   `flags:"notarize,false,request notarization of the signed bundle"`
	Verbose  bool   `flags:"verbose,false,enable verbose logging"`
}

func main() {
	var bf builderFlags
	flags.RegisterAndParseMust("flags", &bf)

	if err := run(bf.Output, bf.Softlink, bf.Install, bf.Notarize, bf.Verbose); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(out string, softlink, install, notarize, verbose bool) error {
	ctx := context.Background()

	cfg, err := loadPluginConfig(ctx, pluginConfigYML)
	if err != nil {
		return err
	}

	// The nested plugin bundle keeps its own identifier from gobundle-app.yml; the
	// outer app takes a distinct one (macOS requires nested identifiers to differ)
	// and needs no provisioning profile.
	// A nested helper has no UI, so drop any icon reference.
	cfg.Info.CFBundleIconFile = ""
	innerInfo := cfg.Info.WithDefaults(pluginExecutable)
	pluginID := innerInfo.CFBundleIdentifier

	outerInfo := buildtools.InfoPlist{
		LSMinimumSystemVersion: innerInfo.LSMinimumSystemVersion,
	}.WithDefaults(clientExecutable)
	outerInfo.CFBundleIdentifier = pluginID + ".app"

	env := buildtools.GoBuildEnvForMacOSVersion(innerInfo.LSMinimumSystemVersion)

	client, cleanupClient, err := build(ctx, ".", clientExecutable, env)
	if err != nil {
		return err
	}
	defer cleanupClient()
	plugin, cleanupPlugin, err := build(ctx, pluginPackage, pluginExecutable, env)
	if err != nil {
		return err
	}
	defer cleanupPlugin()

	return buildBundle(ctx, out, softlink, install, cfg, outerInfo, innerInfo, client, plugin, notarize, verbose)
}

func buildBundle(ctx context.Context, out string, softlink, install bool, cfg gobundleconfig.T, outerInfo, innerInfo buildtools.InfoPlist, client, plugin string, notarize, verbose bool) error {
	outer := buildtools.AppBundle{Path: out, Info: outerInfo}
	inner := buildtools.AppBundle{
		Path: outer.Contents(nestedDir, pluginExecutable+".app"),
		Info: innerInfo,
	}

	runner := buildtools.NewRunner(buildtools.WithStepVerbose(verbose))
	runner.AddSteps(outer.Clean()...)
	runner.AddSteps(outer.Create()...)

	// Nested plugin bundle: the entitled executable, authorized by its own profile.
	runner.AddSteps(inner.Create()...)
	runner.AddSteps(inner.WriteInfoPlist(), inner.CopyExecutable(plugin))
	if cfg.ProvisioningProfile != "" {
		runner.AddSteps(inner.InstallProvisioningProfile(cfg.ProvisioningProfile))
	}

	// Outer client app.
	runner.AddSteps(outer.WriteInfoPlist(), outer.CopyExecutable(client))

	if cfg.Identity != "" {
		// Sign the nested plugin (with entitlements) fully first, then the client
		// (no entitlements), then seal the outer bundle.
		entitled := cfg.Signer()
		plain := buildtools.NewSigner(cfg.Identity, nil, nil, cfg.CodesignArguments)
		runner.AddSteps(
			inner.SignExecutable(entitled),
			inner.Sign(entitled),
			outer.SignExecutable(plain),
			outer.Sign(plain),
		)
	}

	if notarize {
		if cfg.Identity == "" {
			return fmt.Errorf("-notarize requires a signing identity in %s", pluginConfigYML)
		}
		if !cfg.NotaryConfig.Configured() {
			return fmt.Errorf("-notarize requires a notary section in %s", pluginConfigYML)
		}
		runner.AddSteps(outer.Notarize(cfg.NotaryConfig)...)
	}

	runner.AddSteps(inner.SetExecutablePermissions(plugin, cfg.Permissions.ExecutableMode()))
	runner.AddSteps(inner.SetMacOSDirPermissions(cfg.Permissions.MacOSDirMode()))

	link := filepath.Join(filepath.Dir(out), clientExecutable)
	if softlink {
		runner.AddSteps(outer.SymlinkExecutable(link))
	}
	if install {
		runner.AddSteps(outer.Install(softlink))
	}

	results := runner.Run(ctx, buildtools.NewCommandRunner())
	for _, r := range results {
		if r.Error() != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n%s\n", r.CommandLine(), r.Error(), r.Output())
		}
	}
	if err := results.Error(); err != nil {
		return err
	}
	fmt.Printf("created app bundle %s\n", out)
	if softlink {
		if dest, err := os.Readlink(link); err == nil {
			fmt.Printf("created symlink %s -> %s\n", link, dest)
		}
	}
	if install {
		if installDir, err := buildtools.InstallDir(ctx); err == nil {
			fmt.Printf("installed app bundle to %s\n", filepath.Join(installDir, filepath.Base(out)))
			if softlink {
				installedLink := filepath.Join(installDir, clientExecutable)
				if dest, err := os.Readlink(installedLink); err == nil {
					fmt.Printf("created symlink %s -> %s\n", installedLink, dest)
				}
			}
		}
	}
	return nil
}

// loadPluginConfig reads the plugin's gobundle-app.yml, expands ${ENV}
// references, and unmarshals the fields the nested bundle needs.
func loadPluginConfig(ctx context.Context, path string) (gobundleconfig.T, error) {
	var cfg gobundleconfig.T
	parser := cmdyaml.NewParser(cmdyaml.WithStrictFields(true), cmdyaml.WithExpandMapping(os.Getenv))
	if err := parser.ParseFiles(ctx, &cfg, path); err != nil {
		return gobundleconfig.T{}, fmt.Errorf("parsing %s: %w", path, err)
	}

	return cfg, nil
}

// build compiles the Go package pkg to a temporary file named executable,
// returning its path and a cleanup function.
func build(ctx context.Context, pkg, executable string, env []string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "keychain-build-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	bin := dir + "/" + executable

	gobin, gobinargs, err := executil.GoBuildArgs(bin, pkg)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("getting Go build args: %w", err)
	}
	cmd := exec.CommandContext(ctx, gobin, gobinargs...)
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("building %s: %w", pkg, err)
	}
	return bin, cleanup, nil
}
