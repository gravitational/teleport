module github.com/gravitational/teleport/integrations/terraform-mwi

go 1.24.4

replace (
	github.com/gravitational/teleport => ../..
	github.com/gravitational/teleport/api => ../../api
)

// replace statements from teleport
replace (
	github.com/alecthomas/kingpin/v2 => github.com/gravitational/kingpin/v2 v2.1.11-0.20230515143221-4ec6b70ecd33
	github.com/crewjam/saml => github.com/gravitational/saml v0.4.15-teleport.2
	github.com/datastax/go-cassandra-native-protocol => github.com/gravitational/go-cassandra-native-protocol v0.0.0-teleport.1
	github.com/go-mysql-org/go-mysql => github.com/gravitational/go-mysql v1.9.1-teleport.4
	github.com/gogo/protobuf => github.com/gravitational/protobuf v1.3.2-teleport.2
	github.com/julienschmidt/httprouter => github.com/gravitational/httprouter v1.3.1-0.20220408074523-c876c5e705a5
	github.com/keys-pub/go-libfido2 => github.com/gravitational/go-libfido2 v1.5.3-teleport.1
	github.com/microsoft/go-mssqldb => github.com/gravitational/go-mssqldb v1.8.1-teleport.2
	github.com/opencontainers/selinux => github.com/gravitational/selinux v1.13.0-teleport
	github.com/redis/go-redis/v9 => github.com/gravitational/redis/v9 v9.6.1-teleport.1
	github.com/vulcand/predicate => github.com/gravitational/predicate v1.3.4
)
