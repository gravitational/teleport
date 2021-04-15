module go-client

go 1.15

replace github.com/gravitational/teleport/api => ../../api

require (
	github.com/gravitational/teleport/api v0.0.0
	github.com/pborman/uuid v1.2.1
)
