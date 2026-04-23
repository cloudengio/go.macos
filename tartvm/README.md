# Package [cloudeng.io/macos/tartvm](https://pkg.go.dev/cloudeng.io/macos/tartvm?tab=doc)

```go
import cloudeng.io/macos/tartvm
```

Package tartvm implements cloudeng.io/vms.Instance using the tart CLI on
macOS.

## Constants
### DefaultPollingInterval, DefaultOutputBufferSize, DefaultRunTimeout, DefaultForceStopTimeout
```go
DefaultPollingInterval = 100 * time.Millisecond
DefaultOutputBufferSize = 16 * 1024 // 16KiB

DefaultRunTimeout = 2 * time.Minute
DefaultForceStopTimeout = 2 * time.Second

```



## Functions
### Func DefaultLinuxRunOptions
```go
func DefaultLinuxRunOptions() []string
```

### Func DefaultMacOSRunOptions
```go
func DefaultMacOSRunOptions() []string
```

### Func DefaultRunOptions
```go
func DefaultRunOptions() []string
```
DefaultRunOptions are safe defaults that work with mac and linux tart VMs.
Linux does not currently support suspend.



## Types
### Type Instance
```go
type Instance struct {
	// contains filtered or unexported fields
}
```
Instance implements vms.Instance backed by the tart CLI. source is the OCI
reference used for cloning; name is the local VM name. All images must
have the tart agent installed and be compatible with the tart CLI version
installed locally.

### Functions

```go
func New(ctx context.Context, source, name string, opts ...Option) *Instance
```
New returns an Instance in StateInitial, source is the tart image or OCI
reference to clone from; name is the local VM name.



### Methods

```go
func (inst *Instance) Clone(ctx context.Context) error
```
Clone runs "tart clone <source> <name>" and transitions to StateReadyToRun.


```go
func (inst *Instance) Delete(ctx context.Context) error
```
Delete runs "tart delete <name>" and transitions to StateDeleted.


```go
func (inst *Instance) Exec(ctx context.Context, stdout, stderr io.Writer, cmd string, args ...string) error
```
Exec runs "tart exec <name> <cmd> <args...>" with the output connected to
the provided writers. It returns when the command completes.


```go
func (inst *Instance) Name() string
```
Name returns the local VM name.


```go
func (inst *Instance) Properties(ctx context.Context) (vms.Properties, error)
```
Properties returns VM properties. It calls "tart ip" to obtain the IP
address and returns SSH connection arguments for the default admin user.


```go
func (inst *Instance) Start(ctx context.Context, stdout, stderr io.Writer) error
```
Start runs "tart run <name> --no-graphics --suspendable" by starting the
tart process in the background, then blocks until tart reports the VM
is running, an IP address is available, and a tart exec readiness check
succeeds. On success, the instance transitions to StateRunning..


```go
func (inst *Instance) State(_ context.Context) vms.State
```
State returns the current state and any error from a running instance that
terminated without being stopped or suspend.


```go
func (inst *Instance) Stop(ctx context.Context, timeout time.Duration) (runErr, stopErr error)
```


```go
func (inst *Instance) Suspend(ctx context.Context) error
```
Suspend runs "tart suspend <name>" and transitions to StateSuspended.


```go
func (inst *Instance) Suspendable() bool
```




### Type ListEntries
```go
type ListEntries []ListEntry
```

### Functions

```go
func ListAll(ctx context.Context) (ListEntries, error)
```
ListAll calls "tart list --format json" and returns the entries.



### Methods

```go
func (e ListEntries) Len() int
```


```go
func (e ListEntries) Lookup(name string) (ListEntry, bool)
```
Lookup returns the entry for name, or (zero, false) if the VM is not
present.


```go
func (e ListEntries) LookupSourceName(source, name string) (ListEntry, bool)
```
LookupSourceName returns the entry for source and name, or (zero, false) if
the VM is not present.




### Type ListEntry
```go
type ListEntry struct {
	State    string
	Name     string
	Size     int
	Accessed time.Time
	Source   string
	Disk     int
	Running  bool
}
```
ListEntry represents an entry in the output of "tart list --format json".

### Methods

```go
func (e ListEntry) VMSState() vms.State
```
VMSState maps the tart state to a vms.State.




### Type Option
```go
type Option func(o *options)
```
Option represents an Option to New.

### Functions

```go
func WithForceStopTimeout(timeout time.Duration) Option
```
WithForceStopTimeout sets the timeout for forcefully stopping a VM when a
run operation, or other operation, fails and the error recovery needs to
stop the VM.


```go
func WithPollingInterval(interval time.Duration) Option
```
WithPollingInterval sets the interval to use for polling the state of the VM
when waiting for state transitions, network availability, etc.

    The default is DefaultPollingInterval.


```go
func WithRunOptions(opts ...string) Option
```
WithRunOptions sets additional options to pass to the "tart run" command.
The default is the value returned by DefaultRunOptions.


```go
func WithRunTimeout(timeout time.Duration) Option
```
WithRunTimeout sets a timeout for the VM to reach a running state after
Start is called. The default is DefaultRunTimeout.







