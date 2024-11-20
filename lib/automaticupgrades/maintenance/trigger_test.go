package maintenance

import (
	"context"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"testing"
)

// checkTraceError is a test helper that converts trace.IsXXXError into a require.ErrorAssertionFunc
func checkTraceError(check func(error) bool) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, i ...interface{}) {
		require.True(t, check(err), i...)
	}
}

func TestFailoverTrigger_CanStart(t *testing.T) {
	t.Parallel()

	// Test setup
	ctx := context.Background()
	tests := []struct {
		name         string
		triggers     []Trigger
		expectResult bool
		expectErr    require.ErrorAssertionFunc
	}{
		{
			name:         "nil",
			triggers:     nil,
			expectResult: false,
			expectErr:    checkTraceError(trace.IsNotFound),
		},
		{
			name:         "empty",
			triggers:     []Trigger{},
			expectResult: false,
			expectErr:    checkTraceError(trace.IsNotFound),
		},
		{
			name: "first trigger success firing",
			triggers: []Trigger{
				StaticTrigger{canStart: true},
				StaticTrigger{canStart: false},
			},
			expectResult: true,
			expectErr:    require.NoError,
		},
		{
			name: "first trigger success not firing",
			triggers: []Trigger{
				StaticTrigger{canStart: false},
				StaticTrigger{canStart: true},
			},
			expectResult: false,
			expectErr:    require.NoError,
		},
		{
			name: "first trigger failure",
			triggers: []Trigger{
				StaticTrigger{err: trace.LimitExceeded("got rate-limited")},
				StaticTrigger{canStart: true},
			},
			expectResult: false,
			expectErr:    checkTraceError(trace.IsLimitExceeded),
		},
		{
			name: "first trigger skipped, second getter success",
			triggers: []Trigger{
				StaticTrigger{err: trace.NotImplemented("proxy does not seem to implement RFD-184")},
				StaticTrigger{canStart: true},
			},
			expectResult: true,
			expectErr:    require.NoError,
		},
		{
			name: "first trigger skipped, second getter failure",
			triggers: []Trigger{
				StaticTrigger{err: trace.NotImplemented("proxy does not seem to implement RFD-184")},
				StaticTrigger{err: trace.LimitExceeded("got rate-limited")},
			},
			expectResult: false,
			expectErr:    checkTraceError(trace.IsLimitExceeded),
		},
		{
			name: "first trigger skipped, second getter skipped",
			triggers: []Trigger{
				StaticTrigger{err: trace.NotImplemented("proxy does not seem to implement RFD-184")},
				StaticTrigger{err: trace.NotImplemented("proxy does not seem to implement RFD-184")},
			},
			expectResult: false,
			expectErr:    checkTraceError(trace.IsNotFound),
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				// Test execution
				trigger := FailoverTrigger(tt.triggers)
				result, err := trigger.CanStart(ctx, nil)
				require.Equal(t, tt.expectResult, result)
				tt.expectErr(t, err)
			},
		)
	}
}

func TestFailoverTrigger_Name(t *testing.T) {
	tests := []struct {
		name         string
		triggers     []Trigger
		expectResult string
	}{
		{
			name:         "nil",
			triggers:     nil,
			expectResult: "",
		},
		{
			name:         "empty",
			triggers:     []Trigger{},
			expectResult: "",
		},
		{
			name: "one trigger",
			triggers: []Trigger{
				StaticTrigger{name: "proxy"},
			},
			expectResult: "proxy",
		},
		{
			name: "two triggers",
			triggers: []Trigger{
				StaticTrigger{name: "proxy"},
				StaticTrigger{name: "version-server"},
			},
			expectResult: "proxy, failover version-server",
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				// Test execution
				trigger := FailoverTrigger(tt.triggers)
				result := trigger.Name()
				require.Equal(t, tt.expectResult, result)
			},
		)
	}
}
