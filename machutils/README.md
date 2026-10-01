# Package [cloudeng.io/macos/machutils](https://pkg.go.dev/cloudeng.io/macos/machutils?tab=doc)

```go
import cloudeng.io/macos/machutils
```

Package machutils provides low level utilities for interacting with the
macOS kernel.

## Constants
### AppSandboxEntitlement
```go
AppSandboxEntitlement = "com.apple.security.app-sandbox"

```
AppSandboxEntitlement is the entitlement that places a process in the macOS
App Sandbox.



## Variables
### ErrFailedToRetrieveEntitlements
```go
ErrFailedToRetrieveEntitlements = errors.New("failed to retrieve code signing entitlements from the kernel")

```
ErrFailedToRetrieveEntitlements is returned when the code signing
entitlements of the running process cannot be retrieved from the kernel.

### ErrFailedToRetrieveExecutablePath
```go
ErrFailedToRetrieveExecutablePath = errors.New("failed to retrieve the executable path from the kernel")

```
ErrFailedToRetrieveExecutablePath is returned when the path of the running
executable cannot be retrieved from the kernel.

### ErrFailedToRetrieveParentUID
```go
ErrFailedToRetrieveParentUID = errors.New("failed to retrieve parent process UID from the kernel")

```
ErrFailedToRetrieveParentUID is returned when the parent process UID cannot
be retrieved from the kernel.

### ErrFailedToRetrievePeerPID
```go
ErrFailedToRetrievePeerPID = errors.New("failed to retrieve peer process ID from the kernel")

```
ErrFailedToRetrievePeerPID is returned when the process ID of the peer of
a Unix domain socket connection cannot be retrieved from the kernel.

### ErrNoSelfRequirement
```go
ErrNoSelfRequirement = errors.New("could not determine this process's own designated code signing requirement")

```
ErrNoSelfRequirement is returned by SelfRequirementString when the running
binary's own designated code signing requirement cannot be determined,
typically because it is unsigned.

### ErrPeerCodeSignatureInvalid
```go
ErrPeerCodeSignatureInvalid = errors.New("peer code signature is not valid or does not satisfy the requirement")

```
ErrPeerCodeSignatureInvalid is returned by VerifyPeerCodeSignature when the
peer either cannot be identified, is no longer running, or does not satisfy
the given requirement.



## Functions
### Func EnsureParentProcessSafe
```go
func EnsureParentProcessSafe() error
```
EnsureParentProcessSafe checks that:
 1. the parent process UID matches the executable's UID
 2. the current process UID matches that of the parent
 3. the executable is not group- or world-writable or executable
 4. the current process is not running with elevated privileges (SUID/SGID)
 5. the process has not been orphaned (reparented to launchd)

1 ensures that only the executable's owner can launch it, 2 ensures that the
current process UID matches that of the executable owner, 3 ensures that
the executable cannot be modified or run by other users, 4 ensures that the
process has not escalated privileges, and 5 ensures that parent identity
cannot be spoofed via orphaning.

### Func Entitlements
```go
func Entitlements() ([]byte, error)
```
Entitlements returns the code signing entitlements of the running
executable, as the property list recorded in its signature, or nil if it was
signed without any.

Only the property list form of the entitlements is consulted. A signature
that carries them solely in their DER form, which the kernel keeps
separately, is reported here as having none.

### Func ExecutablePath
```go
func ExecutablePath() (string, error)
```
ExecutablePath returns the path of the binary that this process is running,
as the kernel records it for the executing file.

It exists because os.Executable does not answer that question on macOS.
What os.Executable reports is the path the process was launched with, made
absolute against the working directory when it is relative. That names the
symbolic link when a process is started through one, and for a process in
an App Sandbox it can name a path that does not exist at all, the recorded
path having been joined to the container's redirected home directory rather
than to the directory the binary occupies. Asking the kernel avoids both:
the path returned is that of the file being executed, with symbolic links
already resolved.

### Func GetExecutableInfo
```go
func GetExecutableInfo() (uint32, os.FileInfo, error)
```
GetExecutableInfo retrieves the owner UID and file info of the executable.

The executable is identified by ExecutablePath rather than by os.Executable,
so that what is examined is the binary the kernel is running rather than the
path it was launched through. The two differ for a process started through a
symbolic link, and for one in an App Sandbox the latter can name a path that
does not exist.

### Func GetParentUID
```go
func GetParentUID() (uint32, error)
```
GetParentUID retrieves the real user ID (RUID) of the parent process.

### Func IsSandboxed
```go
func IsSandboxed() (bool, error)
```
IsSandboxed reports whether the running executable is confined by the macOS
App Sandbox, which it determines from the com.apple.security.app-sandbox
entitlement in its code signature.

That entitlement is what places a process in the App Sandbox: it is applied
by the kernel when the process is executed, and a process carrying it with
no container to run in is killed rather than run unconfined. Its presence
is therefore conclusive. App extensions, Safari web extensions among them,
are always signed with it.

It says nothing about the other ways a process can be confined on macOS.
A profile applied by sandbox_init, or by being launched under sandbox-exec,
leaves no trace in the code signature and is not reported here.

### Func PeerPID
```go
func PeerPID(conn *net.UnixConn) (int32, error)
```
PeerPID returns the process ID of the process on the other end of conn,
a Unix domain socket connection, via the LOCAL_PEEREPID socket option:
a public, documented macOS extension to getsockopt (see unix(4)), unlike
the csops interface the rest of this package otherwise resorts to, so no
cgo is needed here.

The PID alone proves nothing about which binary is running as that process;
pair it with VerifyPeerCodeSignature to check that.

### Func SelfRequirementString
```go
func SelfRequirementString() (string, error)
```
SelfRequirementString returns the running binary's own designated code
signing requirement, in the same requirement-string language
VerifyPeerCodeSignature takes. One use for it: a process can hand its own
requirement string to a child process it spawns, so that child can verify
the parent back over the same connection, making the check mutual rather
than one-directional.

### Func VerifyPeerCodeSignature
```go
func VerifyPeerCodeSignature(conn *net.UnixConn, requirement string) error
```
VerifyPeerCodeSignature verifies that the process connected via conn, a
Unix domain socket connection, is a currently valid, unmodified binary
satisfying requirement: a code signing requirement string in the language
codesign and Xcode use (see `man csreq`), typically naming a Team ID and/or
bundle identifier, e.g.

    anchor apple generic and certificate leaf[subject.OU] = "TEAMID" and identifier "ai.onyourbehalf.router-helper"

Filesystem or socket permissions on their own only prove that the
connecting process is allowed to reach the socket (e.g. membership of the
same App Group container); they say nothing about which binary is on the
other end of it. This is the additional check needed to prove that it is
a specific, currently valid, signed binary, using the same mechanism
SecCodeCheckValidity applies to the calling process, applied instead to the
peer of a local socket connection, identified by PeerPID.

There is an unavoidable, narrow race between reading the peer's PID and
resolving it to a running code object: the peer could in principle exit
and have its PID recycled by an unrelated process in between. This is the
same assumption system software on macOS makes when authenticating local
socket peers this way; the window is on the order of the time between two
syscalls, not something a connecting process can reliably exploit.

A caller should perform this check immediately after accepting conn and
before reading or acting on anything it sends, closing conn without further
use if it returns an error.


