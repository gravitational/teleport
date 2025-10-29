# BPF 101

If you're reading this, you were probably tasked with writing a BPF code.
This document will help you get started.

#### Before you start here are some useful links:

* https://docs.kernel.org/bpf/ - BPF documentation
* https://github.com/cilium/ebpf - Go library to interact with BPF
* https://github.com/iovisor/bcc - BPF Compiler Collection (a lot of examples)
* https://github.com/cilium/cilium - BPF-based networking, security, and observability (more examples)
* https://man7.org/linux/man-pages/man7/bpf-helpers.7.html - BPF helpers documentation

## Environment setup:

We support BPFs only on ARM64 and x86_64 architectures.

1. VM with Ubuntu 20.04+ (or any other Linux distro with the kernel 5.8+ - Docker doesn't work for many BPF features)
2. Install `clang` (required for compiling BPF code) - `apt install clang` should be enough
3. Install `ebpf` (required for loading BPF code) - get the required version from our Docker file

You can also use our Dockerfile to build the environment:

```bash
make -C build.assets bpf-bytecode
```

## BPF code structure

BPF code can be divided into two parts:

1. BPF program - the actual BPF code that will be loaded into the kernel
2. User space code - the code that will load the BPF program into the kernel

BPF programs are located in `bpf` directory in our repository. They are C like programs with some limitations.
The most important limitation is that you can't use any system calls in BPF programs. You can use only BPF helpers.
The BPF programs are compiled using `clang` 12+, GCC is not supported.

User space code is located in `lib/bpf` directory. It's written in Go and uses the `cilium/ebpf` library to load BPF programs into the kernel. `cilium/ebpf ` is a pure-Go library to read, modify and load eBPF programs and attach them to various hooks in the Linux kernel.

### BPF license

BPF programs are compiled into ELF files. ELF files have a license field that is used to verify that the BPF program
is allowed to run in the kernel. The license field is set to `GPL` by default. Teleport uses `Dual BSD/GPL` which
disables some BPF features (like logging). To enable all BPF features set the license to `GPL` in the BPF program
and revert back before merging the code.

### Logging

BPF programs can log messages to the kernel log. Then you use `bpf_printk` helper to log messages. The messages will be logged to the kernel log. You can get the messages
from `/sys/kernel/debug/tracing/trace_pipe`. Uncomment the line in `common.h` the defines `PRINT_DEBUG_MSGS` or else `bpf_printk` calls
will be compiled out.

Here is the best explanation that I found so far https://nakryiko.com/posts/bpf-tips-printk/.

### Communication between BPF programs and user space

BPF programs can communicate with user space using maps. Maps are key-value stores that can be accessed from both
BPF programs and user space. BPF programs can only access maps using BPF helpers. User space can access maps using
`aquasecurity/libbpfgo` library. Maps are defined in BPF programs and can be referenced by name from user space code.


## BPF in Teleport

Teleport uses BPF to implement enhanced session recording. Enhanced session recording works only on Linux with
the kernel 5.8+. Enhanced session recording records all:
* exec family system calls
* open family system calls
* network connections

All events are recorded in the audit log. See https://goteleport.com/docs/enroll-resources/server-access/guides/bpf-session-recording/.
