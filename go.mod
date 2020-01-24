module github.com/gravitational/teleport

go 1.13

require (
	cloud.google.com/go v0.44.3
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Microsoft/go-winio v0.4.9
	github.com/PuerkitoBio/purell v1.1.0 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/alecthomas/assert v0.0.0-20170929043011-405dbfeb8e38 // indirect
	github.com/alecthomas/colour v0.1.0 // indirect
	github.com/alecthomas/repr v0.0.0-20181024024818-d37bc2a10ba1 // indirect
	github.com/armon/go-radix v1.0.0
	github.com/aws/aws-sdk-go v1.17.4
	github.com/beevik/etree v0.0.0-20170418002358-cda1c0026246
	github.com/boombuler/barcode v0.0.0-20161226211916-fe0f26ff6d26 // indirect
	github.com/cjbassi/drawille-go v0.1.0 // indirect
	github.com/codahale/hdrhistogram v0.0.0-20150708134006-954f16e8b9ef
	github.com/coreos/bbolt v1.3.3 // indirect
	github.com/coreos/etcd v3.3.15+incompatible
	github.com/coreos/go-oidc v0.0.3
	github.com/coreos/go-semver v0.2.0
	github.com/coreos/go-systemd v0.0.0-20190620071333-e64a0ec8b42a // indirect
	github.com/coreos/pkg v0.0.0-20160314094717-1914e367e85e // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/docker/docker v1.4.2-0.20180721085148-1ef1cc838816
	github.com/docker/spdystream v0.0.0-20170912183627-bc6354cbbc29 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/elazarl/goproxy v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/emicklei/go-restful v2.7.0+incompatible // indirect
	github.com/fsouza/fake-gcs-server v1.11.6
	github.com/ghodss/yaml v1.0.0
	github.com/gizak/termui v0.0.0-20190224181052-63c2a0d70943
	github.com/go-openapi/jsonpointer v0.0.0-20180322222829-3a0015ad55fa // indirect
	github.com/go-openapi/jsonreference v0.0.0-20180322222742-3fb327e6747d // indirect
	github.com/go-openapi/spec v0.0.0-20180415031709-bcff419492ee // indirect
	github.com/go-openapi/swag v0.0.0-20180405201759-811b1089cde9 // indirect
	github.com/gogo/protobuf v1.2.1
	github.com/gokyle/hotp v0.0.0-20160218004637-c180d57d286b
	github.com/golang/protobuf v1.3.2
	github.com/google/btree v1.0.0
	github.com/google/gops v0.3.1
	github.com/google/uuid v1.1.1 // indirect
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/gorilla/websocket v1.4.1 // indirect
	github.com/gravitational/configure v0.0.0-20160909185025-1db4b84fe9db
	github.com/gravitational/form v0.0.0-20151109031454-c4048f792f70
	github.com/gravitational/kingpin v2.1.11-0.20190130013101-742f2714c145+incompatible
	github.com/gravitational/oxy v0.0.0-20180629203109-e4a7e35311e6
	github.com/gravitational/roundtrip v1.0.0
	github.com/gravitational/trace v0.0.0-20190218181455-5d6afe38af2b
	github.com/gravitational/ttlmap v0.0.0-20171116003245-91fd36b9004c
	github.com/gregjones/httpcache v0.0.0-20180305231024-9cad4c3443a7 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.1.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.12.2 // indirect
	github.com/howeyc/gopass v0.0.0-20170109162249-bf9dde6d0d2c // indirect
	github.com/imdario/mergo v0.3.4 // indirect
	github.com/iovisor/gobpf v0.0.1
	github.com/johannesboyne/gofakes3 v0.0.0-20191228161223-9aee1c78a252
	github.com/jonboulle/clockwork v0.1.1-0.20190114141812-62fb9bc030d1
	github.com/json-iterator/go v1.1.7
	github.com/juju/ratelimit v1.0.1 // indirect
	github.com/julienschmidt/httprouter v1.2.0
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0
	github.com/kr/pty v1.1.1
	github.com/kylelemons/godebug v0.0.0-20160406211939-eadb3ce320cb
	github.com/mailgun/lemma v0.0.0-20160211003854-e8b0cd607f58
	github.com/mailgun/metrics v0.0.0-20150124003306-2b3c4565aafd // indirect
	github.com/mailgun/minheap v0.0.0-20131208021033-7c28d80e2ada // indirect
	github.com/mailgun/timetools v0.0.0-20141028012446-7e6055773c51
	github.com/mailgun/ttlmap v0.0.0-20150816203249-16b258d86efc
	github.com/mailru/easyjson v0.0.0-20180323154445-8b799c424f57 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/mattn/go-sqlite3 v1.10.0
	github.com/mdp/rsc v0.0.0-20160131164516-90f07065088d // indirect
	github.com/mitchellh/go-wordwrap v1.0.0 // indirect
	github.com/pborman/uuid v0.0.0-20170612153648-e790cca94e6c
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pquerna/otp v0.0.0-20160912161815-54653902c20e
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90
	github.com/prometheus/common v0.6.0
	github.com/prometheus/procfs v0.0.4 // indirect
	github.com/russellhaering/gosaml2 v0.0.0-20170515204909-8908227c114a
	github.com/russellhaering/goxmldsig v0.0.0-20170515183101-605161228693
	github.com/satori/go.uuid v1.1.1-0.20170321230731-5bf94b69c6b6 // indirect
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/shabbyrobe/gocovmerge v0.0.0-20190829150210-3e036491d500 // indirect
	github.com/sirupsen/logrus v1.4.2
	github.com/soheilhy/cmux v0.1.4 // indirect
	github.com/spf13/pflag v1.0.1 // indirect
	github.com/tmc/grpc-websocket-proxy v0.0.0-20200122045848-3419fae592fc // indirect
	github.com/tstranex/u2f v0.0.0-20160508205855-eb799ce68da4
	github.com/vulcand/predicate v1.1.0
	github.com/xeipuuv/gojsonpointer v0.0.0-20151027082146-e0fe6f683076 // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20150808065054-e02fc20de94c // indirect
	github.com/xeipuuv/gojsonschema v0.0.0-20151204154511-3988ac14d6f6 // indirect
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2 // indirect
	go.etcd.io/bbolt v1.3.3 // indirect
	go.opencensus.io v0.22.1 // indirect
	golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550
	golang.org/x/net v0.0.0-20191002035440-2ec189313ef0
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sys v0.0.0-20200116001909-b77594299b42
	golang.org/x/text v0.3.2
	golang.org/x/tools v0.0.0-20191227053925-7b8e75db28f4 // indirect
	google.golang.org/api v0.10.0
	google.golang.org/appengine v1.6.3 // indirect
	google.golang.org/genproto v0.0.0-20190927181202-20e1ac93f88c
	google.golang.org/grpc v1.24.0
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.2.4
	gotest.tools v2.2.0+incompatible // indirect
	k8s.io/api v0.0.0-20180510142701-9273ee02527c
	k8s.io/apimachinery v0.0.0-20180510142256-21efb2924c7c
	k8s.io/client-go v6.0.0+incompatible
	k8s.io/kube-openapi v0.0.0-20180524221615-41e43949ca69 // indirect
	launchpad.net/gocheck v0.0.0-20140225173054-000000000087 // indirect
	sigs.k8s.io/yaml v1.1.0 // indirect
)

// replace github.com/coreos/go-oidc v0.0.3 => github.com/gravitational/go-oidc v0.0.3
replace github.com/coreos/go-oidc => github.com/gravitational/go-oidc v0.0.3

// replace github.com/iovisor/gobpf v0.0.1 => github.com/gravitational/gobpf v0.0.1
replace github.com/iovisor/gobpf => github.com/gravitational/gobpf v0.0.1

// replace github.com/sirupsen/logrus 8ab1e1b91d5f1a6124287906f8b0402844d3a2b3 => github.com/gravitational/logrus v0.10.1-0.20171120195323-8ab1e1b91d5f
replace github.com/sirupsen/logrus => github.com/gravitational/logrus v0.10.1-0.20171120195323-8ab1e1b91d5f
