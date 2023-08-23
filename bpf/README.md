# BPF 101

If you're reading this, you were probably tasked with writing a BPF code.
This document will help you get started.

#### Before you start here are some useful links:

https://docs.kernel.org/bpf/ - BPF documentation
https://github.com/aquasecurity/libbpfgo - Go bindings for libbpf
https://github.com/iovisor/bcc - BPF Compiler Collection (a lot of examples)
https://github.com/cilium/cilium - BPF-based networking, security, and observability (more examples)
https://man7.org/linux/man-pages/man7/bpf-helpers.7.html - BPF helpers documentation

## Environment setup:

We support BPFs only on ARM64 and x86_64 architectures.

1. VM with Ubuntu 20.04+ (or any other Linux distro with the kernel 5.8+ - Docker doesn't work for many BPF features)
2. Install `clang` and `llvm` (required for compiling BPF code)
3. Install `libbpf` (required for loading BPF code) - get the required version from our Docker file

You can also use our Dockerfile to build the environment:

```bash
make -C build.assets release-centos7
```

## BPF code structure

BPF code can be divided into two parts:

1. BPF program - the actual BPF code that will be loaded into the kernel
2. User space code - the code that will load the BPF program into the kernel

BPF programs are located in `bpf` directory in our repository. They are C like programs with some limitations.
The most important limitation is that you can't use any system calls in BPF programs. You can use only BPF helpers.
The BPF programs are compiled using `clang` 10+, GCC is not supported. 

User space code is located in `lib/bpf` directory. It's written in Go and uses `libbpfgo` library to load BPF programs into the kernel.
`libbpfgo` is a Go wrapper around `libbpf` library. It's a low-level library that allows you to load BPF programs into the kernel.

Note: `libbpfgo` is not backward compatible and you need a matching version of `libbpf` library. For that reason
`libbpfgo` is not using semantic versioning, but tags have the following format `v0.4.5-libbpf-1.0.1`. 
The first part is the version of `libbpfgo` and the second part is the version of `libbpf` library. 
Using the wrong version of `libbpfgo` will result in a runtime/compilation error.

### BPF license

BPF programs are compiled into ELF files. ELF files have a license field that is used to verify that the BPF program
is allowed to run in the kernel. The license field is set to `GPL` by default. Teleport uses `Dual BSD/GPL` which 
disables some BPF features (like logging). To enable all BPF features to set the license to `GPL` in the BPF program
and revert back before merging the code.

### Logging

BPF programs can log messages to the kernel log. To enable logging, you need to set the license to `GPL`. Then you
can use `bpf_printk` helper to log messages. The messages will be logged to the kernel log. You can get the messages
 from `/sys/kernel/debug/tracing/trace_pipe`. Here is the best explanation that I found so far https://nakryiko.com/posts/bpf-tips-printk/