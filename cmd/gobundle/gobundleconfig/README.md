# Package [cloudeng.io/macos/cmd/gobundle/gobundleconfig](https://pkg.go.dev/cloudeng.io/macos/cmd/gobundle/gobundleconfig?tab=doc)

```go
import cloudeng.io/macos/cmd/gobundle/gobundleconfig
```


## Constants
### SharedBundleEnvVar, AppBundleEnvVar, DefaultSharedConfigFile, DefaultAppConfigFile
```go
SharedBundleEnvVar = "GOBUNDLE_SHARED_CONFIG"
AppBundleEnvVar = "GOBUNDLE_APP_CONFIG"
DefaultSharedConfigFile = "gobundle-shared"
DefaultAppConfigFile = "gobundle-app"

```



## Functions
### Func LocateConfigFiles
```go
func LocateConfigFiles(sharedConfig, appConfig string) []string
```
LocateConfigFiles returns the list of configuration files to be used
for building a Go application bundle via gobundle. It first checks for
environment variables GOBUNDLE_SHARED_CONFIG and GOBUNDLE_APP_CONFIG,
and if not set, it uses the provided sharedConfig and appConfig as defaults.
It then searches for configuration files in the current directory and the
user's home directory.



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







