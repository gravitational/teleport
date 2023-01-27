// Copyright 2022 Gravitational, Inc
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

package devicetrust_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport/lib/devicetrust"
)

func TestHandleUnimplemented(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantChanged bool
	}{
		{
			name: "unrelated err",
			err:  errors.New("something bad"),
		},
		{
			name: "trace.NotImplemented",
			err:  trace.NotImplemented("not implemented"), // ignored, we want a gRPC not implemented error.
		},
		{
			name:        "status with codes.Unimplemented",
			err:         status.Errorf(codes.Unimplemented, "method not implemented"),
			wantChanged: true,
		},
		{
			name:        "wrapped status with codes.Unimplemented",
			err:         trace.Wrap(fmt.Errorf("wrapped: %w", status.Errorf(codes.Unimplemented, "method not implemented"))),
			wantChanged: true,
		},
		{
			name: "unrelated status error",
			err:  status.Errorf(codes.InvalidArgument, "llamas required"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := devicetrust.HandleUnimplemented(test.err)
			if test.wantChanged {
				assert.ErrorContains(t, got, "device trust not supported")
			} else {
				assert.Equal(t, test.err, got,
					"HandleUnimplemented returned a modified error, wanted the same as the original")
			}
		})
	}
}
