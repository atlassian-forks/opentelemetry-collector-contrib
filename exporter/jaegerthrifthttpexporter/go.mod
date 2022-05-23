module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter

go 1.15

require (
	github.com/apache/thrift v0.16.0
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/google/go-cmp v0.5.7
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/jaegertracing/jaeger v1.34.1
	github.com/stretchr/testify v1.7.1
	go.opentelemetry.io/collector v0.27.0
	go.uber.org/zap v1.21.0
	google.golang.org/protobuf v1.28.0
)
