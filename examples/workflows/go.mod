module example

go 1.16

replace github.com/gravitational/teleport/api => ../../api

require (
	github.com/gravitational/teleport/api v0.0.0
	github.com/gravitational/trace v1.1.15
	github.com/pelletier/go-toml v1.9.0
)
