//go:build linux

package filesystemscraper

import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"

func (cfg *Config) ExtractParentConfig(parentCfg hostmetricsreceiver.Config) {
	cfg.RootPath = parentCfg.RootPath
}
