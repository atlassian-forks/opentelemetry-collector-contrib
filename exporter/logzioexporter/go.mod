module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter

go 1.15

require (
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/hashicorp/go-hclog v1.2.0
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/jaegertracing/jaeger v1.33.0
	github.com/logzio/jaeger-logzio v0.0.0-20201026090333-8336e3e13ec6
	github.com/stretchr/testify v1.7.1
	go.opentelemetry.io/collector v0.27.0
	go.uber.org/zap v1.21.0
	google.golang.org/protobuf v1.28.0
)
