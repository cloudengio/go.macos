# Package [cloudeng.io/macos/keychain/plugin](https://pkg.go.dev/cloudeng.io/macos/keychain/plugin?tab=doc)

```go
import cloudeng.io/macos/keychain/plugin
```


## Constants
### DefaultPluginBinary, DefaultPluginAppBundlePath
```go
// DefaultPluginBinary is the default name of the plugin binary.
DefaultPluginBinary = "macos-keychain-plugin"
// DefaultPluginAppBundlePath is the default app bundle path.
DefaultPluginAppBundlePath = "/Applications/macos-keychain-plugin.app"

```



## Functions
### Func LocatePluginBinary
```go
func LocatePluginBinary(appBundle, binary string) (string, error)
```
LocatePluginBinary attempts to locate the plugin binary by first checking
the specified app bundle and then looking in the PATH for binary.

### Func LookupPluginBinary
```go
func LookupPluginBinary(appBundle, binary string) (string, error)
```
LookupPluginBinary attempts to locate the plugin binary by first checking
the specified app bundle and then looking in the PATH for binary. If the
app bundle is not specified, it defaults to DefaultPluginAppBundlePath.
If the binary is not specified, it defaults to DefaultPluginBinary. If the
binary cannot be found in either location, an error is returned.

### Func NewRequest
```go
func NewRequest(keyname string, cfg Config) (plugins.Request, error)
```
NewRequest creates a new plugin request for the specified keyname and

### Func NewWriteRequest
```go
func NewWriteRequest(keyname string, contents []byte, cfg Config) (plugins.Request, error)
```
NewWriteRequest creates a new plugin request for writing the specified
contents to the keychain with the specified keyname and configuration.



## Types
### Type Accessibility
```go
type Accessibility keychain.Accessibility
```
Accessibility represents the accessibility level for a keychain item.
It aliases keychain.Accessibility in order to add flag.Value support.

### Methods

```go
func (a *Accessibility) Set(v string) error
```


```go
func (a *Accessibility) String() string
```




### Type Config
```go
type Config struct {
	Binary        string                 `yaml:"plugin_binary" doc:"plugin binary to use, if not specified it defaults to DefaultPluginBinary, the binary must be present in the PATH or the specified app bundle or be an absolute path"`
	UseApp        string                 `yaml:"app_bundle" doc:"app bundle that contains the plugin binary, if specified it takes precedence over Binary for locating the plugin binary, it defaults to DefaultPluginAppBundlePath"`
	Type          keychain.Type          `yaml:"keychain_type"`
	Account       string                 `yaml:"account"`
	UpdateInPlace bool                   `yaml:"update_in_place"`
	Accessibility keychain.Accessibility `yaml:"accessibility,omitempty"`
}
```
Config represents the configuration for a keychain plugin.

### Functions

```go
func DefaultConfigForReading() Config
```
DefaultConfigForReading returns a Config with default values suitable for
reading from the keychain.



### Methods

```go
func (pc Config) FS() *plugins.FS
```
FS returns a plugins.FS based on the Config. If the config does not specify
a binary DefaultPluginBinaryPath will be used.




### Type KeychainFlags
```go
type KeychainFlags struct {
	Binary  string `subcmd:"keychain-plugin,,path to the plugin binary"`
	UseApp  string `subcmd:"keychain-use-app,,'if empty, defaults to /Applications/macos-keychain-plugin.app, but can be set to any app bundle that contains the plugin binary (macos-keychain-plugin)'"`
	Account string `subcmd:"keychain-account,,account that the keychain item belongs to"`
}
```
KeychainFlags are commonly required flags for working with the MacOS
keychain plugin.


### Type Option
```go
type Option func(*options)
```
Option configures a Server created by NewServer.

### Functions

```go
func WithLogger(logger *slog.Logger) Option
```
WithLogger sets the logger for the Server. If no logger is provided,
a default logger that discards all logs will be used.




### Type ReadFlags
```go
type ReadFlags struct {
	KeychainFlags
	// Note that the default value is 'all' for reading but 'icloud' for writing.
	Type ReadType `subcmd:"keychain-type,all,'the type of keychain plugin to use: file, data-protection, icloud or all'"`
}
```
ReadFlags are used for reading from the keychain plugin.

### Methods

```go
func (f ReadFlags) Config() (Config, error)
```




### Type ReadType
```go
type ReadType keychain.Type
```
ReadType represents the type of keychain plugin to use for reading.
It aliases keychain.Type in order to add flag.Value support.

### Methods

```go
func (t *ReadType) Set(v string) error
```


```go
func (t *ReadType) String() string
```




### Type Server
```go
type Server struct {
	// contains filtered or unexported fields
}
```
Server provides a plugin for handling plugin requests to access the macos
keychain. A plugin binary can use this to handle requests and return
responses.

### Functions

```go
func NewServer(opts ...Option) *Server
```
NewServer creates a new Server with the provided options.



### Methods

```go
func (ps *Server) HandleRequest(ctx context.Context, cfg *Config, req plugins.Request) *plugins.Response
```
HandleRequest handles the provided plugin request and returns a response.
This implements the interaction with the actual OS keychain.


```go
func (ps *Server) ReadRequest(ctx context.Context, rd io.Reader) (*Config, plugins.Request, *plugins.Response)
```
ReadRequest reads a plugin request from the provided reader and returns the
request. If any errors are encountered then the returned response represents
an error and should be returned to the plugin caller. Otherwise the response
is nil.


```go
func (ps *Server) SendResponse(ctx context.Context, w io.Writer, resp *plugins.Response)
```
SendResponse sends the provided response to the plugin caller.




### Type WriteFlags
```go
type WriteFlags struct {
	KeychainFlags
	// Note that the default value is 'all' for reading but 'icloud' for writing.
	Type          WriteType     `subcmd:"keychain-type,icloud,'the type of keychain plugin to use: file, data-protection or icloud'"`
	UpdateInPlace bool          `subcmd:"keychain-update-in-place,false,set to true to update existing note in place"`
	Accessibility Accessibility `subcmd:"keychain-accessibility,,optional accessibility level for the keychain item"`
}
```
WriteFlags are used for writing to the keychain plugin.

### Methods

```go
func (f WriteFlags) Config() (Config, error)
```




### Type WriteType
```go
type WriteType keychain.Type
```
WriteType represents the type of keychain plugin to use for writing.
It aliases keychain.Type in order to add flag.Value support.

### Methods

```go
func (t *WriteType) Set(v string) error
```


```go
func (t *WriteType) String() string
```







