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
	"fmt"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

const (
	errInvalidContext = "invalid context"
)

// Exporter implements an OpenTelemetry exporter that pushes OpenTelemetry data to AWS Kinesis
type Exporter struct {
	producer   producer
	logger     *zap.Logger
	marshaller Marshaller
}

// NewExporter creates a new exporter with the passed in configurations.
// It starts the AWS session and setups the relevant connections.
func NewExporter(c *Config, logger *zap.Logger) (*Exporter, error) {
	// Get marshaller based on config
	marshaller := defaultMarshallers()[c.Encoding]
	if marshaller == nil {
		return nil, fmt.Errorf("unrecognized encoding")
	}

	pr, err := newKinesisProducer(c, logger)
	if err != nil {
		return nil, err
	}

	return &Exporter{producer: pr, marshaller: marshaller, logger: logger}, nil
}

func (e Exporter) start(ctx context.Context, _ component.Host) error {
	if ctx == nil || ctx.Err() != nil {
		return fmt.Errorf(errInvalidContext)
	}

	e.producer.start()
	return nil
}

func (e Exporter) close(ctx context.Context) error {
	if ctx == nil || ctx.Err() != nil {
		return fmt.Errorf(errInvalidContext)
	}

	e.producer.stop()
	return nil
}

func (e Exporter) tracesDataPusher(ctx context.Context, td pdata.Traces) error {
	if ctx == nil || ctx.Err() != nil {
		return fmt.Errorf(errInvalidContext)
	}

	pBatches, err := e.marshaller.MarshalTraces(td)
	if err != nil {
		e.logger.Error("error translating span batch", zap.Error(err))
		return consumererror.Permanent(err)
	}

	if err = e.producer.put(pBatches, uuid.New().String()); err != nil {
		e.logger.Error("error exporting span to kinesis", zap.Error(err))
		return err
	}

	return nil
}

func (e Exporter) metricsDataPusher(ctx context.Context, md pdata.Metrics) error {
	if ctx == nil || ctx.Err() != nil {
		return fmt.Errorf(errInvalidContext)
	}

	pBatches, err := e.marshaller.MarshalMetrics(md)
	if err != nil {
		e.logger.Error("error translating metrics batch", zap.Error(err))
		return consumererror.Permanent(err)
	}

	if err = e.producer.put(pBatches, uuid.New().String()); err != nil {
		e.logger.Error("error exporting metrics to kinesis", zap.Error(err))
		return err
	}

	return nil
}
