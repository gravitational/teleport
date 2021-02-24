module go-client

go 1.15

require (
	github.com/gravitational/teleport v0.0.0-00010101000000-000000000000
	github.com/gravitational/teleport/api v0.0.0
	github.com/pborman/uuid v1.2.1
)

replace (
	github.com/coreos/go-oidc => github.com/gravitational/go-oidc v0.0.3
	github.com/gogo/protobuf => github.com/gravitational/protobuf v1.3.2-0.20201123192827-2b9fcfaffcbf
	github.com/gravitational/teleport => ../../
	github.com/gravitational/teleport/api => ../../api
	github.com/iovisor/gobpf => github.com/gravitational/gobpf v0.0.1
	github.com/siddontang/go-mysql v1.1.0 => github.com/gravitational/go-mysql v1.1.1-0.20210212011549-886316308a77
)
