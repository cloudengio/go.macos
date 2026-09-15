// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cloudeng.io/aws/awsconfig"
	"cloudeng.io/aws/awstestutil"
	"cloudeng.io/cmdutil/keys"
	"cloudeng.io/file"
	"cloudeng.io/file/filetestutil"
	"cloudeng.io/security/keys/keychain/keychaintestutil"
	"cloudeng.io/security/keys/keychain/plugins"
	"github.com/aws/smithy-go"
)

var awsInstance *awstestutil.AWS

func TestMain(m *testing.M) {
	awstestutil.AWSTestMain(m, &awsInstance, awstestutil.WithSecretsManager())
}

func runCLI(ctx context.Context, args ...string) error {
	cmd := cli()
	return cmd.DispatchWithArgs(ctx, "cloudeng-aws-secrets", args...)
}

func TestErrors(t *testing.T) {
	ctx := context.Background()
	p := keychaintestutil.New()
	inMemFS := keychaintestutil.NewFS(p, true)
	ctx = file.ContextWithReadWriteFS(ctx, inMemFS)

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "secret.txt")
	_ = os.WriteFile(tmpFile, []byte("val"), 0600)

	// 1. Missing ARN
	if err := runCLI(ctx, "read", tmpFile); err == nil || !strings.Contains(err.Error(), "missing secret ARN or name") {
		t.Errorf("expected missing ARN error, got: %v", err)
	}

	// 2. Missing keychain item
	if err := runCLI(ctx, "read", "--arn=test-secret", tmpFile); err == nil || !strings.Contains(err.Error(), "no keychain item provided") {
		t.Errorf("expected missing keychain item error, got: %v", err)
	}

	// 3. Missing key info ID
	if err := runCLI(ctx, "read", "--arn=test-secret", "--keychain-item=kc-item", tmpFile); err == nil || !strings.Contains(err.Error(), "no key info ID provided") {
		t.Errorf("expected missing key info ID error, got: %v", err)
	}

	// 4. Missing keychain file in store
	if err := runCLI(ctx, "read", "--arn=test-secret", "--keychain-item=missing-kc", "--aws-key-info-id=my-key", tmpFile); err == nil || !strings.Contains(err.Error(), "failed to read") {
		t.Errorf("expected failed to read error, got: %v", err)
	}

	// 5. Keychain file exists but does not contain the requested key ID
	ims := keys.NewInMemoryKeyStore()
	if err := ims.WriteYAML(ctx, inMemFS, "empty-kc", 0600); err != nil {
		t.Fatalf("writing empty keystore: %v", err)
	}
	if err := runCLI(ctx, "read", "--arn=test-secret", "--keychain-item=empty-kc", "--aws-key-info-id=nonexistent-key", tmpFile); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected key info not found error, got: %v", err)
	}
}

func TestContextWithReadOnlyFS(t *testing.T) {
	ctx := context.Background()
	p := keychaintestutil.New()
	readOnlyFS := keychaintestutil.NewFS(p, false)
	ctx = file.ContextWithFS(ctx, readOnlyFS)

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "secret.txt")
	_ = os.WriteFile(tmpFile, []byte("val"), 0600)

	// Should successfully read from ContextWithFS rather than falling back to system keychain
	ims := keys.NewInMemoryKeyStore()
	if err := ims.WriteYAML(context.Background(), keychaintestutil.NewFS(p, true), "ro-kc", 0600); err != nil {
		t.Fatalf("writing keystore: %v", err)
	}

	if err := runCLI(ctx, "read", "--arn=test-secret", "--keychain-item=ro-kc", "--aws-key-info-id=nonexistent-key", tmpFile); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected key info not found error from injected read-only FS, got: %v", err)
	}
}

func TestHandleError(t *testing.T) {
	if err := handleError(nil); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	sampleErr := errors.New("sample error")
	if err := handleError(sampleErr); !errors.Is(err, sampleErr) {
		t.Errorf("expected sampleErr, got %v", err)
	}

	// 1. Test smithy.GenericAPIError
	apiErr := &smithy.GenericAPIError{
		Code:    "ResourceNotFoundException",
		Message: "Secret not found in SecretsManager",
	}
	out, err := filetestutil.CaptureStderr(func() error {
		return handleError(apiErr)
	})
	if !errors.Is(err, apiErr) {
		t.Errorf("expected apiErr, got %v", err)
	}
	if !strings.Contains(string(out), "ResourceNotFoundException") || !strings.Contains(string(out), "Secret not found in SecretsManager") {
		t.Errorf("stderr missing expected AWS API error text: %s", out)
	}

	// 2. Test smithy.OperationError wrapping apiErr
	opErr := &smithy.OperationError{
		ServiceID:     "Secrets Manager",
		OperationName: "GetSecretValue",
		Err:           apiErr,
	}
	out, err = filetestutil.CaptureStderr(func() error {
		return handleError(opErr)
	})
	if !errors.Is(err, opErr) {
		t.Errorf("expected opErr, got %v", err)
	}
	if !strings.Contains(string(out), "Secrets Manager/GetSecretValue") || !strings.Contains(string(out), "ResourceNotFoundException") {
		t.Errorf("stderr missing expected AWS Operation error text: %s", out)
	}

	// 3. Test plugins.Error
	pluginErr := &plugins.Error{
		Message: "keychain locked",
		Detail:  "user needs to unlock",
		Stderr:  "auth failure",
	}
	out, err = filetestutil.CaptureStderr(func() error {
		return handleError(pluginErr)
	})
	if !errors.Is(err, pluginErr) {
		t.Errorf("expected pluginErr, got %v", err)
	}
	if !strings.Contains(string(out), "keychain locked") || !strings.Contains(string(out), "auth failure") {
		t.Errorf("stderr missing expected plugin error text: %s", out)
	}
}

func TestIntegration(t *testing.T) {
	awstestutil.SkipAWSTests(t)
	t.Setenv("AWS_ENDPOINT_URL", awsInstance.URL())
	t.Setenv("AWS_REGION", "us-east-1")
	ctx := context.Background()

	p := keychaintestutil.New()
	inMemFS := keychaintestutil.NewFS(p, true)
	ctx = file.ContextWithReadWriteFS(ctx, inMemFS)

	// Set up AWS credentials in in-memory keychain store
	ki := awsconfig.NewKeyInfo("test-user", "test-key", []byte("test-secret-key"), awsconfig.KeyInfoExtra{
		AccessKeyID: "test-access-key",
		Region:      "us-east-1",
	})
	ims := keys.NewInMemoryKeyStore()
	ims.Add(ki)
	if err := ims.WriteYAML(ctx, inMemFS, "aws-keychain", 0600); err != nil {
		t.Fatalf("writing keystore to in-memory FS: %v", err)
	}

	// Inject AWS config connected to local mock SecretsManager
	cfg := awstestutil.DefaultAWSConfig()
	resClient := awsInstance.SecretsManager(cfg)
	_ = resClient
	ctx = awsconfig.ContextWith(ctx, cfg)

	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("super-secret-aws-value"), 0600); err != nil {
		t.Fatalf("writing secret file: %v", err)
	}

	arnName := "arn:aws:secretsmanager:us-east-1:000000000000:secret:test-secret"

	// 1. Write secret to AWS SecretsManager
	if err := runCLI(ctx, "write",
		"--arn="+arnName,
		"--keychain-item=aws-keychain",
		"--aws-key-info-id=test-key",
		"--aws-key-info-user=test-user",
		secretFile); err != nil {
		t.Fatalf("runCLI write: %v", err)
	}

	// 2. Read back secret from AWS SecretsManager
	outFile := filepath.Join(tmpDir, "read-back.txt")
	if err := runCLI(ctx, "read",
		"--arn="+arnName,
		"--keychain-item=aws-keychain",
		"--aws-key-info-id=test-key",
		"--aws-key-info-user=test-user",
		outFile); err != nil {
		t.Fatalf("runCLI read: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading read-back file: %v", err)
	}
	if string(data) != "super-secret-aws-value" {
		t.Errorf("read secret = %q, want super-secret-aws-value", string(data))
	}

	// 3. Test writing from stdin
	err = filetestutil.FeedStdin("stdin-aws-secret-data", func() error {
		return runCLI(ctx, "write",
			"--arn=arn:aws:secretsmanager:us-east-1:000000000000:secret:stdin-secret",
			"--keychain-item=aws-keychain",
			"--aws-key-info-id=test-key",
			"--aws-key-info-user=test-user",
			"-")
	})
	if err != nil {
		t.Fatalf("write from stdin: %v", err)
	}

	// 4. Test reading to stdout
	out, err := filetestutil.CaptureStdout(func() error {
		return runCLI(ctx, "read",
			"--arn=arn:aws:secretsmanager:us-east-1:000000000000:secret:stdin-secret",
			"--keychain-item=aws-keychain",
			"--aws-key-info-id=test-key",
			"--aws-key-info-user=test-user",
			"-")
	})
	if err != nil {
		t.Fatalf("read to stdout: %v", err)
	}
	if string(out) != "stdin-aws-secret-data" {
		t.Errorf("stdout = %q, want stdin-aws-secret-data", out)
	}
}
