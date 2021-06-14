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
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/producer"
)

// Exporter implements an OpenTelemetry trace exporter that exports all spans to AWS Kinesis
type Exporter struct {
	producer producer.Batcher
	batcher  batch.Encoder
	logger   *zap.Logger
}

var (
	_ component.TracesExporter  = (*Exporter)(nil)
	_ component.MetricsExporter = (*Exporter)(nil)
	_ component.LogsExporter    = (*Exporter)(nil)
)

// Start tells the exporter to start. The exporter may prepare for exporting
// by connecting to the endpoint. Host parameter can be used for communicating
// with the host after Start() has already returned. If error is returned by
// Start() then the collector startup will be aborted.
func (e Exporter) Start(ctx context.Context, _ component.Host) error {
	return e.producer.Ready(ctx)
}

// Capabilities implements the consumer interface.
func (e Exporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// Shutdown is invoked during exporter shutdown.
func (e Exporter) Shutdown(context.Context) error {
	return nil
}

// ConsumeTraces receives a span batch and exports it to AWS Kinesis
func (e Exporter) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	bt, err := e.batcher.Traces(td)
	if err != nil {
		return err
	}
	return e.producer.Put(ctx, bt)
}

func (e Exporter) ConsumeMetrics(ctx context.Context, md pdata.Metrics) error {
	bt, err := e.batcher.Metrics(md)
	if err != nil {
		return err
	}
	return e.producer.Put(ctx, bt)
}

func (e Exporter) ConsumeLogs(ctx context.Context, ld pdata.Logs) error {
	bt, err := e.batcher.Logs(ld)
	if err != nil {
		return err
	}
	return e.producer.Put(ctx, bt)
}
