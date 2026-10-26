// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// BundleSpec is what a top-level bundle, see Bundle, and the bundles that
// are nested within it, see SubBundle, have in common: how the contents of a
// single bundle are described and signed. It is read from YAML and embedded,
// inline, in each of them:
//
//	info_plist:
//	  CFBundleIdentifier: com.example.app
//	signing:
//	  entitlements:
//	    keychain-access-groups: [TEAMID.com.example.app]
//	permissions:
//	  executable: 0755
//	provisioning_profile: example.provisionprofile
//	icon: icon.png
//	sub_bundles: [...]
//
// A SubBundle's own SubBundles are nested within it, to any depth.
type BundleSpec struct {
	// Info holds the fields of the bundle's Info.plist.
	Info InfoPlist `yaml:"info_plist"`
	// Signing holds the signing identity, arguments and entitlements for the
	// bundle. The identity and arguments of a bundle that has none are those
	// of the bundle that contains it. The entitlements are never inherited:
	// they are specific to each bundle.
	Signing SigningConfig `yaml:"signing"`
	// Permissions are the permissions for the executable and for the MacOS
	// directory of the bundle.
	Permissions PermissionsConfig `yaml:"permissions,omitempty"`
	// ProvisioningProfile is the path of the profile to embed in the bundle as
	// Contents/embedded.provisionprofile. Entitlements that are restricted,
	// see Entitlements.Restricted, are only authorized by one.
	ProvisioningProfile string `yaml:"provisioning_profile"`
	// Icon is the path of the image that the icon of the bundle is created
	// from. CFBundleIconFile defaults to AppIcon.icns.
	Icon string `yaml:"icon"`
	// SubBundles are the bundles nested within this one, in the order that they
	// are signed.
	SubBundles []SubBundle `yaml:"sub_bundles"`
}

// SubBundle is a bundle that is nested within another, eg. an application
// extension, an XPC service or a helper application.
type SubBundle struct {
	BundleSpec `yaml:",inline"`
	// Path is the location of the bundle relative to the Contents directory of the
	// bundle that contains it, eg. PlugIns/Example.appex. It must be relative
	// and must not leave that directory.
	Path string `yaml:"path"`
}

// Bundle is a top-level bundle, which is the one that is launched,
// notarized and distributed:
//
//	bundle: build/example.app
//	info_plist:
//	  CFBundleIdentifier: com.example.app
//	signing:
//	  identity: "Developer ID Application: Example (TEAMID)"
//	notary:
//	  keychain_profile: example-notary
//	sub_bundles:
//	  - path: PlugIns/Example.appex
//	    info_plist:
//	      CFBundleIdentifier: .Extension
//	    signing:
//	      entitlements:
//	        com.apple.security.app-sandbox: true
//
// The CFBundleIdentifier of a bundle that nests others, or one of the nested
// bundles, may start with a '.', in which case it is that of the bundle that
// contains it followed by it, see ResolveIdentifier.
type Bundle struct {
	BundleSpec `yaml:",inline"`
	// Path is the location of the application bundle to create.
	Path string `yaml:"bundle"`
	// Notary holds the credentials used to notarize the signed bundle with
	// Apple's notarization service, when that is asked for. Notarizing the
	// bundle covers the bundles within it.
	Notary NotaryConfig `yaml:"notary"`
}

// BuildOption is an option for Bundle.Steps.
//
// The bundles are keyed by their location: the key of the top-level bundle is
// the empty string, and that of a nested bundle is its Path relative to
// the Contents directory of the bundle that contains it, preceded by the key of
// that bundle and its Contents directory unless it is the top-level bundle,
// eg. PlugIns/Example.appex or PlugIns/Example.appex/Contents/Frameworks/Lib.framework.
type BuildOption func(*buildOptions)

type buildOptions struct {
	notarize bool
	binaries map[string]string
	contents map[string][]Step
}

// WithNotarize requests that the signed bundle is notarized, which requires
// notary credentials and a Developer ID signing identity.
func WithNotarize(notarize bool) BuildOption {
	return func(o *buildOptions) { o.notarize = notarize }
}

// WithBinary supplies the executable to copy into the bundle with the
// given key. Every bundle must have one.
func WithBinary(key, path string) BuildOption {
	return func(o *buildOptions) {
		if o.binaries == nil {
			o.binaries = map[string]string{}
		}
		o.binaries[key] = path
	}
}

// WithContents adds steps that add to the contents of the bundle with the
// given key, eg. resources. They are run after its executable is copied and
// before any bundle is signed.
func WithContents(key string, steps ...Step) BuildOption {
	return func(o *buildOptions) {
		if o.contents == nil {
			o.contents = map[string][]Step{}
		}
		o.contents[key] = append(o.contents[key], steps...)
	}
}

// ResolveIdentifier returns id, or, if it starts with a '.', containingID
// followed by id: the usual shorthand for "the identifier of the bundle that
// contains this one with a suffix".
func ResolveIdentifier(containingID, id string) string {
	if strings.HasPrefix(id, ".") {
		return containingID + id
	}
	return id
}

// AppBundle returns the bundle described by b.
func (b Bundle) AppBundle() AppBundle {
	return AppBundle{Path: b.Path, Info: b.Info}
}

// AppBundle returns the bundle described by s, nested in parent. Its
// identifier is resolved, see ResolveIdentifier.
func (s SubBundle) AppBundle(parent AppBundle) AppBundle {
	info := s.Info
	info.CFBundleIdentifier = ResolveIdentifier(parent.Info.CFBundleIdentifier, info.CFBundleIdentifier)
	return AppBundle{Path: parent.Contents(s.Path), Info: info}
}

// signingDefaults are the identity and arguments that a bundle inherits.
type signingDefaults struct {
	identity string
	args     []string
}

func (s BundleSpec) signingDefaults(inherited signingDefaults) signingDefaults {
	if s.Signing.Identity != "" {
		inherited.identity = s.Signing.Identity
	}
	if len(s.Signing.CodesignArguments) > 0 {
		inherited.args = s.Signing.CodesignArguments
	}
	return inherited
}

// subKey returns the key of the nested bundle s of the bundle with the key key.
func subKey(key string, s SubBundle) string {
	if key == "" {
		return filepath.ToSlash(filepath.Clean(s.Path))
	}
	return key + "/Contents/" + filepath.ToSlash(filepath.Clean(s.Path))
}

// Validate checks that the bundles are described consistently: the path of
// the bundle is set, the paths of the bundles nested in it are relative to it
// and stay within it, their identifiers are unique, and the
// bundles that are signed with restricted entitlements embed a provisioning
// profile.
func (b Bundle) Validate() error {
	if b.Path == "" {
		return fmt.Errorf("the path of the bundle is not set: set bundle in the configuration")
	}
	ids := map[string]string{}
	return b.validate("", b.AppBundle(), signingDefaults{}, ids)
}

func (s BundleSpec) validate(key string, app AppBundle, inherited signingDefaults, ids map[string]string) error {
	where := "the bundle"
	if key != "" {
		where = fmt.Sprintf("the bundle %q", key)
	}
	if id := app.Info.CFBundleIdentifier; id != "" {
		if strings.HasPrefix(id, ".") {
			return fmt.Errorf("%s has the identifier %q, which starts with a '.' but is not nested in a bundle with an identifier", where, id)
		}
		if other, ok := ids[id]; ok {
			return fmt.Errorf("%s and %s have the same identifier, %q", where, other, id)
		}
		ids[id] = where
	}
	effective := s.signingDefaults(inherited)
	if effective.identity != "" && s.ProvisioningProfile == "" && s.Signing.Entitlements.Restricted() {
		return fmt.Errorf("%s is signed with restricted entitlements, which require a provisioning profile: set provisioning_profile in the configuration", where)
	}
	for _, sub := range s.SubBundles {
		if sub.Path == "" || filepath.IsAbs(sub.Path) || !filepath.IsLocal(sub.Path) {
			return fmt.Errorf("%s has a nested bundle with the path %q, which must be relative to its Contents directory and stay within it", where, sub.Path)
		}
		if err := sub.validate(subKey(key, sub), sub.AppBundle(app), effective, ids); err != nil {
			return err
		}
	}
	return nil
}

// keys returns the keys of the bundles, depth first.
func (s BundleSpec) keys(key string) []string {
	keys := []string{key}
	for _, sub := range s.SubBundles {
		keys = append(keys, sub.keys(subKey(key, sub))...)
	}
	return keys
}

// validateBuild checks that the bundle can be built with opts.
func (b Bundle) validateBuild(opts buildOptions) error {
	if err := b.Validate(); err != nil {
		return err
	}
	for _, key := range b.keys("") {
		if opts.binaries[key] == "" {
			return fmt.Errorf("no executable has been supplied for the bundle %q", key)
		}
	}
	if err := b.validateFiles(); err != nil {
		return err
	}
	if opts.notarize {
		if !b.Notary.Configured() {
			return fmt.Errorf("notarization requires notary credentials: set a 'notary' section in the configuration")
		}
		return b.Notary.ValidateSigning(b.Signing)
	}
	return nil
}

func (s BundleSpec) validateFiles() error {
	if s.Icon != "" {
		if _, err := os.Stat(s.Icon); err != nil {
			return fmt.Errorf("icon %q is not accessible: %w", s.Icon, err)
		}
	}
	if s.ProvisioningProfile != "" {
		if _, err := os.Stat(s.ProvisioningProfile); err != nil {
			return fmt.Errorf("provisioning profile %q is not accessible: %w", s.ProvisioningProfile, err)
		}
	}
	for _, sub := range s.SubBundles {
		if err := sub.validateFiles(); err != nil {
			return err
		}
	}
	return nil
}

// Steps returns the steps to build the bundle and the bundles nested in it
// after validating them, in the order that they must be run: the bundle is
// removed and created, and the contents of every bundle are added to it, the
// outermost first, then the bundles are signed, the innermost first, so that
// each is signed once all that it contains is, and finally the bundle is
// notarized, if requested. Everything that changes the contents of a bundle
// precedes signing, which any later change would invalidate.
func (b Bundle) Steps(options ...BuildOption) ([]Step, error) {
	var opts buildOptions
	for _, o := range options {
		o(&opts)
	}
	if err := b.validateBuild(opts); err != nil {
		return nil, err
	}
	app := b.AppBundle()
	var steps []Step
	steps = append(steps, app.Clean()...)
	steps = append(steps, b.populate("", app, opts)...)
	steps = append(steps, b.sign("", app, opts, signingDefaults{})...)
	if opts.notarize {
		steps = append(steps, app.Notarize(b.Notary)...)
	}
	return steps, nil
}

// populate returns the steps that create the bundle and add its contents,
// followed by those for the bundles nested in it.
func (s BundleSpec) populate(key string, app AppBundle, opts buildOptions) []Step {
	binary := opts.binaries[key]
	app = s.withDefaults(app, binary)
	steps := app.Create()
	if s.ProvisioningProfile != "" {
		steps = append(steps, app.InstallProvisioningProfile(s.ProvisioningProfile))
	}
	steps = append(steps, app.WriteInfoPlist(), app.CopyExecutable(binary))
	steps = append(steps, opts.contents[key]...)
	if s.Icon != "" {
		steps = append(steps, appIconStep(app, s.Icon))
	}
	steps = append(steps,
		app.SetExecutablePermissions(binary, s.Permissions.ExecutableMode()),
		app.SetMacOSDirPermissions(s.Permissions.MacOSDirMode()))
	for _, sub := range s.SubBundles {
		steps = append(steps, sub.populate(subKey(key, sub), sub.AppBundle(app), opts)...)
	}
	return steps
}

// withDefaults defaults the CFBundleExecutable and CFBundleName of app to
// the name of its binary, and its CFBundleIconFile to the icon created from
// the Icon, if there is one.
func (s BundleSpec) withDefaults(app AppBundle, binary string) AppBundle {
	base := filepath.Base(binary)
	if s.Icon != "" {
		app.Info.CFBundleIconFile = cmp.Or(app.Info.CFBundleIconFile, "AppIcon.icns")
	}
	app.Info.CFBundleExecutable = cmp.Or(app.Info.CFBundleExecutable, base)
	app.Info.CFBundleName = cmp.Or(app.Info.CFBundleName, base)
	return app
}

// sign returns the steps that sign the bundles nested in the bundle, and then
// the bundle, if there is an identity to sign it with.
func (s BundleSpec) sign(key string, app AppBundle, opts buildOptions, inherited signingDefaults) []Step {
	app = s.withDefaults(app, opts.binaries[key])
	effective := s.signingDefaults(inherited)
	var steps []Step
	for _, sub := range s.SubBundles {
		steps = append(steps, sub.sign(subKey(key, sub), sub.AppBundle(app), opts, effective)...)
	}
	if effective.identity != "" {
		signer := NewSigner(effective.identity, s.Signing.Entitlements, s.Signing.PerFileEntitlements, effective.args)
		steps = append(steps, app.SignExecutable(signer), app.Sign(signer))
	}
	return steps
}

// appIconStep returns a step that creates the icon of the bundle from the image
// icon, and adds it to the bundle as Resources/AppIcon.icns. It does so in a
// temporary directory that it removes.
func appIconStep(app AppBundle, icon string) Step {
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		tempDir, err := os.MkdirTemp("", "bundle-icon")
		if err != nil {
			return NewStepResult("icon", []string{icon}, nil, err), err
		}
		defer os.RemoveAll(tempDir)
		iconDir := filepath.Join(tempDir, "AppIcon.iconset")
		if err := os.Mkdir(iconDir, 0700); err != nil {
			return NewStepResult("icon", []string{icon}, nil, err), err
		}
		iconSet := IconSet{Icon: icon, Dir: iconDir}
		steps := iconSet.CreateIconVariants(iconSet.Icon, iconDir)
		steps = append(steps, iconSet.CreateIcns())
		steps = slices.Concat(steps, app.CopyIcons(iconSet))
		var out bytes.Buffer
		for _, step := range steps {
			res, err := step.Run(ctx, cmdRunner)
			fmt.Fprintf(&out, "%s\n%s", res.CommandLine(), res.Output())
			if err != nil {
				return NewStepResult("icon", []string{icon}, out.Bytes(), err), err
			}
		}
		return NewStepResult("icon", []string{icon}, out.Bytes(), nil), nil
	})
}
