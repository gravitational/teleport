module go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc

go 1.16

replace go.opentelemetry.io/contrib => ../../../../

require (
	github.com/golang/protobuf v1.5.2
	go.opentelemetry.io/otel v1.6.1
	go.opentelemetry.io/otel/trace v1.6.1
	google.golang.org/grpc v1.45.0
)
