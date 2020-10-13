module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kinesisexporter

go 1.14

require (
	github.com/aws/aws-sdk-go v1.34.9
	github.com/google/uuid v1.1.2
	github.com/signalfx/omnition-kinesis-producer v0.5.0
	github.com/signalfx/opencensus-go-exporter-kinesis v0.6.3
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.12.1-0.20201012183541-526f34200197
	go.uber.org/zap v1.16.0
	google.golang.org/grpc/examples v0.0.0-20200728194956-1c32b02682df // indirect
)
