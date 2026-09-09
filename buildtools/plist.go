// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"howett.net/plist"
)

// InfoPlist represents the contents of a macOS bundle's Info.plist file.
// Commonly used keys have fields of their own; every other key is captured by
// Extra, so an InfoPlist round-trips without loss.
type InfoPlist struct {
	CFBundleIdentifier     string            `yaml:"CFBundleIdentifier"`
	CFBundleName           string            `yaml:"CFBundleName"`
	CFBundleExecutable     string            `yaml:"CFBundleExecutable"`
	CFBundleIconFile       string            `yaml:"CFBundleIconFile"`
	CFBundlePackageType    string            `yaml:"CFBundlePackageType"`
	LSMinimumSystemVersion string            `yaml:"LSMinimumSystemVersion"`
	CFBundleDisplayName    string            `yaml:"CFBundleDisplayName"`
	XPCService             *XPCServicePlist  `yaml:"XPCService"`
	NSExtension            *NSExtensionPlist `yaml:"NSExtension"`

	// CFBundleVersion is the build identifier and CFBundleShortVersionString
	// the human-readable release version, eg. "1.2.0+3f2a9c11" and "1.2.0".
	CFBundleVersion            string `yaml:"CFBundleVersion"`
	CFBundleShortVersionString string `yaml:"CFBundleShortVersionString"`

	// Extra holds every key that has no field of its own, so that keys this
	// package does not know about are preserved rather than discarded.
	Extra map[string]any `yaml:",inline"`
}

func ifEmpty(s, def string) string {
	if len(s) == 0 {
		return def
	}
	return s
}

// WithDefaults returns a copy of ipl with any empty fields populated with
// default values derived from the binary name. The filepath.Base of the
// binary is used as a default for CFBundleName, CFBundleExecutable and
// CFBundleDisplayName, and "com.example.<base>" for CFBundleIdentifier.
// CFBundlePackageType is set to "APPL" and LSMinimumSystemVersion to "10.15".
// CFBundleVersion is set to "0.0.0" if empty.
func (ipl InfoPlist) WithDefaults(binary string) InfoPlist {
	base := filepath.Base(binary)
	ipl.CFBundleIdentifier = ifEmpty(ipl.CFBundleIdentifier, "com.example."+base)
	ipl.CFBundleName = ifEmpty(ipl.CFBundleName, base)
	ipl.CFBundleExecutable = ifEmpty(ipl.CFBundleExecutable, base)
	ipl.CFBundleDisplayName = ifEmpty(ipl.CFBundleDisplayName, base)
	ipl.CFBundlePackageType = ifEmpty(ipl.CFBundlePackageType, "APPL")
	ipl.LSMinimumSystemVersion = ifEmpty(ipl.LSMinimumSystemVersion, "10.15")
	ipl.CFBundleVersion = ifEmpty(ipl.CFBundleVersion, "0.0.0")
	return ipl
}

// LaunchAgentPlist represents the contents of a launchd job plist, ie. a
// LaunchAgent or LaunchDaemon. As for InfoPlist, keys without a field of their
// own are captured by Extra.
type LaunchAgentPlist struct {
	Label string `yaml:"Label"`
	// ProgramArguments is the command to run and its arguments. launchd also
	// accepts a Program key instead, which has no field here and can be
	// supplied via Extra.
	ProgramArguments []string `yaml:"ProgramArguments"`
	// RunAtLoad and KeepAlive are written only when true, since launchd treats
	// an absent key as false.
	RunAtLoad            bool              `yaml:"RunAtLoad"`
	KeepAlive            bool              `yaml:"KeepAlive"`
	EnvironmentVariables map[string]string `yaml:"EnvironmentVariables"`
	StandardOutPath      string            `yaml:"StandardOutPath"`
	StandardErrorPath    string            `yaml:"StandardErrorPath"`

	Extra map[string]any `yaml:",inline"`
}

// XPCServicePlist represents the contents of an XPCService dictionary, which
// may appear within a bundle's Info.plist. As for InfoPlist, keys without a
// field of their own are captured by Extra.
type XPCServicePlist struct {
	ServiceName string         `yaml:"ServiceName"`
	Extra       map[string]any `yaml:",inline"`
}

// NSExtensionPlist represents the contents of an NSExtension dictionary, which
// appears within the Info.plist of an application extension, such as the
// .appex that contains a Safari web extension. As for InfoPlist, keys without
// a field of their own are captured by Extra.
type NSExtensionPlist struct {
	// NSExtensionPointIdentifier names the kind of extension, eg.
	// SafariWebExtensionPointIdentifier.
	NSExtensionPointIdentifier string `yaml:"NSExtensionPointIdentifier"`
	// NSExtensionPrincipalClass names the class that Safari instantiates to
	// handle requests, ie. the one that receives native messages.
	NSExtensionPrincipalClass string         `yaml:"NSExtensionPrincipalClass"`
	Extra                     map[string]any `yaml:",inline"`
}

// missingKey reports the first of the supplied key/value pairs whose value is
// empty as an error, or nil if all of them are set.
func missingKey(pairs ...struct{ key, value string }) error {
	for _, p := range pairs {
		if p.value == "" {
			return fmt.Errorf("key %q not found or not a string", p.key)
		}
	}
	return nil
}

func required(key, value string) struct{ key, value string } {
	return struct{ key, value string }{key, value}
}

// Validate reports whether the keys required to describe a launchable bundle
// are present, including those of any XPCService dictionary.
func (ipl InfoPlist) Validate() error {
	if err := missingKey(
		required("CFBundleIdentifier", ipl.CFBundleIdentifier),
		required("CFBundleName", ipl.CFBundleName),
		required("CFBundleExecutable", ipl.CFBundleExecutable),
		required("CFBundlePackageType", ipl.CFBundlePackageType),
		required("LSMinimumSystemVersion", ipl.LSMinimumSystemVersion),
		required("CFBundleDisplayName", ipl.CFBundleDisplayName),
		required("CFBundleVersion", ipl.CFBundleVersion),
	); err != nil {
		return err
	}
	if ipl.XPCService != nil {
		if err := ipl.XPCService.Validate(); err != nil {
			return err
		}
	}
	if ipl.NSExtension != nil {
		return ipl.NSExtension.Validate()
	}
	return nil
}

// Validate reports whether the keys launchd requires of a job are present: a
// Label, and something to run, ie. ProgramArguments or a Program key supplied
// via Extra.
func (lap LaunchAgentPlist) Validate() error {
	if err := missingKey(required("Label", lap.Label)); err != nil {
		return err
	}
	if filepath.Base(lap.Label) != lap.Label || strings.ContainsAny(lap.Label, `/\`) {
		return fmt.Errorf("invalid Label %q: must not contain path separators", lap.Label)
	}
	if len(lap.ProgramArguments) > 0 {
		return nil
	}
	if v, ok := lap.Extra["Program"]; ok {
		if s, ok := v.(string); ok && len(strings.TrimSpace(s)) > 0 {
			return nil
		}
	}
	return fmt.Errorf("key %q not found: a launchd job must specify ProgramArguments or Program", "ProgramArguments")
}

// Validate reports whether the keys required of an XPCService dictionary are
// present.
func (x XPCServicePlist) Validate() error {
	return missingKey(required("ServiceName", x.ServiceName))
}

// Validate reports whether the keys required of an NSExtension dictionary are
// present.
func (n NSExtensionPlist) Validate() error {
	return missingKey(
		required("NSExtensionPointIdentifier", n.NSExtensionPointIdentifier),
		required("NSExtensionPrincipalClass", n.NSExtensionPrincipalClass),
	)
}

// newKeys returns a map seeded with the contents of extra, with room for n
// further keys.
func newKeys(extra map[string]any, n int) map[string]any {
	out := make(map[string]any, len(extra)+n)
	maps.Copy(out, extra)
	return out
}

// setString sets key to value unless value is empty.
func setString(out map[string]any, key, value string) {
	if value != "" {
		out[key] = value
	}
}

// keys returns the plist contents: the keys captured by Extra together with
// every non-zero typed field. After unmarshalling the two cannot overlap, since
// Extra holds only keys without a field of their own; a value populated by hand
// that sets both has its typed field take precedence.
func (ipl InfoPlist) keys() map[string]any {
	out := newKeys(ipl.Extra, 10)
	setString(out, "CFBundleIdentifier", ipl.CFBundleIdentifier)
	setString(out, "CFBundleName", ipl.CFBundleName)
	setString(out, "CFBundleExecutable", ipl.CFBundleExecutable)
	setString(out, "CFBundleIconFile", ipl.CFBundleIconFile)
	setString(out, "CFBundlePackageType", ipl.CFBundlePackageType)
	setString(out, "LSMinimumSystemVersion", ipl.LSMinimumSystemVersion)
	setString(out, "CFBundleDisplayName", ipl.CFBundleDisplayName)
	setString(out, "CFBundleVersion", ipl.CFBundleVersion)
	setString(out, "CFBundleShortVersionString", ipl.CFBundleShortVersionString)
	if ipl.XPCService != nil {
		out["XPCService"] = ipl.XPCService.keys()
	}
	if ipl.NSExtension != nil {
		out["NSExtension"] = ipl.NSExtension.keys()
	}
	return out
}

// keys returns the launchd job's plist contents, as per InfoPlist.keys.
func (lap LaunchAgentPlist) keys() map[string]any {
	out := newKeys(lap.Extra, 7)
	setString(out, "Label", lap.Label)
	setString(out, "StandardOutPath", lap.StandardOutPath)
	setString(out, "StandardErrorPath", lap.StandardErrorPath)
	if len(lap.ProgramArguments) > 0 {
		out["ProgramArguments"] = lap.ProgramArguments
	}
	if len(lap.EnvironmentVariables) > 0 {
		out["EnvironmentVariables"] = lap.EnvironmentVariables
	}
	if lap.RunAtLoad {
		out["RunAtLoad"] = true
	}
	if lap.KeepAlive {
		out["KeepAlive"] = true
	}
	return out
}

// keys returns the XPCService dictionary contents, as per InfoPlist.keys.
func (x XPCServicePlist) keys() map[string]any {
	out := newKeys(x.Extra, 1)
	setString(out, "ServiceName", x.ServiceName)
	return out
}

// keys returns the NSExtension dictionary contents, as per InfoPlist.keys.
func (n NSExtensionPlist) keys() map[string]any {
	out := newKeys(n.Extra, 2)
	setString(out, "NSExtensionPointIdentifier", n.NSExtensionPointIdentifier)
	setString(out, "NSExtensionPrincipalClass", n.NSExtensionPrincipalClass)
	return out
}

func (n NSExtensionPlist) MarshalPlist() (any, error) { return n.keys(), nil }

func (n NSExtensionPlist) MarshalYAML() (any, error) { return n.keys(), nil }

func (ipl InfoPlist) MarshalPlist() (any, error) { return ipl.keys(), nil }

func (ipl InfoPlist) MarshalYAML() (any, error) { return ipl.keys(), nil }

func (lap LaunchAgentPlist) MarshalPlist() (any, error) { return lap.keys(), nil }

func (lap LaunchAgentPlist) MarshalYAML() (any, error) { return lap.keys(), nil }

func (x XPCServicePlist) MarshalPlist() (any, error) { return x.keys(), nil }

func (x XPCServicePlist) MarshalYAML() (any, error) { return x.keys(), nil }

func writeInfoPlist(path string, info any) Step {
	name := filepath.Base(path)
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		if err := ctx.Err(); err != nil {
			return ErrorStep(err, "write "+name, path).Run(ctx, cmdRunner)
		}
		if cmdRunner.DryRun() {
			return NewStepResult("write "+name, []string{path}, nil, nil), nil
		}
		data, err := plist.MarshalIndent(info, plist.XMLFormat, "\t")
		if err != nil {
			return NewStepResult("write "+name, []string{path}, nil, err), err
		}
		err = os.WriteFile(path, data, 0644) //nolint:gosec // G306
		return NewStepResult("write "+name, []string{path}, nil, err), err
	})
}
