package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	tagv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/tag/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
)

func TestValidateTag(t *testing.T) {
	tests := []struct {
		name    string
		input   *tagv1.Tag
		wantErr bool
	}{
		{
			name:    "nil resource",
			input:   nil,
			wantErr: true,
		},
		{
			name:    "nil metadata",
			input:   &tagv1.Tag{},
			wantErr: true,
		},
		{
			name:    "empty name",
			input: &tagv1.Tag{
				Metadata: &headerv1.Metadata{Name: ""},
			},
			wantErr: true,
		},
		{
			name:    "nil spec",
			input: &tagv1.Tag{
				Metadata: &headerv1.Metadata{Name: "test"},
			},
			wantErr: true,
		},
		{
			name: "valid minimal",
			input: &tagv1.Tag{
				Metadata: &headerv1.Metadata{Name: "test"},
				Spec:     &tagv1.TagSpec{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTag(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
