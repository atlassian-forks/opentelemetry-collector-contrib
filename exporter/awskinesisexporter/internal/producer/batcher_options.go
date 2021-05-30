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

package producer

import (
	"errors"

	"go.uber.org/zap"
)

type BatchedExporterOption func(*batcher) error

func WithLogger(l *zap.Logger) BatchedExporterOption {
	return func(p *batcher) error {
		if l == nil {
			return errors.New("nil logger trying to be assigned")
		}
		p.log = l
		return nil
	}
}
