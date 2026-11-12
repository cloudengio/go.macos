# Package [cloudeng.io/macos/tartvm](https://pkg.go.dev/cloudeng.io/macos/tartvm?tab=doc)

```go
import cloudeng.io/macos/tartvm
```

Package tartvm implements cloudeng.io/vms.Instance using the tart CLI on
macOS.

## Constants
### DefaultOutputBufferSize, DefaultTartBinary
```go
const (
	DefaultOutputBufferSize = 16 * 1024 // 16KiB
	DefaultTartBinary       = "tart"
)
```

### ProviderName
```go
const ProviderName = "tart"
```
ProviderName is the name a tart Provider reports, see Provider.Name.



## Functions
### Func DefaultForceStopBackoff
```go
func DefaultForceStopBackoff() ratecontrol.ExponentialBackoffConfig
```
DefaultForceStopBackoff returns the default backoff bounding forcefully
stopping a VM during error recovery: 500ms initial delay doubling over 5
steps, for a total delay budget of ~15 seconds.

### Func DefaultLinuxRunOptions
```go
func DefaultLinuxRunOptions() []string
```

### Func DefaultMacOSRunOptions
```go
func DefaultMacOSRunOptions() []string
```

### Func DefaultRunBackoff
```go
func DefaultRunBackoff() ratecontrol.ExponentialBackoffConfig
```
DefaultRunBackoff returns the default backoff bounding how long to wait for
the VM to reach a running state after Start: 1s initial delay doubling over
7 steps, for a total delay budget of ~127 seconds.

### Func DefaultRunOptions
```go
func DefaultRunOptions() []string
```
DefaultRunOptions are safe defaults that work with mac and linux tart VMs.
Linux does not currently support suspend.

### Func DefaultStateBackoff
```go
func DefaultStateBackoff() ratecontrol.ExponentialBackoffConfig
```
DefaultStateBackoff returns the default backoff used when polling the state
of the VM: 100ms initial delay doubling over 10 steps, for a total delay
budget of ~102 seconds.

### Func Pull
```go
func Pull(ctx context.Context, tartBinary string, ref ociimage.Reference, opts ociimage.PullOptions) error
```
Pull fetches ref into the local OCI cache with "tart pull", so that a
subsequent clone uses the newly fetched copy rather than whatever was cached
before. tartBinary may be empty, in which case DefaultTartBinary is used.

Only a reference naming a registry can be pulled; a local image name has
nothing to pull from, and tart reports that as an error. Use ociimage.Pull
to skip such a reference rather than fail it.

### Func SetResources
```go
func SetResources(ctx context.Context, tartBinary, image string, resources vmspool.VMResources) error
```
SetResources updates the resource configuration of a tart VM image using
"tart set".



## Types
### Type CloneInfo
```go
type CloneInfo struct {
	Source string
	Name   string
}
```

### Methods

```go
func (c CloneInfo) String() string
```




### Type Config
```go
type Config struct {
	OS               string                               `yaml:"os" doc:"The operating system of the tart VM, either 'macos' or 'linux'"`
	StateBackoff     ratecontrol.ExponentialBackoffConfig `yaml:"state_backoff" doc:"The backoff to use when polling the state of the VM when waiting for state transitions, network availability, etc."`
	RunBackoff       ratecontrol.ExponentialBackoffConfig `yaml:"run_backoff" doc:"The backoff bounding how long to wait for the VM to reach a running state after Start is called."`
	ForceStopBackoff ratecontrol.ExponentialBackoffConfig `yaml:"force_stop_backoff" doc:"The backoff bounding forcefully stopping a VM when a run operation, or other operation, fails and the error recovery needs to stop the VM."`
	RunOptions       []string                             `yaml:"run_options,flow" doc:"Additional options to pass to the tart run command."`
	TartBinary       string                               `yaml:"binary" doc:"Path to the tart binary, defaults to 'tart' on PATH."`
}
```
Config contains configuration for tart VM pools.

### Methods

```go
func (c *Config) Options() []Option
```




### Type Constructor
```go
type Constructor = func(ctx context.Context) (vms.Instance, error)
```
Constructor creates a new, uninitialized tart VM instance. Each call must
return a distinct vms.Instance (typically via New with a unique name).
ctx governs any work done to construct the instance. It returns an error if
the instance could not be created.


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
Clone runs "tart clone <source> <name>" and transitions to StateStopped.
If resources are configured, "tart set <name> ..." is run while the instance
is still in StateStopped. If setting the resources fails the instance is
left in StateStopped.


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
func (inst *Instance) ID() string
```
ID returns the local VM's ID/name.


```go
func (inst *Instance) Properties(ctx context.Context) (vms.Properties, error)
```
Properties returns VM properties. If the VM is running, it returns the IP
address.


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
func ListAll(ctx context.Context, tartBinary string) (ListEntries, error)
```
ListAll calls "tart list --format json" and returns the entries.
If tartBinary is empty then DefaultTartBinary is used.



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
func WithForceStopBackoff(cfg ratecontrol.ExponentialBackoffConfig) Option
```
WithForceStopBackoff sets the backoff bounding forcefully stopping a VM when
a run operation, or other operation, fails and the error recovery needs to
stop the VM; its total delay budget is used as the graceful shutdown timeout
passed to "tart stop --timeout". The default is DefaultForceStopBackoff().


```go
func WithLogger(logger *slog.Logger) Option
```
WithLogger sets a logger to use for logging tart commands and state
transitions.


```go
func WithObtainIPAtStart(ipAtStart bool) Option
```
WithObtainIPAtStart sets whether to obtain the IP address of the VM at start
time, disable for faster execution of Start and with the IP address obtained
on demand.


```go
func WithResources(resources vmspool.VMResources) Option
```
WithResources sets the resources to apply to the tart VM after it is cloned,
using 'tart set'. A resource left at zero is not set, so the VM keeps the
value its image was built with.


```go
func WithRunBackoff(cfg ratecontrol.ExponentialBackoffConfig) Option
```
WithRunBackoff sets the backoff bounding how long to wait for the VM to
reach a running state after Start is called; its total delay budget also
bounds obtaining the VM's IP address and the readiness check. The default is
DefaultRunBackoff().


```go
func WithRunOptions(opts ...string) Option
```
WithRunOptions sets additional options to pass to the "tart run" command.
The default is the value returned by DefaultRunOptions.


```go
func WithStateBackoff(cfg ratecontrol.ExponentialBackoffConfig) Option
```
WithStateBackoff sets the backoff to use when polling the state of the VM
when waiting for state transitions, network availability, etc.

    The default is DefaultStateBackoff().


```go
func WithTartBinary(tartBinary string) Option
```
WithTartBinary sets the path to the tart binary to use for running tart
commands.


```go
func WithVerification(v ociimage.Verifier, policy ociimage.Policy) Option
```
WithVerification requires that the image that is cloned has a signature that
satisfies policy, as verified by v, and clones the verified digest rather
than the reference that was given: a tag could be moved between verifying
it and cloning it. An image that is not signed, or not signed as the policy
requires, is an error, as is a local image, which has no registry to hold a
signature. Without this option nothing is verified.




### Type Provider
```go
type Provider struct {
	// contains filtered or unexported fields
}
```
Provider is a vmspool.Provider backed by tart. It delegates VM construction
to a caller-supplied Constructor and implements List, Get and Delete
directly via the tart CLI, so using tart with a vmspool.Pool only requires
supplying the construction function.

### Functions

```go
func NewProvider(constructor Constructor, opts ...ProviderOption) *Provider
```
NewProvider returns a Provider that constructs VMs with constructor and
implements the remaining vmspool.Provider methods via the tart CLI.



### Methods

```go
func (p *Provider) DefaultResources() vmspool.VMResources
```
DefaultResources implements vmspool.Provider, reporting the resources the
Provider's VMs are created with, see WithProviderResources. As with the
image, a Provider cannot discover them for itself, and a zero figure means
unknown: tart keeps the image's own value for anything the clone did not
set, and Get reports what a VM actually has once it exists.


```go
func (p *Provider) Delete(ctx context.Context, stopTimeout time.Duration) ([]string, error)
```
Delete implements vmspool.Provider, stopping (if running) and deleting every
VM the Provider manages. Deletion continues past individual failures.


```go
func (p *Provider) Details() vmspool.ProviderDetails
```
Details implements vmspool.Provider, reporting the image the Provider's VMs
are cloned from, or the zero Reference if it was not configured with one:
a Provider creates its VMs through a caller-supplied Constructor and so
cannot discover the image for itself, see WithProviderImage.


```go
func (p *Provider) Get(ctx context.Context, vmName string) (vmspool.VMDetail, error)
```
Get implements vmspool.Provider, returning the resources allocated to a
single VM via "tart get".


```go
func (p *Provider) List(ctx context.Context) ([]vmspool.VMInfo, error)
```
List implements vmspool.Provider, returning the local tart VMs whose names
match the configured prefix.


```go
func (p *Provider) Name() string
```
Name implements vmspool.Provider by naming the VM technology, which is what
identifies the provider behind a VM in status output and in errors.


```go
func (p *Provider) New(ctx context.Context) (vms.Instance, error)
```
New implements vmspool.Provider.


```go
func (p *Provider) Pull(ctx context.Context, ref ociimage.Reference, opts ociimage.PullOptions) error
```
Pull implements ociimage.Puller with the Provider's own tart binary, see
WithProviderTartBinary. ref is pulled whatever the Provider's own image is:
the Provider reports what its Constructor clones but does not run it, so the
caller that owns the Constructor says what to pull, see WithProviderImage.




### Type ProviderOption
```go
type ProviderOption func(*Provider)
```
ProviderOption configures a Provider.

### Functions

```go
func WithNamePrefix(prefix string) ProviderOption
```
WithNamePrefix scopes List and Delete to VMs whose names start with prefix.
Without it a Provider manages every local tart VM, which is rarely desirable
when more than one pool shares the host.


```go
func WithPoolName(name string) ProviderOption
```
WithPoolName sets the pool name reported in VMInfo.Pool for the Provider's
VMs.


```go
func WithProviderImage(ref ociimage.Reference) ProviderOption
```
WithProviderImage records the image the Provider's Constructor clones its
VMs from, which Details then reports and which Provider.Pull pulls. The
Constructor owns the creation of a VM, so this is what it was told to clone,
stated for anything reporting on the pool.


```go
func WithProviderResources(resources vmspool.VMResources) ProviderOption
```
WithProviderResources records the resources the Provider's Constructor
creates its VMs with, which DefaultResources then reports, see WithResources
for the option that applies them to a VM. Like WithProviderImage it only
states what the Constructor was told to do.


```go
func WithProviderTartBinary(path string) ProviderOption
```
WithProviderTartBinary sets the tart binary path to use for all Provider
operations.







