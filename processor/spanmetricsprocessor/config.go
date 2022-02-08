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

package spanmetricsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor"

import (
	"regexp"
	"time"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/pdata"
)

const (
	delta      = "AGGREGATION_TEMPORALITY_DELTA"
	cumulative = "AGGREGATION_TEMPORALITY_CUMULATIVE"
)

// Dimension defines the key and optional default value if the key is missing from a span attribute.
type Dimension struct {
	Name    string  `mapstructure:"name"`
	Default *string `mapstructure:"default"`
}

// Config defines the configuration options for spanmetricsprocessor.
type Config struct {
	config.ProcessorSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	// MetricsExporter is the name of the metrics exporter to use to ship metrics.
	MetricsExporter string `mapstructure:"metrics_exporter"`

	// LatencyHistogramBuckets is the list of durations representing latency histogram buckets.
	// See defaultLatencyHistogramBucketsMs in processor.go for the default value.
	LatencyHistogramBuckets []time.Duration `mapstructure:"latency_histogram_buckets"`

	// Dimensions defines the list of additional dimensions on top of the provided:
	// - operation
	// - span.kind
	// - status.code
	// The dimensions will be fetched from the span's attributes. Examples of some conventionally used attributes:
	// https://github.com/open-telemetry/opentelemetry-collector/blob/main/model/semconv/opentelemetry.go.
	Dimensions []Dimension `mapstructure:"dimensions"`

	// DimensionsCacheSize defines the size of cache for storing Dimensions, which helps to avoid cache memory growing
	// indefinitely over the lifetime of the collector.
	// Optional. See defaultDimensionsCacheSize in processor.go for the default value.
	DimensionsCacheSize int `mapstructure:"dimensions_cache_size"`

	AggregationTemporality string `mapstructure:"aggregation_temporality"`

	// ResourceAttributes defines the list of additional resource attributes to attach to metrics on top of the provided:
	// - service.name
	// These will be fetched from the span's resource attributes. For more details, see:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/resource/sdk.md
	// and https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/resource/semantic_conventions/README.md.
	ResourceAttributes []Dimension `mapstructure:"resource_attributes"`

	// ResourceAttributesCacheSize defines the size of cache for storing ResourceAttributes, which helps to avoid cache
	// memory growing indefinitely over the lifetime of the collector.
	// Optional. See defaultResourceAttributesCacheSize in processor.go for the default value.
	ResourceAttributesCacheSize int `mapstructure:"resource_attributes_cache_size"`

	// AttachSpanAndTraceID attaches span id and trace id to metrics generated from spans.
	// The default value is set to `false`.
	AttachSpanAndTraceID bool `mapstructure:"attach_span_and_trace_id"`

	// InheritInstrumentationLibraryName defines whether metrics generated from spans should inherit
	// the instrumentation library name from the span.
	// Optional. The default value is `false` which will define the instrumentation library name on metrics as `spanmetricsprocessor`.
	InheritInstrumentationLibraryName bool `mapstructure:"inherit_instrumentation_library_name"`

	// EnableFeatureFlag defines whether the LaunchDarkly feature flag is enabled.
	// Optional. The default value is `false`. i.e. generate metrics from every service.
	EnableFeatureFlag bool `mapstructure:"enable_feature_flag"`

	// LaunchDarklyKey defines the LaunchDarkly key.
	// Optional. Only required when `EnableFeatureFlag` is `true`.
	LaunchDarklyKey string `mapstructure:"launch_darkly_key"`

	// Renames takes in a list of Actions to perform on metrics generated by span metrics processor.
	// This allows a user to specify metrics with certain attributes all matching
	// to have a specific metric name. By default, the generated metric names for Rate / Error is "calls_total" and Duration is
	// "latency".
	Renames []Rename `mapstructure:"renames"`
}

// Rename defines the new metric names and the attributes to match on in order for the rename to occur
// `NewCallsTotalMetricName` and `NewLatencyMetricName` are mandatory arguments that can not be empty.
// If no attributes are specified, then all metrics will be caught under this case and renamed.
type Rename struct {
	Attributes              []AttributeRenameMatchValues `mapstructure:"attributes"`
	NewCallsTotalMetricName string                       `mapstructure:"new_calls_total_metric_name"`
	NewLatencyMetricName    string                       `mapstructure:"new_latency_metric_name"`
}

type AttributeRenameMatchValues struct {
	Attribute              Dimension `mapstructure:"attribute"`
	AttributeValueRegex    string    `mapstructure:"attribute_value_regex"`
	AttributeValueRegexObj *regexp.Regexp
}

func (r Rename) allAttributesKVMatched(attributesOnMetric *pdata.AttributeMap) bool {
	// If no attribute specified then it is the default/ catch-all case
	if len(r.Attributes) == 0 {
		return true
	}

	// check if all attributes specified to match on in rename exists on attributes that will be attached to metric
	for _, attribute := range r.Attributes {
		value, found := attributesOnMetric.Get(attribute.Attribute.Name)

		// check if attribute exists
		if !found {
			return false
		}

		//check if value matches specified regex
		matched := attribute.AttributeValueRegexObj.Match([]byte(value.StringVal()))
		if !matched {
			return false
		}

	}

	return true
}

// GetAggregationTemporality converts the string value given in the config into a MetricAggregationTemporality.
// Returns cumulative, unless delta is correctly specified.
func (c Config) GetAggregationTemporality() pdata.AggregationTemporality {
	if c.AggregationTemporality == delta {
		return pdata.AggregationTemporalityDelta
	}
	return pdata.AggregationTemporalityCumulative
}
