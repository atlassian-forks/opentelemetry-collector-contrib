// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hostmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
)

const (
	scrapersKey = "scrapers"
)

var globalRootPath = &syncRootPath{
	mu: sync.Mutex{},
}

// Config defines configuration for HostMetrics receiver.
type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	Scrapers                                map[string]internal.Config `mapstructure:"-"`
	RootPath                                string                     `mapstructure:"root_path"`
}

var _ config.Receiver = (*Config)(nil)
var _ confmap.Unmarshaler = (*Config)(nil)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	var multiError error
	if len(cfg.Scrapers) == 0 {
		multiError = multierr.Append(multiError, errors.New("must specify at least one scraper when using hostmetrics receiver"))
	}

	if cfg.RootPath != "" {
		_, err := os.Stat(cfg.RootPath)
		multiError = multierr.Append(multiError, fmt.Errorf("bad root_path: %w", err))
	}
	multiError = multierr.Append(multiError, globalRootPath.setRootPath(cfg.RootPath))

	return multiError
}

// Unmarshal a config.Parser into the config struct.
func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		return nil
	}

	// load the non-dynamic config normally
	err := componentParser.Unmarshal(cfg)
	if err != nil {
		return err
	}

	// dynamically load the individual collector configs based on the key name

	cfg.Scrapers = map[string]internal.Config{}

	scrapersSection, err := componentParser.Sub(scrapersKey)
	if err != nil {
		return err
	}

	for key := range scrapersSection.ToStringMap() {
		factory, ok := getScraperFactory(key)
		if !ok {
			return fmt.Errorf("invalid scraper key: %s", key)
		}

		collectorCfg := factory.CreateDefaultConfig()
		collectorViperSection, err := scrapersSection.Sub(key)
		if err != nil {
			return err
		}
		err = collectorViperSection.Unmarshal(collectorCfg, confmap.WithErrorUnused())
		if err != nil {
			return fmt.Errorf("error reading settings for scraper type %q: %w", key, err)
		}

		collectorCfg.ExtractParentConfig(*cfg)

		cfg.Scrapers[key] = collectorCfg
	}

	return nil
}

type syncRootPath struct {
	mu         sync.Mutex
	rootPath   string
	hasBeenSet bool
}

func (s *syncRootPath) setRootPath(rootPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.hasBeenSet && rootPath != s.rootPath {
		return fmt.Errorf("detected instances of hostmetricsreceiver with differing root_path configurations")
	}
	s.rootPath = rootPath
	s.hasBeenSet = true
	return nil
}

func (s *syncRootPath) resetForTest() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rootPath = ""
	s.hasBeenSet = false
}
