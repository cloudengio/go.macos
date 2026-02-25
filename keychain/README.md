# Package [cloudeng.io/macos/keychain](https://pkg.go.dev/cloudeng.io/macos/keychain?tab=doc)

```go
import cloudeng.io/macos/keychain
```

Package keychain provides a simple interface for reading and writing secure
notes to the macOS keychain.

## Constants
### AccessibleDefault, AccessibleWhenUnlocked, AccessibleAfterFirstUnlock, AccessibleAlways, AccessibleWhenPasscodeSetThisDeviceOnly, AccessibleWhenUnlockedThisDeviceOnly, AccessibleAfterFirstUnlockThisDeviceOnly, AccessibleAccessibleAlwaysThisDeviceOnly
```go
AccessibleDefault = Accessibility(keychain.AccessibleDefault)
AccessibleWhenUnlocked = Accessibility(keychain.AccessibleWhenUnlocked)
AccessibleAfterFirstUnlock = Accessibility(keychain.AccessibleAfterFirstUnlock)
AccessibleAlways = Accessibility(keychain.AccessibleAlways)
AccessibleWhenPasscodeSetThisDeviceOnly = Accessibility(keychain.AccessibleWhenPasscodeSetThisDeviceOnly)
AccessibleWhenUnlockedThisDeviceOnly = Accessibility(keychain.AccessibleWhenUnlockedThisDeviceOnly)
AccessibleAfterFirstUnlockThisDeviceOnly = Accessibility(keychain.AccessibleAfterFirstUnlockThisDeviceOnly)
AccessibleAccessibleAlwaysThisDeviceOnly = Accessibility(keychain.AccessibleAccessibleAlwaysThisDeviceOnly)

```



## Types
### Type Accessibility
```go
type Accessibility int
```
Accessibility is the items accessibility

### Functions

```go
func ParseAccessibility(s string) (Accessibility, error)
```
ParseAccessibility parses a string into an Accessibility.



### Methods

```go
func (a Accessibility) String() string
```




### Type Option
```go
type Option func(o *options)
```
Option represents an option for configuring a keychain.T

### Functions

```go
func WithAccessibility(v Accessibility) Option
```
WithAccessibility sets the accessibility option for a keychain.T.


```go
func WithUpdateInPlace(v bool) Option
```
WithUpdateInPlace sets the updateInPlace option for a keychain.T.




### Type SecureNoteReader
```go
type SecureNoteReader interface {
	ReadSecureNote(service string) (data []byte, err error)
}
```
SecureNoteReader defines the interface for reading secure notes from the
keychain.

### Functions

```go
func NewReadonly(typ Type, account string, opts ...Option) SecureNoteReader
```
NewReadonly creates a new readonly Keychain.




### Type T
```go
type T struct {
	// contains filtered or unexported fields
}
```
T represents a keychain that can be used to read and write secure notes.

### Functions

```go
func New(typ Type, account string, opts ...Option) *T
```
New creates a new Keychain.



### Methods

```go
func (kc T) DeleteSecureNote(service string) error
```
DeleteSecureNote deletes a secure note from the keychain.


```go
func (kc T) ReadFile(service string) ([]byte, error)
```


```go
func (kc T) ReadFileCtx(_ context.Context, service string) ([]byte, error)
```


```go
func (kc T) ReadSecureNote(service string) (data []byte, err error)
```
ReadSecureNote reads a secure note from the keychain.


```go
func (kc T) UpdateSecureNote(service string, data []byte) error
```
UpdateSecureNote updates an existing secure note in the keychain.


```go
func (kc T) WriteFile(service string, data []byte, _ fs.FileMode) error
```


```go
func (kc T) WriteFileCtx(_ context.Context, service string, data []byte, _ fs.FileMode) error
```


```go
func (kc T) WriteSecureNote(service string, data []byte) error
```
WriteSecureNote writes a secure note to the keychain. It will update an
existing note if it WithUpdateInPlace was set to true.




### Type Type
```go
type Type int
```
Type represents the type of keychain to use.

### Constants
### KeychainFileBased, KeychainDataProtectionLocal, KeychainICloud, KeychainAll
```go
// KeychainFileBased represents the file-based keychain.
// This is the legacy, local only, file based keychain.
KeychainFileBased Type = iota
// KeychainDataProtectionLocal represents the data protection
// keychain which is local, but integrated with the system's secure
// enclave. Applications that use must be signed and have
// appropriate entitlements.
KeychainDataProtectionLocal
// KeychainICloud represents the iCloud keychain that can be synced
// across devices.
// Applications that use must be signed and have appropriate
// entitlements.
KeychainICloud
// KeychainAll represents any keychain type, it can only be used for
// reading and indicates that all keychains will be searched for
// the requested item.
KeychainAll

```



### Functions

```go
func ParseType(s string) (Type, error)
```
ParseType parses a string into a KeychainType.



### Methods

```go
func (t Type) String() string
```







