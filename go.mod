module github.com/gravitational/teleport

go 1.15

require (
	cloud.google.com/go/firestore v1.1.1
	cloud.google.com/go/pubsub v1.2.0 // indirect
	cloud.google.com/go/storage v1.5.0
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/HdrHistogram/hdrhistogram-go v0.9.1-0.20201006155429-aada4ab574ea
	github.com/Microsoft/go-winio v0.4.9
	github.com/alecthomas/assert v0.0.0-20170929043011-405dbfeb8e38 // indirect
	github.com/alecthomas/colour v0.1.0 // indirect
	github.com/alecthomas/repr v0.0.0-20200325044227-4184120f674c // indirect
	github.com/armon/go-radix v1.0.0
	github.com/aws/aws-sdk-go v1.35.19
	github.com/beevik/etree v1.1.0
	github.com/boombuler/barcode v0.0.0-20161226211916-fe0f26ff6d26 // indirect
	github.com/cjbassi/drawille-go v0.1.0 // indirect
	github.com/coreos/go-oidc v0.0.3
	github.com/coreos/go-semver v0.3.0
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/docker v17.12.0-ce-rc1.0.20180721085148-1ef1cc838816+incompatible
	github.com/docker/spdystream v0.0.0-20170912183627-bc6354cbbc29 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/fsouza/fake-gcs-server v1.11.6
	github.com/ghodss/yaml v1.0.0
	github.com/gizak/termui v0.0.0-20190224181052-63c2a0d70943
	github.com/gogo/protobuf v1.3.1
	github.com/gokyle/hotp v0.0.0-20160218004637-c180d57d286b
	github.com/golang/protobuf v1.4.2
	github.com/google/btree v1.0.0
	github.com/google/go-cmp v0.5.2
	github.com/google/gops v0.3.1
	github.com/gravitational/configure v0.0.0-20160909185025-1db4b84fe9db
	github.com/gravitational/form v0.0.0-20151109031454-c4048f792f70
	github.com/gravitational/kingpin v2.1.11-0.20190130013101-742f2714c145+incompatible
	github.com/gravitational/license v0.0.0-20180912170534-4f189e3bd6e3
	github.com/gravitational/oxy v0.0.0-20200916204440-3eb06d921a1d
	github.com/gravitational/reporting v0.0.0-20180907002058-ac7b85c75c4c
	github.com/gravitational/roundtrip v1.0.0
	github.com/gravitational/trace v1.1.13
	github.com/gravitational/ttlmap v0.0.0-20171116003245-91fd36b9004c
	github.com/hashicorp/golang-lru v0.5.4
	github.com/iovisor/gobpf v0.0.1
	github.com/johannesboyne/gofakes3 v0.0.0-20191228161223-9aee1c78a252
	github.com/jonboulle/clockwork v0.2.2
	github.com/json-iterator/go v1.1.10
	github.com/julienschmidt/httprouter v1.2.0
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0
	github.com/kr/pty v1.1.1
	github.com/kylelemons/godebug v0.0.0-20160406211939-eadb3ce320cb
	github.com/mailgun/lemma v0.0.0-20160211003854-e8b0cd607f58
	github.com/mailgun/metrics v0.0.0-20150124003306-2b3c4565aafd // indirect
	github.com/mailgun/minheap v0.0.0-20131208021033-7c28d80e2ada // indirect
	github.com/mailgun/timetools v0.0.0-20141028012446-7e6055773c51
	github.com/mailgun/ttlmap v0.0.0-20150816203249-16b258d86efc
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/mattn/go-sqlite3 v1.10.0
	github.com/mdp/rsc v0.0.0-20160131164516-90f07065088d // indirect
	github.com/mitchellh/go-wordwrap v1.0.0 // indirect
	github.com/pborman/uuid v1.2.0
	github.com/pquerna/otp v0.0.0-20160912161815-54653902c20e
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4
	github.com/prometheus/common v0.6.0
	github.com/prometheus/procfs v0.0.4 // indirect
	github.com/russellhaering/gosaml2 v0.6.0
	github.com/russellhaering/goxmldsig v1.1.0
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/shabbyrobe/gocovmerge v0.0.0-20190829150210-3e036491d500 // indirect
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.6.1
	github.com/tstranex/u2f v0.0.0-20160508205855-eb799ce68da4
	github.com/vulcand/predicate v1.1.0
	github.com/xeipuuv/gojsonpointer v0.0.0-20151027082146-e0fe6f683076 // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20150808065054-e02fc20de94c // indirect
	github.com/xeipuuv/gojsonschema v0.0.0-20151204154511-3988ac14d6f6 // indirect
	go.etcd.io/etcd v0.5.0-alpha.5.0.20200306183522-221f0cc107cb
	go.opencensus.io v0.22.4 // indirect
	go.uber.org/atomic v1.4.0
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/exp v0.0.0-20200224162631-6cc2880d07d6 // indirect
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	golang.org/x/net v0.0.0-20200707034311-ab3426394381
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sys v0.0.0-20200803210538-64077c9b5642
	golang.org/x/text v0.3.3
	google.golang.org/api v0.22.0
	google.golang.org/appengine v1.6.6 // indirect
	google.golang.org/genproto v0.0.0-20200806141610-86f49bd18e98
	google.golang.org/grpc v1.27.0
	google.golang.org/protobuf v1.25.0
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f
	gopkg.in/square/go-jose.v2 v2.5.1
	gopkg.in/yaml.v2 v2.3.0
	gotest.tools v2.2.0+incompatible // indirect
	k8s.io/api v0.0.0-20200821051526-051d027c14e1
	k8s.io/apimachinery v0.20.0-alpha.1.0.20200922235617-829ed199f4e0
	k8s.io/client-go v0.0.0-20200827131824-5d33118d4742
	launchpad.net/gocheck v0.0.0-20140225173054-000000000087 // indirect
)

replace (
	github.com/coreos/go-oidc => github.com/gravitational/go-oidc v0.0.3
	github.com/gogo/protobuf => github.com/gravitational/protobuf v1.3.2-0.20201123192827-2b9fcfaffcbf
	github.com/iovisor/gobpf => github.com/gravitational/gobpf v0.0.1
)
