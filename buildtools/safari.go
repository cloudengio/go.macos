// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// SafariWebExtensionPointIdentifier is the NSExtensionPointIdentifier of a
	// Safari web extension, ie. one that uses the WebExtensions API and a
	// manifest.json.
	SafariWebExtensionPointIdentifier = "com.apple.Safari.web-extension"

	// safariAppExtensionPointIdentifier is the extension point of the older
	// Safari app extensions, which are not supported by this package.
	safariAppExtensionPointIdentifier = "com.apple.Safari.extension"

	// appExtensionPackageType is the CFBundlePackageType of an .appex.
	appExtensionPackageType = "XPC!"
)

// SafariWebExtension describes a Safari web extension that is shipped inside
// an app bundle. Safari does not launch a separate native messaging host the
// way Chrome and Firefox do: a web extension is packaged as an application
// extension, an .appex within the containing app's Contents/PlugIns, and
// messages sent by the extension with runtime.sendNativeMessage are delivered
// to the principal class of that .appex. That class is the native helper.
//
// Only Safari web extensions are supported. The older Safari app extensions,
// which use the com.apple.Safari.extension extension point and do not use a
// manifest.json, are rejected.
//
// See https://developer.apple.com/documentation/safariservices/safari-web-extensions
type SafariWebExtension struct {
	// Name is the name of the .appex within Contents/PlugIns, without the
	// .appex suffix.
	Name string

	// Executable is the path to the built handler binary, which is copied to
	// the .appex's Contents/MacOS directory. It must be named by
	// Info.CFBundleExecutable.
	Executable string

	// Resources is the directory holding the web extension itself: its
	// manifest.json and the scripts, pages and assets that it references. Its
	// contents are copied into the .appex's Contents/Resources directory.
	Resources string

	// PrincipalClass is the NSExtensionPrincipalClass, ie. the class that
	// Safari instantiates to receive native messages from the extension, such
	// as "ExampleExtension.SafariWebExtensionHandler".
	PrincipalClass string

	// Info is the .appex's Info.plist. Its NSExtension dictionary is set by
	// AddSafariWebExtension and need not be filled in; CFBundlePackageType
	// defaults to XPC!. CFBundleIdentifier must be prefixed by the
	// identifier of the containing app, as macOS requires of any app
	// extension.
	Info InfoPlist
}

// ExtensionConfig is the declarative, typically YAML-loaded configuration for
// a Safari web extension packaged inside an app bundle: where its handler
// binary comes from (prebuilt, or built from a Swift package), and its
// identity. SafariWebExtension is the resolved form built from it, once the
// handler binary exists (building it, e.g. from the Swift package at
// SwiftDir, is the caller's job, not ExtensionConfig's) and the rest of its
// Info.plist (such as version information) has been decided.
type ExtensionConfig struct {
	// Name is the .appex name within Contents/PlugIns, without the .appex
	// suffix.
	Name string `yaml:"name"`
	// Identifier is the extension's CFBundleIdentifier. A leading "." is
	// shorthand for the containing app's identifier plus this suffix, e.g.
	// ".Extension"; see ResolveIdentifier.
	Identifier string `yaml:"identifier"`
	// Executable is the CFBundleExecutable name the handler binary is copied
	// to inside the .appex, e.g. "ExampleHandler".
	Executable string `yaml:"executable"`
	// Binary is the path to a prebuilt handler binary. If empty, the caller
	// is expected to build one from the Swift package at SwiftDir.
	Binary string `yaml:"binary"`
	// Resources is the directory holding the web extension itself: its
	// manifest.json and the scripts, pages and assets that it references.
	Resources string `yaml:"resources"`
	// PrincipalClass is the NSExtensionPrincipalClass, e.g.
	// "ExampleExtension.SafariWebExtensionHandler".
	PrincipalClass string `yaml:"principal_class"`
	// SwiftDir is the directory of the Swift package that builds the handler
	// binary, used by the caller when Binary is empty.
	SwiftDir string `yaml:"swift_dir"`
}

// ResolveIdentifier returns c.Identifier, or, if it starts with ".",
// containingAppID+c.Identifier: the usual shorthand for "this extension's
// identifier is the containing app's plus a suffix".
func (c ExtensionConfig) ResolveIdentifier(containingAppID string) string {
	if strings.HasPrefix(c.Identifier, ".") {
		return containingAppID + c.Identifier
	}
	return c.Identifier
}

// InfoPlist returns the base Info.plist for the extension, with
// CFBundleIdentifier (see ResolveIdentifier), CFBundleName, CFBundleExecutable
// and CFBundleDisplayName filled in from c. Every other field, such as
// version information or additional keys, is left for the caller to set on
// the result.
func (c ExtensionConfig) InfoPlist(containingAppID string) InfoPlist {
	return InfoPlist{
		CFBundleIdentifier:  c.ResolveIdentifier(containingAppID),
		CFBundleName:        c.Name,
		CFBundleExecutable:  c.Executable,
		CFBundleDisplayName: c.Name,
	}
}

// SafariWebExtension returns the resolved SafariWebExtension described by c:
// executable (typically the result of Resolve) as its handler binary, and
// info (typically c.InfoPlist, with version information and any other keys
// added) as its Info.plist.
func (c ExtensionConfig) SafariWebExtension(executable string, info InfoPlist) SafariWebExtension {
	return SafariWebExtension{
		Name:           c.Name,
		Executable:     executable,
		Resources:      c.Resources,
		PrincipalClass: c.PrincipalClass,
		Info:           info,
	}
}

// PlugIns returns the path to elem within the bundle's Contents/PlugIns
// directory, which is where an application extension is placed.
func (b AppBundle) PlugIns(elem ...string) string {
	return filepath.Join(b.Path, "Contents", "PlugIns", filepath.Join(elem...))
}

// SafariWebExtensionPath returns the path of the extension's .appex within
// the bundle.
func (b AppBundle) SafariWebExtensionPath(ext SafariWebExtension) string {
	return b.PlugIns(ext.Name + ".appex")
}

// infoPlist returns the .appex's Info.plist with the keys that make it a
// Safari web extension filled in.
func (ext SafariWebExtension) infoPlist() InfoPlist {
	info := ext.Info
	if info.CFBundlePackageType == "" {
		info.CFBundlePackageType = appExtensionPackageType
	}
	nsext := NSExtensionPlist{
		NSExtensionPointIdentifier: SafariWebExtensionPointIdentifier,
		NSExtensionPrincipalClass:  ext.PrincipalClass,
	}
	if info.NSExtension != nil {
		nsext.Extra = info.NSExtension.Extra
		if info.NSExtension.NSExtensionPrincipalClass != "" {
			nsext.NSExtensionPrincipalClass = info.NSExtension.NSExtensionPrincipalClass
		}
	}
	info.NSExtension = &nsext
	return info
}

// validate reports whether ext describes a Safari web extension that can be
// built, given the identifier of the containing app.
func (ext SafariWebExtension) validate(containingID string) error {
	if ext.Name == "" {
		return fmt.Errorf("name not specified for the safari web extension")
	}
	if ext.Executable == "" {
		return fmt.Errorf("executable not specified for the safari web extension %q", ext.Name)
	}
	if ext.Resources == "" {
		return fmt.Errorf("resources directory not specified for the safari web extension %q", ext.Name)
	}
	if ext.PrincipalClass == "" && (ext.Info.NSExtension == nil || ext.Info.NSExtension.NSExtensionPrincipalClass == "") {
		return fmt.Errorf("principal class not specified for the safari web extension %q", ext.Name)
	}
	// Reject the older app extensions rather than silently building
	// something Safari will treat as one.
	if ext.Info.NSExtension != nil {
		if id := ext.Info.NSExtension.NSExtensionPointIdentifier; id != "" && id != SafariWebExtensionPointIdentifier {
			if id == safariAppExtensionPointIdentifier {
				return fmt.Errorf("safari app extensions (%v) are not supported, only web extensions (%v)",
					safariAppExtensionPointIdentifier, SafariWebExtensionPointIdentifier)
			}
			return fmt.Errorf("unsupported extension point %q, expected %q", id, SafariWebExtensionPointIdentifier)
		}
	}
	// A web extension is defined by its manifest.json; without one Safari has
	// nothing to load.
	manifest := filepath.Join(ext.Resources, "manifest.json")
	if _, err := os.Stat(manifest); err != nil {
		return fmt.Errorf("safari web extension %q: %v not found: a web extension must provide a manifest.json", ext.Name, manifest)
	}
	// macOS requires an app extension's identifier to be prefixed by that of
	// the app that contains it.
	id := ext.Info.CFBundleIdentifier
	if id == "" {
		return fmt.Errorf("CFBundleIdentifier not specified for the safari web extension %q", ext.Name)
	}
	if containingID != "" && !strings.HasPrefix(id, containingID+".") {
		return fmt.Errorf("CFBundleIdentifier %q must be prefixed by the containing app's identifier %q", id, containingID)
	}
	return nil
}

// AddSafariWebExtension returns the steps required to add a Safari web
// extension to the bundle: the .appex is created in Contents/PlugIns with the
// Info.plist that identifies it as a web extension, the handler executable is
// copied into it, and the extension's own resources are copied into its
// Resources directory verbatim.
//
// AddSafariWebExtensionSteps is the same, but for a caller that needs to
// populate Contents/Resources some other way, e.g. building it rather than
// copying it as is.
func (b AppBundle) AddSafariWebExtension(ext SafariWebExtension) []Step {
	appex := b.SafariWebExtensionPath(ext)
	// The trailing "/." copies the contents of the directory rather than the
	// directory itself. filepath.Join would clean it away, so it is appended
	// directly.
	copyResources := CopyDir(filepath.Clean(ext.Resources)+string(filepath.Separator)+".",
		filepath.Join(appex, "Contents", "Resources"))
	return b.AddSafariWebExtensionSteps(ext, copyResources)
}

// AddSafariWebExtensionSteps is AddSafariWebExtension, except that
// resourceSteps, run last, populate the .appex's Contents/Resources directory
// (already created) instead of a verbatim copy of ext.Resources.
func (b AppBundle) AddSafariWebExtensionSteps(ext SafariWebExtension, resourceSteps ...Step) []Step {
	if err := ext.validate(b.Info.CFBundleIdentifier); err != nil {
		return []Step{ErrorStep(err, "safari-web-extension", ext.Name)}
	}
	info := ext.infoPlist()
	appex := b.SafariWebExtensionPath(ext)
	executable := info.CFBundleExecutable
	if executable == "" {
		executable = filepath.Base(ext.Executable)
		info.CFBundleExecutable = executable
	}
	steps := []Step{
		MkdirAll(filepath.Join(appex, "Contents", "MacOS")),
		MkdirAll(filepath.Join(appex, "Contents", "Resources")),
		WritePlistFile(info, filepath.Join(appex, "Contents", "Info.plist")),
		Copy(ext.Executable, filepath.Join(appex, "Contents", "MacOS", executable)),
	}
	return append(steps, resourceSteps...)
}

// SignSafariWebExtension returns the steps required to sign the extension's
// .appex within the app bundle: the handler executable inside the .appex is
// signed first, and then the .appex bundle itself. An .appex is a nested
// bundle and so must be signed inside out, and before the bundle that
// contains it.
func (b AppBundle) SignSafariWebExtension(signer Signer, ext SafariWebExtension) []Step {
	if ext.Name == "" {
		return []Step{ErrorStep(fmt.Errorf("safari web extension name not specified"), "codesign")}
	}
	info := ext.infoPlist()
	executable := info.CFBundleExecutable
	if executable == "" {
		executable = filepath.Base(ext.Executable)
	}
	if executable == "" || executable == "." {
		return []Step{ErrorStep(fmt.Errorf("safari web extension executable not specified for %q", ext.Name), "codesign")}
	}
	appex := ext.Name + ".appex"
	return []Step{
		signer.SignPath(b.Path, filepath.Join("Contents", "PlugIns", appex, "Contents", "MacOS", executable)),
		signer.SignPath(b.Path, filepath.Join("Contents", "PlugIns", appex)),
	}
}
