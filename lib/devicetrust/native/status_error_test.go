// Copyright 2023 Gravitational, Inc
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

package native

import (
	"errors"
	"testing"

	"github.com/gravitational/teleport/lib/devicetrust"
)

func TestStatusError_Is(t *testing.T) {
	errNotFound := &statusError{status: errSecItemNotFound}
	errMissingEntitlement := &statusError{status: errSecMissingEntitlement}
	errOtherStatus := &statusError{status: -12345}

	tests := []struct {
		name   string
		err    *statusError
		target error
		want   bool
	}{
		{
			name:   "same statuses are equal",
			err:    errOtherStatus,
			target: &statusError{status: errOtherStatus.status}, // distinct instance
			want:   true,
		},
		{
			name:   "distinct statuses are not equal",
			err:    errNotFound,
			target: errMissingEntitlement,
			want:   false,
		},
		{
			name:   "errSecItemNotFound is the same as ErrDeviceKeyNotFound",
			err:    errNotFound,
			target: devicetrust.ErrDeviceKeyNotFound,
			want:   true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := errors.Is(test.err, test.target)
			if got != test.want {
				t.Errorf("errors.Is() = %v, want %v", got, test.want)
			}
		})
	}
}
