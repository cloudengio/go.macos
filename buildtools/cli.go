// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"cloudeng.io/cmdutil/cmdconfig"
	"cloudeng.io/cmdutil/cmdtypes"
	"cloudeng.io/cmdutil/cmdyaml"
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
	Verbose    bool   `subcmd:"verbose,false,'if set, print verbose output'"`
	// Config holds the build specification files: YAML files, applied in order,
	// and a file of NAME=VALUE environment variable definitions (.env) that
	// ${NAME} references in them are expanded from, see Load.
	Config cmdconfig.FilesFlag `subcmd:"config,'spec.yaml','comma separated list of the YAML build specification files and of the file of NAME=VALUE environment variable definitions (.env) for use in them, replacing the defaults'"`

	files []string // the YAML files that Load read.
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

// Load locates the files named by the Config flag, searching for them in the
// current directory and the user's configuration directories for appDir, see
// cmdconfig.SearchPath, and parses the YAML files into cfg, expanding ${NAME}
// references from the environment and the environment variables file; one that
// is not defined is replaced by an empty string. Unknown fields are ignored.
// It is an error for there to be no YAML file.
func (f *CommonFlags) Load(ctx context.Context, cfg any, appDir string) (*cmdconfig.Located, error) {
	spec := cmdconfig.Spec{Path: cmdconfig.SearchPath(appDir), FirstOnly: []cmdconfig.FileType{cmdconfig.FileTypeEnv}}
	loc, err := cmdconfig.Load(ctx, cfg, &f.Config, spec, cmdyaml.WithStrictFields(false), cmdyaml.WithUnresolvedEmpty())
	if err != nil {
		return loc, err
	}
	if len(loc.YAML) == 0 {
		return loc, fmt.Errorf("no build specification found, looked for %v in any of %v",
			strings.Join(f.Config.Files(cmdconfig.FileTypeYAML, false), ", "), strings.Join(filepath.SplitList(loc.SearchPath), ", "))
	}
	f.files = loc.YAML
	return loc, nil
}

// Apply applies the overrides given by the BundlePath and Signer flags to cfg.
func (f CommonFlags) Apply(cfg *Bundle) {
	if cfg == nil {
		return
	}
	if f.BundlePath != "" {
		cfg.Path = f.BundlePath
	}
	if f.Signer != "" {
		cfg.Signing.Identity = f.Signer
	}
}

// specFiles returns the files that the specification was read from, or those
// that it would be.
func (f CommonFlags) specFiles() string {
	files := f.files
	if len(files) == 0 {
		files = f.Config.Files(cmdconfig.FileTypeYAML, false)
	}
	return strings.Join(files, ", ")
}

// PermissionsConfig represents permissions configuration for bundle contents.
type PermissionsConfig struct {
	Executable cmdtypes.Permissions `yaml:"executable,omitempty"`
	MacOSDir   cmdtypes.Permissions `yaml:"macos_dir,omitempty"`
}

// ExecutableMode returns the executable file mode, or 0 if not configured.
func (p PermissionsConfig) ExecutableMode() fs.FileMode {
	return p.Executable.FileMode()
}

// MacOSDirMode returns the MacOS directory file mode, or 0 if not configured.
func (p PermissionsConfig) MacOSDirMode() fs.FileMode {
	return p.MacOSDir.FileMode()
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
			fmt.Fprintf(os.Stderr, "Failed to marshal spec parsed from %v: %v\n", f.specFiles(), err)
		} else {
			fmt.Printf("%v: %s\n", f.specFiles(), out)
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
