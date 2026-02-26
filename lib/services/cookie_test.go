package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	cookiev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/cookie/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
)

func TestValidateCookie(t *testing.T) {
	tests := []struct {
		name    string
		input   *cookiev1.Cookie
		wantErr bool
	}{
		{
			name:    "nil resource",
			input:   nil,
			wantErr: true,
		},
		{
			name:    "nil metadata",
			input:   &cookiev1.Cookie{},
			wantErr: true,
		},
		{
			name:    "empty name",
			input: &cookiev1.Cookie{
				Metadata: &headerv1.Metadata{Name: ""},
			},
			wantErr: true,
		},
		{
			name:    "nil spec",
			input: &cookiev1.Cookie{
				Metadata: &headerv1.Metadata{Name: "test"},
			},
			wantErr: true,
		},
		{
			name: "valid minimal",
			input: &cookiev1.Cookie{
				Metadata: &headerv1.Metadata{Name: "test"},
				Spec:     &cookiev1.CookieSpec{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCookie(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
