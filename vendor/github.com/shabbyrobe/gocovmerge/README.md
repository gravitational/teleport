gocovmerge
==========

This is a fork of https://github.com/wadey/gocovmerge. It converts it into a
library and introduces idiomatic error handling. The original command's
functionality is provided by github.com/shabbyrobe/gocovmerge/cmd/gocovmerge.

--

gocovmerge takes the results from multiple `go test -coverprofile` runs and
merges them into one profile.

usage
-----

    gocovmerge [coverprofiles...]

gocovmerge takes the source coverprofiles as the arguments (output from
`go test -coverprofile coverage.out`) and outputs a merged version of the
files to standard out. You can only merge profiles that were generated from the
same source code. If there are source lines that overlap or do not merge, the
process will exit with an error code.
