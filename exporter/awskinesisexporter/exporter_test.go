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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap/zaptest"
)

type producerMock struct {
	mock.Mock
}

func (m *producerMock) start() {
	m.Called()
}

func (m *producerMock) stop() {
	m.Called()
}

func (m *producerMock) put(data []byte, partitionKey string) error {
	args := m.Called(data, partitionKey)
	return args.Error(0)
}

func TestNewKinesisExporter(t *testing.T) {
	t.Parallel()
	cfg := createDefaultConfig().(*Config)
	require.NotNil(t, cfg)

	exp, err := NewExporter(cfg, zaptest.NewLogger(t))
	assert.NotNil(t, exp)
	assert.NoError(t, err)
}

func TestNewKinesisExporterStartUp(t *testing.T) {
	t.Parallel()
	cfg := createDefaultConfig().(*Config)
	require.NotNil(t, cfg)

	exp, err := NewExporter(cfg, zaptest.NewLogger(t))
	require.NoError(t, err)
	mockProducer := new(producerMock)
	exp.producer = mockProducer
	require.NotNil(t, exp)

	mockProducer.On("start").Return(nil)

	err = exp.start(context.Background(), nil)
	assert.NoError(t, err)
}

func TestNewKinesisExporterShutdown(t *testing.T) {
	t.Parallel()
	cfg := createDefaultConfig().(*Config)
	require.NotNil(t, cfg)

	exp, err := NewExporter(cfg, zaptest.NewLogger(t))
	require.NoError(t, err)
	mockProducer := new(producerMock)
	exp.producer = mockProducer
	require.NotNil(t, exp)

	mockProducer.On("stop").Return(nil)

	err = exp.close(context.Background())
	assert.NoError(t, err)
}

func TestNewKinesisExporterBadEncoding(t *testing.T) {
	t.Parallel()
	cfg := createDefaultConfig().(*Config)
	require.NotNil(t, cfg)
	cfg.Encoding = ""

	exp, err := NewExporter(cfg, zaptest.NewLogger(t))
	assert.Nil(t, exp)
	assert.Errorf(t, err, "unrecognized encoding")
}

func TestPushingTracesToKinesisQueue(t *testing.T) {
	t.Parallel()
	cfg := createDefaultConfig().(*Config)
	require.NotNil(t, cfg)

	exp, err := NewExporter(cfg, zaptest.NewLogger(t))
	require.NoError(t, err)
	mockProducer := new(producerMock)
	exp.producer = mockProducer
	require.NotNil(t, exp)

	mockProducer.On("put", mock.Anything, mock.AnythingOfType("string")).Return(nil)

	err = exp.tracesDataPusher(context.Background(), pdata.NewTraces())
	require.NoError(t, err)
}

func TestErrorPushingTracesToKinesisQueue(t *testing.T) {
	t.Parallel()
	cfg := createDefaultConfig().(*Config)
	require.NotNil(t, cfg)

	exp, err := NewExporter(cfg, zaptest.NewLogger(t))
	require.NoError(t, err)
	mockProducer := new(producerMock)
	exp.producer = mockProducer
	require.NotNil(t, exp)

	mockProducer.On("put", mock.Anything, mock.AnythingOfType("string")).Return(fmt.Errorf("someerror"))

	err = exp.tracesDataPusher(context.Background(), pdata.NewTraces())
	require.Error(t, err)
}

func TestPushingMetricsToKinesisQueue(t *testing.T) {
	t.Parallel()
	cfg := createDefaultConfig().(*Config)
	require.NotNil(t, cfg)

	exp, err := NewExporter(cfg, zaptest.NewLogger(t))
	require.NoError(t, err)
	mockProducer := new(producerMock)
	exp.producer = mockProducer
	require.NotNil(t, exp)

	mockProducer.On("put", mock.Anything, mock.AnythingOfType("string")).Return(nil)

	err = exp.metricsDataPusher(context.Background(), pdata.NewMetrics())
	require.NoError(t, err)
}

func TestErrorPushingMetricsToKinesisQueue(t *testing.T) {
	t.Parallel()
	cfg := createDefaultConfig().(*Config)
	require.NotNil(t, cfg)

	exp, err := NewExporter(cfg, zaptest.NewLogger(t))
	require.NoError(t, err)
	mockProducer := new(producerMock)
	exp.producer = mockProducer
	require.NotNil(t, exp)

	mockProducer.On("put", mock.Anything, mock.AnythingOfType("string")).Return(fmt.Errorf("someerror"))

	err = exp.metricsDataPusher(context.Background(), pdata.NewMetrics())
	require.Error(t, err)
}
