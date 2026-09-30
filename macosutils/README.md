# Package [cloudeng.io/macos/macosutils](https://pkg.go.dev/cloudeng.io/macos/macosutils?tab=doc)

```go
import cloudeng.io/macos/macosutils
```

Package macosutils contains macos specific utilities.

## Variables
### ErrFailedToLaunch, ErrLaunchedAppFailed, ErrAlreadyLaunched
```go
ErrFailedToLaunch = errors.New("failed to launch application")
ErrLaunchedAppFailed = errors.New("launched application failed")
ErrAlreadyLaunched = errors.New("application already launched")

```



## Functions
### Func ExecutablePath
```go
func ExecutablePath() (string, error)
```
ExecutablePath returns the path of the executable that started the current
process, following softlinks.

Caveat for sandboxed processes started with a relative path: this is
os.Executable(), which, on Darwin, returns the path exactly as passed to
execve, resolving it against the current directory only if it was relative.
The App Sandbox changes the process's current directory to its container
(e.g. ~/Library/Containers/<bundle-id>/Data) as part of exec, before any Go
code runs, so by the time ExecutablePath resolves a relative path, it joins
it with the container directory, not the directory the process was actually
launched from. The result is a path that does not exist. There is no way to
recover the original directory once this has happened.

This only affects a sandboxed binary launched with a relative path,
for example by running "build/Foo.app/Contents/MacOS/Foo" directly from
a shell to see its output live rather than via Console.app. It does not
affect `open`, even given a relative path, since `open` resolves the path
to absolute itself before launching the app. Launching with an absolute path
avoids the problem entirely.

### Func InBundle
```go
func InBundle(path string, parents ...string) (string, bool)
```
InBundle returns true if the specified path has the specified parents and
the top-level parent is an app bundle, that is ends in .app and contains a
Contents/Info.plist file.

### Func IsAppBundle
```go
func IsAppBundle(path string) bool
```
IsAppBundle returns true if path is a directory ending with .app and
contains a Contents/Info.plist file.

### Func IsExecutable
```go
func IsExecutable(mode fs.FileMode) bool
```
IsExecutable returns true if the provided file mode has any of the
executable bits set (ie mode&0o111 != 0).

### Func IsReadable
```go
func IsReadable(mode fs.FileMode) bool
```
IsReadable returns true if the provided file mode has any of the readable
bits set (ie mode&0o444 != 0).

### Func IsServiceInstalled
```go
func IsServiceInstalled(serviceLabel string) bool
```
IsServiceInstalled returns true if the specified service is installed in the
current user's LaunchAgents directory.

### Func LocateInBundle
```go
func LocateInBundle(bundlePath, filename string, matchPerms func(fs.FileMode) bool) (string, bool)
```
LocateInBundle finds the requested file whose permissions are matched by
the matchPerms function, eg. use IsExecutable to find any file with an
executable bit set. It will descend into subpackages to locate the requested
file. If matchPerms is nil, IsExecutable is used. The returned path is
absolute.

### Func LocateInNestedBundle
```go
func LocateInNestedBundle(bundle, filename string, matchPerms func(fs.FileMode) bool, parents ...string) (string, bool)
```
LocateInNestedBundle finds the requested file in its immediately enclosing
bundle specified by bundle, and if not found, then in the bundle enclosing
that one, and so on, until a match is found or a non-bundle directory is
reached. The returned path is absolute. The parents are the expected names
of the enclosing directories, starting with the top-level directory directly
inside the bundle and ending with the immediate parent of bundle (i.e.
top-down order, matching InBundle). If any of the parents do not match,
or if the top-level parent is not a bundle, then no match is found. The
search stops at the first match, so if a file exists in multiple bundles,
only the innermost one is returned.

This function is useful when a file may be located in a bundle nested inside
another bundle, and you want to find it starting from the inner bundle and
searching outwards. For example, if you have an app bundle that contains a
nested framework bundle, and you want to locate a resource file that may
be in either the framework or the app bundle, you can use this function to
search for it starting from the framework bundle.

### Func LookPathBundle
```go
func LookPathBundle(bundle, pathList string) string
```
LookPathBundle is like exec.LookPath but for app bundles.

### Func LookPathBundleAll
```go
func LookPathBundleAll(bundle, pathList string) []string
```
LookPathBundleAll is like LookPathBundle but returns all instances of bundle
on pathList without duplicates.

### Func LookupBundleBinary
```go
func LookupBundleBinary(bundle, binary, pathList string) (string, string, bool)
```
LookupBundleBinary iterates over all instances of bundle in pathList to
locate the first one that contains binary returning the absolute pathname of
the bundle and binary in that bundle or empty strings if not found.

### Func ProcessInBundle
```go
func ProcessInBundle() (string, bool)
```
ProcessInBundle determines if the executable that started the running
process is within an app bundle and returns the path of that bundle.
It uses InBundle(executable, "Contents", "MacOS") as the heuristic;
use LocateInBundle to then find a file within the returned bundle.

A bundle nested inside another must therefore be placed in the outer
bundle's Contents/MacOS, ie. <outer>.app/Contents/MacOS/<inner>.app,
so that the same heuristic resolves the inner bundle to the outer one.
A bundle placed in Contents/Library is reachable by LocateInBundle, which
walks the whole tree, but not by InBundle or ProcessInBundle.

For a sandboxed process, this inherits the caveat documented on
ExecutablePath: if the process was launched with a relative path directly
(not via `open`), the executable path, and so the bundle path, will be
wrong, and ProcessInBundle will report false negatives (or, in principle,
match an unrelated bundle that happens to exist at the bogus path).

### Func TailBytes
```go
func TailBytes(filename string, n int) ([]byte, error)
```
TailBytes returns the last n bytes of the specified file. If n <= 0,
it returns nil, nil.



## Types
### Type LaunchOption
```go
type LaunchOption func(o *launchOptions)
```
LaunchOption defines a function type for configuring the Launcher.

### Functions

```go
func WithCmdEnv(env func() []string) LaunchOption
```
WithCmdEnv sets the environment variables for the command to be launched.


```go
func WithStdoutStderr(stdout, stderr io.Writer) LaunchOption
```
WithStdoutStderr sets the stdout and stderr writers for the launched
command.


```go
func WithWorkingDir(dir string) LaunchOption
```
WithWorkingDir sets the working directory for the command to be launched.




### Type Launcher
```go
type Launcher struct {
	// contains filtered or unexported fields
}
```
Launcher provides an interface for launching macOS applications.

### Functions

```go
func NewLauncher(opts ...LaunchOption) *Launcher
```
NewLauncher creates a new Launcher with the provided options.



### Methods

```go
func (l *Launcher) LaunchApp(ctx context.Context, cmd string, args ...string) error
```
LaunchApp launches a long-running application and waits for it to exit.
Use TerminateLaunchedApp to signal the application to exit. Interrupt and
terminate signals are forwarded to the launched application.


```go
func (l *Launcher) RunApp(ctx context.Context, cmd string, args ...string) (string, error)
```
RunApp executes the specified command with the provided arguments and
returns its combined output. Use it for short running commands, eg.
to configure or setup the application.


```go
func (l *Launcher) TerminateLaunchedApp() bool
```







