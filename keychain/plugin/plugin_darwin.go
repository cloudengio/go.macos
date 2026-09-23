// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package plugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"cloudeng.io/cmdutil/flags"
	"cloudeng.io/encoding/json/jsonmsgs"
	"cloudeng.io/logging/ctxlog"
	"cloudeng.io/macos/keychain"
	"cloudeng.io/macos/macosutils"
	"cloudeng.io/security/keys/keychain/plugins"
)

// NewRequest creates a new plugin request for the specified keyname and
func NewRequest(keyname string, cfg Config) (plugins.Request, error) {
	return plugins.NewRequest(keyname, cfg)
}

// NewWriteRequest creates a new plugin request for writing the specified
// contents to the keychain with the specified keyname and configuration.
func NewWriteRequest(keyname string, contents []byte, cfg Config) (plugins.Request, error) {
	return plugins.NewWriteRequest(keyname, contents, cfg)
}

// KeychainFlags are commonly required flags for working with
// the MacOS keychain plugin.
type KeychainFlags struct {
	Binary               string `subcmd:"keychain-plugin,,direct path to the plugin binary, leave empty to use the default"`
	KeychainBundle       string `subcmd:"keychain-app-bundle,,'bundle that contains the plugin binary, use directly if the bundle is an absolute path, otherwise look for in /Applications and $PATH', leave empty to use the default"`
	KeychainPluginBundle string `subcmd:"keychain-plugin-bundle,'bundle that contains the plugin binary, use directly if the bundle is an absolute path, otherwise look for in /Applications and $PATH', leave empty to use the default"`
	Account              string `subcmd:"keychain-account,,account that the keychain item belongs to"`
	OnlyUsePlugin        bool   `subcmd:"keychain-only-use-plugin,false,'require the keychain plugin; if false and no plugin binary is found the keychain is accessed directly, in-process'"`
}

// ReadFlags are used for reading from the keychain plugin.
type ReadFlags struct {
	KeychainFlags
	// Note that the default value is 'all' for reading but 'icloud' for writing.
	Type flags.Enum[keychain.Type] `subcmd:"keychain-type,all,'the type of keychain plugin to use'"`
}

// WriteFlags are used for writing to the keychain plugin.
type WriteFlags struct {
	KeychainFlags
	// Note that the default value is 'all' for reading but 'icloud' for writing.
	// 'all' is not accepted here: it names a search across all keychains.
	Type          flags.Enum[keychain.WriteType]     `subcmd:"keychain-type,icloud,'the type of keychain plugin to use: data-protection-local, file or icloud'"`
	UpdateInPlace bool                               `subcmd:"keychain-update-in-place,false,set to true to update existing note in place"`
	Accessibility flags.Enum[keychain.Accessibility] `subcmd:"keychain-accessibility,when-unlocked,optional accessibility level for the keychain item"`
}

const (
	// DefaultPluginBinary is the default name of the plugin binary.
	DefaultPluginBinary = "cloudeng-keychain-plugin"

	// DefaultKeyChainAppBundle is the default app bundle path for the keychain
	// command that has the keychain plugin nested inside it.
	DefaultKeyChainAppBundle = "cloudeng-keychain.app"

	// DefaultKeyChainPluginBundle is the default app bundle path for the keychain
	// plugin binary. This is used when the plugin is not nested inside the host
	// app bundle.
	DefaultKeyChainPluginBundle = "cloudeng-keychain-plugin.app"
)

// LocatePluginBinary attempts to locate the plugin binary by first looking
// for thje keychain and plugin bundles that contain the plugin binary and
// then looking in the PATH for the binary itself. keychainBundle, pluginBundle
// and pluginBinary default to DefaultKeyChainAppBundle, DefaultKeyChainPluginBundle
// and DefaultPluginBinary respectively if not specified.
func LocatePluginBinary(keychainBundle, pluginBundle, pluginBinary string) (string, error) {
	if pluginBinary == "" {
		pluginBinary = DefaultPluginBinary
	}
	if keychainBundle == "" {
		keychainBundle = DefaultKeyChainAppBundle
	}
	if pluginBundle == "" {
		pluginBundle = DefaultKeyChainPluginBundle
	}
	for _, bundleCandidate := range []string{keychainBundle, pluginBundle} {
		_, binary, err := LocateKeychainBinaryInAppBundle(bundleCandidate, pluginBinary)
		if err == nil {
			return binary, nil
		}
	}
	path, err := exec.LookPath(pluginBinary)
	if err != nil {
		// Wrap with %w so callers can test for exec.ErrNotFound.
		return "", fmt.Errorf("failed to find plugin binary %q in PATH: %w", pluginBinary, err)
	}
	return path, nil
}

// LocateKeychainBinaryInAppBundle finds an app bundle by name that contains the specified
// binary. If appBundle is an absolute path, it is checked directly.
// Otherwise, /Applications and each directory in $PATH are searched. The
// bundle must contain an executable file named binary somewhere within its
// directory tree. If found, the path to the bundle and the absolute path to the
// binary are returned. If not found, an error is returned containing exec.ErrNotFound.
func LocateKeychainBinaryInAppBundle(appBundle, binary string) (string, string, error) {
	if filepath.IsAbs(appBundle) {
		if location, ok := macosutils.LocateInBundle(appBundle, binary, macosutils.IsExecutable); ok {
			return appBundle, location, nil
		}
		return "", "", fmt.Errorf("app bundle %q: executable %q not found: %w", appBundle, binary, exec.ErrNotFound)
	}

	// A relative bundle is searched for in /Applications and each $PATH entry,
	// so it must be a bare name; a path such as "../.." would escape those roots
	// since filepath.Join cleans the result.
	if appBundle != filepath.Base(appBundle) || appBundle == "." || appBundle == ".." {
		return "", "", fmt.Errorf("app bundle %q must be a bare bundle name, not a path", appBundle)
	}

	var searchPath string
	if path := os.Getenv("PATH"); len(path) > 0 {
		searchPath = path + string(os.PathListSeparator) + filepath.Join("/", "Applications")
	} else {
		searchPath = filepath.Join("/", "Applications")
	}
	bundlePath, pluginPath, ok := macosutils.LookupBundleBinary(appBundle, binary, searchPath)
	if ok {
		return bundlePath, pluginPath, nil
	}
	return "", "", fmt.Errorf("app bundle %q not found in /Applications or PATH: %w", appBundle, exec.ErrNotFound)
}

// BundledPluginApp returns the path to the executable binary of the plugin app
// bundle nested inside the app bundle that contains the currently running
// executable, and whether it was found. This lets a host app (e.g. the keychain
// client) ship the plugin as a nested bundle at
// <host>.app/Contents/MacOS/macos-keychain-plugin.app and use it with no
// configuration.
//
// The running executable is resolved through symlinks first: it may be invoked
// via a symlink (gobundle creates one, and PATH entries are often symlinks), and
// only the resolved path reveals the real location inside the bundle. The
// executable is expected at <host>.app/Contents/MacOS/<name>, so the plugin
// bundle is a sibling under Contents/MacOS.
func BundledPluginApp(pluginBinary string) (string, bool) {
	// bundle is the path to the enclosing parent bundle, not the bundle that
	// contains the plugin.
	bundle, ok := macosutils.ProcessInBundle()
	if !ok {
		return "", false
	}
	if len(pluginBinary) == 0 {
		pluginBinary = DefaultPluginBinary
	}
	return macosutils.LocateInBundle(bundle, pluginBinary, macosutils.IsExecutable)
}

// pluginConfig returns a Config based on the KeychainFlags. The account defaults
// to os.Getenv("USER") if not specified. Locating the plugin binary is deferred
// to Config.FS.
func (f KeychainFlags) pluginConfig() Config {
	account := f.Account
	if account == "" {
		account = os.Getenv("USER")
	}
	return Config{
		Binary:               f.Binary,
		KeychainBundle:       f.KeychainBundle,
		KeychainPluginBundle: f.KeychainPluginBundle,
		OnlyUsePlugin:        f.OnlyUsePlugin,
		Account:              account,
	}
}

func (f ReadFlags) Config() (Config, error) {
	c := f.pluginConfig()
	c.Type = f.Type.Value
	return c, nil
}

func (f WriteFlags) Config() (Config, error) {
	cfg := f.pluginConfig()
	cfg.Type = keychain.Type(f.Type.Value)
	cfg.UpdateInPlace = f.UpdateInPlace
	cfg.Accessibility = f.Accessibility.Value
	cfg.WriteType = cfg.Type
	return cfg, nil
}

// DefaultConfigForReading returns a Config with default values
// suitable for reading from the keychain.
func DefaultConfigForReading() Config {
	cfg := KeychainFlags{}.pluginConfig()
	cfg.Type = keychain.KeychainAll
	cfg.WriteType = keychain.KeychainAll
	return cfg
}

// DefaultConfigForWriting returns a Config with default values
// suitable for both reading and writing to the keychain.
func DefaultConfigForReadWrite() Config {
	cfg := KeychainFlags{}.pluginConfig()
	cfg.Type = keychain.KeychainAll
	cfg.UpdateInPlace = true
	cfg.WriteType = keychain.KeychainICloud
	cfg.Accessibility = keychain.AccessibleWhenUnlocked
	return cfg
}

// Config represents the configuration for a keychain plugin. It is also
// used as the PluginSpecific field in plugin requests and responses.
type Config struct {
	Binary               string                 `yaml:"plugin_binary" doc:"plugin binary to use, if not specified it defaults to DefaultPluginBinary, the binary must be present in the PATH or the specified app bundle or be an absolute path" json:"-"`
	KeychainBundle       string                 `yaml:"keychain_app_bundle" doc:"app bundle that contains the plugin binary, if specified it takes precedence over Binary for locating the plugin binary, it defaults to DefaultPluginAppBundlePath" json:"-"`
	KeychainPluginBundle string                 `yaml:"keychain_plugin_bundle" doc:"app bundle that contains the plugin binary, if specified it takes precedence over Binary for locating the plugin binary, it defaults to DefaultKeyChainPluginBundlePath" json:"-"`
	Type                 keychain.Type          `yaml:"keychain_type" doc:"the type of keychain to use, currently supported types are: file, data-protection and icloud" json:"type"`
	WriteType            keychain.Type          `yaml:"keychain_write_type" doc:"the type of keychain to use for writing, currently supported types are: file, data-protection and icloud, this is needed because the 'all' type is not valid for writing" json:"write_type"`
	Account              string                 `yaml:"account" doc:"account that the keychain item belongs to" json:"account"`
	UpdateInPlace        bool                   `yaml:"update_in_place" doc:"set to true to update existing item in place" json:"update_in_place,omitempty"`
	Accessibility        keychain.Accessibility `yaml:"accessibility,omitempty" doc:"optional accessibility level for the keychain item" json:"accessibility,omitempty"`
	OnlyUsePlugin        bool                   `yaml:"only_use_plugin" doc:"require the keychain plugin; if false and no plugin binary is found the keychain is accessed directly, in-process" json:"-"`
}

// FS reads and writes keychain items. It is implemented both by the
// out-of-process plugin (plugins.FS) and by direct, in-process keychain access
// (keychain.T). It is a superset of file.ReadFileFS, so it can be passed to APIs
// such as keys.ReadYAML.
type FS interface {
	ReadFile(name string) ([]byte, error)
	ReadFileCtx(ctx context.Context, name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
	WriteFileCtx(ctx context.Context, name string, data []byte, mode fs.FileMode) error
}

// FS returns the FS used to read and write keychain items. It locates the plugin
// binary (see LookupPluginBinary); if found, access goes through the
// out-of-process plugin. If the plugin binary is not found and OnlyUsePlugin is
// false, it falls back to accessing the keychain directly, in-process — the same
// access the plugin itself performs. "Not found" covers both a plugin that is
// not on the PATH (exec.ErrNotFound) and an explicitly specified path that does
// not exist (fs.ErrNotExist). If OnlyUsePlugin is true, a missing plugin is an
// error, as is any other lookup failure.
func (pc Config) FS(writable bool) (FS, error) {
	binary, err := LocatePluginBinary(pc.KeychainBundle, pc.KeychainPluginBundle, pc.Binary)
	if err == nil {
		return plugins.NewFS(binary, writable, pc), nil
	}
	notFound := errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist)
	if notFound {
		if binary, ok := BundledPluginApp(pc.Binary); ok {
			return plugins.NewFS(binary, writable, pc), nil
		}
	}
	if pc.OnlyUsePlugin || !notFound {
		return nil, err
	}
	return pc.directFS(), nil
}

// directFS returns an FS that accesses the keychain directly, in-process,
// exactly as the plugin server does when handling a request.
func (pc Config) directFS() FS {
	return keychain.New(pc.Type, pc.Account,
		keychain.WithUpdateInPlace(pc.UpdateInPlace),
		keychain.WithAccessibility(pc.Accessibility),
		keychain.WithWriteType(pc.WriteType))
}

// Server provides a plugin for handling plugin requests to access
// the macos keychain. A plugin binary can use this to handle requests
// and return responses.
type Server struct {
	opts options
}

type options struct {
	logger *slog.Logger
}

// Option configures a Server created by NewServer.
type Option func(*options)

// WithLogger sets the logger for the Server. If no logger is provided, a
// default logger that discards all logs will be used.
func WithLogger(logger *slog.Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}

// NewServer creates a new Server with the provided options.
func NewServer(opts ...Option) *Server {
	var opt options
	for _, o := range opts {
		o(&opt)
	}
	if opt.logger == nil {
		opt.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Server{
		opts: opt,
	}
}

// ReadRequest reads a plugin request from the provided messager and returns
// the request. If any errors are encountered then the returned response represents
// an error and should be returned to the plugin caller. Otherwise the response is nil.
func (ps *Server) ReadRequest(ctx context.Context, msgr *jsonmsgs.Messager) (*Config, plugins.Request, *plugins.Response) {
	req, err := plugins.ReadRequest(msgr)
	if err != nil {
		return nil, plugins.Request{}, errorResponse(ctx, req, "failed to decode request", err.Error())
	}
	if verr := req.CheckVersion(); verr != nil {
		ctxlog.Error(ctx, "plugin error", "id", req.ID, "message", verr.Message, "error", verr.Detail)
		return nil, plugins.Request{}, req.NewResponse(nil, verr)
	}
	var cfg Config
	if err := req.UnmarshalPluginSpecific(&cfg); err != nil {
		return nil, plugins.Request{}, errorResponse(ctx, req, "failed to unmarshal plugin_specific", err.Error())
	}
	ps.opts.logger.Info("new request",
		"version", req.Version,
		"id", req.ID,
		"account", cfg.Account,
		"key", req.Keyname,
		"type", cfg.Type,
		"write_type", cfg.WriteType,
		"accessibility", cfg.Accessibility.String(),
		"write", req.Write,
		"update_in_place", cfg.UpdateInPlace)
	return &cfg, req, nil
}

func errorResponse(ctx context.Context, req plugins.Request, message, detail string) *plugins.Response {
	ctxlog.Error(ctx, "plugin error", "id", req.ID, "message", message, "error", detail)
	return req.NewResponse(nil, &plugins.Error{
		Message: message,
		Detail:  detail,
	})
}

func (ps *Server) handleWrite(ctx context.Context, kc *keychain.T, req plugins.Request) *plugins.Response {
	if err := kc.WriteSecureNote(req.Keyname, req.Contents); err != nil {
		if err == fs.ErrExist {
			return req.NewResponse(nil, plugins.NewErrorKeyExists(req.Keyname))
		}
		return errorResponse(ctx, req, "failed to write secure note", err.Error())
	}
	return req.NewResponse(nil, nil)
}

func (ps *Server) handleRead(ctx context.Context, kc *keychain.T, req plugins.Request) *plugins.Response {
	data, err := kc.ReadSecureNote(req.Keyname)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return req.NewResponse(nil, plugins.NewErrorKeyNotFound(req.Keyname))
		}
		return errorResponse(ctx, req, "failed to read secure note", err.Error())
	}
	return req.NewResponse(data, nil)
}

// HandleRequest handles the provided plugin request and returns a response.
// This implements the interaction with the actual OS keychain.
func (ps *Server) HandleRequest(ctx context.Context, cfg *Config, req plugins.Request) *plugins.Response {
	if req.Write {
		kc := keychain.New(cfg.Type, cfg.Account,
			keychain.WithUpdateInPlace(cfg.UpdateInPlace),
			keychain.WithAccessibility(cfg.Accessibility),
			keychain.WithWriteType(cfg.WriteType))
		return ps.handleWrite(ctx, kc, req)
	}
	kc := keychain.New(cfg.Type, cfg.Account,
		keychain.WithUpdateInPlace(cfg.UpdateInPlace),
		keychain.WithAccessibility(cfg.Accessibility))
	return ps.handleRead(ctx, kc, req)
}

// SendResponse sends the provided response to the plugin caller.
func (ps *Server) SendResponse(_ context.Context, msgr *jsonmsgs.Messager, resp *plugins.Response) {
	resp.PluginSpecific = nil
	if err := plugins.WriteResponse(msgr, *resp); err != nil {
		ps.opts.logger.Error("failed to write response", "error", err)
		return
	}
	ps.opts.logger.Info("sent response", "id", resp.ID, "error", resp.Error)
}
