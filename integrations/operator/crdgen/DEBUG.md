## Debugging tips for `crdgen`

The `protoc` request can be saved to a file and loaded by a debug build of
the plugin. This allows easier and more reproductible tests and is especially
helpful if you want to attach a debugger.

### 1. Dumping `protoc` request

`protoc` request can be dumped with the dummy `protoc-gen-dump` plugin in
`./hack`. The `make debug-dump-request` target generates a file for each source
proto file:

```shell
$ make debug-dump-request

for proto in teleport/loginrule/v1/loginrule.proto teleport/legacy/types/types.proto; do \
        protoc \
                -I=testdata/protofiles \
                -I=/Users/shaka/go/pkg/mod/github.com/gravitational/protobuf@v1.3.2-teleport.1 \
                --plugin=./hack/protoc-gen-dump \
                --dump_out="." \
                "${proto}"; \
done
Output written to /var/folders/vz/qfyzlg092_dgktzq3nzp5s040000gn/T/tmp.q6KOY07KwO
Output written to /var/folders/vz/qfyzlg092_dgktzq3nzp5s040000gn/T/tmp.WQWM1TXWku
```

### 2. Building a debug `crdgen`

Configure your IDE to set the tag `debug` on build, or manually build the plugin
```
go build github.com/gravitational/teleport/integrations/operator/crdgen/cmd/protoc-gen-crd -tags debug
```

This debug build won't load requests from stdin, but from a dump file.

### 3. Run the debug build and load instructions from the file

You must instruct the debug build to load instructions from a dump file by
setting the `TELEPORT_PROTOC_READ_FILE` environment variable.

```shell
export TELEPORT_PROTOC_READ_FILE="/var/folders/vz/qfyzlg092_dgktzq3nzp5s040000gn/T/tmp.WQWM1TXWku"
./protoc-gen-crd-debug
```

From there you can invoke the plugin from `delve`, or configure your favorite
IDE to set the environment variable when debugging the binary.
