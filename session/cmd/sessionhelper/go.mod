module github.com/gravitational/teleport/session/cmd/sessionhelper

go 1.25.9

require github.com/gravitational/teleport v0.0.0-00010101000000-000000000000

require (
	cyphar.com/go-pathrs v0.2.1 // indirect
	github.com/DataDog/datadog-agent/pkg/template v0.78.2 // indirect
	github.com/cyphar/filepath-securejoin v0.6.1 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/gravitational/teleport/api v0.0.0-00010101000000-000000000000 // indirect
	github.com/gravitational/trace v1.5.3 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/mdlayher/netlink v1.8.0 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/opencontainers/selinux v1.13.2-0.20260424110006-f148739380ba // indirect
	github.com/pkg/sftp v1.13.10 // indirect
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
)

replace (
	github.com/alecthomas/kingpin/v2 => github.com/gravitational/kingpin/v2 v2.1.11-0.20260417152838-9efcbe7e5d61
	github.com/crewjam/saml => github.com/gravitational/saml v0.4.15-teleport.2
	github.com/datastax/go-cassandra-native-protocol => github.com/gravitational/go-cassandra-native-protocol v0.0.0-teleport.3
	github.com/go-mysql-org/go-mysql => github.com/gravitational/go-mysql v1.9.1-teleport.4
	github.com/gogo/protobuf => github.com/gravitational/protobuf v1.3.2-teleport.2
	github.com/gravitational/teleport => ../../..
	github.com/gravitational/teleport/api => ../../../api
	github.com/hashicorp/terraform-plugin-docs => github.com/gravitational/terraform-plugin-docs v0.19.5-0.20250326215846-2e10ca5fcbdf
	github.com/hinshun/vt10x => github.com/gravitational/vt10x v0.0.3-teleport.1
	github.com/julienschmidt/httprouter => github.com/gravitational/httprouter v1.3.1-0.20220408074523-c876c5e705a5
	github.com/keys-pub/go-libfido2 => github.com/gravitational/go-libfido2 v1.5.3-teleport.1
	github.com/microsoft/go-mssqldb => github.com/gravitational/go-mssqldb v1.8.1-teleport.2
	github.com/redis/go-redis/v9 => github.com/gravitational/redis/v9 v9.6.1-teleport.1
	github.com/vulcand/predicate => github.com/gravitational/predicate v1.3.4
	golang.org/x/crypto => ./x-crypto-stub
)
