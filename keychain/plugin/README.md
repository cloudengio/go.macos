# Package [cloudeng.io/macos/keychain/plugin](https://pkg.go.dev/cloudeng.io/macos/keychain/plugin?tab=doc)

```go
import cloudeng.io/macos/keychain/plugin
```


## Constants
### DefaultPluginBinary, DefaultKeyChainAppBundle, DefaultKeyChainPluginBundle
```go
// DefaultPluginBinary is the default name of the plugin binary.
DefaultPluginBinary = "cloudeng-keychain-plugin"
// DefaultKeyChainAppBundle is the default app bundle path for the keychain
// command that has the keychain plugin nested inside it.
DefaultKeyChainAppBundle = "cloudeng-keychain.app"
// DefaultKeyChainPluginBundle is the default app bundle path for the keychain
// plugin binary. This is used when the plugin is not nested inside the host
// app bundle.
DefaultKeyChainPluginBundle = "cloudeng-keychain-plugin.app"

```



## Functions
### Func BundledPluginApp
```go
func BundledPluginApp(pluginBinary string) (string, bool)
```
BundledPluginApp returns the path to the executable binary of the plugin
app bundle nested inside the app bundle that contains the currently
running executable, and whether it was found. This lets a host app
(e.g. the keychain client) ship the plugin as a nested bundle at
<host>.app/Contents/MacOS/macos-keychain-plugin.app and use it with no
configuration.

The running executable is resolved through symlinks first: it may be invoked
via a symlink (gobundle creates one, and PATH entries are often symlinks),
and only the resolved path reveals the real location inside the bundle.
The executable is expected at <host>.app/Contents/MacOS/<name>, so the
plugin bundle is a sibling under Contents/MacOS.

### Func LocateKeychainBinaryInAppBundle
```go
func LocateKeychainBinaryInAppBundle(appBundle, binary string) (string, string, error)
```
LocateKeychainBinaryInAppBundle finds an app bundle by name that contains
the specified binary. If appBundle is an absolute path, it is checked
directly. Otherwise, /Applications and each directory in $PATH are searched.
The bundle must contain an executable file named binary somewhere within
its directory tree. If found, the path to the bundle and the absolute path
to the binary are returned. If not found, an error is returned containing
exec.ErrNotFound.

### Func LocatePluginBinary
```go
func LocatePluginBinary(keychainBundle, pluginBundle, pluginBinary string) (string, error)
```
LocatePluginBinary attempts to locate the plugin binary by first looking
for thje keychain and plugin bundles that contain the plugin binary
and then looking in the PATH for the binary itself. keychainBundle,
pluginBundle and pluginBinary default to DefaultKeyChainAppBundle,
DefaultKeyChainPluginBundle and DefaultPluginBinary respectively if not
specified.

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
### Type Config
```go
type Config struct {
	Binary               string                 `yaml:"plugin_binary" doc:"plugin binary to use, if not specified it defaults to DefaultPluginBinary, the binary must be present in the PATH or the specified app bundle or be an absolute path" json:"-"`
	KeychainBundle       string                 `yaml:"keychain_app_bundle" doc:"app bundle that contains the plugin binary, if specified it takes precedence over Binary for locating the plugin binary, it defaults to DefaultPluginAppBundlePath" json:"-"`
	KeychainPluginBundle string                 `yaml:"keychain_plugin_bundle" doc:"app bundle that contains the plugin binary, if specified it takes precedence over Binary for locating the plugin binary, it defaults to DefaultKeyChainPluginBundlePath" json:"-"`
	Type                 keychain.Type          `yaml:"keychain_type" doc:"the type of keychain to use, currently supported types are: file, data-protection and icloud" json:"type"`
	WriteType            keychain.Type          `yaml:"keychain_write_type" doc:"the type of keychain to use for writing, currently supported types are: file, data-protection and icloud, this is needed because the 'all' type is not valid for writing" json:"write_type"`
	Account              string                 `yaml:"account" doc:"account that the keychain item belongs to" json:"account"`
	UpdateInPlace        bool                   `yaml:"update_in_place" doc:"set to true to update existing item in place" json:"update_in_place,omitempty"`
	Accessibility        keychain.Accessibility `yaml:"accessibility,omitempty" doc:"optional accessibility level for the keychain item" json:"accessibility,omitempty"`
	OnlyUsePlugin        bool                   `yaml:"only_use_plugin" doc:"require the keychain plugin; if false and no plugin binary is found the keychain is accessed directly, in-process" json:"-"`

	// Op is the operation requested of the plugin; its zero value reads or
	// writes an item's contents. It is set by Query and SetProperties and
	// acted on by Server.HandleRequest. Op and the two fields below are not
	// part of the configuration read from a yaml file.
	Op Op `yaml:"-" json:"op,omitzero"`

	// QueryFilter is the filter for OpQuery. Its zero value matches every
	// item, which is why Op, rather than this field's presence, is what
	// selects the operation.
	QueryFilter keychain.FilterSpec `yaml:"-" json:"query_filter,omitzero"`

	// Properties are the attributes to be changed by OpSetProperties. Its
	// zero value changes nothing, which SetProperties refuses.
	Properties keychain.Properties `yaml:"-" json:"properties,omitzero"`

	// Direct requires that the keychain be accessed in-process, ie. that no
	// plugin be run even when one can be found. It is not part of the
	// configuration read from a yaml file.
	Direct bool `yaml:"-" json:"-"`
}
```
Config represents the configuration for a keychain plugin. It is also used
as the PluginSpecific field in plugin requests and responses.

### Functions

```go
func DefaultConfigForReadWrite() Config
```
DefaultConfigForWriting returns a Config with default values suitable for
both reading and writing to the keychain.


```go
func DefaultConfigForReading() Config
```
DefaultConfigForReading returns a Config with default values suitable for
reading from the keychain.



### Methods

```go
func (pc Config) FS(writable bool) (FS, error)
```
FS returns the FS used to read and write keychain items. It locates the
plugin binary (see LookupPluginBinary); if found, access goes through the
out-of-process plugin. If the plugin binary is not found and OnlyUsePlugin
is false, it falls back to accessing the keychain directly, in-process — the
same access the plugin itself performs. "Not found" covers both a plugin
that is not on the PATH (exec.ErrNotFound) and an explicitly specified path
that does not exist (fs.ErrNotExist). If OnlyUsePlugin is true, a missing
plugin is an error, as is any other lookup failure.


```go
func (pc Config) Query(ctx context.Context, spec keychain.FilterSpec) ([]keychain.Attributes, string, error)
```
Query returns the attributes of the keychain items that match spec, together
with the path of the plugin binary that serviced the query, or an empty path
if the keychain was queried directly, in-process. Item contents are never
requested and so are never returned.

The plugin binary is located exactly as it is for FS, and a query is
serviced by the plugin whenever one is found. This matters for more than
uniformity: the data-protection and iCloud keychains can only be searched by
a binary that holds the keychain-access-groups entitlement, which the plugin
holds and its caller does not, so a query of those keychains that is made
directly finds nothing at all. The items that the plugin writes there belong
to the plugin's access group and are only visible to the plugin.


```go
func (pc Config) SetProperties(ctx context.Context, service string, props keychain.Properties) (string, error)
```
SetProperties changes the attributes of the single keychain item named by
service, returning the path of the plugin binary that made the change,
or an empty path if it was made in-process. The item's contents are neither
read nor written.

The plugin binary is located exactly as it is for FS and Query,
and the change is made by the plugin whenever one is found: an item in the
data-protection or iCloud keychain can only be modified by a binary holding
the keychain-access-groups entitlement, which the plugin holds and its
caller does not.




### Type FS
```go
type FS interface {
	ReadFile(name string) ([]byte, error)
	ReadFileCtx(ctx context.Context, name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
	WriteFileCtx(ctx context.Context, name string, data []byte, mode fs.FileMode) error
}
```
FS reads and writes keychain items. It is implemented both by the
out-of-process plugin (plugins.FS) and by direct, in-process keychain access
(keychain.T). It is a superset of file.ReadFileFS, so it can be passed to
APIs such as keys.ReadYAML.


### Type KeychainFlags
```go
type KeychainFlags struct {
	Binary               string `subcmd:"keychain-plugin,,'direct path to the plugin binary; leave empty to use the default'"`
	KeychainBundle       string `subcmd:"keychain-app-bundle,,'bundle that contains the plugin binary, use directly if the bundle is an absolute path, otherwise look for in /Applications and $PATH; leave empty to use the default.'"`
	KeychainPluginBundle string `subcmd:"keychain-plugin-bundle,,'bundle that contains the plugin binary, use directly if the bundle is an absolute path, otherwise look for in /Applications and $PATH; leave empty to use the default'"`
	Account              string `subcmd:"keychain-account,,account that the keychain item belongs to"`
	OnlyUsePlugin        bool   `subcmd:"keychain-only-use-plugin,false,'require the keychain plugin; if false and no plugin binary is found the keychain is accessed directly, in-process'"`
}
```
KeychainFlags are commonly required flags for working with the MacOS
keychain plugin.

### Methods

```go
func (f KeychainFlags) WriteConfig(typ keychain.WriteType) Config
```
WriteConfig returns a Config for modifying the keychain of the requested
type, for use by a command that changes an item without writing its contents
and hence has no accessibility of its own to apply. The account is not
defaulted to the current user as it is by pluginConfig: a command that
identifies an item by its account must say which account it means.




### Type Op
```go
type Op string
```
Op names the operation that a request asks of the plugin. It is carried
explicitly rather than inferred from the presence of the fields that each
operation needs: those fields cross the wire through encoding/json/v2,
whose omitempty and omitzero omit a struct that is entirely zero,
so an operation inferred from such a field would silently become a different
operation whenever all of its parameters happened to be empty.

### Constants
### OpAccess, OpQuery, OpSetProperties
```go
// OpAccess reads or writes the contents of the item named by the
// request's keyname, as determined by Request.Write. It is the zero
// value, so a request that names no operation, including one from a
// client that predates Op, asks for this.
OpAccess Op = ""
// OpQuery searches for the items whose attributes match a filter and
// returns those attributes.
OpQuery Op = "query"
// OpSetProperties changes the attributes of the item named by the
// request's keyname.
OpSetProperties Op = "set-properties"

```




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
	Type flags.Enum[keychain.Type] `subcmd:"keychain-type,all,'the type of keychain plugin to use'"`
}
```
ReadFlags are used for reading from the keychain plugin.

### Methods

```go
func (f ReadFlags) Config() (Config, error)
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
func (ps *Server) ReadRequest(ctx context.Context, msgr *jsonmsgs.Messager) (*Config, plugins.Request, *plugins.Response)
```
ReadRequest reads a plugin request from the provided messager and returns
the request. If any errors are encountered then the returned response
represents an error and should be returned to the plugin caller. Otherwise
the response is nil.


```go
func (ps *Server) SendResponse(_ context.Context, msgr *jsonmsgs.Messager, resp *plugins.Response)
```
SendResponse sends the provided response to the plugin caller.




### Type WriteFlags
```go
type WriteFlags struct {
	KeychainFlags
	// Note that the default value is 'all' for reading but 'icloud' for writing.
	// 'all' is not accepted here: it names a search across all keychains.
	Type          flags.Enum[keychain.WriteType]     `subcmd:"keychain-type,icloud,'the type of keychain plugin to use: data-protection-local, file or icloud'"`
	UpdateInPlace bool                               `subcmd:"keychain-update-in-place,false,set to true to update existing note in place"`
	Accessibility flags.Enum[keychain.Accessibility] `subcmd:"keychain-accessibility,when-unlocked,optional accessibility level for the keychain item"`
}
```
WriteFlags are used for writing to the keychain plugin.

### Methods

```go
func (f WriteFlags) Config() (Config, error)
```







