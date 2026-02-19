// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package discovery

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCombineOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stdout string
		stderr string
		want   string
	}{
		{name: "both empty", stdout: "", stderr: "", want: ""},
		{name: "only stdout", stdout: "hello", stderr: "", want: "hello"},
		{name: "only stderr", stdout: "", stderr: "error!", want: "error!"},
		{name: "both present", stdout: "out", stderr: "err", want: "out\nerr"},
		{name: "whitespace trimmed", stdout: "  out  ", stderr: "  err  ", want: "out\nerr"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, combineOutput(tt.stdout, tt.stderr))
		})
	}
}

func TestShellQuoteArg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "simple", input: "hello", want: "hello"},
		{name: "empty", input: "", want: "''"},
		{name: "spaces", input: "hello world", want: "'hello world'"},
		{name: "single quote", input: "it's", want: "'it'\\''s'"},
		{name: "parens", input: "foo(bar)", want: "'foo(bar)'"},
		{name: "dollar sign", input: "$HOME", want: "'$HOME'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, shellQuoteArg(tt.input))
		})
	}
}

func TestHumanizeEnumValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "with prefix", input: "DISCOVERY_CONFIG_STATE_RUNNING", want: "Running"},
		{name: "multi word", input: "DISCOVERY_CONFIG_STATE_SYNCING_RESOURCES", want: "Syncing Resources"},
		{name: "no prefix", input: "ACTIVE", want: "Active"},
		{name: "empty", input: "", want: "Unknown"},
		{name: "just spaces", input: "   ", want: "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, humanizeEnumValue(tt.input))
		})
	}
}

func TestFormatDurationParts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		d        time.Duration
		detailed bool
		want     string
	}{
		{"30 seconds brief", 30 * time.Second, false, "30s"},
		{"5 minutes brief", 5 * time.Minute, false, "5m"},
		{"2 hours brief", 2 * time.Hour, false, "2h"},
		{"3 days brief", 3 * 24 * time.Hour, false, "3d"},
		{"2h30m detailed", 2*time.Hour + 30*time.Minute, true, "2h 30m"},
		{"1d 5h detailed", 29 * time.Hour, true, "1d 5h"},
		{"sub-second brief", 500 * time.Millisecond, false, "1s"},
		{"negative duration", -2 * time.Hour, false, "2h"},
		{"zero duration brief", 0, false, "1s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDurationParts(tt.d, tt.detailed)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestFormatRelativeDelta(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		ts       time.Time
		detailed bool
		want     string
	}{
		{name: "zero time", ts: time.Time{}, detailed: false, want: "never"},
		{name: "seconds ago", ts: now.Add(-30 * time.Second), detailed: false, want: "30s ago"},
		{name: "minutes ago", ts: now.Add(-5 * time.Minute), detailed: false, want: "5m ago"},
		{name: "hours ago", ts: now.Add(-3 * time.Hour), detailed: false, want: "3h ago"},
		{name: "days ago", ts: now.Add(-2 * 24 * time.Hour), detailed: false, want: "2d ago"},
		{name: "future", ts: now.Add(10 * time.Minute), detailed: false, want: "10m from now"},
		{name: "detailed hours+minutes", ts: now.Add(-3*time.Hour - 15*time.Minute), detailed: true, want: "3h 15m ago"},
		{name: "detailed days+hours", ts: now.Add(-2*24*time.Hour - 5*time.Hour), detailed: true, want: "2d 5h ago"},
		{name: "detailed days only", ts: now.Add(-2 * 24 * time.Hour), detailed: true, want: "2d ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, formatRelativeDelta(tt.ts, now, tt.detailed))
		})
	}
}

func TestParseDurationWithDays(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "hours", input: "2h", want: 2 * time.Hour},
		{name: "minutes", input: "30m", want: 30 * time.Minute},
		{name: "days", input: "7d", want: 7 * 24 * time.Hour},
		{name: "days and hours", input: "2d12h", want: 60 * time.Hour},
		{name: "one day", input: "1d", want: 24 * time.Hour},
		{name: "invalid", input: "abc", wantErr: true},
		{name: "invalid day count", input: "xd", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDurationWithDays(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestEstimateRequiredLimit(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC)

	t.Run("no expansion needed", func(t *testing.T) {
		// Requested range fully covered by fetched data.
		from := now.Add(-1 * time.Hour)
		oldest := now.Add(-2 * time.Hour)
		require.Equal(t, 0, estimateRequiredLimit(1000, oldest, now, from))
	})

	t.Run("expansion needed", func(t *testing.T) {
		// Fetched covers 1h, requested covers 2h → ~2x expansion.
		from := now.Add(-2 * time.Hour)
		oldest := now.Add(-1 * time.Hour)
		result := estimateRequiredLimit(1000, oldest, now, from)
		require.Greater(t, result, 2000)
	})

	t.Run("zero covered", func(t *testing.T) {
		require.Equal(t, 0, estimateRequiredLimit(1000, now, now, now.Add(-1*time.Hour)))
	})
}

func TestFormatExpiryTime(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		ts   time.Time
		want string
	}{
		{name: "zero time", ts: time.Time{}, want: "never"},
		{name: "future", ts: now.Add(2 * time.Hour), want: "in 2h"},
		{name: "past", ts: now.Add(-30 * time.Minute), want: "expired 30m ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, formatExpiryTime(tt.ts, now))
		})
	}
}

func TestFormatCountLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		count    int
		singular string
		plural   string
		want     string
	}{
		{name: "singular", count: 1, singular: "task", plural: "tasks", want: "1 task"},
		{name: "plural zero", count: 0, singular: "task", plural: "tasks", want: "0 tasks"},
		{name: "plural many", count: 5, singular: "task", plural: "tasks", want: "5 tasks"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, formatCountLabel(tt.count, tt.singular, tt.plural))
		})
	}
}

func TestFormatHelpText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty string", input: "", want: ""},
		{name: "whitespace only", input: "   \n  ", want: ""},
		{name: "plain text unchanged", input: "This is plain text.", want: "This is plain text."},
		{
			name:  "markdown link converted",
			input: "See [docs](https://example.com/docs) for details.",
			want:  "See docs: https://example.com/docs for details.",
		},
		{
			name:  "inline code backticks removed",
			input: "Run `tctl discovery status` to check.",
			want:  "Run tctl discovery status to check.",
		},
		{
			name:  "bold line becomes uppercase heading",
			input: "**Important Note**",
			want:  "IMPORTANT NOTE:",
		},
		{
			name:  "inline bold not on its own line",
			input: "This is **bold** text.",
			want:  "This is bold text.",
		},
		{
			name:  "multiline with blank lines preserved",
			input: "Line one.\n\nLine three.",
			want:  "Line one.\n\nLine three.",
		},
		{
			name:  "combined markdown features",
			input: "**Usage**\nRun `tctl` with [flag docs](https://example.com).",
			want:  "USAGE:\nRun tctl with flag docs: https://example.com.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, formatHelpText(tt.input))
		})
	}
}
