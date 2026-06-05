/*
Copyright 2026 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resource153

import (
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

func TestCopyFromTimestamp(t *testing.T) {
	t.Parallel()

	timestamp := timestamppb.New(time.Date(2026, time.June, 1, 12, 0, 0, 0, time.UTC))

	for _, tc := range []struct {
		name     string
		input    tfschema.TimeValue
		expected *timestamppb.Timestamp
	}{
		{
			name:     "null",
			input:    tfschema.TimeValue{Null: true},
			expected: nil,
		},
		{
			name:     "unknown",
			input:    tfschema.TimeValue{Unknown: true},
			expected: nil,
		},
		{
			name:     "non-nil",
			input:    tfschema.TimeValue{Value: timestamp.AsTime()},
			expected: timestamp,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diags := diag.Diagnostics{}
			output := timestamppb.Now()
			CopyFromTimestamp(diags, tc.input, &output)

			require.Empty(t, diags)
			require.Equal(t, tc.expected, output)
		})
	}
}

func TestCopyToTimestamp(t *testing.T) {
	t.Parallel()

	timestamp := timestamppb.New(time.Date(2026, time.June, 1, 12, 0, 0, 0, time.UTC))

	for _, tc := range []struct {
		name     string
		input    *timestamppb.Timestamp
		expected tfschema.TimeValue
	}{
		{
			name:  "null",
			input: nil,
			expected: tfschema.TimeValue{
				Null:   true,
				Format: time.RFC3339,
			},
		},
		{
			name:  "non-nil",
			input: timestamp,
			expected: tfschema.TimeValue{
				Value:  timestamp.AsTime(),
				Format: time.RFC3339,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diags := diag.Diagnostics{}
			valueInitial := tfschema.TimeValue{
				Unknown: true,
				Format:  time.RFC3339,
			}

			value := CopyToTimestamp(diags, tc.input, tfschema.UseRFC3339Time(), valueInitial)
			require.Empty(t, diags)
			require.Equal(t, tc.expected, value)
		})
	}
}

func TestCopyFromDuration(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		input    tfschema.DurationValue
		expected *durationpb.Duration
	}{
		{
			name:     "null",
			input:    tfschema.DurationValue{Null: true},
			expected: nil,
		},
		{
			name:     "unknown",
			input:    tfschema.DurationValue{Unknown: true},
			expected: nil,
		},
		{
			name: "non-nil",
			input: tfschema.DurationValue{
				Value: time.Minute,
			},
			expected: durationpb.New(time.Minute),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diags := diag.Diagnostics{}
			output := durationpb.New(time.Hour)
			CopyFromDuration(diags, tc.input, &output)

			require.Empty(t, diags)
			require.Equal(t, tc.expected, output)
		})
	}
}

func TestCopyToDuration(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		input    *durationpb.Duration
		expected tfschema.DurationValue
	}{
		{
			name:  "null",
			input: nil,
			expected: tfschema.DurationValue{
				Null: true,
			},
		},
		{
			name:  "non-nil",
			input: durationpb.New(time.Minute),
			expected: tfschema.DurationValue{
				Value: time.Minute,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diags := diag.Diagnostics{}
			initialValue := tfschema.DurationValue{
				Unknown: true,
			}

			value := CopyToDuration(diags, tc.input, tfschema.DurationType{}, initialValue)
			require.Empty(t, diags)
			require.Equal(t, tc.expected, value)
		})
	}
}
