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
	"fmt"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

const (
	errInvalidMarshaller = "invalid marshaller encoding"
)

// Exporter implements an OpenTelemetry exporter that pushes OpenTelemetry data to AWS Kinesis
type Exporter struct {
	producer   producer
	logger     *zap.Logger
	marshaller Marshaller
}

// newExporter creates a new exporter with the passed in configurations.
// It starts the AWS session and setups the relevant connections.
func newExporter(c *Config, logger *zap.Logger) (*Exporter, error) {
	marshaller, valid := defaultMarshallers()[c.Encoding]
	if !valid {
		return nil, fmt.Errorf(errInvalidMarshaller)
	}

	pr, err := newKinesisProducer(c, logger)
	if err != nil {
		return nil, err
	}

	return &Exporter{producer: pr, logger: logger, marshaller: marshaller}, nil
}

// Start tells the exporter to start. The exporter may prepare for exporting
// by connecting to the endpoint. Host parameter can be used for communicating
// with the host after start() has already returned. If error is returned by
// start() then the collector startup will be aborted.
func (e *Exporter) Start(_ context.Context, _ component.Host) error {
	e.producer.start()
	return nil
}

// Shutdown is invoked during exporter shutdown
func (e *Exporter) Shutdown(_ context.Context) error {
	e.producer.stop()
	return nil
}

func (e *Exporter) pushTraces(_ context.Context, td pdata.Traces) (int, error) {
	data, err := e.marshaller.MarshalTraces(td)
	var exportErr []error
	if err != nil {
		e.logger.Error("error translating span batch", zap.Error(err))
		exportErr = append(exportErr, consumererror.Permanent(err))

		if data == nil {
			return td.SpanCount(), err
		}
	}

	var droppedSpans int
	for _, msg := range data {
		if msg.Value != nil {
			if err = e.producer.put(msg.Value, uuid.New().String()); err != nil {
				e.logger.Error("error exporting span to kinesis", zap.Error(err))
				exportErr = append(exportErr, err)
			}
		} else {
			// if the value is nil then the marshaling was unsuccessful
			droppedSpans++
		}
	}

	return droppedSpans, componenterror.CombineErrors(exportErr)
}
