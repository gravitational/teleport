/*
Copyright 2016 Gravitational, Inc.

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

package trail

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TestConversion makes sure we convert all trace supported errors
// to and back from GRPC codes
func TestConversion(t *testing.T) {
	tests := []struct {
		name string
		err  error
		fn   func(error) bool
	}{
		{
			name: "io.EOF",
			err:  io.EOF,
			fn:   func(err error) bool { return errors.Is(err, io.EOF) },
		},
		{
			name: "os.ErrNotExist",
			err:  os.ErrNotExist,
			fn:   trace.IsNotFound,
		},
		{
			name: "AccessDenied",
			err:  trace.AccessDenied("access denied"),
			fn:   trace.IsAccessDenied,
		},
		{
			name: "AlreadyExists",
			err:  trace.AlreadyExists("already exists"),
			fn:   trace.IsAlreadyExists,
		},
		{
			name: "BadParameter",
			err:  trace.BadParameter("bad parameter"),
			fn:   trace.IsBadParameter,
		},
		{
			name: "CompareFailed",
			err:  trace.CompareFailed("compare failed"),
			fn:   trace.IsCompareFailed,
		},
		{
			name: "ConnectionProblem",
			err:  trace.ConnectionProblem(nil, "problem"),
			fn:   trace.IsConnectionProblem,
		},
		{
			name: "LimitExceeded",
			err:  trace.LimitExceeded("exceeded"),
			fn:   trace.IsLimitExceeded,
		},
		{
			name: "NotFound",
			err:  trace.NotFound("not found"),
			fn:   trace.IsNotFound,
		},
		{
			name: "NotImplemented",
			err:  trace.NotImplemented("not implemented"),
			fn:   trace.IsNotImplemented,
		},
		{
			name: "Aggregated BadParameter",
			err:  trace.NewAggregate(trace.BadParameter("bad parameter")),
			fn:   trace.IsBadParameter,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			grpcError := ToGRPC(test.err)
			assert.Equal(t, test.err.Error(), status.Convert(grpcError).Message(), "Error message mismatch")

			out := FromGRPC(grpcError)
			assert.True(t, test.fn(out), "Predicate failed")
			assert.Regexp(t, ".*trail_test.go.*", line(trace.DebugReport(out)))
			assert.NotRegexp(t, ".*trail.go.*", line(trace.DebugReport(out)))
		})
	}
}

// TestNil makes sure conversions of nil to and from GRPC are no-op
func TestNil(t *testing.T) {
	out := FromGRPC(ToGRPC(nil))
	assert.NoError(t, out)
}

// TestFromEOF makes sure that non-grpc error such as io.EOF is preserved well.
func TestFromEOF(t *testing.T) {
	out := FromGRPC(trace.Wrap(io.EOF))
	assert.True(t, trace.IsEOF(out))
}

// TestTraces makes sure we pass traces via metadata and can decode it back
func TestTraces(t *testing.T) {
	err := trace.BadParameter("param")
	meta := metadata.New(nil)
	setDebugInfo(err, meta)
	err2 := FromGRPC(ToGRPC(err), meta)
	assert.Regexp(t, ".*trail_test.go.*", line(trace.DebugReport(err)))
	assert.Regexp(t, ".*trail_test.go.*", line(trace.DebugReport(err2)))
}

func line(s string) string {
	return strings.ReplaceAll(s, "\n", "")
}

func TestToGRPCKeepCode(t *testing.T) {
	err := status.Errorf(codes.PermissionDenied, "denied")
	err = ToGRPC(err)
	if code := status.Code(err); code != codes.PermissionDenied {
		t.Errorf("after ToGRPC, got error code %v, want %v, error: %v", code, codes.PermissionDenied, err)
	}
	err = FromGRPC(err)
	if !trace.IsAccessDenied(err) {
		t.Errorf("after FromGRPC, trace.IsAccessDenied is false, want true, error: %v", err)
	}
}

func TestToGRPC_statusError(t *testing.T) {
	err1 := status.Errorf(codes.NotFound, "not found")
	err2 := fmt.Errorf("go wrap: %w", trace.Wrap(err1))

	tests := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "unwrapped status",
			err:  err1,
			want: err1, // Exact same error.
		},
		{
			name: "wrapped status",
			err:  err2,
			want: status.Errorf(codes.NotFound, "%s", err2.Error()),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ToGRPC(test.err)

			got, ok := status.FromError(err)
			if !ok {
				t.Fatalf("Failed to convert `got` to a status.Status: %#v", err)
			}
			want, ok := status.FromError(test.want)
			if !ok {
				t.Fatalf("Failed to convert `want` to a status.Status: %#v", err)
			}

			if got.Code() != want.Code() || got.Message() != want.Message() {
				t.Errorf("ToGRPC = %#v, want %#v", got, test.want)
			}
		})
	}
}
