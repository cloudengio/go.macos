// Copyright 2024 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

// Package keychain provides a simple interface for reading and writing
// secure notes to the macOS keychain.
package keychain

// The following are important references for working with the macOS keychain:
// https://developer.apple.com/documentation/technotes/tn3137-on-mac-keychains
// https://developer.apple.com/forums/thread/724013

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"log/slog"

	"github.com/cloudengio/go-keychain"
)

// Option represents an option for configuring a keychain.T
type Option func(o *options)

type options struct {
	updateInPlace bool
	accessibility Accessibility
	customTypes   bool
	writeType     Type
	logger        *slog.Logger
}

// WithUpdateInPlace sets the updateInPlace option for a keychain.T.
func WithUpdateInPlace(v bool) Option {
	return func(o *options) {
		o.updateInPlace = v
	}
}

// WithAccessibility sets the accessibility option for a keychain.T.
func WithAccessibility(v Accessibility) Option {
	return func(o *options) {
		o.accessibility = v
	}
}

// WithWriteType sets the read and write types for a keychain.T. The
// default is to use the type specified when a keychain.T is created for both
// reading and writing.
func WithWriteType(write Type) Option {
	return func(o *options) {
		o.customTypes = true
		o.writeType = write
	}
}

// WithLogger sets the logger for a keychain.T. The default is to use a
// logger that discards all logs.
func WithLogger(logger *slog.Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}

func (kc T) writeTypeOrDefault() Type {
	if kc.opts.customTypes {
		return kc.opts.writeType
	}
	return kc.typ
}

const (
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
)

// Accessibility is the items accessibility
type Accessibility int

const (
	AccessibleDefault                        = Accessibility(keychain.AccessibleDefault)
	AccessibleWhenUnlocked                   = Accessibility(keychain.AccessibleWhenUnlocked)
	AccessibleAfterFirstUnlock               = Accessibility(keychain.AccessibleAfterFirstUnlock)
	AccessibleAlways                         = Accessibility(keychain.AccessibleAlways)
	AccessibleWhenPasscodeSetThisDeviceOnly  = Accessibility(keychain.AccessibleWhenPasscodeSetThisDeviceOnly)
	AccessibleWhenUnlockedThisDeviceOnly     = Accessibility(keychain.AccessibleWhenUnlockedThisDeviceOnly)
	AccessibleAfterFirstUnlockThisDeviceOnly = Accessibility(keychain.AccessibleAfterFirstUnlockThisDeviceOnly)
	AccessibleAccessibleAlwaysThisDeviceOnly = Accessibility(keychain.AccessibleAccessibleAlwaysThisDeviceOnly)
)

func (a Accessibility) String() string {
	switch a {
	case AccessibleDefault:
		return "default"
	case AccessibleWhenUnlocked:
		return "when-unlocked"
	case AccessibleAfterFirstUnlock:
		return "after-first-unlock"
	case AccessibleAlways:
		return "always"
	case AccessibleWhenPasscodeSetThisDeviceOnly:
		return "when-passcode-set-this-device-only"
	case AccessibleWhenUnlockedThisDeviceOnly:
		return "when-unlocked-this-device-only"
	case AccessibleAfterFirstUnlockThisDeviceOnly:
		return "after-first-unlock-this-device-only"
	case AccessibleAccessibleAlwaysThisDeviceOnly:
		return "always-this-device-only"
	default:
		return "unknown"
	}
}

// ParseAccessibility parses a string into an Accessibility.
func ParseAccessibility(s string) (Accessibility, error) {
	switch s {
	case "default":
		return AccessibleDefault, nil
	case "when-unlocked":
		return AccessibleWhenUnlocked, nil
	case "after-first-unlock":
		return AccessibleAfterFirstUnlock, nil
	case "always":
		return AccessibleAlways, nil
	case "when-passcode-set-this-device-only":
		return AccessibleWhenPasscodeSetThisDeviceOnly, nil
	case "when-unlocked-this-device-only":
		return AccessibleWhenUnlockedThisDeviceOnly, nil
	case "after-first-unlock-this-device-only":
		return AccessibleAfterFirstUnlockThisDeviceOnly, nil
	case "always-this-device-only":
		return AccessibleAccessibleAlwaysThisDeviceOnly, nil
	default:
		return 0, fmt.Errorf("invalid accessibility: %s", s)
	}
}

func (t Type) String() string {
	switch t {
	case KeychainFileBased:
		return "file"
	case KeychainDataProtectionLocal:
		return "data-protection-local"
	case KeychainICloud:
		return "icloud"
	case KeychainAll:
		return "all"
	default:
		return "unknown"
	}
}

// ParseType parses a string into a KeychainType.
func ParseType(s string) (Type, error) {
	switch s {
	case "file", "default":
		return KeychainFileBased, nil
	case "data-protection-local", "data-protection", "local":
		return KeychainDataProtectionLocal, nil
	case "icloud":
		return KeychainICloud, nil
	case "all", "":
		return KeychainAll, nil
	default:
		return 0, fmt.Errorf("invalid keychain type: %s", s)
	}
}

// T represents a keychain that can be used to read and write secure notes.
type T struct {
	typ            Type
	opts           options
	account        string
	readonlyUpdate bool
}

func newKeychain(readonly bool, typ Type, account string, opts ...Option) *T {
	var options options
	options.accessibility = Accessibility(keychain.AccessibleWhenUnlocked)
	for _, opt := range opts {
		opt(&options)
	}
	readonlyUpdate := readonly && options.updateInPlace
	if options.logger == nil {
		options.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &T{typ: typ, readonlyUpdate: readonlyUpdate, account: account, opts: options}
}

// New creates a new Keychain.
func New(typ Type, account string, opts ...Option) *T {
	return newKeychain(false, typ, account, opts...)
}

// NewReadonly creates a new readonly Keychain.
func NewReadonly(typ Type, account string, opts ...Option) SecureNoteReader {
	return newKeychain(true, typ, account, opts...)
}

func (kc T) configure(item *keychain.Item, typ Type) {
	item.SetSecClass(keychain.SecClassGenericPassword)
	switch typ {
	case KeychainFileBased:
	case KeychainDataProtectionLocal:
		item.SetDataProtectionKeyChain(true)
	case KeychainICloud:
		item.SetSynchronizable(keychain.SynchronizableYes)
	}
}

// WriteSecureNote writes a secure note to the keychain. It will update
// an existing note if WithUpdateInPlace was set to true.
func (kc T) WriteSecureNote(service string, data []byte) error {
	if kc.readonlyUpdate {
		return fmt.Errorf("cannot write to readonly keychain, but update in place is enabled")
	}
	typ := kc.writeTypeOrDefault()
	if typ == KeychainAll {
		return fmt.Errorf("cannot write to keychain of type 'all'")
	}
	kc.opts.logger.Info("writing secure note",
		"account", kc.account,
		"service", service,
		"accessibility", kc.opts.accessibility.String(),
		"type", typ.String(),
	)
	item := keychain.NewItem()
	kc.configure(&item, typ)
	item.SetService(service)
	item.SetLabel(service)
	item.SetAccount(kc.account)
	item.SetDescription("secure note")
	item.SetData(data)
	item.SetAccessible(keychain.Accessible(kc.opts.accessibility))
	err := keychain.AddItem(item)
	if err == keychain.ErrorDuplicateItem {
		if kc.opts.updateInPlace {
			kc.opts.logger.Info("item already exists, updating in place",
				"account", kc.account,
				"service", service,
				"type", typ.String(),
			)
			return kc.updateSecureNote(service, data, typ)
		}
		err = fs.ErrExist
	}
	return err
}

// UpdateSecureNote updates an existing secure note in the keychain.
func (kc T) UpdateSecureNote(service string, data []byte) error {
	if kc.readonlyUpdate {
		return fmt.Errorf("cannot write to readonly keychain, but update in place is enabled")
	}
	kc.opts.logger.Info("updating secure note",
		"account", kc.account,
		"service", service,
		"type", kc.writeTypeOrDefault().String(),
	)
	typ := kc.writeTypeOrDefault()
	if typ == KeychainAll {
		return fmt.Errorf("cannot update keychain of type 'all'")
	}
	return kc.updateSecureNote(service, data, typ)
}

func (kc T) updateSecureNote(service string, data []byte, typ Type) error {
	item := keychain.NewItem()
	item.SetData(data)
	query := kc.queryItem(kc.account, service, typ)
	return keychain.UpdateItem(query, item)
}

func (kc T) queryItem(account, service string, typ Type) keychain.Item {
	query := keychain.NewItem()
	kc.configure(&query, typ)
	query.SetService(service)
	query.SetAccount(account)
	query.SetReturnData(true)
	query.SetMatchLimit(keychain.MatchLimitOne)
	query.SetReturnAttributes(true)
	return query
}

func (kc T) queryNote(service string, typ Type) (keychain.QueryResult, error) {
	query := kc.queryItem(kc.account, service, typ)
	results, err := keychain.QueryItem(query)
	if err != nil {
		return keychain.QueryResult{}, err
	}
	if len(results) == 0 {
		return keychain.QueryResult{}, fs.ErrNotExist
	}
	return results[0], nil
}

var searchListAll = []Type{
	KeychainICloud,
	KeychainDataProtectionLocal,
	KeychainFileBased,
}

// ReadSecureNote reads a secure note from the keychain.
func (kc T) ReadSecureNote(service string) (data []byte, err error) {
	searchList := []Type{kc.typ}
	if kc.typ == KeychainAll {
		searchList = searchListAll
	}
	kc.opts.logger.Info("reading secure note",
		"account", kc.account,
		"service", service,
		"type", kc.typ.String(),
	)
	for _, typ := range searchList {
		result, err := kc.queryNote(service, typ)
		if err != nil {
			if err == fs.ErrNotExist {
				continue
			}
			return nil, err
		}
		data, err := extractKeychainNote(result.Data)
		if err == io.EOF {
			// Maybe not an XML plist document.
			if len(result.Data) > 0 {
				return result.Data, nil
			}
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		kc.opts.logger.Info("read secure note, found item",
			"account", kc.account,
			"service", service,
			"type", typ.String(),
		)
		return data, err
	}
	return nil, fs.ErrNotExist
}

// DeleteSecureNote deletes a secure note from the keychain.
func (kc T) DeleteSecureNote(service string) error {
	if kc.readonlyUpdate {
		return fmt.Errorf("cannot write to readonly keychain, but update in place is enabled")
	}

	typ := kc.writeTypeOrDefault()
	if typ == KeychainAll {
		return fmt.Errorf("cannot delete from keychain of type 'all'")
	}
	kc.opts.logger.Info("deleting secure note",
		"account", kc.account,
		"service", service,
		"type", typ.String(),
	)
	result, err := kc.queryNote(service, typ)
	if err != nil {
		return err
	}
	item := keychain.NewItem()
	kc.configure(&item, typ)
	item.SetService(result.Service)
	item.SetAccount(result.Account)
	item.SetDescription(result.Description)
	return keychain.DeleteItem(item)
}

type plist struct {
	Dict dict `xml:"dict"`
}

type dict struct {
	Entries []entry `xml:",any"`
}

type entry struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

func extractKeychainNote(data []byte) ([]byte, error) {
	dec := xml.NewDecoder(bytes.NewBuffer(data))
	var pl plist
	if err := dec.Decode(&pl); err != nil {
		return nil, err
	}
	for i, v := range pl.Dict.Entries {
		if v.XMLName.Local == "key" && v.Value == "NOTE" {
			if i+1 < len(pl.Dict.Entries) && pl.Dict.Entries[i+1].XMLName.Local == "string" {
				return []byte(pl.Dict.Entries[i+1].Value), nil
			}
		}
	}
	return nil, fs.ErrNotExist
}
