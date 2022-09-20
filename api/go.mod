module github.com/gravitational/teleport/api

go 1.15

require (
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.7
	github.com/gravitational/trace v1.1.17
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.1
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.31.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.31.0
	go.opentelemetry.io/otel v1.7.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.7.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.7.0
	go.opentelemetry.io/otel/sdk v1.7.0
	go.opentelemetry.io/otel/trace v1.7.0
	go.opentelemetry.io/proto/otlp v0.16.0
	golang.org/x/crypto v0.0.0-20220126234351-aa10faf2a1f8
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd
	google.golang.org/grpc v1.49.0
	gopkg.in/yaml.v2 v2.4.0
)
