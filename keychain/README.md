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

### MaxRegexpLength
```go
MaxRegexpLength = 2048

```
MaxRegexpLength is the maximum allowed length in bytes for a regular
expression pattern in a FilterSpec.

### SecureNoteDescription
```go
SecureNoteDescription = "secure note"

```
SecureNoteDescription is the description attribute used by WriteSecureNote
for the items that it creates. Secure notes created by the macOS Keychain
Access application use the same description and are hence matched by
Filter.SecureNotesOnly also.



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
func (Accessibility) EnumValues() map[string]Accessibility
```
EnumValues satisfies flags.EnumType[Accessibility].


```go
func (a Accessibility) String() string
```




### Type Attributes
```go
type Attributes struct {
	Keychain    Type   `json:"keychain" yaml:"keychain"`
	Service     string `json:"service" yaml:"service"`
	Account     string `json:"account" yaml:"account"`
	Label       string `json:"label,omitempty" yaml:"label,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Comment     string `json:"comment,omitempty" yaml:"comment,omitempty"`
	AccessGroup string `json:"access_group,omitempty" yaml:"access_group,omitempty"`

	// Accessibility names when the item's contents can be decrypted, eg.
	// when-unlocked, as one of the values accepted by ParseAccessibility. It
	// is recorded for items in the data-protection and iCloud keychains; an
	// item in the file-based keychain, whose access is governed by an access
	// control list rather than by this attribute, reports an empty value.
	Accessibility string `json:"accessibility,omitempty" yaml:"accessibility,omitempty"`

	Created  time.Time `json:"created,omitzero" yaml:"created,omitempty"`
	Modified time.Time `json:"modified,omitzero" yaml:"modified,omitempty"`
}
```
Attributes describes a single keychain item: the attributes that identify
and describe it, but never its contents. Query never asks the keychain for
item data and hence Attributes has no field to report it in.

### Functions

```go
func Query(typ Type, filter Filter, opts ...Option) ([]Attributes, error)
```
Query returns the attributes of every item in the requested keychain(s) that
matches filter, for all accounts. Use New followed by T.Query to restrict
the search to a single account.



### Methods

```go
func (a Attributes) IsSecureNote() bool
```
IsSecureNote returns true if the item's description identifies it as a
secure note.




### Type Filter
```go
type Filter struct {
	// Any matches an item if any one of its service, account, label,
	// description or comment attributes matches.
	Any         *regexp.Regexp
	Service     *regexp.Regexp
	Account     *regexp.Regexp
	Label       *regexp.Regexp
	Description *regexp.Regexp
	Comment     *regexp.Regexp

	// SecureNotesOnly restricts the results to secure notes, ie. to items
	// whose description is SecureNoteDescription.
	SecureNotesOnly bool
}
```
Filter selects keychain items by matching regular expressions against their
attributes. All of the criteria that are set must match, ie. they are
and'ed together, and a zero Filter matches every item. The expressions are
unanchored and case sensitive, use ^, $ and (?i) as required.

### Methods

```go
func (f Filter) Matches(attrs Attributes) bool
```
Matches returns true if attrs satisfies every criterion set in the filter.




### Type FilterSpec
```go
type FilterSpec struct {
	Any             string `json:"any,omitempty" yaml:"any,omitempty"`
	Service         string `json:"service,omitempty" yaml:"service,omitempty"`
	Account         string `json:"account,omitempty" yaml:"account,omitempty"`
	Label           string `json:"label,omitempty" yaml:"label,omitempty"`
	Description     string `json:"description,omitempty" yaml:"description,omitempty"`
	Comment         string `json:"comment,omitempty" yaml:"comment,omitempty"`
	SecureNotesOnly bool   `json:"secure_notes_only,omitempty" yaml:"secure_notes_only,omitempty"`
}
```
FilterSpec is the uncompiled, and hence serializable, form of a Filter.
A query that is serviced by an out-of-process keychain plugin is sent as a
FilterSpec, the plugin compiling the expressions itself since a compiled
Filter cannot cross a process boundary.

### Methods

```go
func (s FilterSpec) Compile() (Filter, error)
```
Compile compiles the spec's regular expressions into a Filter. An expression
that cannot be compiled is reported as an error that names the attribute it
applies to, ie. one of any, service, account, label, description or comment;
the names are those of the spec's fields, lowercased, so that a caller whose
flags are named after them can report the error against the offending flag.




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
func WithLogger(logger *slog.Logger) Option
```
WithLogger sets the logger for a keychain.T. The default is to use a logger
that discards all logs.


```go
func WithUpdateInPlace(v bool) Option
```
WithUpdateInPlace sets the updateInPlace option for a keychain.T.


```go
func WithWriteType(write Type) Option
```
WithWriteType sets the read and write types for a keychain.T. The default is
to use the type specified when a keychain.T is created for both reading and
writing.




### Type Properties
```go
type Properties struct {
	Account       string `json:"account,omitempty" yaml:"account,omitempty"`
	Label         string `json:"label,omitempty" yaml:"label,omitempty"`
	Description   string `json:"description,omitempty" yaml:"description,omitempty"`
	Accessibility string `json:"accessibility,omitempty" yaml:"accessibility,omitempty"`
}
```
Properties are the attributes of a keychain item that SetProperties can
change. An empty field leaves the attribute it names as it is; there is no
way to clear an attribute, only to replace it.

Accessibility is named by one of the values that ParseAccessibility accepts.
It applies only to items in the data-protection and iCloud keychains;
an item in the file-based keychain has no such attribute, its access being
governed by an access control list instead, and the keychain silently
ignores an attempt to set one. SetProperties reports that as an error rather
than appearing to have made a change that it has not.

### Methods

```go
func (p Properties) Empty() bool
```
Empty returns true if no property is to be changed.


```go
func (p Properties) Validate(typ Type) error
```
Validate returns an error if the properties cannot be applied to an item
in the keychain of the requested type: if they would change nothing, if the
accessibility they name is not one of those that ParseAccessibility accepts,
or if they set an accessibility for an item in a keychain that has no such
attribute. It is used by SetProperties, and by the in-memory keychain that
stands in for it in tests, so that both refuse the same things.




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
func (kc T) Query(filter Filter) ([]Attributes, error)
```
Query returns the attributes of the items in the keychain that match filter.
If the keychain was created with a non-empty account then only that
account's items are searched, the match being an exact one performed by the
keychain itself; filter.Account can be used to match accounts by regular
expression instead.

Item contents are never requested from the keychain and hence are never
returned. As well as keeping secrets out of the results this avoids the
authorization prompts that reading an item's contents may otherwise trigger.

A type of KeychainAll searches the iCloud, data-protection and file-based
keychains; an item that is present in more than one of them is reported once
per keychain. A keychain that cannot be searched, eg. the data-protection
keychain when the calling binary lacks the entitlements it requires,
is logged and skipped; an error is returned only if every keychain searched
failed.

The results are sorted by service, account and then keychain.


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
func (kc T) SetProperties(service string, props Properties) error
```
SetProperties changes the attributes of the single item identified
by service and by the account the keychain was created with. Only the
attributes that props names are changed; the item's contents are neither
read nor written.

The keychain's write type identifies the keychain holding the item, so a
type of KeychainAll is an error, as is a props that would change nothing.
An item that does not exist, or that is in a keychain this binary is not
entitled to modify, is reported as not existing.


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
existing note if WithUpdateInPlace was set to true.




### Type Type
```go
type Type int
```
Type represents the type of keychain to use, it maps to the underlying
system keychain types but also includes the pseudotype 'all' which can be
used, only when reading, to specify all keychains.

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
func (Type) EnumValues() map[string]Type
```
EnumValues satisfies flags.EnumType[Type].


```go
func (t Type) MarshalJSON() ([]byte, error)
```


```go
func (t Type) MarshalText() ([]byte, error)
```


```go
func (t Type) MarshalYAML() (any, error)
```


```go
func (t Type) String() string
```


```go
func (t *Type) UnmarshalJSON(data []byte) error
```


```go
func (t *Type) UnmarshalText(text []byte) error
```


```go
func (t *Type) UnmarshalYAML(node *yaml.Node) error
```




### Type WriteType
```go
type WriteType int
```
WriteType is like Type, except that it does not allow the value 'all'.

### Methods

```go
func (WriteType) EnumValues() map[string]WriteType
```
EnumValues satisfies flags.EnumType[WriteType].


```go
func (t WriteType) String() string
```







