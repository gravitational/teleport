module github.com/gravitational/teleport/build.assets/tooling

go 1.24.1

require (
	github.com/Masterminds/sprig/v3 v3.3.0
	github.com/alecthomas/kingpin/v2 v2.4.0 // replaced
	github.com/awalterschulze/goderive v0.5.1
	github.com/bmatcuk/doublestar/v4 v4.8.1
	github.com/gogo/protobuf v1.3.2
	github.com/google/go-github/v41 v41.0.0
	github.com/gravitational/trace v1.5.0
	github.com/stretchr/testify v1.10.0
	github.com/waigani/diffparser v0.0.0-20190828052634-7391f219313d
	golang.org/x/mod v0.24.0
	golang.org/x/oauth2 v0.28.0
	helm.sh/helm/v3 v3.17.1
	howett.net/plist v1.0.1
	k8s.io/apiextensions-apiserver v0.32.2
)

require (
	dario.cat/mergo v1.0.1 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.3.0 // indirect
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kisielk/gotool v1.0.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rogpeppe/go-internal v1.12.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/spf13/cast v1.7.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xhit/go-str2duration/v2 v2.1.0 // indirect
	golang.org/x/crypto v0.35.0 // indirect
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	golang.org/x/tools v0.26.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/apimachinery v0.32.2 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/utils v0.0.0-20241104100929-3ea5e8cea738 // indirect
	sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.2 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

require gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect

replace github.com/alecthomas/kingpin/v2 => github.com/gravitational/kingpin/v2 v2.1.11-0.20230515143221-4ec6b70ecd33
