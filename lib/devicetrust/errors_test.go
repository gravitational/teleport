/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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
