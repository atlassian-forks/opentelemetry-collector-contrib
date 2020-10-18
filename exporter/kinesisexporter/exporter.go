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

package kinesisexporter

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

// exporter implements an OpenTelemetry exporter that pushes OpenTelemetry data to AWS Kinesis
type exporter struct {
	producer   producer
	logger     *zap.Logger
	marshaller Marshaller
}

// newExporter creates a new exporter with the passed in configurations.
// It starts the AWS session and setups the relevant connections.
func newExporter(c *Config, logger *zap.Logger) (*exporter, error) {
	// Get marshaller based on config
	marshaller := defaultMarshallers()[c.Encoding]
	if marshaller == nil {
		return nil, fmt.Errorf("unrecognized encoding")
	}

	pr, err := newKinesisProducer(c, logger)
	if err != nil {
		return nil, err
	}

	return &exporter{producer: pr, marshaller: marshaller, logger: logger}, nil
}

// start tells the exporter to start. The exporter may prepare for exporting
// by connecting to the endpoint. Host parameter can be used for communicating
// with the host after start() has already returned. If error is returned by
// start() then the collector startup will be aborted.
func (e *exporter) start(ctx context.Context, _ component.Host) error {
	if ctx == nil || ctx.Err() != nil {
		return errors.New(`invalid context`)
	}

	e.producer.start()
	return nil
}

// shutdown is invoked during exporter shutdown
func (e *exporter) shutdown(ctx context.Context) error {
	if ctx == nil || ctx.Err() != nil {
		return errors.New(`invalid context`)
	}

	e.producer.stop()
	return nil
}

func (e *exporter) pushTraces(ctx context.Context, td pdata.Traces) (int, error) {
	if ctx == nil || ctx.Err() != nil {
		return 0, errors.New(`invalid context`)
	}

	pBatches, err := e.marshaller.MarshalTraces(td)
	if err != nil {
		e.logger.Error("error translating span batch", zap.Error(err))
		return td.SpanCount(), consumererror.Permanent(err)
	}

	if err = e.producer.put(pBatches, uuid.New().String()); err != nil {
		e.logger.Error("error exporting span to kinesis", zap.Error(err))
		return td.SpanCount(), err
	}

	return 0, nil
}

func (e *exporter) pushMetrics(ctx context.Context, td pdata.Metrics) (int, error) {
	if ctx == nil || ctx.Err() != nil {
		return 0, errors.New(`invalid context`)
	}

	pBatches, err := e.marshaller.MarshalMetrics(td)
	if err != nil {
		e.logger.Error("error translating metrics batch", zap.Error(err))
		return td.MetricCount(), consumererror.Permanent(err)
	}

	if err = e.producer.put(pBatches, uuid.New().String()); err != nil {
		e.logger.Error("error exporting metrics to kinesis", zap.Error(err))
		return td.MetricCount(), err
	}

	return 0, nil
}
