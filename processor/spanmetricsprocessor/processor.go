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

package spanmetricsprocessor

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
	"go.uber.org/zap"
)

const (
	serviceNameKey     = conventions.AttributeServiceName
	operationKey       = "operation" // is there a constant we can refer to?
	spanKindKey        = tracetranslator.TagSpanKind
	statusCodeKey      = tracetranslator.TagStatusCode
	metricKeySeparator = string(byte(0))
)

var (
	maxDuration   = time.Duration(math.MaxInt64)
	maxDurationMs = float64(maxDuration.Milliseconds())

	defaultLatencyHistogramBucketsMs = []float64{
		2, 4, 6, 8, 10, 50, 100, 200, 400, 800, 1000, 1400, 2000, 5000, 10_000, 15_000, maxDurationMs,
	}
)

// dimKV represents the dimension key-value pairs for a metric.
type dimKV map[string]string

type metricKey string

type processorImp struct {
	lock   sync.RWMutex
	logger *zap.Logger
	config Config

	metricsExporter component.MetricsExporter
	nextConsumer    consumer.Traces

	// Additional dimensions to add to metrics.
	dimensions []Dimension

	// Additional resourceAttributes to add to metrics.
	resourceAttributes []Dimension

	// The starting time of the data points.
	startTime time.Time

	// Call & Error counts.
	callSum map[metricKey]map[metricKey]int64

	// Latency histogram.
	latencyCount        map[metricKey]map[metricKey]uint64
	latencySum          map[metricKey]map[metricKey]float64
	latencyBucketCounts map[metricKey]map[metricKey][]uint64
	latencyBounds       []float64

	// List of seen resource attributes.
	// Map structure for faster lookup.
	resourceAttrList map[metricKey]bool

	// A cache of dimension and resource attribute key-value maps keyed by a unique identifier formed by a concatenation of its values:
	// e.g. { "foo/barOK": { "serviceName": "foo", "operation": "/bar", "status_code": "OK" }}
	metricKeyToDimensions map[metricKey]dimKV
}

func newProcessor(logger *zap.Logger, config config.Processor, nextConsumer consumer.Traces) (*processorImp, error) {
	logger.Info("Building spanmetricsprocessor")
	pConfig := config.(*Config)

	bounds := defaultLatencyHistogramBucketsMs
	if pConfig.LatencyHistogramBuckets != nil {
		bounds = mapDurationsToMillis(pConfig.LatencyHistogramBuckets, func(duration time.Duration) float64 {
			return float64(duration.Milliseconds())
		})

		// "Catch-all" bucket.
		if bounds[len(bounds)-1] != maxDurationMs {
			bounds = append(bounds, maxDurationMs)
		}
	}

	if err := validateDimensions(pConfig.Dimensions, []string{spanKindKey, statusCodeKey}); err != nil {
		return nil, err
	}
	if err := validateDimensions(pConfig.ResourceAttributes, []string{serviceNameKey}); err != nil {
		return nil, err
	}

	return &processorImp{
		logger:                logger,
		config:                *pConfig,
		startTime:             time.Now(),
		callSum:               make(map[metricKey]map[metricKey]int64),
		latencyBounds:         bounds,
		latencySum:            make(map[metricKey]map[metricKey]float64),
		latencyCount:          make(map[metricKey]map[metricKey]uint64),
		latencyBucketCounts:   make(map[metricKey]map[metricKey][]uint64),
		nextConsumer:          nextConsumer,
		dimensions:            pConfig.Dimensions,
		resourceAttributes:    pConfig.ResourceAttributes,
		resourceAttrList:      make(map[metricKey]bool),
		metricKeyToDimensions: make(map[metricKey]dimKV),
	}, nil
}

func mapDurationsToMillis(vs []time.Duration, f func(duration time.Duration) float64) []float64 {
	vsm := make([]float64, len(vs))
	for i, v := range vs {
		vsm[i] = f(v)
	}
	return vsm
}

// validateDimensions checks duplicates for reserved dimensions and additional dimensions. Considering
// the usage of Prometheus related exporters, we also validate the dimensions after sanitization.
func validateDimensions(dimensions []Dimension, defaults []string) error {
	labelNames := make(map[string]struct{})
	for _, key := range defaults {
		labelNames[key] = struct{}{}
		labelNames[sanitize(key)] = struct{}{}
	}
	labelNames[operationKey] = struct{}{}

	for _, key := range dimensions {
		if _, ok := labelNames[key.Name]; ok {
			return fmt.Errorf("duplicate dimension name %s", key.Name)
		}
		labelNames[key.Name] = struct{}{}

		sanitizedName := sanitize(key.Name)
		if sanitizedName == key.Name {
			continue
		}
		if _, ok := labelNames[sanitizedName]; ok {
			return fmt.Errorf("duplicate dimension name %s after sanitization", sanitizedName)
		}
		labelNames[sanitizedName] = struct{}{}
	}

	return nil
}

// Start implements the component.Component interface.
func (p *processorImp) Start(ctx context.Context, host component.Host) error {
	p.logger.Info("Starting spanmetricsprocessor")
	exporters := host.GetExporters()

	var availableMetricsExporters []string

	// The available list of exporters come from any configured metrics pipelines' exporters.
	for k, exp := range exporters[config.MetricsDataType] {
		metricsExp, ok := exp.(component.MetricsExporter)
		if !ok {
			return fmt.Errorf("the exporter %q isn't a metrics exporter", k.String())
		}

		availableMetricsExporters = append(availableMetricsExporters, k.String())

		p.logger.Debug("Looking for spanmetrics exporter from available exporters",
			zap.String("spanmetrics-exporter", p.config.MetricsExporter),
			zap.Any("available-exporters", availableMetricsExporters),
		)
		if k.String() == p.config.MetricsExporter {
			p.metricsExporter = metricsExp
			p.logger.Info("Found exporter", zap.String("spanmetrics-exporter", p.config.MetricsExporter))
			break
		}
	}
	if p.metricsExporter == nil {
		return fmt.Errorf("failed to find metrics exporter: '%s'; please configure metrics_exporter from one of: %+v",
			p.config.MetricsExporter, availableMetricsExporters)
	}
	p.logger.Info("Started spanmetricsprocessor")
	return nil
}

// Shutdown implements the component.Component interface.
func (p *processorImp) Shutdown(ctx context.Context) error {
	p.logger.Info("Shutting down spanmetricsprocessor")
	return nil
}

// Capabilities implements the consumer interface.
func (p *processorImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces implements the consumer.Traces interface.
// It aggregates the trace data to generate metrics, forwarding these metrics to the discovered metrics exporter.
// The original input trace data will be forwarded to the next consumer, unmodified.
func (p *processorImp) ConsumeTraces(ctx context.Context, traces pdata.Traces) error {
	p.aggregateMetrics(traces)

	m := p.buildMetrics()

	// Firstly, export metrics to avoid being impacted by downstream trace processor errors/latency.
	if err := p.metricsExporter.ConsumeMetrics(ctx, *m); err != nil {
		return err
	}

	// Forward trace data unmodified.
	return p.nextConsumer.ConsumeTraces(ctx, traces)
}

// buildMetrics collects the computed raw metrics data, builds the metrics object and
// writes the raw metrics data into the metrics object.
func (p *processorImp) buildMetrics() *pdata.Metrics {
	m := pdata.NewMetrics()

	rms := m.ResourceMetrics()
	for key := range p.resourceAttrList {
		p.lock.RLock()
		raKey := metricKey(key)
		resourceAttributesMap := p.metricKeyToDimensions[raKey]

		// if the service name doesn't exist, we treat it as invalid and do not generate a trace
		if _, ok := resourceAttributesMap[serviceNameKey]; !ok {
			continue
		}

		rm := rms.AppendEmpty()

		// append resource attributes
		for attrName, attrValue := range resourceAttributesMap {
			value := pdata.NewAttributeValueString(attrValue)
			rm.Resource().Attributes().Insert(attrName, value)
		}

		ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()
		ilm.InstrumentationLibrary().SetName("spanmetricsprocessor")

		// build metrics per resource
		p.collectCallMetrics(ilm, raKey)
		p.collectLatencyMetrics(ilm, raKey)
		p.lock.RUnlock()
	}

	return &m
}

// collectLatencyMetrics collects the raw latency metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectLatencyMetrics(ilm pdata.InstrumentationLibraryMetrics, resAttrKey metricKey) {
	for metricKey := range p.latencyCount[resAttrKey] {
		mLatency := ilm.Metrics().AppendEmpty()
		mLatency.SetDataType(pdata.MetricDataTypeIntHistogram)
		mLatency.SetName("latency")
		mLatency.IntHistogram().SetAggregationTemporality(pdata.AggregationTemporalityCumulative)

		dpLatency := mLatency.IntHistogram().DataPoints().AppendEmpty()
		dpLatency.SetStartTimestamp(pdata.TimestampFromTime(p.startTime))
		dpLatency.SetTimestamp(pdata.TimestampFromTime(time.Now()))
		dpLatency.SetExplicitBounds(p.latencyBounds)
		dpLatency.SetBucketCounts(p.latencyBucketCounts[resAttrKey][metricKey])
		dpLatency.SetCount(p.latencyCount[resAttrKey][metricKey])
		dpLatency.SetSum(int64(p.latencySum[resAttrKey][metricKey]))

		dpLatency.LabelsMap().InitFromMap(p.metricKeyToDimensions[metricKey])
	}
}

// collectCallMetrics collects the raw call count metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectCallMetrics(ilm pdata.InstrumentationLibraryMetrics, resAttrKey metricKey) {
	for metricKey := range p.callSum[resAttrKey] {
		mCalls := ilm.Metrics().AppendEmpty()
		mCalls.SetDataType(pdata.MetricDataTypeIntSum)
		mCalls.SetName("calls_total")
		mCalls.IntSum().SetIsMonotonic(true)
		mCalls.IntSum().SetAggregationTemporality(pdata.AggregationTemporalityCumulative)

		dpCalls := mCalls.IntSum().DataPoints().AppendEmpty()
		dpCalls.SetStartTimestamp(pdata.TimestampFromTime(p.startTime))
		dpCalls.SetTimestamp(pdata.TimestampFromTime(time.Now()))
		dpCalls.SetValue(p.callSum[resAttrKey][metricKey])

		dpCalls.LabelsMap().InitFromMap(p.metricKeyToDimensions[metricKey])
	}
}

// aggregateMetrics aggregates the raw metrics from the input trace data.
// Each metric is identified by a key that is built from the service name
// and span metadata such as operation, kind, status_code and any additional
// dimensions and resource attributes the user has configured.
func (p *processorImp) aggregateMetrics(traces pdata.Traces) {
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rspans := traces.ResourceSpans().At(i)
		r := rspans.Resource()

		attr, ok := r.Attributes().Get(conventions.AttributeServiceName)
		if !ok {
			continue
		}
		serviceName := attr.StringVal()
		p.aggregateMetricsForServiceSpans(rspans, serviceName)
	}
}

func (p *processorImp) aggregateMetricsForServiceSpans(rspans pdata.ResourceSpans, serviceName string) {
	ilsSlice := rspans.InstrumentationLibrarySpans()
	for j := 0; j < ilsSlice.Len(); j++ {
		ils := ilsSlice.At(j)
		spans := ils.Spans()
		for k := 0; k < spans.Len(); k++ {
			span := spans.At(k)
			p.aggregateMetricsForSpan(serviceName, span, rspans.Resource().Attributes())
		}
	}
}

func (p *processorImp) aggregateMetricsForSpan(serviceName string, span pdata.Span, resourceAttr pdata.AttributeMap) {
	latencyInMilliseconds := float64(span.EndTimestamp()-span.StartTimestamp()) / float64(time.Millisecond.Nanoseconds())

	// Binary search to find the latencyInMilliseconds bucket index.
	index := sort.SearchFloat64s(p.latencyBounds, latencyInMilliseconds)

	mKey := buildMetricKey(span, p.dimensions)
	raKey := buildResourceAttrKey(serviceName, p.resourceAttributes, resourceAttr)

	p.lock.Lock()
	p.resourceAttrList[raKey] = true
	p.cacheMetricKey(span, mKey)
	p.cacheResourceAttrKey(serviceName, resourceAttr, raKey)
	p.updateCallMetrics(raKey, mKey)
	p.updateLatencyMetrics(raKey, mKey, latencyInMilliseconds, index)
	p.lock.Unlock()
}

// updateCallMetrics increments the call count for the given metric key.
func (p *processorImp) updateCallMetrics(raKey metricKey, mkey metricKey) {
	if _, ok := p.callSum[raKey]; ok {
		p.callSum[raKey][mkey]++
	} else {
		p.callSum[raKey] = map[metricKey]int64{mkey: 1}
	}
}

// updateLatencyMetrics increments the histogram counts for the given metric key and bucket index.
func (p *processorImp) updateLatencyMetrics(raKey metricKey, mKey metricKey, latency float64, index int) {
	if _, ok := p.latencyBucketCounts[raKey]; ok {
		if _, ok := p.latencyBucketCounts[raKey][mKey]; !ok {
			p.latencyBucketCounts[raKey][mKey] = make([]uint64, len(p.latencyBounds))
		}
	} else {
		p.latencyBucketCounts[raKey] = make(map[metricKey][]uint64)
		p.latencyBucketCounts[raKey][mKey] = make([]uint64, len(p.latencyBounds))
	}

	p.latencyBucketCounts[raKey][mKey][index]++

	if _, ok := p.latencySum[raKey]; ok {
		p.latencySum[raKey][mKey] += latency
	} else {
		p.latencySum[raKey] = map[metricKey]float64{mKey: latency}
	}

	if _, ok := p.latencyCount[raKey]; ok {
		p.latencyCount[raKey][mKey]++
	} else {
		p.latencyCount[raKey] = map[metricKey]uint64{mKey: 1}
	}
}

func buildDimensionKVs(span pdata.Span, optionalDims []Dimension) dimKV {
	dims := make(dimKV)
	dims[operationKey] = span.Name()
	dims[spanKindKey] = span.Kind().String()
	dims[statusCodeKey] = span.Status().Code().String()
	spanAttr := span.Attributes()
	for _, d := range optionalDims {
		if attr, ok := spanAttr.Get(d.Name); ok {
			dims[d.Name] = tracetranslator.AttributeValueToString(attr)
		} else if d.Default != nil {
			// Set the default if configured, otherwise this metric should have no value set for the dimension.
			dims[d.Name] = *d.Default
		}
	}
	return dims
}

func buildResourceAttrKVs(serviceName string, optionalResourceAttrs []Dimension, resourceAttrs pdata.AttributeMap) dimKV {
	dims := make(dimKV)
	dims[serviceNameKey] = serviceName
	for _, ra := range optionalResourceAttrs {
		if attr, ok := resourceAttrs.Get(ra.Name); ok {
			dims[ra.Name] = tracetranslator.AttributeValueToString(attr)
		} else if ra.Default != nil {
			// Set the default if configured, otherwise this metric should have no value set for the resource attribute.
			dims[ra.Name] = *ra.Default
		}
	}
	return dims
}

func concatDimensionValue(metricKeyBuilder *strings.Builder, value string) {
	// It's worth noting that from pprof benchmarks, WriteString is the most expensive operation of this processor.
	// Specifically, the need to grow the underlying []byte slice to make room for the appended string.
	if metricKeyBuilder.Len() != 0 {
		metricKeyBuilder.WriteString(metricKeySeparator)
	}
	metricKeyBuilder.WriteString(value)
}

// buildMetricKey builds the metric key from the service name and span metadata such as operation, kind, status_code and
// any additional dimensions the user has configured.
// The metric key is a simple concatenation of dimension values.
func buildMetricKey(span pdata.Span, optionalDims []Dimension) metricKey {
	var metricKeyBuilder strings.Builder
	concatDimensionValue(&metricKeyBuilder, span.Name())
	concatDimensionValue(&metricKeyBuilder, span.Kind().String())
	concatDimensionValue(&metricKeyBuilder, span.Status().Code().String())

	spanAttr := span.Attributes()
	var value string
	for _, d := range optionalDims {
		// Set the default if configured, otherwise this metric will have no value set for the dimension.
		if d.Default != nil {
			value = *d.Default
		}
		if attr, ok := spanAttr.Get(d.Name); ok {
			value = tracetranslator.AttributeValueToString(attr)
		}
		concatDimensionValue(&metricKeyBuilder, value)
	}

	k := metricKey(metricKeyBuilder.String())
	return k
}

func buildResourceAttrKey(serviceName string, optionalResourceAttrs []Dimension, resourceAttr pdata.AttributeMap) metricKey {
	var metricKeyBuilder strings.Builder
	concatDimensionValue(&metricKeyBuilder, serviceName)

	var value string
	for _, ra := range optionalResourceAttrs {
		// Set the default if configured, otherwise this metric will have no value set for the resource attribute.
		if ra.Default != nil {
			value = *ra.Default
		}
		if attr, ok := resourceAttr.Get(value); ok {
			value = tracetranslator.AttributeValueToString(attr)
		}
		concatDimensionValue(&metricKeyBuilder, value)
	}

	k := metricKey(metricKeyBuilder.String())
	return k
}

// cache the dimension key-value map for the metricKey if there is a cache miss.
// This enables a lookup of the dimension key-value map when constructing the metric like so:
//   LabelsMap().InitFromMap(p.metricKeyToDimensions[key])
func (p *processorImp) cacheMetricKey(span pdata.Span, k metricKey) {
	if _, ok := p.metricKeyToDimensions[k]; !ok {
		p.metricKeyToDimensions[k] = buildDimensionKVs(span, p.dimensions)
	}
}

// cache the dimension key-value map for the resourceAttrKey if there is a cache miss.
// This enables a lookup of the dimension key-value map when constructing the metric like so:
//   LabelsMap().InitFromMap(p.metricKeyToDimensions[key])
func (p *processorImp) cacheResourceAttrKey(serviceName string, resourceAttrs pdata.AttributeMap, k metricKey) {
	if _, ok := p.metricKeyToDimensions[k]; !ok {
		p.metricKeyToDimensions[k] = buildResourceAttrKVs(serviceName, p.resourceAttributes, resourceAttrs)
	}
}

// copied from prometheus-go-metric-exporter
// sanitize replaces non-alphanumeric characters with underscores in s.
func sanitize(s string) string {
	if len(s) == 0 {
		return s
	}

	// Note: No length limit for label keys because Prometheus doesn't
	// define a length limit, thus we should NOT be truncating label keys.
	// See https://github.com/orijtech/prometheus-go-metrics-exporter/issues/4.
	s = strings.Map(sanitizeRune, s)
	if unicode.IsDigit(rune(s[0])) {
		s = "key_" + s
	}
	if s[0] == '_' {
		s = "key" + s
	}
	return s
}

// copied from prometheus-go-metric-exporter
// sanitizeRune converts anything that is not a letter or digit to an underscore
func sanitizeRune(r rune) rune {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return r
	}
	// Everything else turns into an underscore
	return '_'
}
