// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io"

	"cloudeng.io/cmdutil/keys"
)

type format string

type message struct {
	Format  format `json:"format"`
	Payload []byte `json:"payload"`
}

func writeIMS(wr io.Writer, ims *keys.InMemoryKeyStore) error {
	msg := message{
		Format: "inmemory_key_store",
	}
	data, err := json.Marshal(ims)
	if err != nil {
		return err
	}
	msg.Payload = data
	enc := json.NewEncoder(wr)
	return enc.Encode(msg)
}

func readIMS(rd io.Reader) (*keys.InMemoryKeyStore, error) {
	dec := json.NewDecoder(rd)
	var m message
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	if m.Format != "inmemory_key_store" {
		return nil, fmt.Errorf("unknown format: %q", m.Format)
	}
	var ims keys.InMemoryKeyStore
	if err := json.Unmarshal(m.Payload, &ims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal in-memory key store: %w", err)
	}
	return &ims, nil
}
