module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter

go 1.15

require (
	github.com/jaegertracing/jaeger v1.23.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr v0.0.0-00010101000000-000000000000
	github.com/signalfx/sapm-proto v0.7.0
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.27.0
	go.uber.org/zap v1.17.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk => ../../internal/splunk

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr => ../../pkg/batchperresourceattr
