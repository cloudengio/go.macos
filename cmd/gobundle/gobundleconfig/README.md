# Package [cloudeng.io/macos/cmd/gobundle/gobundleconfig](https://pkg.go.dev/cloudeng.io/macos/cmd/gobundle/gobundleconfig?tab=doc)

```go
import cloudeng.io/macos/cmd/gobundle/gobundleconfig
```


## Constants
### SharedBundleEnvVar, AppBundleEnvVar, EnvBundleEnvVar, DefaultSharedConfigFile, DefaultAppConfigFile, DefaultEnvConfigFile, EnvConfigSuffix, ConfigDir
```go
SharedBundleEnvVar = "GOBUNDLE_SHARED_CONFIG"
AppBundleEnvVar = "GOBUNDLE_APP_CONFIG"
EnvBundleEnvVar = "GOBUNDLE_ENV_CONFIG"
DefaultSharedConfigFile = "gobundle-shared"
DefaultAppConfigFile = "gobundle-app"
DefaultEnvConfigFile = "gobundle-env"
// EnvConfigSuffix is the filename suffix of the environment variables
// file, whose contents are NAME=VALUE definitions rather than YAML.
EnvConfigSuffix = ".env"
// ConfigDir is the application directory that the configuration files
// are searched for in, within each of the locations that
// cmdutil.ConfigFilePath returns.
ConfigDir = "gobundle"

```



## Functions
### Func EnvVarMapping
```go
func EnvVarMapping(envFile string) (func(string) string, error)
```
EnvVarMapping returns the mapping to be used, with
cmdyaml.WithExpandMapping, to expand $VAR and ${VAR} references in the
configuration files.

When envFile names a file, the definitions it contains supply the value for
any variable that the process environment does not define, ie. the shell
environment overrides the file, so that a value configured for everyday use
can be changed for a single invocation without editing the file. Note that
a variable set to an empty value is indistinguishable from one that is not
set, so an empty value in the environment does not hide a definition in the
file.

A value taken from the file may itself contain references, as in

    PROFILE=$HOME/Downloads/example.provisionprofile

and those are expanded too, from the environment first and then from the
file, since envfile does not expand them when it reads the file and the
configuration files are expanded in a single pass, which would otherwise
leave the $HOME above in the value. A reference that resolves to nothing is
left as it is, as it is in the configuration files, so that it is visible
rather than silently empty, and expansion is bounded so that a definition
referring to itself terminates.

Values taken from the environment are used as they are: the shell has
already expanded them, and a value that merely contains a '$', such as a
password, must not be altered.

An empty envFile, as returned by LocateEnvConfigFile when there is no such
file, yields the process environment alone.

### Func LocateConfigFiles
```go
func LocateConfigFiles(sharedConfig, appConfig, envConfig string) ([]string, func(string) string, error)
```
LocateConfigFiles returns the configuration files to be used for building
a Go application bundle via gobundle, together with the mapping to expand
$VAR and ${VAR} references in them with, ie. everything needed to parse the
configuration:

    files, expand, err := gobundleconfig.LocateConfigFiles(shared, app, env)
    parser := cmdyaml.NewParser(cmdyaml.WithExpandMapping(expand))
    err = parser.ParseFiles(ctx, &cfg, files...)

The files are the shared and app specific configuration files, in that
order, and only those that exist are returned. The mapping takes its values
from the process environment and, for any variable the environment does not
define, from the environment variables file, see LocateEnvConfigFile and
EnvVarMapping. An error is returned if an environment variables file was
found but could not be read or parsed, or if one was specified, by envConfig
or by GOBUNDLE_ENV_CONFIG, and is not there. The file is otherwise optional:
when none is specified and the default is absent the process environment is
the only source of values.

Each of sharedConfig, appConfig and envConfig may be empty, in which
case the default name for that file is used, and each is overridden by
its environment variable: GOBUNDLE_SHARED_CONFIG, GOBUNDLE_APP_CONFIG
and GOBUNDLE_ENV_CONFIG respectively. All three are searched for in the
current directory and the user's configuration directories, as returned by
cmdutil.ConfigFilePath.

### Func LocateEnvConfigFile
```go
func LocateEnvConfigFile(envConfig string) (string, error)
```
LocateEnvConfigFile returns the file containing the environment variable
definitions to be used when expanding references in the configuration
files, or an empty string if there is none. It checks the environment
variable GOBUNDLE_ENV_CONFIG first and uses the supplied envConfig,
or DefaultEnvConfigFile, otherwise, and then searches the same locations
as LocateConfigFiles does, ie. the current directory and the user's
configuration directories.

The file is searched for with the EnvConfigSuffix suffix rather than
the yaml ones the configuration files use, since its contents are
the NAME=VALUE definitions that cloudeng.io/cmdutil/envfile parses:
the default is gobundle-env.env, or .gobundle-env.env. A name that already
ends in that suffix is used as it is, as is an absolute path. The default
name follows those of the configuration files, so the three files are
gobundle-shared.yaml, gobundle-app.yaml and gobundle-env.env.

Whether a file that is not there is an error depends on how it was asked
for. A file that was specified, ie. named by envConfig or by the environment
variable, and that is not there is an error: it was asked for by name,
so silently carrying on without it would hide a typo, or a file that is not
where it is thought to be, until the bundle turns out to have been built
with the wrong identity. The default name is not an error when it is absent,
ie. the file is optional until it is asked for, and an empty string and a
nil error are returned, which leaves references in the configuration files
to be expanded from the process environment alone.



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

### Methods

```go
func (cfg T) WithDefaultsForBinary(binary string) T
```
WithDefaultsForBinary returns a copy of the configuration with default
values set for the specified binary. The binary should be an absolute path
to the binary to be bundled.







