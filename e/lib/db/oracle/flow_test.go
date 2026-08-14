package oracle

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseRedirectAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHost string
		wantPort int
		wantErr  string
	}{
		{
			name:     "valid TCPS address",
			input:    "(ADDRESS=(PROTOCOL=TCPS)(HOST=10.0.0.52)(PORT=2452))",
			wantHost: "10.0.0.52",
			wantPort: 2452,
		},
		{
			name:    "invalid protocol",
			input:   "(ADDRESS=(PROTOCOL=TCP)(HOST=10.0.0.52)(PORT=2452))",
			wantErr: `expected TCPS protocol, got "TCP"`,
		},
		{
			name:    "missing host",
			input:   "(ADDRESS=(PROTOCOL=TCPS)(PORT=2452))",
			wantErr: `redirect address "(ADDRESS=(PROTOCOL=TCPS)(PORT=2452))" is missing a host key`,
		},
		{
			name:    "empty host",
			input:   "(ADDRESS=(PROTOCOL=TCPS)(PORT=2452)(HOST=))",
			wantErr: "empty host value",
		},
		{
			name:    "invalid port",
			input:   "(ADDRESS=(PROTOCOL=TCPS)(HOST=10.0.0.52)(PORT=abc))",
			wantErr: `failed to parse port number: "abc"`,
		},
		{
			name:    "malformed input",
			input:   "invalid",
			wantErr: `failed to parse redirect address "invalid"`,
		},
		{
			name:     "wrapped in description",
			input:    "(DESCRIPTION=(ADDRESS=(PROTOCOL=TCPS)(HOST=10.158.140.180)(PORT=1523)))",
			wantHost: "10.158.140.180",
			wantPort: 1523,
		},
		{
			name:     "dummy extra node",
			input:    "(DUMMY=BAR)(DESCRIPTION=(ADDRESS=(PROTOCOL=TCPS)(HOST=10.158.140.180)(PORT=1523)))",
			wantHost: "10.158.140.180",
			wantPort: 1523,
		},
	}

	logger := slog.New(slog.DiscardHandler)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := parseRedirectAddress(context.Background(), logger, tt.input)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantHost, host)
				require.Equal(t, tt.wantPort, port)
			}
		})
	}
}
