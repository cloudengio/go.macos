// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"cloudeng.io/logging/ctxlog"
	"cloudeng.io/macos/keychain"
	"cloudeng.io/os/executil"
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

// WriteType represents the type of keychain plugin to use for writing.
// It aliases keychain.Type in order to add flag.Value support.
type WriteType keychain.Type

func (t *WriteType) Set(v string) error {
	kt, err := keychain.ParseType(v)
	if err != nil {
		return err
	}
	if kt == keychain.KeychainAll {
		return errors.New("type 'all' is not valid for writing")
	}
	*t = WriteType(kt)
	return nil
}

func (t *WriteType) String() string {
	return keychain.Type(*t).String()
}

// ReadType represents the type of keychain plugin to use for reading.
// It aliases keychain.Type in order to add flag.Value support.
type ReadType keychain.Type

func (t *ReadType) Set(v string) error {
	kt, err := keychain.ParseType(v)
	if err != nil {
		return err
	}
	*t = ReadType(kt)
	return nil
}

func (t *ReadType) String() string {
	return keychain.Type(*t).String()
}

// Accessibility represents the accessibility level for a keychain item.
// It aliases keychain.Accessibility in order to add flag.Value support.
type Accessibility keychain.Accessibility

func (a *Accessibility) Set(v string) error {
	ka, err := keychain.ParseAccessibility(v)
	if err != nil {
		return err
	}
	*a = Accessibility(ka)
	return nil
}

func (a *Accessibility) String() string {
	return keychain.Accessibility(*a).String()
}

// KeychainFlags are commonly required flags for working with
// the MacOS keychain plugin.
type KeychainFlags struct {
	Binary  string `subcmd:"keychain-plugin,,path to the plugin binary"`
	UseApp  string `subcmd:"keychain-use-app,,'if empty, defaults to /Applications/macos-keychain-plugin.app, but can be set to any app bundle that contains the plugin binary (macos-keychain-plugin)'"`
	Account string `subcmd:"keychain-account,,account that the keychain item belongs to"`
}

// ReadFlags are used for reading from the keychain plugin.
type ReadFlags struct {
	KeychainFlags
	// Note that the default value is 'all' for reading but 'icloud' for writing.
	Type ReadType `subcmd:"keychain-type,all,'the type of keychain plugin to use: file, data-protection, icloud or all'"`
}

// WriteFlags are used for writing to the keychain plugin.
type WriteFlags struct {
	KeychainFlags
	// Note that the default value is 'all' for reading but 'icloud' for writing.
	Type          WriteType     `subcmd:"keychain-type,icloud,'the type of keychain plugin to use: file, data-protection or icloud'"`
	UpdateInPlace bool          `subcmd:"keychain-update-in-place,false,set to true to update existing note in place"`
	Accessibility Accessibility `subcmd:"keychain-accessibility,,optional accessibility level for the keychain item"`
}

const (
	// DefaultPluginBinary is the default name of the plugin binary.
	DefaultPluginBinary = "macos-keychain-plugin"

	// DefaultPluginAppBundlePath is the default app bundle path.
	DefaultPluginAppBundlePath = "/Applications/macos-keychain-plugin.app"
)

// LocatePluginBinary attempts to locate the plugin binary by first checking the
// specified app bundle and then looking in the PATH for binary.
func LocatePluginBinary(appBundle, binary string) (string, error) {
	appPath := filepath.Join(appBundle, "Contents", "MacOS", "macos-keychain-plugin")
	fi, err := os.Stat(appPath)
	if err == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0100 != 0 {
		return appPath, nil
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return "", fmt.Errorf("failed to find plugin binary in PATH: %v", err)
	}
	return path, nil
}

// LookupPluginBinary attempts to locate the plugin binary by first checking the
// specified app bundle and then looking in the PATH for binary. If the
// app bundle is not specified, it defaults to DefaultPluginAppBundlePath. If
// the binary is not specified, it defaults to DefaultPluginBinary. If the
// binary cannot be found in either location, an error is returned.
func LookupPluginBinary(appBundle, binary string) (string, error) {
	if binary == "" {
		binary = executil.ExecName(DefaultPluginBinary)
	}
	if appBundle == "" {
		appBundle = DefaultPluginAppBundlePath
	}
	candidate := filepath.Join(appBundle, "Contents", "MacOS", binary)
	fi, err := os.Stat(candidate)
	if err == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0100 != 0 {
		return candidate, nil
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return "", fmt.Errorf("failed to find plugin binary in PATH: %v", err)
	}
	return path, nil
}

// Config returns a Config based on the KeychainFlags.
// It provides a default value for the plugin binary if one is not specified
// in the flags. It also provides a default account of os.Getenv("USER") if
func (f KeychainFlags) pluginConfig() (Config, error) {
	// no account is specified.
	binary, err := LookupPluginBinary(f.UseApp, f.Binary)
	if err != nil {
		return Config{}, err
	}
	account := f.Account
	if account == "" {
		account = os.Getenv("USER")
	}
	return Config{
		Binary:  binary,
		Account: account,
	}, nil
}

func (f ReadFlags) Config() (Config, error) {
	c, err := f.pluginConfig()
	if err != nil {
		return Config{}, err
	}
	c.Type = keychain.Type(f.Type)
	return c, nil
}

func (f WriteFlags) Config() (Config, error) {
	cfg, err := f.pluginConfig()
	if err != nil {
		return Config{}, err
	}
	cfg.Type = keychain.Type(f.Type)
	cfg.UpdateInPlace = f.UpdateInPlace
	cfg.Accessibility = keychain.Accessibility(f.Accessibility)
	return cfg, nil
}

// DefaultConfigForReading returns a Config with default values
// suitable for reading from the keychain.
func DefaultConfigForReading() Config {
	cfg, _ := KeychainFlags{}.pluginConfig()
	cfg.Type = keychain.KeychainAll
	return cfg
}

// Config represents the configuration for a keychain plugin.
type Config struct {
	Binary        string                 `yaml:"plugin_binary" doc:"plugin binary to use, if not specified it defaults to DefaultPluginBinary, the binary must be present in the PATH or the specified app bundle or be an absolute path" json:"-"`
	UseApp        string                 `yaml:"app_bundle" doc:"app bundle that contains the plugin binary, if specified it takes precedence over Binary for locating the plugin binary, it defaults to DefaultPluginAppBundlePath" json:"-"`
	Type          keychain.Type          `yaml:"keychain_type" doc:"the type of keychain to use, currently supported types are: file, data-protection and icloud" json:"type"`
	Account       string                 `yaml:"account" doc:"account that the keychain item belongs to" json:"account"`
	UpdateInPlace bool                   `yaml:"update_in_place" doc:"set to true to update existing item in place" json:"update_in_place,omitempty"`
	Accessibility keychain.Accessibility `yaml:"accessibility,omitempty" doc:"optional accessibility level for the keychain item" json:"accessibility,omitempty"`
}

// FS returns a plugins.FS based on the Config. If the config does
// not specify a binary DefaultPluginBinaryPath will be used.
func (pc Config) FS() *plugins.FS {
	binary, _ := LookupPluginBinary(pc.UseApp, pc.Binary)
	return plugins.NewFS(binary, pc)
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

// ReadRequest reads a plugin request from the provided reader and returns
// the request. If any errors are encountered then the returned response represents
// an error and should be returned to the plugin caller. Otherwise the response is nil.
func (ps *Server) ReadRequest(ctx context.Context, rd io.Reader) (*Config, plugins.Request, *plugins.Response) {
	var req plugins.Request
	dec := json.NewDecoder(rd)
	if err := dec.Decode(&req); err != nil {
		return nil, plugins.Request{}, errorResponse(ctx, req, "failed to decode request", err.Error())
	}
	var cfg Config
	if err := json.Unmarshal(req.SysSpecific, &cfg); err != nil {
		return nil, plugins.Request{}, errorResponse(ctx, req, "failed to unmarshal sys_specific", err.Error())
	}
	ps.opts.logger.Info("new request",
		"id", req.ID,
		"account", cfg.Account,
		"key", req.Keyname,
		"type", cfg.Type,
		"accessibility", cfg.Accessibility,
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
	kc := keychain.New(cfg.Type, cfg.Account,
		keychain.WithUpdateInPlace(cfg.UpdateInPlace),
		keychain.WithAccessibility(cfg.Accessibility),
	)
	if req.Write {
		return ps.handleWrite(ctx, kc, req)
	}
	return ps.handleRead(ctx, kc, req)
}

// SendResponse sends the provided response to the plugin caller.
func (ps *Server) SendResponse(ctx context.Context, w io.Writer, resp *plugins.Response) {
	resp.SysSpecific = nil
	output, err := json.Marshal(resp)
	if err != nil {
		resp.Contents = nil
		ps.opts.logger.Error("failed to marshal response", "error", err, "response", resp)
		errResp := errorResponse(ctx, plugins.Request{}, "failed to marshal response", err.Error())
		output, _ = json.Marshal(errResp)
	}
	_, err = w.Write(output)
	if err != nil {
		ps.opts.logger.Error("failed to write response", "error", err)
		return
	}
	ps.opts.logger.Info("sent response", "id", resp.ID, "error", resp.Error)
}
