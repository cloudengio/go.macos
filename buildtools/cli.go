// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools

import (
	"flag"
	"fmt"
	"io/fs"
	"os"

	"cloudeng.io/cmdutil/cmdtypes"
	"cloudeng.io/cmdutil/flags"
	"gopkg.in/yaml.v3"
)

// CommonFlags represents flags commonly used by buildtools command line tools.
type CommonFlags struct {
	DryRun     bool   `subcmd:"dry-run,false,'if set, execute the commands in dry-run mode'"`
	Timing     bool   `subcmd:"timing,false,'if set, print timing information for each step'"`
	Release    bool   `subcmd:"swift-release,false,'if set, use swift release build, otherwise debug'"`
	BundlePath string `subcmd:"bundle-path,'','path for the output bundle, overrides any specified in a config file'"`
	Signer     string `subcmd:"signer,'','signing identity to use, overrides any specified in a config file'"`
	ConfigFile string `subcmd:"config,'spec.yaml','path to the build specification yaml file'"`
	Verbose    bool   `subcmd:"verbose,false,'if set, print verbose output'"`
}

// RegisterFlagsOrDie registers a struct that contains an instance of CommonFlags with the provided
// FlagSet, panicing on error.
func RegisterFlagsOrDie(f any, fs *flag.FlagSet) {
	if err := flags.RegisterFlagsInStruct(fs, "subcmd", f, nil, nil); err != nil {
		panic(err)
	}
}

// CommandRunnerOptions returns options for the CommandRunner based on the flags.
func (f CommonFlags) CommandRunnerOptions() []CommandRunnerOption {
	opts := []CommandRunnerOption{}
	if f.DryRun {
		opts = append(opts, WithDryRun(f.DryRun))
	}
	if f.Timing {
		opts = append(opts, WithCommandTiming(f.Timing))
	}
	return opts
}

// StepRunnerOptions returns options for the StepRunner based on the flags.
func (f CommonFlags) StepRunnerOptions() []StepRunnerOption {
	var opts []StepRunnerOption
	if f.Timing {
		opts = append(opts, WithStepTiming(f.Timing))
	}
	if f.Verbose {
		opts = append(opts, WithStepVerbose(f.Verbose))
	}
	return opts
}

// ParseFile parses the specified config file into cfg.
func (f CommonFlags) ParseFile(cfg any) error {
	data, err := os.ReadFile(f.ConfigFile)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, cfg)
}

// PermissionsConfig represents permissions configuration for bundle contents.
type PermissionsConfig struct {
	Executable cmdtypes.Permissions `yaml:"executable,omitempty"`
	MacOSDir   cmdtypes.Permissions `yaml:"macos_dir,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler to accept both "executable" / "macos_dir"
// and "executable_permissions" / "macos_dir_permissions".
func (p *PermissionsConfig) UnmarshalYAML(node *yaml.Node) error {
	var raw struct {
		Executable            cmdtypes.Permissions `yaml:"executable"`
		ExecutablePermissions cmdtypes.Permissions `yaml:"executable_permissions"`
		MacOSDir              cmdtypes.Permissions `yaml:"macos_dir"`
		MacOSDirPermissions   cmdtypes.Permissions `yaml:"macos_dir_permissions"`
	}
	if err := node.Decode(&raw); err != nil {
		return err
	}
	if raw.Executable != 0 {
		p.Executable = raw.Executable
	} else {
		p.Executable = raw.ExecutablePermissions
	}
	if raw.MacOSDir != 0 {
		p.MacOSDir = raw.MacOSDir
	} else {
		p.MacOSDir = raw.MacOSDirPermissions
	}
	return nil
}

// ExecutableMode returns the executable file mode, or 0 if not configured.
func (p PermissionsConfig) ExecutableMode() fs.FileMode {
	return p.Executable.FileMode()
}

// MacOSDirMode returns the MacOS directory file mode, or 0 if not configured.
func (p PermissionsConfig) MacOSDirMode() fs.FileMode {
	return p.MacOSDir.FileMode()
}

// Config represents common configuration options
// that can be read from a yaml config file.
type Config struct {
	AppBundle             string               `yaml:"bundle"`
	Signing               SigningConfig        `yaml:"signing"`
	Permissions           PermissionsConfig    `yaml:"permissions,omitempty"`
	ExecutablePermissions cmdtypes.Permissions `yaml:"executable_permissions,omitempty"`
	MacOSDirPermissions   cmdtypes.Permissions `yaml:"macos_dir_permissions,omitempty"`
}

// ExecutableMode returns the configured executable permissions, checking both
// the nested permissions section and top-level executable_permissions.
func (c Config) ExecutableMode() fs.FileMode {
	if c.Permissions.Executable != 0 {
		return c.Permissions.Executable.FileMode()
	}
	return c.ExecutablePermissions.FileMode()
}

// MacOSDirMode returns the configured MacOS directory permissions, checking both
// the nested permissions section and top-level macos_dir_permissions.
func (c Config) MacOSDirMode() fs.FileMode {
	if c.Permissions.MacOSDir != 0 {
		return c.Permissions.MacOSDir.FileMode()
	}
	return c.MacOSDirPermissions.FileMode()
}

// SigningConfig represents signing related configuration
// that can be read from a yaml config file.
type SigningConfig struct {
	Identity            string               `yaml:"identity"`
	CodesignArguments   []string             `yaml:"codesign-args"`
	Entitlements        *Entitlements        `yaml:"entitlements"`
	PerFileEntitlements *PerFileEntitlements `yaml:"perfile_entitlements"`
}

// Configured reports whether signing is configured. Signing requires
// an identity to be specified.
func (s SigningConfig) Configured() bool {
	return len(s.Identity) > 0
}

// Signer returns a Signer based on the configuration.
func (s SigningConfig) Signer() Signer {
	return NewSigner(s.Identity, s.Entitlements, s.PerFileEntitlements, s.CodesignArguments)
}

// PrintResultAndExitOnErrorf prints the results of running steps and exits with a non-zero
// status if any of the steps failed.
func (f CommonFlags) PrintResultAndExitOnErrorf(spec any, result RunResult) {
	err := f.PrintResult(spec, result)
	if err != nil {
		os.Exit(1)
	}
}

func (f CommonFlags) PrintResult(spec any, result RunResult) error {
	err := result.Error()
	verbose := f.Verbose || err != nil
	if verbose {
		if out, err := yaml.Marshal(spec); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal spec parsed from %v: %v\n", f.ConfigFile, err)
		} else {
			fmt.Printf("%v: %s\n", f.ConfigFile, out)
		}
		for _, r := range result {
			if r.Error() != nil {
				fmt.Println(r.String())
				continue
			}
			fmt.Println(r.CommandLine())
		}
	}
	return err
}
