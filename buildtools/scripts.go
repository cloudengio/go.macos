// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools

import (
	"fmt"
	"strings"
	"sync"
)

type File struct {
	Src       string `yaml:"src"`
	DstLocal  string `yaml:"local"`
	DstSystem string `yaml:"system"`
}

// OneOf returns the destination path to use for the file.
// If both DstLocal and DstSystem are set an empty string is returned,
// if neither is set the source path is returned, otherwise
// one of the system or local destination paths is returned.
func (f File) OneOf() string {
	if len(f.DstLocal) > 0 && len(f.DstSystem) > 0 {
		return ""
	}
	if len(f.DstLocal) > 0 {
		return f.DstLocal
	}
	if len(f.DstSystem) > 0 {
		return f.DstSystem
	}
	return f.Src
}

func rewriteHome(s string) string {
	s = strings.ReplaceAll(s, "${HOME}", "${TARGET_HOME}")
	return strings.ReplaceAll(s, "$HOME", "${TARGET_HOME}")
}

// RewriteHOME rewrites any occurrences of $HOME or ${HOME} in the source and destination
// paths to ${TARGET_HOME} which is set in the bash script preamble.
// Use this with the BashInstallPreamble to access the current logged in user's home
// directory since $HOME does not refer to the user's home directory from within
// the installer environment.
func (f File) RewriteHOME() File {
	f.Src = rewriteHome(f.Src)
	f.DstLocal = rewriteHome(f.DstLocal)
	f.DstSystem = rewriteHome(f.DstSystem)
	return f
}

// BashScript helps in the construction of bash scripts
// for any pre and post install operations.
type BashScript struct {
	out           *strings.Builder
	installerOnce sync.Once
}

// BashInstallPreamble is the standard preamble for install scripts
// used in pkgbuild packages.
const BashInstallPreamble = `#!/usr/bin/env bash
set -euo pipefail
mount="$1"
installer_pid="$2"
target="$3"

TARGET_USER=$(stat -f "%Su" /dev/console)
TARGET_HOME=$(eval echo ~$TARGET_USER)
`

// installerCopyFunction is a standard function used to copy files
// in install scripts used in pkgbuild packages.
const installerCopyFunction = `
function installer_copy {
  target="$1"
  system="$2"
  src="$3"
  system_dst="$4"
  local_dst="$5"
  manifest="$6"
  if [ "$system" = "true" ]; then
	dst="${system_dst}"
  else
	dst="${local_dst}"
  fi
  if [ ! -z "$target" ]; then
	dst="${target}/${dst}"
  fi
  dst_dir=$(dirname "${dst}")
  if [ ! -d "${dst_dir}" ]; then
	echo "Creating directory ${dst_dir}"
	sudo mkdir -p "${dst_dir}"
	if [ "$system" != "true" ]; then
	  chown "$TARGET_USER" "${dst_dir}"
	fi
  fi
  echo "Copying ${src} to ${dst}"
  cp "${src}" "${dst}"
  if [ -f "${manifest}" ]; then
	echo "${dst}" >> "${manifest}"
  fi
  if [ "$system" != "true" ]; then
	chown "$TARGET_USER" "${dst}"
  fi
}

`

// NewBashScript creates a new BashScript instance with the specified
// preamble.
func NewBashScript(preamble string) *BashScript {
	scr := &BashScript{
		out: &strings.Builder{},
	}
	scr.out.WriteString(preamble)
	return scr
}

// Append appends the specified text to the script.
func (b *BashScript) Append(text string) {
	b.out.WriteString(text)
}

// InstallFile appends the commands to install the specified file
// to the script. If systemWide is true the file is installed to the
// system location otherwise it is installed to the local user location.
// The manifest file, if specified, is updated with the installed file's
// path.
func (b *BashScript) InstallFile(systemWide bool, file File, manifest string) {
	b.installerOnce.Do(func() {
		b.out.WriteString(installerCopyFunction)
	})
	fmt.Fprintf(b.out, `
installer_copy "$target" %t "%s" "%s" "%s" "%s"
`, systemWide, file.Src, file.DstSystem, file.DstLocal, manifest)
}

// CreateInstallManifest appends the commands to create the install manifest
// to the script. If the manifest's source path is empty /dev/null is used
// instead.
func (b *BashScript) CreateInstallManifest(systemWide bool, manifest File) {
	if len(manifest.Src) == 0 {
		manifest.Src = "/dev/null"
	}
	b.InstallFile(systemWide, manifest, "")
}

// Bytes returns the script as a byte slice.
func (b *BashScript) Bytes() []byte {
	return []byte(b.out.String())
}
