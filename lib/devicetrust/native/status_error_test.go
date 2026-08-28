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
