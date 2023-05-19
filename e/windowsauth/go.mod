module github.com/gravitational/teleport-windows-auth

go 1.19

require (
	github.com/alecthomas/kingpin/v2 v2.3.2 // replaced
	github.com/ncruces/zenity v0.10.1
	github.com/sirupsen/logrus v1.9.0
	github.com/stretchr/testify v1.8.2
	golang.org/x/exp v0.0.0-20230420155640-133eef4313cb
	golang.org/x/sys v0.3.0
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
)

require (
	github.com/BurntSushi/toml v1.2.0 // indirect
	github.com/akavel/rsrc v0.10.2 // indirect
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dchest/jsmin v0.0.0-20220218165748-59f39799265f // indirect
	github.com/josephspurrier/goversioninfo v1.4.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/randall77/makefat v0.0.0-20210315173500-7ddd0e42c844 // indirect
	github.com/xhit/go-str2duration/v2 v2.1.0 // indirect
	golang.org/x/image v0.5.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/alecthomas/kingpin/v2 => github.com/gravitational/kingpin/v2 v2.1.11-0.20230515143221-4ec6b70ecd33
