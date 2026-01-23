---
authors: Edoardo Spadolini (edoardo.spadolini@goteleport.com)
state: draft
---

# RFD 0244 - Teleport SSHd helper binary

## Required Approvers

* Engineering: (SME from server access) && (SME from scale)
* Security: @rob-picard-teleport

## What

The Teleport SSH service uses reexecutions of the `teleport` binary for various system interactions, including interactions with PAM, utmp/wtmp, auditd and selinux. This RFD proposes the addition of a dedicated helper binary which will only contain the code that is invoked by the Teleport SSH service during reexecution in child processes.

## Why

The `teleport` binary is very large and contains a lot of dependencies with a lot of init-time code taking up CPU time and memory to the tune of about 14 MB of used heap at the beginning of the `main` function, and around 50 msec of wall clock time, with about 25 MB of total allocations during the initialization. A lot of this init time work happens in dependencies that we have no influence over (the GCP and AWS SDKs for example) but that `teleport` needs in one way or another. Even without the init time work, the sheer size of the binary makes it heavy to load and launch anew - it's likely that the very early code is no longer in cache by the time the Teleport process has been idling for a while and handling connections.

Having the Teleport SSH service launch a tiny binary that contains the functionality that must absolutely run in a separate OS process and nothing else solves the memory and init time problems cleanly, without having to fork dependencies or chase down upstreams to merge changes to reduce init time work (that in some cases would require a breaking change) and without having to sacrifice functionality in Teleport as a whole.

## Details

### Implementation

The Teleport SSH service reexecutes the `teleport` binary as a child process for several subcommands, all related to running and managing user sessions: `teleport exec` is responsible for opening a PAM session, setting the SELinux context, updating utmp/wtmp and then launching the actual shell process for the user, `teleport networking` is tasked with opening, binding and connecting sockets as the user so the SSH service can forward connections with the appropriate permissions and in the appropriate context (PAM on Linux can move user sessions in a different network namespace, for example), and utilities such as `teleport checkhomedir` are used to check permissions after dropping privileges, which is something that in classic UNIX programs would be done after a `fork()`. No other Teleport service needs similar functionality, so the scope of `teleport` reexecutions is entirely limited to the SSH service.

There is precedent in making the subcommands used as a reexec target (other than `teleport sftp`) available in other non-`teleport` binaries in the form of the `lib/srv.RunAndExit` function, called in the `TestMain` function in test packages that need to make use of reexecutions; the proposed `teleport-sshd-helper` binary will then be implemented with a simple call to `lib/srv.RunAndExit(os.Args[1])` similarly to how `TestMain` works, augmented with the missing `sftp` subcommand (which is currently only implemented directly in `tool/teleport`).

Doing this naively would result in a very large binary (around 70MB) but it's possible to get that down to about 10MB just by splitting up packages and moving definitions around (as well as writing out the SFTP audit log events manually, to avoid importing `api/events`). The same subcommands should still be left available as subcommands of the `teleport` binary, to support platforms or environments where it's not practical to use the helper, as will be discussed later.

### Distribution as an embedded binary

Currently, on Linux, the `teleport` reexecutions happen by launching `/proc/self/exe` rather than any particular path on disk. This guarantees that the process that is launched is the same as the running process, so the behavior of the subcommand as well as the interface between the SSH service and the subcommand is guaranteed to match the expectations of the running code. If we were to simply ship `teleport-sshd-helper` in our package and launch whatever `teleport-sshd-helper` binary is available in the `PATH`, this assumption would be broken, since the helper binary would change during upgrades, it might be referring to a different `teleport` install on disk, the binary might get accidentally deleted while `teleport` is running.

It could be possible to only support the helper binary if Teleport is installed as part of Managed Updates, since every version would be installed in a separate directory, but it's possible to sidestep all the aforementioned problems entirely while also avoiding additional work in managing the distribution of another artifact: embedding `teleport-sshd-helper` as data in the `teleport` binary, and launching it from memory. Distributing the helper binary as part of `teleport` means that the only necessary changes in the release process are changes to the build rather than changes to the packaging.

### How the execution from memory works

Executing the embedded `teleport-sshd-helper` boils down to copying the binary into a file and then launching the file. The best option for this, in Linux 3.17 and later, is to use a memfd: an anonymous, memory-backed file that doesn't exist anywhere on disk and has no path, that we can create, copy the embedded binary into, _seal_ it to make it immutable, then launch it from `/proc/<pid>/fd/<n>`. This will load the whole helper binary in memory, but given the manageable size and the significant savings in total system memory used after a single reexecution, it's likely a worthy trade. This technique is used by [`runc`](https://github.com/opencontainers/runc/) to safeguard its binary when launching itself in containers, so it has a proven track record.

It's possible for a system to be configured to disallow creating executable memfds. We're always going to have the option to fall back to the existing reexec implementation and just use the `teleport` binary (through `/proc/self/exe`), but if we wanted to support a broader range of environments, we could also try writing down the helper binary into a temporary file, but we're not guaranteed to have access to a suitable directory for it, since both `/var/lib/teleport` and `/tmp` (or whatever `$TMPDIR` resolves to) are potentially mounted `noexec`. For the temporary file approach we'd either create the file with restrictive permissions and `O_TMPFILE` (or, if `O_TMPFILE` also fails, create it with a random path and unlink it immediately), write the helper binary in the file and then make it executable and read-only through `fchmod`, then launch it through `/proc/<pid>/fd/<n>` like the memfd.

Whatever the approach, the SSH service should fall back to the `teleport` binary if launching the helper fails; depending on how the implementation goes, it might be convenient to define a `teleport-sshd-helper true` subcommand to test the availability of the helper. This self-test is doubly important if Teleport is configured to support SELinux, since it's possible for the helper binary to be executable but not have enough permissions to work correctly; the SELinux integration will also need to be updated appropriately.

It doesn't seem practical to use a similar execution trick on macOS because of the seeming lack of execution from memory (which would force us to persist the helper binary on disk somewhere and refer to it by path), so until a specific business need arises, we're only going to make use of the dedicated helper binary for Linux agents.

### Changes to builds, tests and development workflow

No changes to the build environment should be necessary, the build of the `teleport` binary for Linux will be updated to always unconditionally build the helper binary right before building the `teleport` binary with the appropriate tag (tentatively called `sshd_helper_embed`, mimicking the existing `webassets_embed`).

A lot of tests already use the same entrypoint that the helper will use for reexec functionality, so those will need no changes; tests for the in-memory reexecution mechanism can be implemented by copying the test binary from `/proc/self/exe` into the memfd or temporary file.

Since the embedded helper will require a new build tag, any workflow that's not explicitly updated to build and embed the helper will keep working as is, reexecuting the `teleport` binary. This applies to `go run` and `go build`, as well as any existing scripts that developers might have.

Linting rules will be updated to keep the dependency tree of the helper binary small enough to not cause problems. If this turns out to be too impractical we will consider moving the entire post-reexec code into a separate Go module that can be imported by the main module, as a hard barrier against expanding the dependency tree accidentally.
