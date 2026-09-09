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

	"cloudeng.io/algo/ratecontrol"
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

	asyncWait *executil.AsyncWait  // GUARDED by opMutex, used to track the tart run command when starting the VM.
	runStderr *executil.TailWriter // GUARDED by opMutex, used to capture the stderr of the tart run command for error reporting if the command fails or the VM exits unexpectedly.
}

// Config contains configuration for tart VM pools.
type Config struct {
	OS               string                               `yaml:"os" doc:"The operating system of the tart VM, either 'macos' or 'linux'"`
	StateBackoff     ratecontrol.ExponentialBackoffConfig `yaml:"state_backoff" doc:"The backoff to use when polling the state of the VM when waiting for state transitions, network availability, etc."`
	RunBackoff       ratecontrol.ExponentialBackoffConfig `yaml:"run_backoff" doc:"The backoff bounding how long to wait for the VM to reach a running state after Start is called."`
	ForceStopBackoff ratecontrol.ExponentialBackoffConfig `yaml:"force_stop_backoff" doc:"The backoff bounding forcefully stopping a VM when a run operation, or other operation, fails and the error recovery needs to stop the VM."`
	RunOptions       []string                             `yaml:"run_options,flow" doc:"Additional options to pass to the tart run command."`
	TartBinary       string                               `yaml:"binary" doc:"Path to the tart binary, defaults to 'tart' on PATH."`
}

func (c *Config) Options() []Option {
	runOpts := c.RunOptions
	if len(runOpts) == 0 {
		switch c.OS {
		case "macos":
			runOpts = DefaultMacOSRunOptions()
		case "linux":
			runOpts = DefaultLinuxRunOptions()
		default:
			runOpts = DefaultRunOptions()
		}
	}
	binary := c.TartBinary
	if binary == "" {
		binary = DefaultTartBinary
	}
	return []Option{
		WithStateBackoff(c.StateBackoff),
		WithRunBackoff(c.RunBackoff),
		WithForceStopBackoff(c.ForceStopBackoff),
		WithRunOptions(runOpts...),
		WithTartBinary(binary),
	}
}

// Option represents an Option to New.
type Option func(o *options)

type options struct {
	stateBackoff     ratecontrol.ExponentialBackoffConfig
	outputBufSize    int
	runBackoff       ratecontrol.ExponentialBackoffConfig
	forceStopBackoff ratecontrol.ExponentialBackoffConfig
	runOptions       []string
	logger           *slog.Logger
	ipAtStart        bool
	tartBinary       string
}

// WithStateBackoff sets the backoff to use when polling the state of the VM
// when waiting for state transitions, network availability, etc.
//
//	The default is DefaultStateBackoff().
func WithStateBackoff(cfg ratecontrol.ExponentialBackoffConfig) Option {
	return func(o *options) {
		if cfg.InitialDelay <= 0 || cfg.Steps <= 0 {
			cfg = DefaultStateBackoff()
		}
		o.stateBackoff = cfg
	}
}

// WithRunBackoff sets the backoff bounding how long to wait for the VM to
// reach a running state after Start is called; its total delay budget also
// bounds obtaining the VM's IP address and the readiness check.
// The default is DefaultRunBackoff().
func WithRunBackoff(cfg ratecontrol.ExponentialBackoffConfig) Option {
	return func(o *options) {
		if cfg.InitialDelay <= 0 || cfg.Steps <= 0 {
			cfg = DefaultRunBackoff()
		}
		o.runBackoff = cfg
	}
}

// WithRunOptions sets additional options to pass to the "tart run" command.
// The default is the value returned by DefaultRunOptions.
func WithRunOptions(opts ...string) Option {
	return func(o *options) {
		if len(opts) > 0 {
			o.runOptions = append(o.runOptions, opts...)
		}
	}
}

// WithForceStopBackoff sets the backoff bounding forcefully stopping a VM when
// a run operation, or other operation, fails and the error recovery needs to
// stop the VM; its total delay budget is used as the graceful shutdown timeout
// passed to "tart stop --timeout".
// The default is DefaultForceStopBackoff().
func WithForceStopBackoff(cfg ratecontrol.ExponentialBackoffConfig) Option {
	return func(o *options) {
		if cfg.InitialDelay <= 0 || cfg.Steps <= 0 {
			cfg = DefaultForceStopBackoff()
		}
		o.forceStopBackoff = cfg
	}
}

// WithLogger sets a logger to use for logging tart commands and state transitions.
func WithLogger(logger *slog.Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}

func WithTartBinary(tartBinary string) Option {
	return func(o *options) {
		o.tartBinary = tartBinary
	}
}

// WithObtainIPAtStart sets whether to obtain the IP address of the VM at start time,
// disable for faster execution of Start and with the IP address obtained on demand.
func WithObtainIPAtStart(ipAtStart bool) Option {
	return func(o *options) {
		o.ipAtStart = ipAtStart
	}
}

const (
	DefaultOutputBufferSize = 16 * 1024 // 16KiB
	DefaultTartBinary       = "tart"
	gracePeriod             = 5 * time.Second
)

// DefaultStateBackoff returns the default backoff used when polling the state
// of the VM: 100ms initial delay doubling over 10 steps, for a total delay
// budget of ~102 seconds.
func DefaultStateBackoff() ratecontrol.ExponentialBackoffConfig {
	return ratecontrol.ExponentialBackoffConfig{InitialDelay: 100 * time.Millisecond, Steps: 10}
}

// DefaultRunBackoff returns the default backoff bounding how long to wait for
// the VM to reach a running state after Start: 1s initial delay doubling over
// 7 steps, for a total delay budget of ~127 seconds.
func DefaultRunBackoff() ratecontrol.ExponentialBackoffConfig {
	return ratecontrol.ExponentialBackoffConfig{InitialDelay: time.Second, Steps: 7}
}

// DefaultForceStopBackoff returns the default backoff bounding forcefully
// stopping a VM during error recovery: 500ms initial delay doubling over
// 5 steps, for a total delay budget of ~15 seconds.
func DefaultForceStopBackoff() ratecontrol.ExponentialBackoffConfig {
	return ratecontrol.ExponentialBackoffConfig{InitialDelay: 500 * time.Millisecond, Steps: 5}
}

func durationSeconds(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	return int((d + time.Second - 1) / time.Second)
}

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
		stateBackoff:     DefaultStateBackoff(),
		runBackoff:       DefaultRunBackoff(),
		forceStopBackoff: DefaultForceStopBackoff(),
		outputBufSize:    DefaultOutputBufferSize,
	}
	for _, opt := range opts {
		opt(&options)
	}
	if len(options.runOptions) == 0 {
		options.runOptions = DefaultRunOptions()
	}
	if options.logger == nil {
		options.logger = ctxlog.Logger(ctx)
	}
	if options.tartBinary == "" {
		options.tartBinary = DefaultTartBinary
	}

	options.logger = options.logger.With("module", "tart", "source", source, "name", name)
	inst := &Instance{
		source:      source,
		name:        name,
		logger:      options.logger,
		state:       vms.StateInitial,
		opts:        options,
		suspendable: slices.Contains(options.runOptions, "--suspendable"),
	}
	return inst
}

// ID returns the local VM's ID/name.
func (inst *Instance) ID() string { return inst.name }

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

func (inst *Instance) needIP() (string, bool) {
	inst.stateMu.Lock()
	defer inst.stateMu.Unlock()
	return inst.currentIP, !inst.opts.ipAtStart && len(inst.currentIP) == 0 && inst.state == vms.StateRunning
}

func (inst *Instance) getIP(ctx context.Context) (string, error) {
	ip, needIP := inst.needIP()
	if needIP {
		runCtx, cancel := context.WithTimeout(ctx, inst.opts.runBackoff.TotalTimeout()+gracePeriod)
		defer cancel()
		fetched, err := inst.runIPWait(runCtx)
		if err != nil {
			return "", fmt.Errorf("tart %s: %w: failed to get IP address", inst.name, err)
		}
		if fetched == "" {
			return "", fmt.Errorf("tart %s: got empty IP address", inst.name)
		}
		ip = strings.TrimSpace(fetched)
		inst.stateMu.Lock()
		inst.currentIP = ip
		inst.stateMu.Unlock()
	}
	return ip, nil
}

// State returns the current state and any error from a running
// instance that terminated without being stopped or suspend.
func (inst *Instance) State(_ context.Context) vms.State {
	inst.stateMu.Lock()
	defer inst.stateMu.Unlock()
	return inst.state
}

func (inst *Instance) Suspendable() bool {
	return inst.suspendable
}

// runSyncExclusive runs a tart command synchronously, checking
// that the current state allows the requested transition.
func (inst *Instance) runSyncExclusive(ctx context.Context, action vms.Action, intermediate, target vms.State, args ...string) error {
	if s, allowed := inst.isActionAllowed(action); !allowed {
		return fmt.Errorf("action %s not allowed in state %s", action, s)
	}
	prev := inst.setState(intermediate)
	inst.logger.Info("tart command issued", "args", args)
	stdoutBuf := bytes.NewBuffer(make([]byte, 0, 1024))
	stderrBuf := executil.NewTailWriter(1024)
	start := time.Now()
	cmd := exec.CommandContext(ctx, inst.opts.tartBinary, args...) //nolint:gosec // G204 false positive
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf
	err := cmd.Run()
	inst.logger.Info("tart command completed", "tart", inst.opts.tartBinary, "args", args, "stderr", string(stderrBuf.Bytes()), "error", err, "duration", time.Since(start).String())
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
		return fmt.Errorf("%s: VM does not exist: %s: %v: %w", cl, stderr, err, vms.ErrVMNotFound)
	}
	if isAlreadyStoppedErrorMsg(stderr) {
		return fmt.Errorf("%s: VM is not running: %s: %v: %w", cl, stderr, err, vms.ErrVMNotRunning)
	}
	return fmt.Errorf("%s: %s: %w", cl, stderr, err)
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

// Start runs "tart run <name> --no-graphics --suspendable" by starting the
// tart process in the background, then blocks until tart reports the VM is
// running, an IP address is available, and a tart exec readiness check
// succeeds. On success, the instance transitions to StateRunning..
func (inst *Instance) Start(ctx context.Context, stdout, stderr io.Writer) error {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
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

	stderrCopy := executil.NewTailWriter(1024)
	cmd := exec.CommandContext(ctx, inst.opts.tartBinary, args...) //nolint:gosec // G204 false positive
	cmd.Stdout = stdout
	cmd.Stderr = io.MultiWriter(stderr, stderrCopy)
	cmd.Stdin = nil // Detach stdin entirely
	if err := cmd.Start(); err != nil {
		inst.logger.Error("tart run", "tart", inst.opts.tartBinary, "args", args, "error", err)
		return inst.cmdStartFailed(ctx, prev, fmt.Errorf("tart %s: %w", strings.Join(args, " "), err))
	}
	inst.logger.Info("tart run cmd.Start called", "args", args, "pid", cmd.Process.Pid)

	runBackoff := inst.opts.runBackoff.NewBackoff()
	if err := inst.waitForTartState(ctx, "running", runBackoff); err != nil {
		return inst.runFailed(ctx, prev, cmd,
			fmt.Errorf("tart %s: %w: failed to reach tart 'running' state before the run backoff was exhausted", strings.Join(args, " "), err))
	}

	var ip string
	if inst.opts.ipAtStart {
		var err error
		ip, err = inst.runIPWait(ctx)
		if err != nil || ip == "" {
			return inst.runFailed(ctx, prev, cmd,
				fmt.Errorf("tart %s: %w: failed to get IP address", strings.Join(args, " "), err))
		}
	}

	if err := inst.waitForReadyUsingExec(ctx, runBackoff); err != nil {
		return inst.runFailed(ctx, prev, cmd,
			fmt.Errorf("tart %s: %w: failed to run tart exec", strings.Join(args, " "), err))
	}

	inst.stateMu.Lock()
	inst.currentIP = strings.TrimSpace(ip)
	inst.state = vms.StateRunning
	inst.asyncWait = executil.NewAsyncWait(cmd)
	inst.runStderr = stderrCopy
	inst.stateMu.Unlock()
	inst.logger.Info("tart run completed", "tart", inst.opts.tartBinary, "args", args, "ip", ip, "pid", cmd.Process.Pid, "duration", time.Since(start).String())
	return nil
}

func (inst *Instance) runForceStop(ctx context.Context) error {
	if ctx.Err() != nil {
		ctx = context.WithoutCancel(ctx)
	}
	timeout := inst.opts.forceStopBackoff.TotalTimeout()
	out, err := exec.CommandContext(ctx, inst.opts.tartBinary, "stop", inst.name, "--timeout", strconv.Itoa(durationSeconds(timeout))).CombinedOutput() //nolint:gosec // G204 false positive
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
	if ctx.Err() != nil {
		ctx = context.WithoutCancel(ctx)
	}
	if err := inst.waitForTartState(ctx, "stopped", inst.opts.forceStopBackoff.NewBackoff()); err != nil {
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
	if ctx.Err() != nil {
		ctx = context.WithoutCancel(ctx)
	}
	var errs errors.M
	errs.Append(prevErr)
	err := inst.runForceStop(ctx)
	errs.Append(err)

	if err := cmd.Wait(); err != nil {
		errs.Append(err)
	}
	if err := inst.waitForTartState(ctx, "stopped", inst.opts.forceStopBackoff.NewBackoff()); err != nil {
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
	if ctx.Err() != nil {
		ctx = context.WithoutCancel(ctx)
	}
	return inst.waitForTartState(ctx, state, inst.opts.stateBackoff.NewBackoff()) == nil
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
			stderr := string(inst.runStderr.Bytes())
			args := inst.asyncWait.Cmd().Args
			return convertError(args, stderr, err), nil
		}
		return nil, nil
	}
	inst.logger.Info("tart command issued", "args", args)
	start := time.Now()
	stdoutBuf := bytes.NewBuffer(make([]byte, 0, 1024))
	stderrBuf := executil.NewTailWriter(1024)
	cmd := exec.CommandContext(ctx, inst.opts.tartBinary, args...) //nolint:gosec // G204 false positive
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf
	err = cmd.Run()
	stderr := string(stderrBuf.Bytes())
	inst.logger.Info("tart command completed", "tart", inst.opts.tartBinary, "args", args, "stderr", stderr, "error", err, "duration", time.Since(start).String())
	if err != nil {
		if isAlreadyStoppedErrorMsg(stderr) {
			return nil, nil
		}
		return nil, convertError(args, stderr, err)
	}
	runErr = inst.asyncWait.Wait()
	if runErr != nil {
		stderr := string(inst.runStderr.Bytes())
		args := inst.asyncWait.Cmd().Args
		return convertError(args, stderr, runErr), nil
	}
	return nil, nil
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
		args = append(args, "--timeout", strconv.Itoa(durationSeconds(timeout)))
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

// Properties returns VM properties. If the VM is running, it returns the IP address.
func (inst *Instance) Properties(ctx context.Context) (vms.Properties, error) {
	ip, err := inst.getIP(ctx)
	if err != nil {
		return vms.Properties{
			CloneInfo: CloneInfo{Source: inst.source, Name: inst.name},
		}, err
	}
	return vms.Properties{
		IP:        ip,
		CloneInfo: CloneInfo{Source: inst.source, Name: inst.name},
	}, nil
}

// Exec runs "tart exec <name> <cmd> <args...>" with the output connected to the
// provided writers. It returns when the command completes.
func (inst *Instance) Exec(ctx context.Context, stdout, stderr io.Writer, cmd string, args ...string) error {
	if state := inst.State(ctx); state != vms.StateRunning {
		return fmt.Errorf("exec only available for running VMs, current state: %s", state)
	}
	allArgs := append([]string{"exec", inst.name, cmd}, args...)
	c := exec.CommandContext(ctx, inst.opts.tartBinary, allArgs...) //nolint:gosec // G204 false positive
	c.Stdout = stdout
	c.Stderr = stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("tart exec: cmd %s: %w", cmd, err)
	}
	return nil
}

// waitForReadyUsingExec polls tart exec until the VM responds with the expected
// echo output. Each individual exec attempt is allocated a timeout equal to the
// total run backoff budget (TotalTimeout), allowing slow-starting guest agents
// sufficient time to respond during boot before being retried according to the backoff schedule.
func (inst *Instance) waitForReadyUsingExec(ctx context.Context, backoff ratecontrol.Backoff) error {
	timeout := inst.opts.runBackoff.TotalTimeout()
	found := func(ctx context.Context) (bool, error) {
		return waitForReadyUsingExecOne(ctx, inst.opts.tartBinary, inst.logger, inst.name, timeout)
	}
	return executil.WaitForBackoff(ctx, backoff, found)
}

// waitForReadyUsingExecOne performs a single tart exec attempt with the specified timeout.
func waitForReadyUsingExecOne(ctx context.Context, binary string, logger *slog.Logger, name string, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out := executil.NewTailWriter(1024)
	now := strconv.FormatInt(time.Now().UnixNano(), 10)
	cmd := exec.CommandContext(ctx, binary, "exec", name, "echo", now) //nolint:gosec // G204 false positive
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Stdin = nil // Detach stdin entirely
	if err := cmd.Run(); err != nil {
		logger.Info("waiting for vm to be ready: tart exec failed", "name", name, "error", err, "output", string(out.Bytes()))
		return false, fmt.Errorf("waiting for vm %s to be ready: tart exec: %s\n%w", name, out.Bytes(), err)
	}
	read := strings.TrimSpace(string(out.Bytes()))
	if read != now {
		logger.Info("waiting for vm to be ready: tart exec output mismatch", "name", name, "expected", now, "got", read)
		return true, fmt.Errorf("waiting for vm %s to be ready: tart exec: output does not contain expected string: %s != %s", name, read, now)
	}
	return true, nil
}

func (inst *Instance) runIPWait(ctx context.Context) (string, error) {
	timeout := inst.opts.runBackoff.TotalTimeout()
	args := []string{"ip", inst.name, "--wait", strconv.Itoa(durationSeconds(timeout))}
	cmd := exec.CommandContext(ctx, inst.opts.tartBinary, args...) //nolint:gosec // G204 false positive
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tart %s: (timeout: %v): %w", strings.Join(args, " "), timeout, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func getStateUsingList(ctx context.Context, binary, name, state string) (bool, error) {
	all, err := ListAll(ctx, binary)
	if err != nil {
		return false, fmt.Errorf("failed to list tart VMs: %w", err)
	}
	entry, found := all.Lookup(name)
	if !found {
		return false, fmt.Errorf("tart list: VM %q not found", name)
	}
	return entry.State == state, nil
}

func (inst *Instance) waitForTartState(ctx context.Context, state string, backoff ratecontrol.Backoff) error {
	found := func(ctx context.Context) (bool, error) {
		return getStateUsingList(ctx, inst.opts.tartBinary, inst.name, state)
	}
	return executil.WaitForBackoff(ctx, backoff, found)
}

type CloneInfo struct {
	Source string
	Name   string
}

func (c CloneInfo) String() string {
	return fmt.Sprintf("source=%s name=%s", c.Source, c.Name)
}
