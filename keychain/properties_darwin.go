// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package keychain

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/cloudengio/go-keychain"
)

// Validate returns an error if the properties cannot be applied to an item
// in the keychain of the requested type: if they would change nothing, if
// the accessibility they name is not one of those that ParseAccessibility
// accepts, or if they set an accessibility for an item in a keychain that
// has no such attribute. It is used by SetProperties, and by the in-memory
// keychain that stands in for it in tests, so that both refuse the same
// things.
func (p Properties) Validate(typ Type) error {
	if typ == KeychainAll {
		return fmt.Errorf("cannot set properties in keychain of type 'all'")
	}
	if p.Empty() {
		return fmt.Errorf("no properties to set")
	}
	if _, named, err := p.accessibility(); err != nil {
		return err
	} else if named && typ == KeychainFileBased {
		// The file-based keychain has no accessibility attribute and
		// silently ignores one, which would leave the caller believing that
		// the item had been changed.
		return fmt.Errorf("accessibility cannot be set for an item in the file-based keychain, its access is governed by an access control list instead")
	}
	return nil
}

// accessibility returns the Accessibility that p names and whether one was
// named at all.
func (p Properties) accessibility() (Accessibility, bool, error) {
	if len(p.Accessibility) == 0 {
		return AccessibleDefault, false, nil
	}
	accessible, err := ParseAccessibility(p.Accessibility)
	if err != nil {
		return AccessibleDefault, false, fmt.Errorf("accessibility: %w", err)
	}
	return accessible, true, nil
}

// SetProperties changes the attributes of the single item identified by
// service and by the account the keychain was created with. Only the
// attributes that props names are changed; the item's contents are neither
// read nor written.
//
// The keychain's write type identifies the keychain holding the item, so a
// type of KeychainAll is an error, as is a props that would change nothing.
// An item that does not exist, or that is in a keychain this binary is not
// entitled to modify, is reported as not existing.
func (kc T) SetProperties(service string, props Properties) error {
	if kc.readonlyUpdate {
		return fmt.Errorf("cannot write to readonly keychain, but update in place is enabled")
	}
	if len(service) == 0 {
		return fmt.Errorf("missing service name")
	}
	typ := kc.writeTypeOrDefault()
	if err := props.Validate(typ); err != nil {
		return fmt.Errorf("%v: %w", service, err)
	}
	accessible, setAccessible, err := props.accessibility()
	if err != nil {
		return err
	}
	kc.opts.logger.Info("setting item properties",
		"account", kc.account,
		"service", service,
		"type", typ.String(),
		"new_account", props.Account,
		"new_label", props.Label,
		"new_description", props.Description,
		"new_accessibility", props.Accessibility,
	)
	update := keychain.NewItem()
	if len(props.Account) > 0 {
		update.SetAccount(props.Account)
	}
	if len(props.Label) > 0 {
		update.SetLabel(props.Label)
	}
	if len(props.Description) > 0 {
		update.SetDescription(props.Description)
	}
	if setAccessible {
		update.SetAccessible(keychain.Accessible(accessible))
	}
	// The query asks for neither the item's attributes nor its data: it
	// identifies the item to update and nothing more, so no authorization to
	// read the item's contents is needed.
	query := keychain.NewItem()
	kc.configure(&query, typ)
	query.SetService(service)
	query.SetAccount(kc.account)
	if err := keychain.UpdateItem(query, update); err != nil {
		if errors.Is(err, keychain.ErrorItemNotFound) {
			return fs.ErrNotExist
		}
		if errors.Is(err, keychain.ErrorDuplicateItem) {
			if len(kc.account) == 0 {
				return fmt.Errorf("multiple items match service %q: specify an account with --keychain-account to disambiguate: %w", service, err)
			}
			return fmt.Errorf("cannot update item: an item with service %q and account %q already exists: %w", service, props.Account, err)
		}
		return err
	}
	return nil
}
