# libbpfgo

libbpfgo is a Go library that allows working with the Linux eBPF subsystem via libbpf. libbpf is a C library that's developed as part of the Linux kernel tree which provides an accessible layer of abstraction on top of the raw eBPF system APIs. libbpfgo is just a thin wrapper in Go around libbpf.

## Installing

libbpfgo is using CGO to interop with libbpf and will expect to be linked with libbpf at run or link time. Simply importing libbpfgo is not enough to get started, and you will need to fulfill the required dependency in one of the following ways:

1. Install the libbpf as a shared object in the system. Libbpf may already be packaged for you distribution, if not, you can build and install from source. More info [here](https://github.com/libbpf/libbpf).
2. Embed libbpf into your Go project as a vendored dependency. This means that the libbpf code is statically linked into the resulting binary, and there are no runtime dependencies. [Tracee](https://github.com/aquasecurity/tracee) takes this approach and you can take example from it's [Makefile](https://github.com/aquasecurity/tracee/blob/f8df7da6a27f729610992b6bd52e89d510fcf384/tracee-ebpf/Makefile#L62).

## Concepts
libbpfgo tries to make it natural for Go developers to use, by abstracting away C technicalities. For example, it will translate low level return codes into Go `error`, it will organize functionality around Go `struct`, and it will use `channel` as to let you consume events.

In a high level, this is a typical workflow for working with the library:

0. Compile your bpf program into an object file.
1. Initialize a `Module` struct - that is a unit of BPF functionality around your compiled object file.
2. Load bpf programs from the object file using the `BPFProg` struct.
3. Attach `BPFProg` to system facilities, for example to "raw tracepoints" or "kprobes" using the `BPFProg`'s associated functions.
4. Instantiate and manipulate BPF Maps via the `BPFMap` struct and it's associated methods.
5. Instantiate and manipulate Perf Buffer for communicating events from your BPF program to the driving userspace program, using the `RingBuffer` struct and it's associated objects.

## Example

```go
// initializing
import bpf "github.com/aquasecurity/libbpfgo"
...
bpfModule := bpf.NewModuleFromFile(bpfObjectPath)
bpfModule.BPFLoadObject()

// maps
mymap, _ := bpfModule.GetMap("mymap")
mymap.Update(key, value)

// perf buffer
pb, _ := bpfModule.InitRingBuffer("events", eventsChannel, buffSize)
pb.Start()
e := <-eventsChannel
```

There are many more methods supported and functionality available. We will be documenting this library more extensively in the future, but in the meantime, you can take a look at the [selftest](./selftest) code to get an idea of what's possible, or look at the [tracee-ebpf code](https://github.com/aquasecurity/tracee/tree/main/tracee-ebpf) as a consumer of this library, or just ask us by creating a new [Discussion](https://github.com/aquasecurity/libbpfgo/discussions) and we'd love to help.
