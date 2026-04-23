// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

// Package tartvm implements cloudeng.io/vms.Instance using the tart CLI on macOS.
package tartvm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloudeng.io/errors"
	"cloudeng.io/logging/ctxlog"
	"cloudeng.io/os/executil"
	"cloudeng.io/vms"
)

// Instance implements vms.Instance backed by the tart CLI.
// source is the OCI reference used for cloning;
// name is the local VM name.
// All images must have the tart agent installed and be compatible with the tart CLI
// version installed locally.
type Instance struct {
	source      string
	name        string
	logger      *slog.Logger
	opts        options
	suspendable bool

	stateMu   sync.Mutex
	state     vms.State // GUARDED by stateMu
	currentIP string    // GUARDED by stateMu

	// opMutex used to serialize operations, Clone, Start,
	// Stop, Suspend and Delete are all mutually exclusive for example.
	opMutex sync.Mutex

	asyncWait *executil.AsyncWait // GUARDED by opMutex, used to track the tart run command when starting the VM.
}

// Option represents an Option to New.
type Option func(o *options)

type options struct {
	pollingInterval  time.Duration
	outputBufSize    int
	runTimeout       time.Duration
	forceStopTimeout time.Duration
	runOptions       []string
}

// WithPollingInterval sets the interval to use for polling the
// state of the VM when waiting for state transitions, network availability, etc.
//
//	The default is DefaultPollingInterval.
func WithPollingInterval(interval time.Duration) Option {
	return func(o *options) {
		o.pollingInterval = interval
	}
}

// WithRunTimeout sets a timeout for the VM to reach a running state after Start is called.
// The default is DefaultRunTimeout.
func WithRunTimeout(timeout time.Duration) Option {
	return func(o *options) {
		o.runTimeout = timeout
	}
}

// WithRunOptions sets additional options to pass to the "tart run" command.
// The default is the value returned by DefaultRunOptions.
func WithRunOptions(opts ...string) Option {
	return func(o *options) {
		o.runOptions = append(o.runOptions, opts...)
	}
}

// WithDefaultForceStopTimeout sets the timeout for forcefully stopping a VM when
// a run operation, or other operation, fails and the error recovery needs to
// stop the VM.
func WithDefaultForceStopTimeout(timeout time.Duration) Option {
	return func(o *options) {
		o.forceStopTimeout = timeout
	}
}

const (
	DefaultPollingInterval  = 100 * time.Millisecond
	DefaultOutputBufferSize = 16 * 1024 // 16KiB
	DefaultRunTimeout       = 2 * time.Minute
	DefaultForceStopTimeout = 2 * time.Second
)

// DefaultRunOptions are safe defaults that work with mac and linux tart VMs.
// Linux does not currently support suspend.
func DefaultRunOptions() []string {
	return slices.Clone([]string{"--no-graphics", "--no-audio"})
}

func DefaultMacOSRunOptions() []string {
	return slices.Clone([]string{"--no-graphics", "--no-audio", "--suspendable"})
}

func DefaultLinuxRunOptions() []string {
	return DefaultRunOptions()
}

// New returns an Instance in StateInitial, source is the tart image or OCI
// reference to clone from; name is the local VM name.
func New(ctx context.Context, source, name string, opts ...Option) *Instance {
	options := options{
		pollingInterval:  DefaultPollingInterval,
		runTimeout:       DefaultRunTimeout,
		forceStopTimeout: DefaultForceStopTimeout,
		outputBufSize:    DefaultOutputBufferSize,
	}
	for _, opt := range opts {
		opt(&options)
	}
	if len(options.runOptions) == 0 {
		options.runOptions = DefaultRunOptions()
	}
	logger := ctxlog.Logger(ctx).With("module", "tart", "source", source, "name", name)
	return &Instance{
		source:      source,
		name:        name,
		state:       vms.StateInitial,
		opts:        options,
		logger:      logger,
		suspendable: slices.Contains(options.runOptions, "--suspendable"),
	}
}

// Name returns the local VM name.
func (inst *Instance) Name() string { return inst.name }

func (inst *Instance) setState(state vms.State) vms.State {
	inst.stateMu.Lock()
	defer inst.stateMu.Unlock()
	prev := inst.state
	inst.state = state
	return prev
}

func (inst *Instance) isActionAllowed(action vms.Action) (vms.State, bool) {
	inst.stateMu.Lock()
	defer inst.stateMu.Unlock()
	return inst.state, inst.state.Allowed(action)
}

func (inst *Instance) getIP() string {
	inst.stateMu.Lock()
	defer inst.stateMu.Unlock()
	return inst.currentIP
}

// State returns the current state and any error from a running
// instance that terminated without being stopped or suspend.
func (inst *Instance) State(ctx context.Context) vms.State {
	inst.stateMu.Lock()
	defer inst.stateMu.Unlock()
	return inst.state
}

func (inst *Instance) Suspendable() bool {
	return inst.suspendable
}

// runSyncEcxlusive runs a tart command synchronously, checking
// that the current state allows the requested transition.
func (inst *Instance) runSyncExclusive(ctx context.Context, action vms.Action, intermediate, target vms.State, args ...string) error {
	if s, allowed := inst.isActionAllowed(action); !allowed {
		return fmt.Errorf("action %s not allowed in state %s", action, s)
	}
	prev := inst.setState(intermediate)
	inst.logger.Info("tart command issued", "args", args)
	stdoutBuf := bytes.NewBuffer(make([]byte, 0, 1024))
	stderrBuf := executil.NewTailWriter((1024))
	start := time.Now()
	cmd := exec.CommandContext(ctx, "tart", args...)
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf
	err := cmd.Run()
	inst.logger.Info("tart command completed", "args", args, "stderr", string(stderrBuf.Bytes()), "error", err, "duration", time.Since(start).String())
	if err != nil {
		inst.setState(prev)
		return convertError(args, string(stderrBuf.Bytes()), err)
	}
	inst.setState(target)
	return nil
}

var (
	reVMNotExist   = regexp.MustCompile(`the specified VM "[^"]+" does not exist`)
	reVMNotRunning = regexp.MustCompile(`VM "[^"]+" is not running`)
)

func isAlreadyStoppedErrorMsg(stderr string) bool {
	return reVMNotRunning.MatchString(stderr)
}

func convertError(args []string, stderr string, err error) error {
	cl := strings.Join(args, " ")
	if reVMNotExist.MatchString(stderr) {
		return fmt.Errorf("tart %s: VM does not exist: %s: %v: %w", cl, stderr, err, vms.ErrVMNotFound)
	}
	if isAlreadyStoppedErrorMsg(stderr) {
		return fmt.Errorf("tart %s: VM is not running: %s: %v: %w", cl, stderr, err, vms.ErrVMNotRunning)
	}
	return fmt.Errorf("tart %s: %s: %w", cl, stderr, err)
}

// Clone runs "tart clone <source> <name>" and transitions to StateReadyToRun.
func (inst *Instance) Clone(ctx context.Context) error {
	inst.opMutex.Lock()
	defer inst.opMutex.Unlock()
	return inst.runSyncExclusive(ctx,
		vms.ActionClone,  // action
		vms.StateCloning, // intermediate state
		vms.StateStopped, // target state
		"clone", inst.source, inst.name)
}

// Delete runs "tart delete <name>" and transitions to StateDeleted.
func (inst *Instance) Delete(ctx context.Context) error {
	inst.opMutex.Lock()
	defer inst.opMutex.Unlock()
	return inst.runSyncExclusive(ctx,
		vms.ActionDelete,
		vms.StateDeleting,
		vms.StateDeleted,
		"delete", inst.name)
}

// Start runs "tart run <name> --no-graphics --suspendable" in the background
// and returns immediately. Use vms.WaitForState to block until StateRunning.
func (inst *Instance) Start(ctx context.Context, stdout, stderr io.Writer) error {
	inst.opMutex.Lock()
	defer inst.opMutex.Unlock()
	if s, allowed := inst.isActionAllowed(vms.ActionStart); !allowed {
		return fmt.Errorf("action %s not allowed in state %s", vms.ActionStart, s)
	}

	args := []string{"run", inst.name}
	args = append(args, inst.opts.runOptions...)

	inst.logger.Info("tart run", "args", args)

	start := time.Now()
	prev := inst.setState(vms.StateStarting)

	cmd := exec.CommandContext(ctx, "tart", args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = nil // Detach stdin entirely
	if err := cmd.Start(); err != nil {
		inst.logger.Error("tart run", "args", args, "error", err)
		return inst.cmdStartFailed(ctx, prev, fmt.Errorf("tart %s: %w", strings.Join(args, " "), err))
	}
	inst.logger.Info("tart run cmd.Start called", "args", args, "pid", cmd.Process.Pid)

	if err := inst.waitForTartState(ctx, "running", inst.opts.pollingInterval); err != nil {
		return inst.runFailed(ctx, prev, cmd,
			fmt.Errorf("tart %s: %w: failed to reach tart 'running' state", strings.Join(args, " "), err))
	}

	ip, err := inst.runIPWait(ctx)
	if err != nil || ip == "" {
		return inst.runFailed(ctx, prev, cmd,
			fmt.Errorf("tart %s: %w: failed to get IP address", strings.Join(args, " "), err))
	}

	if err := inst.waitForReadyUsingExec(ctx); err != nil {
		return inst.runFailed(ctx, prev, cmd,
			fmt.Errorf("tart %s: %w: failed to run tart exec", strings.Join(args, " "), err))
	}

	inst.stateMu.Lock()
	inst.currentIP = strings.TrimSpace(ip)
	inst.state = vms.StateRunning
	inst.asyncWait = executil.NewAsyncWait(cmd)
	inst.stateMu.Unlock()
	inst.logger.Info("tart run completed", "args", args, "ip", ip, "pid", cmd.Process.Pid, "duration", time.Since(start).String())
	return nil
}

func (inst *Instance) runForceStop(ctx context.Context, timeout time.Duration) error {
	out, err := exec.CommandContext(ctx, "tart", "stop", inst.name, "--timeout", strconv.Itoa(int(timeout.Seconds()))).CombinedOutput()
	if err != nil {
		if isAlreadyStoppedErrorMsg(string(out)) {
			return nil
		}
		return fmt.Errorf("tart stop --timeout %v: %w", timeout, err)
	}
	return nil
}

// opMutex must be held by the caller.
func (inst *Instance) cmdStartFailed(ctx context.Context, prevState vms.State, prevErr error) error {
	if err := inst.waitForTartState(ctx, "stopped", inst.opts.pollingInterval); err != nil {
		var errs errors.M
		errs.Append(prevErr)
		errs.Append(err)
		inst.setState(vms.StateErrorUnknown)
		inst.logger.Error("tart run cmd.Start failure: revert to StateErrorUnknown", "error", errs.Err())
		return errs.Err()
	}
	inst.logger.Error("tart run cmd.Start failure: revert to previous state", "state", prevState, "error", prevErr)
	inst.setState(prevState)
	return prevErr
}

// opMutex must be held by the caller.
func (inst *Instance) runFailed(ctx context.Context, prevState vms.State, cmd *exec.Cmd, prevErr error) error {
	var errs errors.M
	errs.Append(prevErr)
	err := inst.runForceStop(ctx, inst.opts.forceStopTimeout)
	errs.Append(err)

	if err := cmd.Wait(); err != nil {
		errs.Append(err)
	}
	if err := inst.waitForTartState(ctx, "stopped", inst.opts.pollingInterval); err != nil {
		errs.Append(err)
		inst.setState(vms.StateErrorUnknown)
		inst.logger.Error("tart run failure: revert to StateErrorUnknown", "error", errs.Err())
		return errs.Err()
	}
	inst.logger.Error("tart run failure: revert to previous state", "state", prevState, "error", errs.Err())
	inst.setState(prevState)
	return prevErr
}

func (inst *Instance) clearIP() {
	inst.stateMu.Lock()
	defer inst.stateMu.Unlock()
	inst.currentIP = ""
}

func (inst *Instance) verifyState(ctx context.Context, state string) bool {
	return inst.waitForTartState(ctx, state, inst.opts.pollingInterval) == nil
}

func (inst *Instance) handleStopSuspend(ctx context.Context, args ...string) (runErr, stopErr error) {
	if inst.asyncWait == nil {
		// should never be reached since the state machine prevents stop/suspend from being called in a state where
		// asyncWait would be nil, but just in case, return an error instead of panicking.
		return nil, fmt.Errorf("missing asyncWait for running instance")
	}
	exited, err := inst.asyncWait.WaitDone()
	if exited {
		if err != nil {
			return err, nil
		}
		return nil, nil
	}
	inst.logger.Info("tart command issued", "args", args)
	start := time.Now()
	stdoutBuf := bytes.NewBuffer(make([]byte, 0, 1024))
	stderrBuf := executil.NewTailWriter((1024))
	cmd := exec.CommandContext(ctx, "tart", args...)
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf
	err = cmd.Run()
	stderr := string(stderrBuf.Bytes())
	inst.logger.Info("tart command completed", "args", args, "stderr", stderr, "error", err, "duration", time.Since(start).String())
	if err != nil {
		if isAlreadyStoppedErrorMsg(stderr) {
			return nil, nil
		}
		return nil, convertError(args, stderr, err)
	}
	runErr = inst.asyncWait.Wait()
	return runErr, nil
}

func (inst *Instance) runSyncExclusiveStopSuspend(ctx context.Context, action vms.Action, intermediate, target vms.State, tartState string, args ...string) (runErr, stopSuspendErr error) {
	if s, allowed := inst.isActionAllowed(action); !allowed {
		return nil, fmt.Errorf("action %s not allowed in state %s", action, s)
	}
	prev := inst.setState(intermediate)
	if prev == target {
		inst.setState(target)
		return nil, nil
	}
	runErr, stopSuspendErr = inst.handleStopSuspend(ctx, args...)
	if inst.verifyState(ctx, tartState) {
		// stopped, return any errors, but the state is ok.
		inst.logger.Info("stop/suspend command completed, vm is stopped", "args", args, "runErr", runErr, "stopSuspendErr", stopSuspendErr)
		inst.setState(target)
		return runErr, nil
	}
	inst.logger.Warn("stop/suspend command completed, vm is NOT stopped", "args", args, "runErr", runErr, "stopSuspendErr", stopSuspendErr)
	inst.setState(vms.StateErrorUnknown)
	return runErr, stopSuspendErr
}

func (inst *Instance) Stop(ctx context.Context, timeout time.Duration) (runErr, stopErr error) {
	args := []string{"stop", inst.name}
	if timeout > 0 {
		args = append(args, "--timeout", strconv.Itoa(int(timeout.Seconds())))
	}
	inst.opMutex.Lock()
	defer inst.opMutex.Unlock()
	runErr, stopErr = inst.runSyncExclusiveStopSuspend(ctx,
		vms.ActionStop,    // action
		vms.StateStopping, // intermediate state
		vms.StateStopped,  // target state
		"stopped",
		args...)
	inst.clearIP()
	return runErr, stopErr
}

// Suspend runs "tart suspend <name>" and transitions to StateSuspended.
func (inst *Instance) Suspend(ctx context.Context) error {
	inst.opMutex.Lock()
	defer inst.opMutex.Unlock()
	runErr, suspErr := inst.runSyncExclusiveStopSuspend(ctx,
		vms.ActionSuspend,   // action
		vms.StateSuspending, // intermediate state
		vms.StateSuspended,  // target state
		"suspended",
		"suspend", inst.name)
	if runErr == nil && suspErr == nil {
		return nil
	}
	if runErr != nil {
		if suspErr == nil {
			return fmt.Errorf("failed to suspend VM, it already exited with error: %w", runErr)
		}
		return fmt.Errorf("failed to suspend VM: %w; it already exited with error: %v", suspErr, runErr)
	}
	return suspErr
}

// Properties returns VM properties. It calls "tart ip" to obtain the IP
// address and returns SSH connection arguments for the default admin user.
func (inst *Instance) Properties(ctx context.Context) (vms.Properties, error) {
	if state := inst.State(ctx); state != vms.StateRunning {
		return vms.Properties{}, fmt.Errorf("properties only available for running VMs, current state: %s", state)
	}
	ip := inst.getIP()
	if ip == "" {
		return vms.Properties{}, fmt.Errorf("tart ip %s: no IP address available", inst.name)
	}
	return vms.Properties{
		IP: ip,
	}, nil
}

// Exec runs "tart exec <name> <cmd> <args...>" with the output connected to the
// provided writers. It returns when the command completes.
func (inst *Instance) Exec(ctx context.Context, stdout, stderr io.Writer, cmd string, args ...string) error {
	if state := inst.State(ctx); state != vms.StateRunning {
		return fmt.Errorf("exec only available for running VMs, current state: %s", state)
	}
	allArgs := append([]string{"exec", inst.name, cmd}, args...)
	c := exec.CommandContext(ctx, "tart", allArgs...)
	c.Stdout = stdout
	c.Stderr = stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("tart %s: %w", strings.Join(allArgs, " "), err)
	}
	return nil
}

func (inst *Instance) waitForReadyUsingExec(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, inst.opts.runTimeout)
	defer cancel()
	err := inst.waitForReadyUsingExecOne(ctx)
	if err == nil {
		return nil
	}
	inst.logger.Info("tart run: tart exec failed, retrying", "error", err)
	ticker := time.NewTicker(inst.opts.pollingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			err := inst.waitForReadyUsingExecOne(ctx)
			if err == nil {
				return nil
			}
			inst.logger.Info("tart run: tart exec failed, retrying", "error", err)
		}
	}
}

func (inst *Instance) waitForReadyUsingExecOne(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, inst.opts.runTimeout)
	defer cancel()
	out := executil.NewTailWriter(1024)
	now := time.Now().String()
	cmd := exec.CommandContext(ctx, "tart", "exec", inst.name, "echo", now)
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Stdin = nil // Detach stdin entirely
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tart exec: %s\n%w", out.Bytes(), err)
	}
	read := strings.TrimSpace(string(out.Bytes()))
	if read != now {
		return fmt.Errorf("tart exec: output does not contain expected string: %s != %s", string(read), now)
	}
	return nil
}

func (inst *Instance) runIPWait(ctx context.Context) (string, error) {
	args := []string{"ip", inst.name, "--wait", strconv.Itoa(int(inst.opts.runTimeout.Seconds()))}
	cmd := exec.CommandContext(ctx, "tart", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tart %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (inst *Instance) getStateUsingList(ctx context.Context, state string) (bool, error) {
	all, err := ListAll(ctx)
	if err != nil {
		return true, fmt.Errorf("failed to list tart VMs: %w", err)
	}
	entry, found := all.Lookup(inst.name)
	if !found {
		return true, fmt.Errorf("tart list: VM %q not found", inst.name)
	}
	return entry.State == state, nil
}

func (inst *Instance) waitForTartState(ctx context.Context, state string, interval time.Duration) error {
	inst.logger.Info("waiting for tart state", "state", state)
	if done, err := inst.getStateUsingList(ctx, state); done {
		return err
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if done, err := inst.getStateUsingList(ctx, state); done {
				return err
			}
		}
	}
}
