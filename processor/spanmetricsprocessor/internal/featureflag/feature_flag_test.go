package featureflag

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		ffKey   string
		wantErr bool
	}{
		{
			name:    "should generate featureFlag without ffkey",
			ffKey:   "",
			wantErr: false,
		},
		{
			name:    "should get error with incorrect ffkey",
			ffKey:   "testing",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.ffKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestEnabledForService(t *testing.T) {
	t.Run("if LD client is nil, should return error and disabled value as default", func(t *testing.T) {
		ff := &featureFlag{}
		enabled, err := ff.EnabledForService("service-a")
		require.Error(t, err)
		require.False(t, enabled)
	})
}
