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

	type errInfo struct {
		Message string
		Error   string
	}

	tests := []struct {
		name    string
		info    *SSODiagnosticInfo
		want    interface{}
		wantErr bool
	}{
		{
			name:    "string",
			info:    mkInfo("foo"),
			want:    "foo",
			wantErr: false,
		},
		{
			name:    "int to float",
			info:    mkInfo(123),
			want:    123.0,
			wantErr: false,
		},
		{
			name:    "struct",
			info:    mkInfo(errInfo{Message: "oh!", Error: "no..."}),
			want:    map[string]interface{}{"Message": "oh!", "Error": "no..."},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.info.GetValue()
			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				if err == nil {
					require.Equal(t, tt.want, result)
				}
			}
		})
	}
}

func TestSSODiagnosticInfo_GetValueTyped(t *testing.T) {
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
			err := tt.info.GetValueTyped(tt.arg)
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
