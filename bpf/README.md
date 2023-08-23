# BPF 101

If you're reading this, you were probably tasked with writing a BPF code.
This document will help you get started.

#### Before you start here are some useful links:

https://docs.kernel.org/bpf/ - BPF documentation
https://github.com/aquasecurity/libbpfgo - Go bindings for libbpf
https://github.com/iovisor/bcc - BPF Compiler Collection (a lot of examples)
https://github.com/cilium/cilium - BPF-based networking, security, and observability (more examples)

## Environment setup:

We support BPFs only on ARM64 and x86_64 architectures.

1. VM with Ubuntu 20.04+ (or any other Linux distro with the kernel 5.8+ - Docker doesn't work for many BPF features)
2. Install `clang` and `llvm` (required for compiling BPF code)
3. Install `libbpf` (required for loading BPF code) - get the required version from our Docker file

You can also use our Dockerfile to build the environment:

```bash
make -C build.assets release-centos7
```



