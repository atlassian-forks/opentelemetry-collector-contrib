// Copyright 2019 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awskinesisexporter

import (
	"go.opentelemetry.io/collector/config"
)

// AWSConfig contains AWS specific configuration such as awskinesis stream, region, etc.
type AWSConfig struct {
	StreamName      string `mapstructure:"stream_name"`
	KinesisEndpoint string `mapstructure:"kinesis_endpoint"`
	Region          string `mapstructure:"region"`
	Role            string `mapstructure:"role"`
}

// Config contains the main configuration options for the awskinesis exporter
type Config struct {
	config.ExporterSettings `mapstructure:",squash"`

	AWS                AWSConfig `mapstructure:"aws"`
	MaxRecordsPerBatch int       `mapstructure:"max_records_per_batch"`
	MaxRecordSize      int       `mapstructure:"max_record_size"`
}
