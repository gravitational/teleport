# tshd

tshd is the Teleport Terminal service daemon.

See proto/teleport/terminal/v1/terminal_service.proto for the gRPC API
reference.

## Usage example

```shell
# tab1: start tshd service
go run ./tool/tshd -addr 'localhost:1234'
> {"addr":"tcp://127.0.0.1:1234"}
> INFO[0000] tshd running at tcp://127.0.0.1:1234

# tab2: generate terminal protoset
# (Unfortunately, grpc reflection doesn't work with gogoproto.)
buf build -o terminal.protoset

# Exercise the service
grpcurl \
  -plaintext \
  -protoset terminal.protoset \
  'localhost:1234' teleport.terminal.v1.TerminalService/ListClusters
> ERROR:
>   Code: Unimplemented
>   Message: method ListClusters not implemented
```

## Configuration

tshd accepts both flags (see `tshd -help`) and JSON-format configuration.
Because tshd is meant to run as an embedded process, it makes special guarantees
in its use of STDIN and STOUT, explained below.

If the daemon is started using `tshd -stdin`, it expects the first value of
STDIN to be a valid JSON configuration. Refer to terminal.ServerOpts for the
possible parameters.

tshd also makes a special guarantee about STDOUT: the first value written in
STDOUT is a JSON-formatted terminal.RuntimeOpts instance. This makes it possible
for the daemon to relay back dynamically-generated parameters to its creator
(such as random addr ports).

tshd considers the JSON formats of both ServerOpts and RuntimeOpts as part of
its public API.
