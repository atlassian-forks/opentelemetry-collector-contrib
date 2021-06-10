// Copyright  OpenTelemetry Authors
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

package batch

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/golang/protobuf/proto" //nolint:staticcheck // This is need due to libraries using old proto package
	"go.opentelemetry.io/collector/consumer/consumererror"
)

const (
	maxRecordSize     = 1 << 20 // 1MiB
	maxBatchedRecords = 500
)

var (
	// ErrPartitionKeyLength is used when the given key exceeds the allowed kinesis limit of 256 characters
	ErrPartitionKeyLength = errors.New("partition key size is greater than 256 characters")
	// ErrRecordLength is used when attempted record results in a byte array greater than 1MiB
	ErrRecordLength = consumererror.Permanent(errors.New("record size is greater than 1 MiB"))
)

type Batch struct {
	maxBatchSize  int
	maxRecordSize int
	records       []*kinesis.PutRecordsRequestEntry
}

func New() *Batch {
	return &Batch{
		maxBatchSize:  maxBatchedRecords,
		maxRecordSize: maxRecordSize,
		records:       make([]*kinesis.PutRecordsRequestEntry, 0, maxRecordSize),
	}
}

// Add accepts type that can be marshalled into a protobuf byte array
// with the partition key. An error is returned when the marshaled data
// or the key exceeds the kinesis API limits.
func (b *Batch) Add(message proto.Message, key string) error {
	data, err := proto.Marshal(message)
	if err != nil {
		return err
	}

	if l := len(key); l == 0 || l > 256 {
		return ErrPartitionKeyLength
	}

	if len(data) > b.maxRecordSize {
		return ErrRecordLength
	}

	b.records = append(b.records, &kinesis.PutRecordsRequestEntry{Data: data, PartitionKey: aws.String(key)})
	return nil
}

// Chunk breaks up the iternal queue into blocks that can be used
// to be written to he kinesis.PutRecords endpoint
func (b *Batch) Chunk() [][]*kinesis.PutRecordsRequestEntry {
	length, chunk := len(b.records), make([][]*kinesis.PutRecordsRequestEntry, 0, len(b.records)/maxBatchedRecords+1)
	for i, end := 0, min(length, b.maxBatchSize); i < length; i, end = i+end, i+min(b.maxBatchSize, length-i) {
		chunk = append(chunk, b.records[i:end])
	}
	return chunk
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
