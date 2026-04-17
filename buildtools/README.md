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

### Func RegisterFlagsOrDie
```go
func RegisterFlagsOrDie(f any, fs *flag.FlagSet)
```
RegisterFlagsOrDie registers a struct that contains an instance of
CommonFlags with the provided FlagSet, panicing on error.



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
func (b AppBundle) Clean() Step
```
Clean returns a Step that removes the app bundle directory and all its
contents.


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
func (b AppBundle) InstallProvisioningProfile(profile string) Step
```
InstallProvisioningProfile returns a Step that copies
the provisioning profile into the app bundle. See
https://developer.apple.com/documentation/technotes/tn3125-inside-code-signing-provisioning-profiles
for an explanation of provisioning profiles.


```go
func (b AppBundle) Resources(elem ...string) string
```
Resources returns the path to the specified element within the app bundle's
Resources directory.


```go
func (b AppBundle) SPCtlAsses() Step
```


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
	AppBundle string        `yaml:"bundle"`
	Signing   SigningConfig `yaml:"signing"`
}
```
Config represents common configuration options that can be read from a yaml
config file.


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
RewriteHOME rewrites any occurrences of $HOME in the source and destination
paths to ${TARGET_HOME} which is set in the bash script preamble. Use this
with the BashInstallPreamble to access the current logged in user's home
directory since $HOME does not refer to the user's home directory from
within the installer environment.




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
	CFBundleIdentifier     string
	CFBundleName           string
	CFBundleExecutable     string
	CFBundleIconFile       string
	CFBundlePackageType    string
	LSMinimumSystemVersion string
	CFBundleDisplayName    string
	CFBundleVersion        string
	XPCService             *XPCServicePlist
	Raw                    map[string]any
}
```
InfoPlist represents the contents of a macOS Info.plist file. The struct
fields represent common keys found in such files and are extracted from the
Raw map for convenience and use within this package.

### Methods

```go
func (ipl InfoPlist) MarshalPlist() (any, error)
```


```go
func (ipl InfoPlist) MarshalYAML() (any, error)
```


```go
func (ipl *InfoPlist) UnmarshalYAML(node *yaml.Node) error
```




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
browser.


```go
func (nm *NativeMessagingConfig) ValidateChrome() error
```
ValidateChrome validates the native messaging configuration for Chrome.




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
the specified path with the specified permissions.




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
	ServiceName string
}
```
XPCServicePlist represents the contents of an XPCService dictionary within
an Info.plist file. The Raw field contains the full dictionary contents
while the ServiceName field is extracted for convenience.




## Examples
### [Example_createAppBundle](https://pkg.go.dev/cloudeng.io/macos/buildtools?tab=doc#example-_createAppBundle)
This example demonstrates how to create a basic macOS application bundle
structure with Info.plist and copy resources into it.




