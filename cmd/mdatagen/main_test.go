// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	md "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/mdatagen/internal/metadata"
)

func Test_runContents(t *testing.T) {
	tests := []struct {
		yml                  string
		wantMetricsGenerated bool
		wantConfigGenerated  bool
		wantStatusGenerated  bool
		wantErr              bool
	}{
		{
			yml:     "invalid.yaml",
			wantErr: true,
		},
		{
			yml:                  "metrics_and_type.yaml",
			wantMetricsGenerated: true,
			wantConfigGenerated:  true,
		},
		{
			yml:                 "resource_attributes_only.yaml",
			wantConfigGenerated: true,
		},
		{
			yml:                 "status_only.yaml",
			wantStatusGenerated: true,
		},
		{
			yml:     "multi_line_strings.yaml",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.yml, func(t *testing.T) {
			tmpdir := t.TempDir()
			ymlContent, err := os.ReadFile(filepath.Join("testdata", tt.yml))
			require.NoError(t, err)
			metadataFile := filepath.Join(tmpdir, "metadata.yaml")
			require.NoError(t, os.WriteFile(metadataFile, ymlContent, 0600))
			require.NoError(t, os.WriteFile(filepath.Join(tmpdir, "README.md"), []byte(`
<!-- status autogenerated section -->
foo
<!-- end autogenerated section -->`), 0600))

			err = run(metadataFile)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			if tt.wantMetricsGenerated {
				require.FileExists(t, filepath.Join(tmpdir, "internal/metadata/generated_metrics.go"))
				require.FileExists(t, filepath.Join(tmpdir, "internal/metadata/generated_metrics_test.go"))
				require.FileExists(t, filepath.Join(tmpdir, "documentation.md"))
			} else {
				require.NoFileExists(t, filepath.Join(tmpdir, "internal/metadata/generated_metrics.go"))
				require.NoFileExists(t, filepath.Join(tmpdir, "internal/metadata/generated_metrics_test.go"))
				require.NoFileExists(t, filepath.Join(tmpdir, "documentation.md"))
			}

			if tt.wantConfigGenerated {
				require.FileExists(t, filepath.Join(tmpdir, "internal/metadata/generated_config.go"))
				require.FileExists(t, filepath.Join(tmpdir, "internal/metadata/generated_config_test.go"))
			} else {
				require.NoFileExists(t, filepath.Join(tmpdir, "internal/metadata/generated_config.go"))
				require.NoFileExists(t, filepath.Join(tmpdir, "internal/metadata/generated_config_test.go"))
			}

			if tt.wantStatusGenerated {
				require.FileExists(t, filepath.Join(tmpdir, "internal/metadata/generated_status.go"))
				contents, err := os.ReadFile(filepath.Join(tmpdir, "README.md"))
				require.NoError(t, err)
				require.NotContains(t, string(contents), "foo")
			} else {
				require.NoFileExists(t, filepath.Join(tmpdir, "internal/metadata/generated_status.go"))
				contents, err := os.ReadFile(filepath.Join(tmpdir, "README.md"))
				require.NoError(t, err)
				require.Contains(t, string(contents), "foo")
			}
		})
	}
}

func Test_run(t *testing.T) {
	type args struct {
		ymlPath string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "no argument",
			args:    args{""},
			wantErr: true,
		},
		{
			name:    "no such file",
			args:    args{"/no/such/file"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := run(tt.args.ymlPath); (err != nil) != tt.wantErr {
				t.Errorf("run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_inlineReplace(t *testing.T) {
	tests := []struct {
		name           string
		markdown       string
		outputFile     string
		componentClass string
		warnings       []string
		stability      map[string][]string
		distros        []string
	}{
		{
			name: "readme with empty status",
			markdown: `# Some component

<!-- status autogenerated section -->
<!-- end autogenerated section -->

Some info about a component
`,
			outputFile:     "readme_with_status.md",
			componentClass: "receiver",
			distros:        []string{"contrib"},
		},
		{
			name: "readme with status for extension",
			markdown: `# Some component

<!-- status autogenerated section -->
<!-- end autogenerated section -->

Some info about a component
`,
			outputFile:     "readme_with_status_extension.md",
			componentClass: "extension",
			distros:        []string{"contrib"},
		},
		{
			name: "readme with status table",
			markdown: `# Some component

<!-- status autogenerated section -->
| Status                   |           |
| ------------------------ |-----------|
<!-- end autogenerated section -->

Some info about a component
`,
			outputFile:     "readme_with_status.md",
			componentClass: "receiver",
			distros:        []string{"contrib"},
		},
		{
			name: "readme with no status",
			markdown: `# Some component

Some info about a component
`,
			outputFile: "readme_without_status.md",
			distros:    []string{"contrib"},
		},
		{
			name: "component with warnings",
			markdown: `# Some component

<!-- status autogenerated section -->
<!-- end autogenerated section -->

Some info about a component
### warnings
Some warning there.
`,
			outputFile: "readme_with_warnings.md",
			warnings:   []string{"warning1"},
			distros:    []string{"contrib"},
		},
		{
			name: "readme with multiple signals",
			markdown: `# Some component

<!-- status autogenerated section -->
<!-- end autogenerated section -->

Some info about a component
`,
			outputFile: "readme_with_multiple_signals.md",
			stability:  map[string][]string{"beta": {"metrics"}, "alpha": {"logs"}},
			distros:    []string{"contrib"},
		},
		{
			name: "readme with cmd class",
			markdown: `# Some component

<!-- status autogenerated section -->
<!-- end autogenerated section -->

Some info about a component
`,
			outputFile:     "readme_with_cmd_class.md",
			stability:      map[string][]string{"beta": {"metrics"}, "alpha": {"logs"}},
			componentClass: "cmd",
			distros:        []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stability := map[string][]string{"beta": {"metrics"}}
			if len(tt.stability) > 0 {
				stability = tt.stability
			}
			md := metadata{
				Status: Status{
					Stability:     stability,
					Distributions: tt.distros,
					Class:         tt.componentClass,
					Warnings:      tt.warnings,
				},
			}
			tmpdir := t.TempDir()

			readmeFile := filepath.Join(tmpdir, "README.md")
			require.NoError(t, os.WriteFile(readmeFile, []byte(tt.markdown), 0600))

			err := inlineReplace("templates/readme.md.tmpl", readmeFile, md, statusStart, statusEnd)
			require.NoError(t, err)

			require.FileExists(t, filepath.Join(tmpdir, "README.md"))
			got, err := os.ReadFile(filepath.Join(tmpdir, "README.md"))
			require.NoError(t, err)
			got = bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n"))
			expected, err := os.ReadFile(filepath.Join("testdata", tt.outputFile))
			require.NoError(t, err)
			expected = bytes.ReplaceAll(expected, []byte("\r\n"), []byte("\n"))
			require.Equal(t, expected, got, "got: %s\nexpected: %s", got, expected)
		})
	}
}

func TestGenerateStatusMetadata(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		md       metadata
		expected string
	}{
		{
			name: "foo component with beta status",
			md: metadata{
				Type: "foo",
				Status: Status{
					Stability:     map[string][]string{"beta": {"metrics"}},
					Distributions: []string{"contrib"},
					Class:         "receiver",
				},
			},
			expected: `// Code generated by mdatagen. DO NOT EDIT.

package metadata

import (
	"go.opentelemetry.io/collector/component"
)

const (
	Type             = "foo"
	MetricsStability = component.StabilityLevelBeta
)
`,
		},
		{
			name: "foo component with alpha status",
			md: metadata{
				Type: "foo",
				Status: Status{
					Stability:     map[string][]string{"alpha": {"metrics"}},
					Distributions: []string{"contrib"},
					Class:         "receiver",
				},
			},
			expected: `// Code generated by mdatagen. DO NOT EDIT.

package metadata

import (
	"go.opentelemetry.io/collector/component"
)

const (
	Type             = "foo"
	MetricsStability = component.StabilityLevelAlpha
)
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpdir := t.TempDir()
			err := generateFile("templates/status.go.tmpl",
				filepath.Join(tmpdir, "generated_status.go"), tt.md)
			require.NoError(t, err)
			actual, err := os.ReadFile(filepath.Join(tmpdir, "generated_status.go"))
			require.NoError(t, err)
			require.Equal(t, tt.expected, string(actual))
		})
	}
}

// TestGenerated verifies that the internal/metadata API is generated correctly.
func TestGenerated(t *testing.T) {
	mb := md.NewMetricsBuilder(md.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings())
	m := mb.Emit()
	require.Equal(t, 0, m.ResourceMetrics().Len())
}
