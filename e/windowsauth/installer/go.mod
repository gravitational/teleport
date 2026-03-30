module github.com/gravitational/teleport-windows-auth

go 1.25.0

require (
	github.com/alecthomas/kingpin/v2 v2.3.2 // replaced
	github.com/ncruces/zenity v0.10.9
	github.com/stretchr/testify v1.8.4 // indirect
	golang.org/x/sys v0.9.0
)

require (
	github.com/akavel/rsrc v0.10.2 // indirect
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137 // indirect
	github.com/dchest/jsmin v0.0.0-20220218165748-59f39799265f // indirect
	github.com/josephspurrier/goversioninfo v1.4.0 // indirect
	github.com/randall77/makefat v0.0.0-20210315173500-7ddd0e42c844 // indirect
	github.com/xhit/go-str2duration/v2 v2.1.0 // indirect
	golang.org/x/image v0.38.0 // indirect
)

replace github.com/alecthomas/kingpin/v2 => github.com/gravitational/kingpin/v2 v2.1.11-0.20230515143221-4ec6b70ecd33
