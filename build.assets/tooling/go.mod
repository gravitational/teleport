module github.com/gravitational/teleport/build.assets/tooling

go 1.18

require (
	github.com/alecthomas/kingpin/v2 v2.3.2 // replaced
	github.com/bmatcuk/doublestar/v4 v4.6.0
	github.com/bradleyfalzon/ghinstallation/v2 v2.4.0
	github.com/google/go-github/v53 v53.1.0
	github.com/google/uuid v1.3.0
	github.com/gravitational/trace v1.2.1
	github.com/hashicorp/go-hclog v1.5.0
	github.com/hashicorp/go-retryablehttp v0.7.4
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.8.4
	github.com/waigani/diffparser v0.0.0-20190828052634-7391f219313d
	golang.org/x/exp v0.0.0-20230522175609-2e198f4a06a1
	golang.org/x/mod v0.10.0
	golang.org/x/oauth2 v0.8.0
	howett.net/plist v1.0.0
)

require (
	github.com/ProtonMail/go-crypto v0.0.0-20230528122434-6f98819771a1 // indirect
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137 // indirect
	github.com/cloudflare/circl v1.3.3 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fatih/color v1.15.0 // indirect
	github.com/golang-jwt/jwt/v5 v5.0.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/jonboulle/clockwork v0.4.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mitchellh/gon v0.2.5
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/xhit/go-str2duration/v2 v2.1.0 // indirect
	golang.org/x/crypto v0.9.0 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/term v0.8.0 // indirect
	google.golang.org/appengine/v2 v2.0.3 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

require github.com/google/go-github/v41 v41.0.0

require (
	github.com/golang-jwt/jwt/v4 v4.5.0 // indirect
	github.com/google/go-github/v52 v52.0.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
)

replace github.com/alecthomas/kingpin/v2 => github.com/gravitational/kingpin/v2 v2.1.11-0.20230515143221-4ec6b70ecd33
