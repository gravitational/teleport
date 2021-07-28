module go-client

go 1.15

replace github.com/gravitational/teleport/api/v7 => ../../api

require (
	github.com/gravitational/teleport/api/v7 v7.0.0
	github.com/pborman/uuid v1.2.1
)
