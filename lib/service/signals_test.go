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
