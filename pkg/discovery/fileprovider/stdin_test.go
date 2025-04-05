package fileprovider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Semior001/groxy/pkg/discovery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStdin_Events(t *testing.T) {
	stdin := &Stdin{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := stdin.Events(ctx)

	// Should receive one event immediately
	event := <-events
	assert.Equal(t, "stdin", event)

	// Cancel context to close channel
	cancel()

	// Channel should be closed
	_, ok := <-events
	assert.False(t, ok, "events channel should be closed")
}

func TestStdin_State(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr error
		want    *discovery.State
	}{
		{
			name: "valid config",
			config: `
version: "1"
upstreams:
  test-service:
    address: localhost:8081
    tls: false
    serve-reflection: true
rules:
  - match:
      uri: "/test.Service/Method"
    forward:
      upstream: test-service
`,
			wantErr: nil,
			want: &discovery.State{
				Name: "stdin",
				Rules: []*discovery.Rule{
					{
						Name: "/test.Service/Method",
					},
				},
			},
		},
		{
			name:    "empty config",
			config:  "",
			wantErr: fmt.Errorf("decode stdin"),
		},
		{
			name: "unsupported version",
			config: `
version: "2"
rules: []
`,
			wantErr: fmt.Errorf("unsupported version"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdin := &Stdin{
				Reader: strings.NewReader(tt.config),
			}

			state, err := stdin.State(context.Background())
			
			if tt.wantErr != nil {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr.Error())
				return
			}
			
			require.NoError(t, err)
			require.Equal(t, tt.want.Name, state.Name)
			
			if tt.want.Rules != nil {
				require.Len(t, state.Rules, len(tt.want.Rules))
				for i, r := range tt.want.Rules {
					require.Equal(t, r.Name, state.Rules[i].Name)
				}
			}
		})
	}
}