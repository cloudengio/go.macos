// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"cloudeng.io/cmdutil/cmdyaml"
	"cloudeng.io/cmdutil/flags"
	"cloudeng.io/macos/cmd/gobundle/gobundleconfig"
)

var (
	verbose    bool
	verboseLog = &strings.Builder{}
)

func printf(format string, args ...any) {
	fmt.Fprintf(verboseLog, format, args...)
	if verbose {
		fmt.Printf(format, args...)
	}
}

type localFlags struct {
	Help              bool   `cmd:"help,false,show a help message"`
	ShowConfigAndExit bool   `cmd:"show-config,false,show the configuration and exit"`
	SharedConfig      string `cmd:"shared-config,,shared configuration file"`
	AppConfig         string `cmd:"app-config,,application configuration file"`
	Verbose           bool   `cmd:"verbose,false,enable verbose logging"`
	Notarize          bool   `cmd:"notarize,false,request notarization of the signed bundle"`
}

// signThenRunVerb is a special verb used to indicate that the gobundle command
// should sign the binary and then run it. It is used internally by the gobundle
// command and should not be used directly by users, gobundle is invoked by
// go run via the -exec flag, and the sign-then-run verb is used to indicate
// that the binary should be signed and optionally notarized and then executed.
const signThenRunVerb = "sign-then-run"

func parseFlags(fs *flag.FlagSet) (execBinary string, execArgs []string, err error) {
	if err := fs.Parse(os.Args[1:]); err != nil {
		// command line is of the form gobundle [flags] <go verb> [args...]
		return "", nil, err
	}
	if idx := slices.Index(os.Args, signThenRunVerb); idx > 0 {
		// command line is of the form gobundle <a.out> [flags] sign-then-run [args...]
		if len(os.Args) > 2 {
			execBinary = os.Args[1]
		}
		if err := fs.Parse(os.Args[2:]); err != nil {
			return "", nil, err
		}
		return execBinary, os.Args[idx+1:], nil
	}
	return "", nil, nil
}

// parseGoArgs parses the command line arguments to determine the go verb and its
// arguments.
// This includes special handling to determine the absolute path of the binary to
// be built when the go verb is "build" or "install".
func parseGoArgs(args []string) (binary, verb string, verbArgs []string) {
	if len(args) == 0 {
		return "", "", nil
	}
	verb = args[0]
	switch verb {
	case "build":
		binary = getBuildBinaryAbs(args[1:])
		verbArgs = args[1:]
	case "install":
		binary = getInstallBinaryAbs(args[1:])
		verbArgs = args[1:]
	default:
		verbArgs = args[1:] //		verbArgs = args
	}
	return binary, verb, verbArgs
}

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmdFs := flag.NewFlagSet("gobundle", flag.ExitOnError)
	var lf localFlags
	if err := flags.RegisterFlagsInStruct(cmdFs, "cmd", &lf, nil, nil); err != nil {
		panic(err)
	}

	execBinary, execArgs, err := parseFlags(cmdFs)
	if err != nil || lf.Help {
		printHelpAndExit()
	}

	verbose = lf.Verbose

	files := gobundleconfig.LocateConfigFiles(lf.SharedConfig, lf.AppConfig)

	var cfg gobundleconfig.T
	parser := cmdyaml.NewParser(cmdyaml.WithStrictFields(true), cmdyaml.WithExpandMapping(os.Getenv))
	if err := parser.ParseFiles(ctx, &cfg, files...); err != nil {
		exit(1, "error loading config from %s: %v\n", strings.Join(files, ", "), err)
	}

	binary, goVerb, goVerbArgs := parseGoArgs(cmdFs.Args())
	if len(execBinary) > 0 {
		// exeBinary will always be an absolute path
		binary = execBinary
	}
	if len(binary) > 0 {
		cfg = cfg.WithDefaultsForBinary(binary)
	}

	if lf.ShowConfigAndExit {
		fmt.Printf("parsed config files: %v\n", strings.Join(files, ", "))
		_ = cmdyaml.WriteConfig(os.Stdout, &cfg)
		return
	}

	if len(execBinary) > 0 {
		runAndExit(func() error {
			return handleGoRunExec(ctx, cfg, lf.Notarize, execBinary, execArgs)
		})
	}

	remArgs := cmdFs.Args()
	if len(remArgs) == 0 {
		rungoExit(ctx)
		return
	}

	cwd, _ := os.Getwd()
	printf("verb: %v, current working directory: %s\n", goVerb, cwd)

	switch goVerb {
	case "build":
		runAndExit(func() error {
			return handleGoBuild(ctx, cfg, lf.Notarize, binary, goVerbArgs)
		})
	case "install":
		runAndExit(func() error {
			return handleGoInstall(ctx, cfg, lf.Notarize, binary, goVerbArgs)
		})
	case "run":
		handleGoRun(ctx, cfg, lf.Notarize, goVerbArgs)
	default:
		rungoExit(ctx, goVerbArgs...)
	}
}

func rungo(ctx context.Context, args []string, env ...string) error {
	cmd := exec.CommandContext(ctx, "go", args...) //nolint:gosec // G702 overly restrictive for this use case.
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func rungoExit(ctx context.Context, args ...string) {
	runAndExit(func() error {
		return rungo(ctx, args)
	})
}

func exit(code int, format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(code)
}

func runAndExit(fn func() error) {
	err := fn()
	if exitErr, ok := err.(*exec.ExitError); ok {
		os.Exit(exitErr.ExitCode())
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", verboseLog.String())
		exit(1, "error: %v\n", err)
	}
	os.Exit(0)
}
