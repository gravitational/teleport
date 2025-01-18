package integrationv1

import (
	"context"
	"fmt"
	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
)

func (s *Service) GenerateAzureOIDCToken(ctx context.Context, req *integrationpb.GenerateAzureOIDCTokenRequest) (*integrationpb.GenerateAzureOIDCTokenResponse, error) {
	fmt.Printf("============= GENERATING AZURE TOKEN?!\n")
	return nil, nil
}
