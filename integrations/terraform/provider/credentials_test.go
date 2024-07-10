package provider

import (
	"context"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestActiveSources(t *testing.T) {
	ctx := context.Background()

	activeSource1 := fakeActiveCredentialsSource{"active1"}
	activeSource2 := fakeActiveCredentialsSource{"active2"}
	inactiveSource1 := fakeInactiveCredentialsSource{"inactive1"}
	inactiveSource2 := fakeInactiveCredentialsSource{"inactive2"}

	tests := []struct {
		name            string
		sources         CredentialSources
		expectedSources CredentialSources
		wantErr         bool
	}{
		{
			name:            "no source",
			sources:         CredentialSources{},
			expectedSources: nil,
			wantErr:         true,
		},
		{
			name: "no active source",
			sources: CredentialSources{
				inactiveSource1,
				inactiveSource2,
			},
			expectedSources: nil,
			wantErr:         true,
		},
		{
			name: "single active source",
			sources: CredentialSources{
				activeSource1,
			},
			expectedSources: CredentialSources{activeSource1},
			wantErr:         false,
		},
		{
			name: "multiple active and inactive sources",
			sources: CredentialSources{
				inactiveSource1,
				activeSource1,
				inactiveSource2,
				activeSource2,
			},
			expectedSources: CredentialSources{activeSource1, activeSource2},
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, diags := tt.sources.ActiveSources(ctx, providerData{})
			require.Equal(t, tt.wantErr, diags.HasError())
			require.Equal(t, tt.expectedSources, result)
		})
	}
}

type fakeActiveCredentialsSource struct {
	name string
}

func (f fakeActiveCredentialsSource) Name() string {
	return f.name
}

func (f fakeActiveCredentialsSource) IsActive(data providerData) (bool, string) {
	return true, ""
}

func (f fakeActiveCredentialsSource) Credentials(ctx context.Context, data providerData) (client.Credentials, error) {
	return nil, trace.NotImplemented("not implemented")
}

type fakeInactiveCredentialsSource struct {
	name string
}

func (f fakeInactiveCredentialsSource) Name() string {
	return f.name
}

func (f fakeInactiveCredentialsSource) IsActive(data providerData) (bool, string) {
	return false, ""
}

func (f fakeInactiveCredentialsSource) Credentials(ctx context.Context, data providerData) (client.Credentials, error) {
	return nil, trace.NotImplemented("not implemented")
}
