package common

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// GetAppMatchers returns a list of matchers to be used when accessing the
// application.
func GetAppMatchers(app types.Application, identity tlsca.Identity) []services.RoleMatcher {
	// When accessing AWS management console, check permissions to assume
	// requested IAM role as well.
	var matchers []services.RoleMatcher
	if app.IsAWSConsole() {
		matchers = append(matchers, &services.AWSRoleARNMatcher{
			RoleARN: identity.RouteToApp.AWSRoleARN,
		})
	}
	// When accessing Azure API, check permissions to assume
	// requested Azure identity as well.
	if app.IsAzureCloud() {
		matchers = append(matchers, &services.AzureIdentityMatcher{
			Identity: identity.RouteToApp.AzureIdentity,
		})
	}
	// When accessing GCP API, check permissions to assume
	// requested GCP service account as well.
	if app.IsGCP() {
		matchers = append(matchers, &services.GCPServiceAccountMatcher{
			ServiceAccount: identity.RouteToApp.GCPServiceAccount,
		})
	}
	return matchers
}