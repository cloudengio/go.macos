# Package [cloudeng.io/macos/buildtools](https://pkg.go.dev/cloudeng.io/macos/buildtools?tab=doc)

```go
import cloudeng.io/macos/buildtools
```


## Constants
### BashInstallPreamble
```go
BashInstallPreamble = `#!/usr/bin/env bash
set -euo pipefail
mount="$1"
installer_pid="$2"
target="$3"

TARGET_USER=$(stat -f "%Su" /dev/console)
TARGET_HOME=$(eval echo ~$TARGET_USER)
`

```
BashInstallPreamble is the standard preamble for install scripts used in
pkgbuild packages.

### SafariWebExtensionPointIdentifier
```go
// SafariWebExtensionPointIdentifier is the NSExtensionPointIdentifier of a
// Safari web extension, ie. one that uses the WebExtensions API and a
// manifest.json.
SafariWebExtensionPointIdentifier = "com.apple.Safari.web-extension"

```



## Functions
### Func CWDFromContext
```go
func CWDFromContext(ctx context.Context) string
```
CWDFromContext retrieves the current working directory from the context,
as set by ContextWithCWD. If no directory has been set in the context the
function returns the process's current working directory at the time that
this package was initialized. Note that this may differ from the actual
current working directory of the process if it has changed since the package
was initialized or if another package that was initialized earlier has
changed the current working directory of the process.

### Func ContextWithCWD
```go
func ContextWithCWD(ctx context.Context, cwd string) context.Context
```
ContextWithCWD returns a new context with the specified current working
directory. CommandRunner will use this directory for executing commands.

### Func GUIDomain
```go
func GUIDomain() string
```
GUIDomain returns the launchd GUI domain target for the current user.

### Func GoBuildEnvForMacOSVersion
```go
func GoBuildEnvForMacOSVersion(version string) []string
```
GoBuildEnvForMacOSVersion returns the environment variables needed for go
build and cgo to target the specified macOS version, as typically specified
by LSMinimumSystemVersion in an Info.plist.

The returned environment variables include:
  - MACOSX_DEPLOYMENT_TARGET=<version>
  - CGO_CFLAGS=-mmacosx-version-min=<version>
  - CGO_CXXFLAGS=-mmacosx-version-min=<version>
  - CGO_LDFLAGS=-mmacosx-version-min=<version>

If version is empty after trimming whitespace and any leading 'v' prefix,
nil is returned.

### Func InstallDir
```go
func InstallDir(ctx context.Context, searchDirs ...string) (string, error)
```
InstallDir returns the first directory that exists and is writable from the
candidate directories. If searchDirs is empty, it checks the value of 'go
env GOBIN' followed by each directory in the PATH environment variable.

### Func NativeMessagingHostsDir
```go
func NativeMessagingHostsDir(browser BrowserType, scope NativeMessagingScope, home string) (string, error)
```
NativeMessagingHostsDir returns the directory that browser searches for
native messaging manifests, into which the manifest for a helper must be
installed for an extension to be able to launch it. home is the user's home
directory and is ignored for SystemScope.

Safari does not use native messaging manifests: a Safari web extension sends
native messages to the .appex that contains it, so an error is returned for
it. See AppBundle.AddSafariWebExtension.

See https://developer.chrome.com/docs/apps/nativeMessaging and
https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/Native_manifests

### Func RegisterFlagsOrDie
```go
func RegisterFlagsOrDie(f any, fs *flag.FlagSet)
```
RegisterFlagsOrDie registers a struct that contains an instance of
CommonFlags with the provided FlagSet, panicing on error.

### Func UserLaunchAgentsDir
```go
func UserLaunchAgentsDir() (string, error)
```
UserLaunchAgentsDir returns the current user's LaunchAgents directory.



## Types
### Type AppBundle
```go
type AppBundle struct {
	Path string
	Info InfoPlist
}
```
AppBundle represents a macOS application bundle. See:
https://developer.apple.com/documentation/bundleresources See:
https://developer.apple.com/documentation/bundleresources/placing-content-in-a-bundle

### Methods

```go
func (b AppBundle) AddHelperExecutable(src string, name ...string) []Step
```
AddHelperExecutable returns the steps required to add an additional
executable to the bundle's Contents/Helpers directory, such as a tool that
a native messaging helper invokes, or a helper shared by more than one
browser. name defaults to the base name of src.

Each executable added this way is a separate Mach-O and so must be signed in
its own right, before the bundle that contains it, see SignHelper.


```go
func (b AppBundle) AddNativeMessagingHelper(h NativeMessagingHelper, browsers ...BrowserType) []Step
```
AddNativeMessagingHelper returns the steps required to add a native
messaging helper to the bundle: the Contents/Helpers directory is
created, the helper is copied into it, and a manifest is written into
Contents/Resources/NativeMessagingHosts/<browser> for each browser. Each
manifest is validated for the browser it is written for. Safari does not use
native messaging manifests and is rejected.


```go
func (b AppBundle) AddSafariWebExtension(ext SafariWebExtension) []Step
```
AddSafariWebExtension returns the steps required to add a Safari web
extension to the bundle: the .appex is created in Contents/PlugIns with the
Info.plist that identifies it as a web extension, the handler executable
is copied into it, and the extension's own resources are copied into its
Resources directory.


```go
func (b AppBundle) Clean() []Step
```
Clean returns Steps that remove the app bundle directory and all its
contents. The permissions of the app bundle directory are set to 0700 before
removal to ensure that it can be deleted.


```go
func (b AppBundle) Contents(elem ...string) string
```
Contents returns the path to the specified element within the app bundle's
Contents directory.


```go
func (b AppBundle) CopyContents(src string, dst ...string) Step
```
CopyContents returns the step required to copy a file into the app bundle
dst is relative to the bundle Contents root.


```go
func (b AppBundle) CopyExecutable(src string) Step
```
CopyExecutable returns the step required to copy the executable referenced
in the Info.plist into the app bundle.


```go
func (b AppBundle) CopyIcons(icons ...IconSet) []Step
```
CopyIcons returns steps to copy the specified icons into the app bundle's
Resources directory. If multiple icons are specified and the icon's
BundleIcon field is set or if there is only a single icon then it is copied
to the location specified by the bundle's Info.plist CFBundleIconFile field.
All other icons are copied to their own directories within the Resources
directory.


```go
func (b AppBundle) Create() []Step
```
Create returns the steps required to create the app bundle directory
structure and Info.plist.


```go
func (b AppBundle) ExecutablePath() string
```
ExecutablePath returns the absolute path to the executable inside the app
bundle that is referenced in the Info.plist.


```go
func (b AppBundle) Helpers(elem ...string) string
```
Helpers returns the path to elem within the bundle's Contents/Helpers
directory, which is where a native messaging helper is placed.


```go
func (b AppBundle) Install(softlink bool, searchDirs ...string) Step
```
Install returns a Step that installs the app bundle into the first directory
that exists and is writable, starting with the value of 'go env GOBIN'
followed by the members of PATH (or searchDirs if specified). If softlink
is true, a symbolic link to the bundle's main executable is created in the
install directory with the same name as the executable.


```go
func (b AppBundle) InstallNativeMessagingManifest(h NativeMessagingHelper, browser BrowserType, scope NativeMessagingScope) []Step
```
InstallNativeMessagingManifest returns the step required to install
the manifest for h into the directory that browser searches, so that an
extension can launch the helper. It is intended for development use;
a shipped application would normally install its own manifest when it runs.
Safari does not use native messaging manifests and is rejected.


```go
func (b AppBundle) InstallProvisioningProfile(profile string) Step
```
InstallProvisioningProfile returns a Step that copies
the provisioning profile into the app bundle. See
https://developer.apple.com/documentation/technotes/tn3125-inside-code-signing-provisioning-profiles
for an explanation of provisioning profiles.


```go
func (b AppBundle) NativeMessagingHelperPath(h NativeMessagingHelper) string
```
NativeMessagingHelperPath returns the absolute path of the helper as it will
be once the bundle is installed, which is the path named by the manifest.


```go
func (b AppBundle) NativeMessagingManifest(h NativeMessagingHelper, browser ...BrowserType) NativeMessagingConfig
```
NativeMessagingManifest returns the manifest for h as it is written into the
bundle, with Path set to the helper's installed location and Type defaulted
to "stdio". If a browser is specified, the configuration is tailored to
that browser (e.g. AllowedOrigins for Chrome/Edge, AllowedExtensions for
Firefox).


```go
func (b AppBundle) NativeMessagingManifestPath(h NativeMessagingHelper, browser BrowserType) string
```
NativeMessagingManifestPath returns the path within the bundle of
the manifest written for browser, relative to the bundle's Resources
directory. An installer copies this file into the directory returned by
NativeMessagingHostsDir.


```go
func (b AppBundle) Notarize(cfg NotaryConfig) []Step
```
Notarize returns the steps that submit the bundle to Apple's notarization
service and staple the resulting ticket into it. The bundle must already be
signed with a Developer ID identity using the hardened runtime and a secure
timestamp (Sign does this by default). The bundle is zipped with ditto for
submission (notarytool does not accept a bare .app); the archive is removed
once submission completes.


```go
func (b AppBundle) PlugIns(elem ...string) string
```
PlugIns returns the path to elem within the bundle's Contents/PlugIns
directory, which is where an application extension is placed.


```go
func (b AppBundle) Resources(elem ...string) string
```
Resources returns the path to the specified element within the app bundle's
Resources directory.


```go
func (b AppBundle) SPCtlAsses() Step
```


```go
func (b AppBundle) SafariWebExtensionPath(ext SafariWebExtension) string
```
SafariWebExtensionPath returns the path of the extension's .appex within the
bundle.


```go
func (b AppBundle) SetExecutablePermissions(src string, perms fs.FileMode) Step
```
SetExecutablePermissions returns a Step that sets the permissions of
the executable referenced in the Info.plist within the app bundle.
If the permissions are 0, it returns a NoopStep. Otherwise, if
Info.CFBundleExecutable is not set and src is provided, filepath.Base(src)
is used. If both are empty, it returns an ErrorStep.


```go
func (b AppBundle) SetMacOSDirPermissions(perms fs.FileMode) Step
```
SetMacOSDirPermissions returns a Step that sets the permissions of the MacOS
directory. If the permissions are 0, it returns a NoopStep.


```go
func (b AppBundle) Sign(signer Signer) Step
```


```go
func (b AppBundle) SignContents(signer Signer, dst ...string) Step
```
SignContents returns the step required to sign a file within the app bundle,
dst is relative to the bundle Contents root.


```go
func (b AppBundle) SignExecutable(signer Signer) Step
```
SignExecutable returns the step required to sign the executable within the
app bundle.


```go
func (b AppBundle) SignHelper(signer Signer, name string) Step
```
SignHelper returns the step required to sign the named executable within the
bundle's Contents/Helpers directory.


```go
func (b AppBundle) SignNativeMessagingHelper(signer Signer, h NativeMessagingHelper) Step
```
SignNativeMessagingHelper returns the step required to sign the helper
within the app bundle. A helper is a separate executable and so must be
signed before the bundle that contains it.


```go
func (b AppBundle) SignSafariWebExtension(signer Signer, ext SafariWebExtension) []Step
```
SignSafariWebExtension returns the steps required to sign the extension's
.appex within the app bundle: the handler executable inside the .appex
is signed first, and then the .appex bundle itself. An .appex is a nested
bundle and so must be signed inside out, and before the bundle that contains
it.


```go
func (b AppBundle) Staple() Step
```
Staple returns a Step that staples a notarization ticket into the bundle so
that Gatekeeper can validate it offline.


```go
func (b AppBundle) SymlinkExecutable(link string) Step
```
SymlinkExecutable returns a Step that creates a symbolic link to the
bundle's main executable at the specified link path. If target can be made
relative to the directory containing the link, a relative symlink is used so
that the bundle and link remain valid if moved together.


```go
func (b AppBundle) VerifyContents(signer Signer, dst ...string) Step
```
VerifyContents returns the step required to sign a file within the app
bundle, dst is relative to the bundle Contents root.


```go
func (b AppBundle) VerifySignatures(signer Signer) []Step
```


```go
func (b AppBundle) WriteInfoPlist() Step
```
WriteInfoPlist returns the step required to write the Info.plist file for
the app bundle.


```go
func (b AppBundle) WriteInfoPlistGitBuild(_ context.Context, git Git) []Step
```




### Type BashScript
```go
type BashScript struct {
	// contains filtered or unexported fields
}
```
BashScript helps in the construction of bash scripts for any pre and post
install operations.

### Functions

```go
func NewBashScript(preamble string) *BashScript
```
NewBashScript creates a new BashScript instance with the specified preamble.



### Methods

```go
func (b *BashScript) Append(text string)
```
Append appends the specified text to the script.


```go
func (b *BashScript) Bytes() []byte
```
Bytes returns the script as a byte slice.


```go
func (b *BashScript) CreateInstallManifest(systemWide bool, manifest File)
```
CreateInstallManifest appends the commands to create the install manifest
to the script. If the manifest's source path is empty /dev/null is used
instead.


```go
func (b *BashScript) InstallFile(systemWide bool, file File, manifest string)
```
InstallFile appends the commands to install the specified file to the
script. If systemWide is true the file is installed to the system location
otherwise it is installed to the local user location. The manifest file,
if specified, is updated with the installed file's path.




### Type Browser
```go
type Browser struct{}
```
Browser represents a web browser.

### Methods

```go
func (b Browser) CreateChromeExtensionID() ([]byte, string, error)
```
CreateChromeExtensionID generates a stable Chrome Extension ID suitable for
development use. Note that this ID is derived from a newly generated RSA
key pair each time the function is called, so it will be different on each
invocation. For a stable ID, you would need to persist the generated key
pair.


```go
func (b Browser) ReadChromeExtensionID(keyFile string) ([]byte, string, error)
```
ReadChromeExtensionID reads the RSA private key from the specified
PEM-encoded file to obtain the public key and corresponding Chrome Extension
ID.




### Type BrowserType
```go
type BrowserType int
```

### Constants
### Chrome, Firefox, Safari, Edge
```go
Chrome BrowserType = iota
Firefox
Safari
Edge

```



### Methods

```go
func (b BrowserType) String() string
```




### Type CommandRunner
```go
type CommandRunner struct {
	// contains filtered or unexported fields
}
```
CommandRunner executes system commands.

### Functions

```go
func NewCommandRunner(opts ...CommandRunnerOption) *CommandRunner
```
NewCommandRunner creates a new CommandRunner with the provided options.



### Methods

```go
func (r *CommandRunner) DryRun() bool
```


```go
func (r *CommandRunner) Run(ctx context.Context, name string, args ...string) (StepResult, error)
```
Run executes the specified command with arguments and returns the combined
output and any error encountered.


```go
func (r *CommandRunner) WriteFile(ctx context.Context, path string, data []byte, perm uint32) (string, error)
```




### Type CommandRunnerOption
```go
type CommandRunnerOption func(o *commandRunnerOptions)
```
CommandRunnerOption configures a CommandRunner.

### Functions

```go
func WithCommandTiming(timing bool) CommandRunnerOption
```


```go
func WithDryRun(dryRun bool) CommandRunnerOption
```
WithDryRun configures the CommandRunner to simulate command execution
without actually running commands.


```go
func WithStderr(w io.Writer) CommandRunnerOption
```
WithStderr configures the CommandRunner to write standard error to the
provided io.Writer.


```go
func WithStdout(w io.Writer) CommandRunnerOption
```
WithStdout configures the CommandRunner to write standard output to the
provided io.Writer.




### Type CommonFlags
```go
type CommonFlags struct {
	DryRun     bool   `subcmd:"dry-run,false,'if set, execute the commands in dry-run mode'"`
	Timing     bool   `subcmd:"timing,false,'if set, print timing information for each step'"`
	Release    bool   `subcmd:"swift-release,false,'if set, use swift release build, otherwise debug'"`
	BundlePath string `subcmd:"bundle-path,'','path for the output bundle, overrides any specified in a config file'"`
	Signer     string `subcmd:"signer,'','signing identity to use, overrides any specified in a config file'"`
	ConfigFile string `subcmd:"config,'spec.yaml','path to the build specification yaml file'"`
	Verbose    bool   `subcmd:"verbose,false,'if set, print verbose output'"`
}
```
CommonFlags represents flags commonly used by buildtools command line tools.

### Methods

```go
func (f CommonFlags) CommandRunnerOptions() []CommandRunnerOption
```
CommandRunnerOptions returns options for the CommandRunner based on the
flags.


```go
func (f CommonFlags) ParseFile(cfg any) error
```
ParseFile parses the specified config file into cfg.


```go
func (f CommonFlags) PrintResult(spec any, result RunResult) error
```


```go
func (f CommonFlags) PrintResultAndExitOnErrorf(spec any, result RunResult)
```
PrintResultAndExitOnErrorf prints the results of running steps and exits
with a non-zero status if any of the steps failed.


```go
func (f CommonFlags) StepRunnerOptions() []StepRunnerOption
```
StepRunnerOptions returns options for the StepRunner based on the flags.




### Type Config
```go
type Config struct {
	AppBundle             string               `yaml:"bundle"`
	Signing               SigningConfig        `yaml:"signing"`
	Permissions           PermissionsConfig    `yaml:"permissions,omitempty"`
	ExecutablePermissions cmdtypes.Permissions `yaml:"executable_permissions,omitempty"`
	MacOSDirPermissions   cmdtypes.Permissions `yaml:"macos_dir_permissions,omitempty"`
}
```
Config represents common configuration options that can be read from a yaml
config file.

### Methods

```go
func (c Config) ExecutableMode() fs.FileMode
```
ExecutableMode returns the configured executable permissions, checking both
the nested permissions section and top-level executable_permissions.


```go
func (c Config) MacOSDirMode() fs.FileMode
```
MacOSDirMode returns the configured MacOS directory permissions, checking
both the nested permissions section and top-level macos_dir_permissions.




### Type Entitlements
```go
type Entitlements struct {
	// contains filtered or unexported fields
}
```
Entitlements represents a set of macOS app entitlements that are specified
as YAML.

### Methods

```go
func (e Entitlements) MarshalIndent(indent string) ([]byte, error)
```
MarshalIndent returns the XML plist representation of the entitlements with
the specified indent.


```go
func (e Entitlements) MarshalPlist() (any, error)
```


```go
func (e Entitlements) MarshalYAML() (any, error)
```


```go
func (e *Entitlements) UnmarshalYAML(node *yaml.Node) error
```




### Type File
```go
type File struct {
	Src       string `yaml:"src"`
	DstLocal  string `yaml:"local"`
	DstSystem string `yaml:"system"`
}
```

### Methods

```go
func (f File) OneOf() string
```
OneOf returns the destination path to use for the file. If both DstLocal and
DstSystem are set an empty string is returned, if neither is set the source
path is returned, otherwise one of the system or local destination paths is
returned.


```go
func (f File) RewriteHOME() File
```
RewriteHOME rewrites any occurrences of $HOME or ${HOME} in the source
and destination paths to ${TARGET_HOME} which is set in the bash script
preamble. Use this with the BashInstallPreamble to access the current logged
in user's home directory since $HOME does not refer to the user's home
directory from within the installer environment.




### Type Git
```go
type Git struct {
	// contains filtered or unexported fields
}
```

### Functions

```go
func NewGit(dir string) Git
```
NewGit creates a new Git instance rooted at the specified directory which
must be within a git repository.



### Methods

```go
func (g Git) GetBranch(version string) string
```


```go
func (g Git) Hash(ctx context.Context, cmdRunner *CommandRunner, branch string, n int) (StepResult, error)
```


```go
func (g Git) ReplaceBranch(version, buildID string) string
```




### Type IconSet
```go
type IconSet struct {
	Icon     string           `yaml:"icon"`
	Dir      string           `yaml:"dir"`
	Name     string           `yaml:"name"`       // defaults to AppIcon.icns
	Sizes    []int            `yaml:"sizes,flow"` // optional - defaults to standard sizes if not provided
	Multiple IconSizeMultiple `yaml:"multiple"`   // optional - defaults to 3 (1x, 2x and 3x) if not provided
	Format   string           `yaml:"format"`     // optional - defaults to png if not provided
	// if true, the icon is copied to the bundle Resources directory
	// as the file specified by CFBundleIconFile in the Info.plist
	BundleIcon bool `yaml:"bundle_icon"`
}
```
IconSet represents a directory that contains the variously sized icons
needed to create an .icns file from a single source icon.

### Methods

```go
func (i IconSet) CreateIcns() Step
```


```go
func (i IconSet) CreateIconVariants(src, dir string) []Step
```
CreateIconVariants creates the variously sized icons needed for the icon
set. If no sizes are provided, a default set is used. The highest_multiple
parameter indicates the highest scale factor to use, e.g., 2 for 1x and 2x,
3 for 1x, 2x and 3x.


```go
func (i IconSet) IconFormat() string
```


```go
func (i IconSet) IconSetDir() string
```


```go
func (i IconSet) IconSetFile() string
```


```go
func (i IconSet) IconSetName() string
```




### Type IconSetDir
```go
type IconSetDir string
```
IconSetDir represents a directory for an icon set.


### Type IconSizeMultiple
```go
type IconSizeMultiple int
```
IconSizeMultiple represents the scale factor for icon sizes, e.g., 1x, 2x,
3x.

### Constants
### IconSize1x, IconSize2x, IconSize3x
```go
IconSize1x IconSizeMultiple = 1
IconSize2x IconSizeMultiple = 2
IconSize3x IconSizeMultiple = 3

```



### Methods

```go
func (m IconSizeMultiple) Suffix() string
```
Suffix returns the filename suffix appropriate for the icon size multiple.




### Type InfoPlist
```go
type InfoPlist struct {
	CFBundleIdentifier     string            `yaml:"CFBundleIdentifier"`
	CFBundleName           string            `yaml:"CFBundleName"`
	CFBundleExecutable     string            `yaml:"CFBundleExecutable"`
	CFBundleIconFile       string            `yaml:"CFBundleIconFile"`
	CFBundlePackageType    string            `yaml:"CFBundlePackageType"`
	LSMinimumSystemVersion string            `yaml:"LSMinimumSystemVersion"`
	CFBundleDisplayName    string            `yaml:"CFBundleDisplayName"`
	XPCService             *XPCServicePlist  `yaml:"XPCService"`
	NSExtension            *NSExtensionPlist `yaml:"NSExtension"`

	// CFBundleVersion is the build identifier and CFBundleShortVersionString
	// the human-readable release version, eg. "1.2.0+3f2a9c11" and "1.2.0".
	CFBundleVersion            string `yaml:"CFBundleVersion"`
	CFBundleShortVersionString string `yaml:"CFBundleShortVersionString"`

	// Extra holds every key that has no field of its own, so that keys this
	// package does not know about are preserved rather than discarded.
	Extra map[string]any `yaml:",inline"`
}
```
InfoPlist represents the contents of a macOS bundle's Info.plist file.
Commonly used keys have fields of their own; every other key is captured by
Extra, so an InfoPlist round-trips without loss.

### Functions

```go
func ReadInfoPlist(path string) (InfoPlist, error)
```
ReadInfoPlist reads and unmarshals an InfoPlist from the specified file
path.



### Methods

```go
func (ipl InfoPlist) MarshalPlist() (any, error)
```


```go
func (ipl InfoPlist) MarshalYAML() (any, error)
```


```go
func (ipl InfoPlist) Validate() error
```
Validate reports whether the keys required to describe a launchable bundle
are present, including those of any XPCService dictionary.


```go
func (ipl InfoPlist) WithDefaults(binary string) InfoPlist
```
WithDefaults returns a copy of ipl with any empty fields populated with
default values derived from the binary name. The filepath.Base of the
binary is used as a default for CFBundleName, CFBundleExecutable and
CFBundleDisplayName, and "com.example.<base>" for CFBundleIdentifier.
CFBundlePackageType is set to "APPL" and LSMinimumSystemVersion to "10.15".
CFBundleVersion is set to "0.0.0" if empty.




### Type LaunchAgent
```go
type LaunchAgent struct {
	// Plist describes the job. Its Label names both the installed file and the
	// launchd service, so it must be set.
	Plist LaunchAgentPlist

	// Dir is the directory the plist is installed into. If empty,
	// UserLaunchAgentsDir is used. Set it to install elsewhere, or in tests.
	Dir string

	// Domain is the launchd domain the job is bootstrapped into, eg. "gui/501".
	// If empty, GUIDomain for the current user is used.
	Domain string
}
```
LaunchAgent manages the installation of a launchd job as a per-user
LaunchAgent, ie. a plist in ~/Library/LaunchAgents that launchd starts when
the user logs in.

A LaunchAgent rather than a system-wide LaunchDaemon is the right choice
for a job that needs a logged-in user's GUI session, for example to use
Virtualization.framework.

### Methods

```go
func (l LaunchAgent) Install() []Step
```
Install returns the steps that write the job's plist and load it:
any previously loaded instance is booted out first, so Install doubles as a
reinstall of a changed job.


```go
func (l LaunchAgent) IsInstalled() bool
```
IsInstalled reports whether the job's plist is present. It says nothing
about whether launchd has the job loaded; use Status for that.


```go
func (l LaunchAgent) PlistPath() (string, error)
```
PlistPath returns the path the job's plist is installed at.


```go
func (l LaunchAgent) Restart() Step
```
Restart returns a Step that restarts the job, starting it if it is loaded
but not running.


```go
func (l LaunchAgent) ServiceTarget() (string, error)
```
ServiceTarget returns the launchd service target for the job, ie. the domain
and label that launchctl subcommands operate on.


```go
func (l LaunchAgent) Status() Step
```
Status returns a Step that prints launchd's view of the job. launchctl
reports a useful message for a job that is not loaded, so the step succeeds
either way.


```go
func (l LaunchAgent) Uninstall() []Step
```
Uninstall returns the steps that unload the job and remove its plist.
It succeeds whether or not the job is currently loaded or installed.




### Type LaunchAgentPlist
```go
type LaunchAgentPlist struct {
	Label string `yaml:"Label"`
	// ProgramArguments is the command to run and its arguments. launchd also
	// accepts a Program key instead, which has no field here and can be
	// supplied via Extra.
	ProgramArguments []string `yaml:"ProgramArguments"`
	// RunAtLoad and KeepAlive are written only when true, since launchd treats
	// an absent key as false.
	RunAtLoad            bool              `yaml:"RunAtLoad"`
	KeepAlive            bool              `yaml:"KeepAlive"`
	EnvironmentVariables map[string]string `yaml:"EnvironmentVariables"`
	StandardOutPath      string            `yaml:"StandardOutPath"`
	StandardErrorPath    string            `yaml:"StandardErrorPath"`

	Extra map[string]any `yaml:",inline"`
}
```
LaunchAgentPlist represents the contents of a launchd job plist, ie.
a LaunchAgent or LaunchDaemon. As for InfoPlist, keys without a field of
their own are captured by Extra.

### Methods

```go
func (lap LaunchAgentPlist) MarshalPlist() (any, error)
```


```go
func (lap LaunchAgentPlist) MarshalYAML() (any, error)
```


```go
func (lap LaunchAgentPlist) Validate() error
```
Validate reports whether the keys launchd requires of a job are present: a
Label, and something to run, ie. ProgramArguments or a Program key supplied
via Extra.




### Type NSExtensionPlist
```go
type NSExtensionPlist struct {
	// NSExtensionPointIdentifier names the kind of extension, eg.
	// SafariWebExtensionPointIdentifier.
	NSExtensionPointIdentifier string `yaml:"NSExtensionPointIdentifier"`
	// NSExtensionPrincipalClass names the class that Safari instantiates to
	// handle requests, ie. the one that receives native messages.
	NSExtensionPrincipalClass string         `yaml:"NSExtensionPrincipalClass"`
	Extra                     map[string]any `yaml:",inline"`
}
```
NSExtensionPlist represents the contents of an NSExtension dictionary,
which appears within the Info.plist of an application extension, such as the
.appex that contains a Safari web extension. As for InfoPlist, keys without
a field of their own are captured by Extra.

### Methods

```go
func (n NSExtensionPlist) MarshalPlist() (any, error)
```


```go
func (n NSExtensionPlist) MarshalYAML() (any, error)
```


```go
func (n NSExtensionPlist) Validate() error
```
Validate reports whether the keys required of an NSExtension dictionary are
present.




### Type NativeMessagingConfig
```go
type NativeMessagingConfig struct {
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Path              string   `json:"path"`
	Type              string   `json:"type"`                         // "stdio" or one of the other allowed communication types
	AllowedOrigins    []string `json:"allowed_origins,omitempty"`    // chrome-extension://<extension-id>/
	AllowedExtensions []string `json:"allowed_extensions,omitempty"` // firefox extension ids
}
```
NativeMessagingConfig represents the configuration for a native messaging
host.

### Methods

```go
func (nm *NativeMessagingConfig) AppendChromeOrigin(extensionID string)
```
AppendChromeOrigin appends the specified Chrome extension ID to the list of
allowed origins.


```go
func (nm *NativeMessagingConfig) Validate(browser BrowserType) Step
```
Validate validates the native messaging configuration for the specified
browser. Edge is validated as for Chrome, since it is chromium based
and uses the same manifest format. Safari does not use native messaging
manifests and is reported as unsupported.


```go
func (nm *NativeMessagingConfig) ValidateChrome() error
```
ValidateChrome validates the native messaging configuration for Chrome.


```go
func (nm *NativeMessagingConfig) ValidateFirefox() error
```
ValidateFirefox validates the native messaging configuration for Firefox.




### Type NativeMessagingHelper
```go
type NativeMessagingHelper struct {
	// Executable is the path to the built helper binary that is copied into
	// the bundle.
	Executable string

	// Name is the name of the helper within the bundle. It defaults to the
	// base name of Executable.
	Name string

	// Config is the manifest for the helper. Its Path is filled in by
	// Manifest and need not be set; Type defaults to "stdio".
	Config NativeMessagingConfig

	// InstalledBundlePath is the location the bundle will occupy once
	// installed, such as /Applications/Example.app. The manifest must name
	// the helper's path as it will be at runtime, which is not where the
	// bundle is built. It defaults to the bundle's own path, which is
	// appropriate when the bundle is used from where it is built.
	InstalledBundlePath string
}
```
NativeMessagingHelper describes a native messaging host that is shipped
inside an app bundle, so that a browser extension can launch it.

The helper is an executable placed in the bundle's Contents/Helpers
directory, alongside a manifest for each browser it serves. The manifest
names the absolute path of the helper, which is where the bundle will be
once installed rather than where it is built, hence InstalledBundlePath.


### Type NativeMessagingScope
```go
type NativeMessagingScope int
```
NativeMessagingScope determines whether a native messaging manifest is
installed for the current user or for all users of the machine.

### Constants
### UserScope, SystemScope
```go
// UserScope installs the manifest for the current user only.
UserScope NativeMessagingScope = iota
// SystemScope installs the manifest for all users, which requires
// privileges to write outside of the user's home directory.
SystemScope

```



### Methods

```go
func (s NativeMessagingScope) String() string
```




### Type NotaryConfig
```go
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
```
NotaryConfig holds the credentials used to submit a bundle to Apple's
notarization service via `xcrun notarytool`. Prefer KeychainProfile so that
credentials are not embedded in configuration files.

### Methods

```go
func (n NotaryConfig) Configured() bool
```
Configured reports whether any notarization credentials have been supplied,
i.e. whether Notarize can run.


```go
func (n NotaryConfig) ValidateSigning(signing SigningConfig) error
```
ValidateSigning checks that the signing configuration is compatible with
notarization. Notarization requires that the bundle is signed with a
Developer ID identity, so this checks that the signing configuration is
present and that the identity is not an Apple Development or Mac Developer
identity.


```go
func (n NotaryConfig) ValidateSigningConfig(signing SigningConfig) error
```
ValidateSigningConfig is an alias for ValidateSigning.




### Type PerFileEntitlements
```go
type PerFileEntitlements struct {
	// contains filtered or unexported fields
}
```
PerFileEntitlements represents a set of macOS app entitlements that are
specific to individual files within an app bundle. These are specified
as YAML. The key should be the file within the bundle and the value is
the entitlements dictionary for that file. The file name can be either
the base name (eg. "executable") or the full path within the bundle (e.g.
"Contents/MacOS/executable").

### Methods

```go
func (e PerFileEntitlements) For(path string) (Entitlements, bool)
```
For returns the entitlements for the specified path or nil if none exist.
It will first look for the base name of the path and then the full path.


```go
func (e PerFileEntitlements) MarshalPlist() (any, error)
```


```go
func (e PerFileEntitlements) MarshalYAML() (any, error)
```


```go
func (e *PerFileEntitlements) UnmarshalYAML(node *yaml.Node) error
```




### Type PermissionsConfig
```go
type PermissionsConfig struct {
	Executable cmdtypes.Permissions `yaml:"executable,omitempty"`
	MacOSDir   cmdtypes.Permissions `yaml:"macos_dir,omitempty"`
}
```
PermissionsConfig represents permissions configuration for bundle contents.

### Methods

```go
func (p PermissionsConfig) ExecutableMode() fs.FileMode
```
ExecutableMode returns the executable file mode, or 0 if not configured.


```go
func (p PermissionsConfig) MacOSDirMode() fs.FileMode
```
MacOSDirMode returns the MacOS directory file mode, or 0 if not configured.


```go
func (p *PermissionsConfig) UnmarshalYAML(node *yaml.Node) error
```
UnmarshalYAML implements yaml.Unmarshaler to accept both "executable" /
"macos_dir" and "executable_permissions" / "macos_dir_permissions".




### Type PkgBuild
```go
type PkgBuild struct {
	BuildDir        string `yaml:"build_dir"`        // Directory to use for building the package
	Identifier      string `yaml:"identifier"`       // Package identifier, e.g. com.cloudeng.myapp
	Version         string `yaml:"version"`          // Package version, e.g. 1.0.0
	InstallLocation string `yaml:"install_location"` // Installation location, e.g. /Applications/MyApp.app

}
```
PkgBuild represents the pkgbuild tool and its configuration.

### Methods

```go
func (p PkgBuild) Build(outputPath string) Step
```
Build returns a Step that builds the package using pkgbuild.


```go
func (p PkgBuild) Clean() Step
```
Clean returns a Step that removes the BuildDir directory.


```go
func (p PkgBuild) CopyApplication(src string) Step
```
CopyApplication returns a Step that copies the specified application bundle
to the Applications directory within the package build root.


```go
func (p PkgBuild) CopyLibrary(src, library string) Step
```
CopyLibrary returns a Step that copies the specified library directory to
the Library directory within the package build root. Note that this is one
way of installing files for use by the Installer.


```go
func (p PkgBuild) CopyScripts(src string) Step
```
CopyScripts returns a Step that copies the specified scripts directory to
the scripts directory within the package build root.


```go
func (p PkgBuild) Create() []Step
```
Create returns the steps required to create the pkgbuild directory
structure.


```go
func (p PkgBuild) CreateLibrary(library string) Step
```
CreateLibrary returns a Step that creates the specified library directory


```go
func (p PkgBuild) Install(outputPath string) Step
```
Install returns a Step that installs the package using the system installer
command using sudo.


```go
func (p PkgBuild) LibraryPath(library string) string
```
LibraryPath returns the path to the specified library directory within the
package build root.


```go
func (p PkgBuild) OutputsPath() string
```
OutputsPath returns the path to the outputs directory within the package
build root.


```go
func (p PkgBuild) ScriptsPath() string
```
ScriptsPath returns the path to the scripts directory within the package
build root.


```go
func (p PkgBuild) WritePlist(cfg []PkgComponentPlist) Step
```
WritePlist returns a Step that writes the specified component plist
configuration to the component.plist file within the package build root.


```go
func (p PkgBuild) WriteScript(data []byte, name string) Step
```
WriteScript returns a Step that writes the specified script data to a file
with the given name in the scripts directory within the package build root.




### Type PkgComponentPlist
```go
type PkgComponentPlist struct {
	RootRelativeBundlePath      string `yaml:"RootRelativeBundlePath" plist:"RootRelativeBundlePath"`
	BundleIsRelocatable         bool   `yaml:"BundleIsRelocatable" plist:"BundleIsRelocatable,omitempty"`
	BundleIsVersionChecked      bool   `yaml:"BundleIsVersionChecked" plist:"BundleIsVersionChecked,omitempty"`
	BundleHasStrictIdentifier   bool   `yaml:"BundleHasStrictIdentifier" plist:"BundleHasStrictIdentifier,omitempty"`
	BundleOverwriteAction       string `yaml:"BundleOverwriteAction" plist:"BundleOverwriteAction,omitempty"`
	BundlePreInstallScriptPath  string `yaml:"BundlePreInstallScriptPath" plist:"BundlePreInstallScriptPath,omitempty"`
	BundlePostInstallScriptPath string `yaml:"BundlePostInstallScriptPath" plist:"BundlePostInstallScriptPath,omitempty"`
	BundleInstallScriptTimeout  int    `yaml:"BundleInstallScriptTimeout" plist:"BundleInstallScriptTimeout,omitempty"`
}
```
PkgConfPkgComponentPlist represents the pkgbuild component plist structure.


### Type ProductBuild
```go
type ProductBuild struct {
	PkgBuild
	InstallLocation string // target location for the install, e.g. /
	GUIXML          string // path to the distribution XML file relative to the resources directory
}
```
ProductBuild represents the productbuild tool.

### Methods

```go
func (p ProductBuild) BuildDistribution(outputPkgPath, signingIdentity string) Step
```
BuildDistribution returns a Step that creates a product archive using
productbuild with the specified distribution XML at outputPkgPath.


```go
func (p ProductBuild) CopyResources(src ...string) []Step
```
CopyResources returns a Step that copies the specified resource to the
resources directory within the product build root.


```go
func (p ProductBuild) Create() []Step
```
Create returns steps that create the product build directory structure in
addition to those created by the embedded PkgBuild.


```go
func (p ProductBuild) Install(outputPath string) Step
```
Install returns a Step that installs the package using the system installer
command.


```go
func (p ProductBuild) ResourcesPath() string
```
ResourcesPath returns the path to the resources directory.




### Type ProductBuildResources
```go
type ProductBuildResources struct {
	GUIXML          string   `yaml:"gui_xml"`          // The distribution XML file.
	SigningIdentity string   `yaml:"signing_identity"` // The signing identity to use for signing the product.
	Resources       []string `yaml:"resources"`        // paths to additional resources to include in the product build.
	Packages        []string `yaml:"packages"`         // paths to the component packages to include in the product build.
}
```
ProductBuildResources represents the resources needed to create a
productbuild distribution.


### Type ProductPreInstallRequirements
```go
type ProductPreInstallRequirements struct {
	Raw map[string]any
}
```
ProductPreInstallRequirements represents the productbuild pre-install
requirements for synthesized packages.

### Methods

```go
func (p ProductPreInstallRequirements) MarshalPlist() (any, error)
```


```go
func (p ProductPreInstallRequirements) MarshalYAML() (any, error)
```


```go
func (p *ProductPreInstallRequirements) UnmarshalYAML(node *yaml.Node) error
```




### Type ReformatIcon
```go
type ReformatIcon struct {
	InputPath  string
	OutputPath string
}
```
ReformatIcon represents a step that reformats an icon to the specified
format.

### Methods

```go
func (j ReformatIcon) Convert(format string) Step
```
Convert converts the input image/icon to the specified format.




### Type Resources
```go
type Resources struct {
	Executable string    `yaml:"executable"`
	Icons      []IconSet `yaml:"icons"` // multiple icon sets can be specified
}
```
Resources represents the resources needed to build an app bundle.

### Methods

```go
func (r Resources) IconSetSteps() []Step
```
IconSetSteps returns the steps needed to create the icon sets specified in
the Resources.




### Type RunResult
```go
type RunResult []StepResult
```
RunResult captures the outcome of running the steps.

### Methods

```go
func (r RunResult) Error() error
```
Error returns the last error encountered, if any.




### Type SafariWebExtension
```go
type SafariWebExtension struct {
	// Name is the name of the .appex within Contents/PlugIns, without the
	// .appex suffix.
	Name string

	// Executable is the path to the built handler binary, which is copied to
	// the .appex's Contents/MacOS directory. It must be named by
	// Info.CFBundleExecutable.
	Executable string

	// Resources is the directory holding the web extension itself: its
	// manifest.json and the scripts, pages and assets that it references. Its
	// contents are copied into the .appex's Contents/Resources directory.
	Resources string

	// PrincipalClass is the NSExtensionPrincipalClass, ie. the class that
	// Safari instantiates to receive native messages from the extension, such
	// as "ExampleExtension.SafariWebExtensionHandler".
	PrincipalClass string

	// Info is the .appex's Info.plist. Its NSExtension dictionary is set by
	// AddSafariWebExtension and need not be filled in; CFBundlePackageType
	// defaults to XPC!. CFBundleIdentifier must be prefixed by the
	// identifier of the containing app, as macOS requires of any app
	// extension.
	Info InfoPlist
}
```
SafariWebExtension describes a Safari web extension that is shipped inside
an app bundle. Safari does not launch a separate native messaging host the
way Chrome and Firefox do: a web extension is packaged as an application
extension, an .appex within the containing app's Contents/PlugIns, and
messages sent by the extension with runtime.sendNativeMessage are delivered
to the principal class of that .appex. That class is the native helper.

Only Safari web extensions are supported. The older Safari app extensions,
which use the com.apple.Safari.extension extension point and do not use a
manifest.json, are rejected.

See
https://developer.apple.com/documentation/safariservices/safari-web-extensions


### Type Signer
```go
type Signer struct {
	// contains filtered or unexported fields
}
```

### Functions

```go
func NewSigner(identity string, entitlements *Entitlements, perFileEntitlements *PerFileEntitlements, arguments []string) Signer
```
NewSigner creates a new signer. The most specific entitlements for a given
path will be used. If no file specific entitlement exists, the global one
(if any) is used.



### Methods

```go
func (s Signer) SignPath(bundle, path string) Step
```
SignPath returns a Step that signs the specified path within the specified
bundle. If path is empty, the bundle itself is signed.


```go
func (s Signer) VerifyPath(bundle, path string) Step
```
VerifyPath returns a Step that verifies the signature of the specified
path within the specified bundle. If path is empty, the bundle itself is
verified.




### Type SigningConfig
```go
type SigningConfig struct {
	Identity            string               `yaml:"identity"`
	CodesignArguments   []string             `yaml:"codesign-args"`
	Entitlements        *Entitlements        `yaml:"entitlements"`
	PerFileEntitlements *PerFileEntitlements `yaml:"perfile_entitlements"`
}
```
SigningConfig represents signing related configuration that can be read from
a yaml config file.

### Methods

```go
func (s SigningConfig) Configured() bool
```
Configured reports whether signing is configured. Signing requires an
identity to be specified.


```go
func (s SigningConfig) Signer() Signer
```
Signer returns a Signer based on the configuration.




### Type Step
```go
type Step interface {
	// Run executes the step.
	Run(context.Context, *CommandRunner) (StepResult, error)
}
```
Step represents a single operation that can be executed by the StepRunner.

### Functions

```go
func ChmodAll(d string, perm os.FileMode) Step
```
ChmodAll returns a Step that recursively changes the permissions of a
directory and its contents using chmod -R.


```go
func Copy(oldname, newname string) Step
```
Copy returns a Step that copies a file using cp.


```go
func CopyDir(srcDir, dstDir string) Step
```
CopyDir returns a Step that copies a directory recursively using cp -r.


```go
func DirExists(d string) Step
```
DirExists returns a Step that checks for the existence of the directory.


```go
func ErrorStep(err error, cmd string, args ...string) Step
```


```go
func FileExists(f string) Step
```
FileExists returns a Step that checks for the existence of the file.


```go
func IsValidIconSetDir(id IconSetDir) Step
```
IsValidIsValidIconSetDir returns a Step that checks if the directory has a
.iconset extension.


```go
func MkdirAll(d string) Step
```
MkdirAll returns a Step that creates a directory and all necessary parents
using mkdir -p.


```go
func NoopStep(detail string) Step
```


```go
func Perms(path string, perm os.FileMode) Step
```
Perms returns a Step that changes the permissions of a file or directory
using chmod. Only permission, sticky, setuid, and setgid bits are passed to
chmod; file type bits (such as os.ModeDir) are masked out.


```go
func RSync(src, dst string, args ...string) Step
```
RSync returns a Step that synchronizes files and directories using rsync.


```go
func Rename(oldname, newname string) Step
```
Rename returns a Step that renames a file using mv.


```go
func RmdirAll(d string) Step
```
RmdirAll returns a Step that removes an app bundle and all its contents
using rm -rf.


```go
func StepFunc(f func(context.Context, *CommandRunner) (StepResult, error)) Step
```
StepFunc is a helper to create Steps from functions.


```go
func Symlink(target, link string) Step
```
Symlink returns a Step that creates a symbolic link pointing to target at
link using ln -s -f.


```go
func WriteFile(data []byte, perm os.FileMode, elems ...string) Step
```
WriteFile returns a Step that writes data to the specified path with the
specified permissions.


```go
func WriteJSONFile(v any, elems ...string) Step
```
WriteJSONFile returns a Step that marshals v to JSON and writes it to the
specified path with the specified permissions.


```go
func WritePlistFile(v any, elems ...string) Step
```
WritePlistFile returns a Step that marshals v to a plist and writes it to
the specified path.




### Type StepResult
```go
type StepResult struct {
	// contains filtered or unexported fields
}
```

### Functions

```go
func NewStepResult(executable string, args []string, output []byte, err error) StepResult
```



### Methods

```go
func (le *StepResult) Args() []string
```


```go
func (le *StepResult) CommandLine() string
```


```go
func (le *StepResult) Duration() time.Duration
```


```go
func (le *StepResult) Error() error
```


```go
func (le *StepResult) Executable() string
```


```go
func (le *StepResult) Output() string
```


```go
func (le *StepResult) String() string
```




### Type StepRunner
```go
type StepRunner struct {
	// contains filtered or unexported fields
}
```
StepRunner manages and executes a series of Steps.

### Functions

```go
func NewRunner(opts ...StepRunnerOption) *StepRunner
```
NewRunner creates a new StepRunner with the provided options.



### Methods

```go
func (r *StepRunner) AddSteps(steps ...Step) *StepRunner
```
AddSteps adds one or more steps to the StepRunner.


```go
func (r *StepRunner) Run(ctx context.Context, cmdRunner *CommandRunner) RunResult
```
Run executes all added steps in sequence and returns a RunResult.




### Type StepRunnerOption
```go
type StepRunnerOption func(o *stepRunnerOptions)
```
StepRunnerOption configures a StepRunner.

### Functions

```go
func WithStepTiming(timing bool) StepRunnerOption
```


```go
func WithStepVerbose(verbose bool) StepRunnerOption
```
WithStepVerbose configures the StepRunner to print each step as it is run,
including the step that fails, without the timing information that
WithStepTiming adds.




### Type Suffix
```go
type Suffix string
```

### Methods

```go
func (s Suffix) Assert(path string) Step
```
Assert returns a Step that checks if the provided path has the specified
suffix.




### Type SwiftApp
```go
type SwiftApp struct {
	// contains filtered or unexported fields
}
```
SwiftApp represents the swift build tool.

### Functions

```go
func NewSwiftApp(ctx context.Context, root string, release bool) SwiftApp
```
NewSwiftApp creates a new SwiftApp instance rooted at the specified
directory.



### Methods

```go
func (s SwiftApp) BinDir() string
```
BinDir returns the directory containing the swift build products.


```go
func (s SwiftApp) Build() Step
```
Build returns a Step that builds the swift project.


```go
func (s SwiftApp) CopyIcons(icons []IconSet) []Step
```
CopyIcons returns steps to copy the specified icons into the swift build
tree's Resources directory.


```go
func (s SwiftApp) ExecutablePath(name string) string
```
ExecutablePath returns the path to the specified executable within the swift
build tree.




### Type XPCServicePlist
```go
type XPCServicePlist struct {
	ServiceName string         `yaml:"ServiceName"`
	Extra       map[string]any `yaml:",inline"`
}
```
XPCServicePlist represents the contents of an XPCService dictionary, which
may appear within a bundle's Info.plist. As for InfoPlist, keys without a
field of their own are captured by Extra.

### Methods

```go
func (x XPCServicePlist) MarshalPlist() (any, error)
```


```go
func (x XPCServicePlist) MarshalYAML() (any, error)
```


```go
func (x XPCServicePlist) Validate() error
```
Validate reports whether the keys required of an XPCService dictionary are
present.






## Examples
### [Example_createAppBundle](https://pkg.go.dev/cloudeng.io/macos/buildtools?tab=doc#example-_createAppBundle)
This example demonstrates how to create a basic macOS application bundle
structure with Info.plist and copy resources into it.




