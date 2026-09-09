// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package gobundleconfig

import (
	"os"
	"path/filepath"

	"cloudeng.io/cmdutil/cmdyaml"
	"cloudeng.io/macos/buildtools"
)

// T represents the configuration for building a Go application bundle
// via the gobundle command (see cloudeng.io/macos/cmd/gobundle) which can be
// used to dynamically build a bundle for a Go application via gobundle run
// or gobundle build.
type T struct {
	buildtools.SigningConfig `yaml:",inline"`
	Permissions              buildtools.PermissionsConfig `yaml:"permissions,omitempty"`
	Path                     string                       `yaml:"bundle"`
	Info                     buildtools.InfoPlist         `yaml:"info.plist"`
	ProvisioningProfile      string                       `yaml:"profile"`
	Icon                     string                       `yaml:"icon"`
	buildtools.NotaryConfig  `yaml:"notary"`
}

const (
	SharedBundleEnvVar      = "GOBUNDLE_SHARED_CONFIG"
	AppBundleEnvVar         = "GOBUNDLE_APP_CONFIG"
	DefaultSharedConfigFile = "gobundle-shared"
	DefaultAppConfigFile    = "gobundle-app"
)

func defaultConfigSearchPath() string {
	if home := os.Getenv("HOME"); home != "" {
		return "." + string(filepath.ListSeparator) + home
	}
	return "."
}

// LocateConfigFiles returns the list of configuration files to be used for building
// a Go application bundle via gobundle. It first checks for environment variables
// GOBUNDLE_SHARED_CONFIG and GOBUNDLE_APP_CONFIG, and if not set, it uses the provided
// sharedConfig and appConfig as defaults. It then searches for configuration files
// in the current directory and the user's home directory.
func LocateConfigFiles(sharedConfig, appConfig string) []string {
	searchPath := defaultConfigSearchPath()
	if len(sharedConfig) == 0 {
		sharedConfig = DefaultSharedConfigFile
	}
	if len(appConfig) == 0 {
		appConfig = DefaultAppConfigFile
	}
	shared := os.Getenv(SharedBundleEnvVar)
	if len(shared) == 0 {
		shared = sharedConfig
	}
	app := os.Getenv(AppBundleEnvVar)
	if len(app) == 0 {
		app = appConfig
	}
	files := []string{}
	if len(shared) > 0 {
		if l := cmdyaml.LocateConfigFile(shared, searchPath); len(l) > 0 {
			files = append(files, l)
		}
	}
	if len(app) > 0 {
		if l := cmdyaml.LocateConfigFile(app, searchPath); len(l) > 0 {
			files = append(files, l)
		}
	}
	return files
}

// WithDefaultsForBinary returns a copy of the configuration with default values set for the
// specified binary. The binary should be an absolute path to the binary to be bundled.
func (cfg T) WithDefaultsForBinary(binary string) T {
	cfg.Info = cfg.Info.WithDefaults(binary)
	if cfg.Path == "" {
		cfg.Path = filepath.Join(filepath.Dir(cfg.Path), cfg.Info.CFBundleExecutable+".app")
	}
	return cfg
}
