/*
Copyright 2022 Gravitational, Inc.

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

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSSODiagnosticInfo_GetValue(t *testing.T) {
	mkInfo := func(value interface{}) *SSODiagnosticInfo {
		info, err := NewSSODiagnosticInfo(SSOInfoType_UNKNOWN, value)
		require.NoError(t, err)
		return info
	}

	ptrStr := func(v string) *string { return &v }
	ptrInt := func(v int) *int { return &v }

	type errInfo struct {
		Message string
		Error   string
	}

	tests := []struct {
		name    string
		info    *SSODiagnosticInfo
		arg     interface{}
		want    interface{}
		wantErr bool
	}{
		{
			name:    "string ok",
			info:    mkInfo("foo"),
			arg:     new(string),
			want:    ptrStr("foo"),
			wantErr: false,
		},
		{
			name:    "int ok",
			info:    mkInfo(123),
			arg:     new(int),
			want:    ptrInt(123),
			wantErr: false,
		},
		{
			name:    "bad pointer type",
			info:    mkInfo(123),
			arg:     new(string),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "custom struct",
			info:    mkInfo(errInfo{Message: "oh!", Error: "err"}),
			arg:     new(errInfo),
			want:    &errInfo{Message: "oh!", Error: "err"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.info.GetValue(tt.arg)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if err == nil {
					require.Equal(t, tt.want, tt.arg)
				}
			}
		})
	}
}
