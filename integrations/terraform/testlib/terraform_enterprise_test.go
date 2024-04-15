package testlib

import (
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/stretchr/testify/suite"
	"testing"
)

func TestTerraformOSS(t *testing.T) {
	suite.Run(t, &TerraformSuiteOSS{
		TerraformBaseSuite: TerraformBaseSuite{
			AuthHelper: &integration.MinimalAuthHelper{},
		},
	})
}

func TestTerraformOSSWithCache(t *testing.T) {
	suite.Run(t, &TerraformSuiteOSSWithCache{
		TerraformBaseSuite: TerraformBaseSuite{
			AuthHelper: &integration.MinimalAuthHelper{
				AuthConfig: auth.TestAuthServerConfig{CacheEnabled: true},
			},
		},
	})
}
