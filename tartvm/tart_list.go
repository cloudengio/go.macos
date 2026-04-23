// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package tart

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"cloudeng.io/vms"
)

// ListEntry represents an entry in the output of "tart list --format json".
type ListEntry struct {
	State    string
	Name     string
	Size     int
	Accessed time.Time
	Source   string
	Disk     int
	Running  bool
}

func (e ListEntry) VMSState() vms.State {
	switch e.State {
	case "running":
		return vms.StateRunning
	case "suspended":
		return vms.StateSuspended
	case "stopped":
		return vms.StateStopped
	case "deleted":
		return vms.StateDeleted
	default:
		return vms.StateInitial
	}
}

type ListEntries []ListEntry

func (e ListEntries) Len() int { return len(e) }

func (e ListEntries) Lookup(name string) (ListEntry, bool) {
	for _, entry := range e {
		if entry.Name == name {
			return entry, true
		}
	}
	return ListEntry{}, false
}

func (e ListEntries) LookupSourceName(source, name string) (ListEntry, bool) {
	for _, entry := range e {
		if entry.Name == name && entry.Source == source {
			return entry, true
		}
	}
	return ListEntry{}, false
}

// ListAll calls "tart list --format json" and returns the entries.
func ListAll(ctx context.Context) (ListEntries, error) {
	out, err := exec.CommandContext(ctx, "tart", "list", "--format", "json").Output()
	if err != nil {
		return nil, fmt.Errorf("tart list: %w", err)
	}
	var entries ListEntries
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse json output from tart list: %w", err)
	}
	return entries, nil
}
