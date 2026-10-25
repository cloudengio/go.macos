# Package [cloudeng.io/macos/cmd/gobundle/gobundleconfig](https://pkg.go.dev/cloudeng.io/macos/cmd/gobundle/gobundleconfig?tab=doc)

```go
import cloudeng.io/macos/cmd/gobundle/gobundleconfig
```


## Constants
### ConfigBundleEnvVar, EnvBundleEnvVar, ConfigDir, DefaultSharedConfigFile, DefaultAppConfigFile, DefaultEnvConfigFile
```go
// ConfigBundleEnvVar names the environment variable whose value is a
// comma-separated list of the configuration files to use, in place of the
// default ones, if none were set on the command line.
ConfigBundleEnvVar = "GOBUNDLE_CONFIG"
// EnvBundleEnvVar is like ConfigBundleEnvVar for the file of environment
// variable definitions.
EnvBundleEnvVar = "GOBUNDLE_ENV_CONFIG"
// ConfigDir is the application directory that the configuration files
// are searched for in, within each of the locations that
// cmdconfig.SearchPath returns.
ConfigDir = "gobundle"
DefaultSharedConfigFile = "gobundle-shared.yaml"
DefaultAppConfigFile = "gobundle-app.yaml"
// DefaultEnvConfigFile is the default file of NAME=VALUE definitions
// rather than YAML, as parsed by cloudeng.io/cmdutil/envfile.
DefaultEnvConfigFile = "gobundle-env.env"

```



## Variables
### Spec
```go
Spec = cmdconfig.Spec{
	AppDir: ConfigDir,
	EnvVars: map[cmdconfig.FileType]string{
		cmdconfig.FileTypeYAML: ConfigBundleEnvVar,
		cmdconfig.FileTypeEnv:  EnvBundleEnvVar,
	},
	FirstOnly: []cmdconfig.FileType{cmdconfig.FileTypeEnv},
}

```
Spec specifies how a cmdconfig.FilesFlag is located: the files are
searched for in ConfigDir, the environment variables ConfigBundleEnvVar and
EnvBundleEnvVar replace the defaults, and at most one environment variables
file is used.



## Functions
### Func DefaultConfigFiles
```go
func DefaultConfigFiles() []string
```
DefaultConfigFiles are the default configuration files, for use with
cmdconfig.FilesFlag.SetDefaults: the shared and app specific YAML files,
in the order they are applied, and the file of environment variables.



## Types
### Type T
```go
type T struct {
	buildtools.SigningConfig `yaml:",inline"`
	Permissions              buildtools.PermissionsConfig `yaml:"permissions,omitempty"`
	Path                     string                       `yaml:"bundle"`
	Info                     buildtools.InfoPlist         `yaml:"info.plist"`
	ProvisioningProfile      string                       `yaml:"profile"`
	Icon                     string                       `yaml:"icon"`
	buildtools.NotaryConfig  `yaml:"notary"`
}
```
T represents the configuration for building a Go application bundle via the
gobundle command (see cloudeng.io/macos/cmd/gobundle) which can be used to
dynamically build a bundle for a Go application via gobundle run or gobundle
build.

### Functions

```go
func Load(ctx context.Context, ff *cmdconfig.FilesFlag, extra ...string) (T, *cmdconfig.Located, error)
```
Load returns the configuration for building a Go application bundle via
gobundle, from the files named by ff, searched for as per Spec, together
with the files that were used:

    var ff cmdconfig.FilesFlag
    ff.SetDefaults(gobundleconfig.DefaultConfigFiles()...)
    cfg, loc, err := gobundleconfig.Load(ctx, &ff)

The YAML files are applied in order, the later overriding the earlier,
and those that are defaults and do not exist are omitted. extra, if any,
are further YAML files, applied last, that must exist. $VAR and ${VAR}
references in them are expanded from the process environment and, for any
variable the environment does not define, from the environment variables
file, see cmdyaml.Load. A file that was specified, on the command line or by
an environment variable, and is not there is an error: it was asked for by
name, so silently carrying on without it would hide a typo, or a file that
is not where it is thought to be, until the bundle turns out to have been
built with the wrong identity. The defaults are optional.



### Methods

```go
func (cfg T) WithDefaultsForBinary(binary string) T
```
WithDefaultsForBinary returns a copy of the configuration with default
values set for the specified binary. The binary should be an absolute path
to the binary to be bundled.







