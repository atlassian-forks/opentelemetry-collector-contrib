module github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor

go 1.15

require (
	github.com/hashicorp/golang-lru v0.5.4
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.27.0
	go.uber.org/zap v1.20.0
	google.golang.org/grpc v1.37.1
	gopkg.in/ini.v1 v1.57.0 // indirect
	gopkg.in/launchdarkly/go-sdk-common.v2 v2.5.0
	gopkg.in/launchdarkly/go-server-sdk.v5 v5.8.1
)
