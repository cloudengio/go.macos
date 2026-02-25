# Package [cloudeng.io/macos/keychain/plugin](https://pkg.go.dev/cloudeng.io/macos/keychain/plugin?tab=doc)

```go
import cloudeng.io/macos/keychain/plugin
```


## Constants
### PluginBinaryDefaultName
```go
PluginBinaryDefaultName = "macos-keychain-plugin"

```
PluginBinaryDefaultName is the default name of the plugin binary.



## Functions
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
	Binary        string                 `yaml:"plugin_binary"`
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




### Type KeychainFlags
```go
type KeychainFlags struct {
	Binary  string `subcmd:"keychain-plugin,,path to the plugin binary"`
	Account string `subcmd:"keychain-account,,account that the keychain item belongs to"`
}
```
KeychainFlags are commonly required flags for working with the MacOS
keychain plugin.


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
func (f ReadFlags) Config() Config
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
Server provides of a plugin for handling plugin requests to access the
macos keychain. A plugin binary can use this to handle requests and return
responses.

### Functions

```go
func NewServer(logger *slog.Logger) *Server
```
NewServer creates a new Server with the provided logger. If logger is nil,
a default logger that discards all logs will be used.



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
func (f WriteFlags) Config() Config
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







