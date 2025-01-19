package integrationv1

import (
	"context"
	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/lib/integrations/azureoidc"
	"github.com/gravitational/trace"
)

func (s *Service) GenerateAzureOIDCToken(ctx context.Context, req *integrationpb.GenerateAzureOIDCTokenRequest) (*integrationpb.GenerateAzureOIDCTokenResponse, error) {
	_, err := s.cache.GetIntegration(ctx, req.Integration)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := azureoidc.GenerateEntraOIDCToken(ctx, s.cache, s.keyStoreManager, s.clock)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &integrationpb.GenerateAzureOIDCTokenResponse{Token: token}, nil
}
