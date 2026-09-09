// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cloudeng.io/macos/buildtools"
)

func TestCreateChromeExtensionID(t *testing.T) {
	br := buildtools.Browser{}

	pemBytes, id, err := br.CreateChromeExtensionID()
	if err != nil {
		t.Fatalf("CreateChromeExtensionID failed: %v", err)
	}
	if len(id) != 32 {
		t.Fatalf("expected extension ID length 32, got %d (%q)", len(id), id)
	}
	for _, ch := range id {
		if ch < 'a' || ch > 'p' {
			t.Errorf("invalid character %c in extension ID %q", ch, id)
		}
	}

	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		t.Fatalf("invalid PEM block: %v", block)
	}

	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "test_key.pem")
	if err := os.WriteFile(keyFile, pemBytes, 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	pubKeyBytes, readID, err := br.ReadChromeExtensionID(keyFile)
	if err != nil {
		t.Fatalf("ReadChromeExtensionID failed: %v", err)
	}
	if readID != id {
		t.Errorf("extension ID mismatch: got %q, want %q", readID, id)
	}

	parsedPub, err := x509.ParsePKIXPublicKey(pubKeyBytes)
	if err != nil {
		t.Fatalf("ParsePKIXPublicKey failed (public key should be SPKI): %v", err)
	}
	if _, ok := parsedPub.(*rsa.PublicKey); !ok {
		t.Fatalf("expected *rsa.PublicKey, got %T", parsedPub)
	}
}

func TestReadChromeExtensionIDPKCS8(t *testing.T) {
	br := buildtools.Browser{}
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		t.Fatal(err)
	}
	pkcs8PEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Bytes})
	tmpDir := t.TempDir()
	pkcs8File := filepath.Join(tmpDir, "pkcs8_key.pem")
	if err := os.WriteFile(pkcs8File, pkcs8PEM, 0600); err != nil {
		t.Fatal(err)
	}

	pkcs8PubKey, pkcs8ID, err := br.ReadChromeExtensionID(pkcs8File)
	if err != nil {
		t.Fatalf("ReadChromeExtensionID failed on PKCS#8 key: %v", err)
	}
	if len(pkcs8ID) != 32 {
		t.Errorf("expected 32-char ID for PKCS#8 key, got %q", pkcs8ID)
	}
	if len(pkcs8PubKey) == 0 {
		t.Error("expected non-empty public key for PKCS#8 key")
	}
}

func TestReadChromeExtensionIDErrors(t *testing.T) {
	br := buildtools.Browser{}
	if _, _, err := br.ReadChromeExtensionID("/nonexistent/file.pem"); err == nil {
		t.Error("expected error for nonexistent file")
	}

	tmpDir := t.TempDir()
	invalidPEMFile := filepath.Join(tmpDir, "invalid.pem")
	if err := os.WriteFile(invalidPEMFile, []byte("not pem"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := br.ReadChromeExtensionID(invalidPEMFile); err == nil {
		t.Error("expected error for invalid PEM block")
	}

	wrongTypePEMFile := filepath.Join(tmpDir, "wrong_type.pem")
	wrongPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("dummy")})
	if err := os.WriteFile(wrongTypePEMFile, wrongPEM, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := br.ReadChromeExtensionID(wrongTypePEMFile); err == nil {
		t.Error("expected error for wrong PEM block type")
	}
}

func TestBrowserTypeString(t *testing.T) {
	for _, tc := range []struct {
		b    buildtools.BrowserType
		want string
	}{
		{buildtools.Chrome, "chrome"},
		{buildtools.Firefox, "firefox"},
		{buildtools.Safari, "safari"},
		{buildtools.Edge, "edge"},
		{buildtools.BrowserType(99), "unknown"},
	} {
		if got := tc.b.String(); got != tc.want {
			t.Errorf("BrowserType(%d).String() = %q, want %q", int(tc.b), got, tc.want)
		}
	}
}

func TestNativeMessagingConfigValidation(t *testing.T) {
	ctx := t.Context()
	runner := buildtools.NewCommandRunner()

	// Edge validation
	edgeCfg := buildtools.NativeMessagingConfig{
		Name:           "com.example.edge_host",
		Path:           "/Applications/Example.app/Contents/Helpers/edge_host",
		AllowedOrigins: []string{"chrome-extension://abcdefghijklmnopabcdefghijklmnop/"},
	}
	step := edgeCfg.Validate(buildtools.Edge)
	if _, err := step.Run(ctx, runner); err != nil {
		t.Errorf("expected Edge validation to succeed, got %v", err)
	}

	// Edge with empty path
	edgeEmptyPath := buildtools.NativeMessagingConfig{
		Name:           "com.example.edge_host",
		AllowedOrigins: []string{"chrome-extension://abcdefghijklmnopabcdefghijklmnop/"},
	}
	if _, err := edgeEmptyPath.Validate(buildtools.Edge).Run(ctx, runner); err == nil {
		t.Error("expected error for Edge with empty path")
	}

	// Firefox with empty path
	ffEmptyPath := buildtools.NativeMessagingConfig{
		Name:              "com.example.ff",
		AllowedExtensions: []string{"ext@example.com"},
	}
	if _, err := ffEmptyPath.Validate(buildtools.Firefox).Run(ctx, runner); err == nil {
		t.Error("expected error for Firefox with empty path")
	}

	// Safari is rejected
	safariCfg := buildtools.NativeMessagingConfig{
		Name: "com.example.safari",
		Path: "/path/to/host",
	}
	if _, err := safariCfg.Validate(buildtools.Safari).Run(ctx, runner); err == nil {
		t.Error("expected error for Safari native messaging manifest validation")
	}

	// Unknown browser is rejected
	if _, err := edgeCfg.Validate(buildtools.BrowserType(99)).Run(ctx, runner); err == nil {
		t.Error("expected error for unknown browser")
	}

	// AppendChromeOrigin
	cfg := buildtools.NativeMessagingConfig{Name: "com.example.app"}
	cfg.AppendChromeOrigin("abcdefghijklmnopabcdefghijklmnop")
	if len(cfg.AllowedOrigins) != 1 || !strings.Contains(cfg.AllowedOrigins[0], "abcdefghijklmnopabcdefghijklmnop") {
		t.Errorf("AppendChromeOrigin failed, got: %v", cfg.AllowedOrigins)
	}
}
