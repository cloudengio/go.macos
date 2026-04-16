// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

//go:embed seccomp/with-key-ctl.json
var defaultSeccompProfile []byte

// SeccompProfile matches the structure of the Docker seccomp JSON
//  type SeccompProfile struct {
//	  map[string]any
//  }

// SyscallRule represents a specific whitelist rule
type SyscallRule struct {
	Names  []string `json:"names"`
	Action string   `json:"action"`
	Args   []any    `json:"args,omitempty"` // Use 'any' to preserve existing filters unmodified
}

func (dc dockerCmds) writeSeccompToTempFile() (string, error) {
	f, err := os.CreateTemp("", "docker-entrypoint-seccomp")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(defaultSeccompProfile); err != nil {
		return "", err
	}
	return f.Name(), nil
}

type seccompFlags struct {
	Output string `subcmd:"output,,write seccomp profile to file"`
}

func (dc dockerCmds) createSeccompProfile(_ context.Context, f any, _ []string) error {
	fv := f.(*seccompFlags)
	resp, err := http.Get("https://raw.githubusercontent.com/moby/profiles/refs/heads/main/seccomp/default.json")
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}

	var profile map[string]any
	if err := json.Unmarshal(data, &profile); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Create a new rule that allows the 'keyctl' syscall.
	// By leaving 'Args' empty or nil, we allow ALL arguments (read, write, link, search, etc).
	keyctlRule := SyscallRule{
		Names:  []string{"keyctl", "add_key"},
		Action: "SCMP_ACT_ALLOW",
	}

	profile["syscalls"] = append([]any{keyctlRule}, profile["syscalls"].([]any)...)

	output, err := json.MarshalIndent(profile, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	out := os.Stdout
	if len(fv.Output) != 0 {
		out, err = os.OpenFile(fv.Output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("failed to open output file: %w", err)
		}
		defer out.Close()
	}
	if _, err := out.Write(output); err != nil {
		return fmt.Errorf("failed to write to output: %w", err)
	}
	return nil
}
