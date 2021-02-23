module go-client

go 1.15

require (
	github.com/gravitational/teleport/api v0.0.0-00010101000000-000000000000
	github.com/gravitational/trace v1.1.13
	github.com/pborman/uuid v1.2.1
)

replace github.com/gravitational/teleport/api => ../../api
