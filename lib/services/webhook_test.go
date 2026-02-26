package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	webhookv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/webhook/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
)

func TestValidateWebhook(t *testing.T) {
	tests := []struct {
		name    string
		input   *webhookv1.Webhook
		wantErr bool
	}{
		{
			name:    "nil resource",
			input:   nil,
			wantErr: true,
		},
		{
			name:    "nil metadata",
			input:   &webhookv1.Webhook{},
			wantErr: true,
		},
		{
			name:    "empty name",
			input: &webhookv1.Webhook{
				Metadata: &headerv1.Metadata{Name: ""},
			},
			wantErr: true,
		},
		{
			name:    "nil spec",
			input: &webhookv1.Webhook{
				Metadata: &headerv1.Metadata{Name: "test"},
			},
			wantErr: true,
		},
		{
			name: "valid minimal",
			input: &webhookv1.Webhook{
				Metadata: &headerv1.Metadata{Name: "test"},
				Spec:     &webhookv1.WebhookSpec{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWebhook(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
