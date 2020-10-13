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
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
	producer "github.com/signalfx/omnition-kinesis-producer"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	jaegertranslator "go.opentelemetry.io/collector/translator/trace/jaeger"
	"go.uber.org/zap"
)

// Exporter implements an OpenTelemetry trace exporter that exports all spans to AWS Kinesis
type Exporter struct {
	producer *producer.Producer
	logger   *zap.Logger
}

// newKinesisExporter creates a new Exporter with the passed in configurations.
// It starts the AWS session and setups the relevant connections.
func newKinesisExporter(c *Config, logger *zap.Logger) (*Exporter, error) {
	awsConfig := aws.NewConfig().WithRegion(c.AWS.Region).WithEndpoint(c.AWS.KinesisEndpoint)
	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, err
	}
	client := kinesis.New(sess)

	pr := producer.New(&producer.Config{
		Client:     client,
		StreamName: c.AWS.StreamName,
		// KPL parameters
		FlushInterval:       time.Duration(c.KPL.FlushIntervalSeconds) * time.Second,
		BatchCount:          c.KPL.BatchCount,
		BatchSize:           c.KPL.BatchSize,
		AggregateBatchCount: c.KPL.AggregateBatchCount,
		AggregateBatchSize:  c.KPL.AggregateBatchSize,
		BacklogCount:        c.KPL.BacklogCount,
		MaxConnections:      c.KPL.MaxConnections,
		MaxRetries:          c.KPL.MaxRetries,
		MaxBackoffTime:      time.Duration(c.KPL.MaxBackoffSeconds) * time.Second,
	}, nil)

	return &Exporter{producer: pr, logger: logger}, nil
}

// Start tells the exporter to start. The exporter may prepare for exporting
// by connecting to the endpoint. Host parameter can be used for communicating
// with the host after Start() has already returned. If error is returned by
// Start() then the collector startup will be aborted.
func (e Exporter) Start(_ context.Context, _ component.Host) error {
	e.producer.Start()
	return nil
}

// Shutdown is invoked during exporter shutdown.
func (e Exporter) Shutdown(context.Context) error {
	e.producer.Stop()
	return nil
}

// ConsumeTraceData receives a span batch and exports it to AWS Kinesis
func (e Exporter) ConsumeTraces(_ context.Context, td pdata.Traces) error {
	pBatches, err := jaegertranslator.InternalTracesToJaegerProto(td)
	if err != nil {
		e.logger.Error("error translating span batch", zap.Error(err))
		return consumererror.Permanent(err)
	}
	// TODO: Use a multi error type
	var exportErr error
	for _, pBatch := range pBatches {
		for _, span := range pBatch.GetSpans() {
			if span.Process == nil {
				span.Process = pBatch.Process
			}
			// Temporary span marshaling
			d, err := span.Marshal()
			if err != nil {
				e.logger.Error("error marshaling span", zap.Error(err))
			}
			err = e.producer.Put(d, span.SpanID.String())
			if err != nil {
				e.logger.Error("error exporting span to kinesis", zap.Error(err))
				exportErr = err
			}
		}
	}
	return exportErr
}
