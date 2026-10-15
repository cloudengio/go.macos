// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package keychain

import (
	"cmp"
	"fmt"
	"slices"

	"cloudeng.io/errors"
	"github.com/cloudengio/go-keychain"
)

// Query returns the attributes of every item in the requested keychain(s)
// that matches filter, for all accounts. Use New followed by T.Query to
// restrict the search to a single account.
func Query(typ Type, filter Filter, opts ...Option) ([]Attributes, error) {
	return newKeychain(true, typ, "", opts...).Query(filter)
}

// Query returns the attributes of the items in the keychain that match
// filter. If the keychain was created with a non-empty account then only
// that account's items are searched, the match being an exact one performed
// by the keychain itself; filter.Account can be used to match accounts by
// regular expression instead.
//
// Item contents are never requested from the keychain and hence are never
// returned. As well as keeping secrets out of the results this avoids the
// authorization prompts that reading an item's contents may otherwise
// trigger.
//
// A type of KeychainAll searches the iCloud, data-protection and file-based
// keychains; an item that is present in more than one of them is reported
// once per keychain. A keychain that cannot be searched, eg. the
// data-protection keychain when the calling binary lacks the entitlements it
// requires, is logged and skipped; an error is returned only if every
// keychain searched failed.
//
// The results are sorted by service, account and then keychain.
func (kc T) Query(filter Filter) ([]Attributes, error) {
	searchList := []Type{kc.typ}
	if kc.typ == KeychainAll {
		searchList = searchListAll
	}
	kc.opts.logger.Info("querying keychain",
		"account", kc.account,
		"type", kc.typ.String(),
		"secure-notes-only", filter.SecureNotesOnly,
	)
	var errs errors.M
	var failed int
	var matched []Attributes
	for _, typ := range searchList {
		results, err := kc.queryAttributes(typ)
		if err != nil {
			kc.opts.logger.Warn("failed to query keychain",
				"account", kc.account,
				"type", typ.String(),
				"error", err,
			)
			errs.Append(fmt.Errorf("%v keychain: %w", typ, err))
			failed++
			continue
		}
		kc.opts.logger.Info("queried keychain",
			"type", typ.String(),
			"items", len(results),
		)
		for _, result := range results {
			attrs := Attributes{
				Keychain:    typ,
				Service:     result.Service,
				Account:     result.Account,
				Label:       result.Label,
				Description: result.Description,
				Comment:     result.Comment,
				AccessGroup: result.AccessGroup,
				Created:     result.CreationDate,
				Modified:    result.ModificationDate,
			}
			// An item that carries no accessibility attribute is reported
			// as having none, rather than as having the default: the
			// attribute is absent for file-based items, whose access is
			// governed by an access control list instead.
			if accessible := Accessibility(result.Accessible); accessible != AccessibleDefault {
				attrs.Accessibility = accessible.String()
			}
			if filter.Matches(attrs) {
				matched = append(matched, attrs)
			}
		}
	}
	if failed == len(searchList) {
		return nil, errs.Err()
	}
	slices.SortFunc(matched, func(a, b Attributes) int {
		return cmp.Or(
			cmp.Compare(a.Service, b.Service),
			cmp.Compare(a.Account, b.Account),
			cmp.Compare(a.Keychain, b.Keychain),
		)
	})
	kc.opts.logger.Info("keychain query completed", "matched", len(matched))
	return matched, nil
}

func (kc T) queryAttributes(typ Type) ([]keychain.AccessibleResult, error) {
	query := keychain.NewItem()
	kc.configure(&query, typ)
	if kc.account != "" {
		query.SetAccount(kc.account)
	}
	query.SetMatchLimit(keychain.MatchLimitAll)
	query.SetReturnAttributes(true)
	// SetReturnData is deliberately not called: item contents must never be
	// requested by a query, both to keep them out of the results and so that
	// the query needs no authorization to access them.
	//
	// QueryItemAccessible, rather than QueryItem, because the accessibility
	// recorded for an item is one of the attributes reported.
	return keychain.QueryItemAccessible(query)
}
