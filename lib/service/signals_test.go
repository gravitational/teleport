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

package service

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func Test_getShutdownTimeout(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     time.Duration
	}{
		{
			name:     "no override",
			envValue: "",
			want:     defaultShutdownTimeout,
		},
		{
			name:     "accept valid override, one second",
			envValue: "1s",
			want:     time.Second * 1,
		},
		{
			name:     "accept valid override, one minute",
			envValue: "1m",
			want:     time.Minute * 1,
		},
		{
			name:     "ignore invalid override",
			envValue: "one moment",
			want:     defaultShutdownTimeout,
		},
		{
			name:     "valid override above maximum, trim",
			envValue: "3000h",
			want:     maxShutdownTimeout,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TELEPORT_UNSTABLE_SHUTDOWN_TIMEOUT", tt.envValue)
			require.Equal(t, tt.want, getShutdownTimeout(logrus.StandardLogger()))
		})
	}
}
