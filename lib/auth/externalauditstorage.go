// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

// GenerateExternalAuditStorageOIDCToken generates a signed OIDC token for use by
// the External Audit Storage feature when authenticating to customer AWS accounts.
func (a *Server) GenerateExternalAuditStorageOIDCToken(ctx context.Context, integration string) (string, error) {
	token, err := awsoidc.GenerateAWSOIDCToken(ctx, a, a.GetKeyStore(), awsoidc.GenerateAWSOIDCTokenRequest{
		Integration: integration,
		Username:    a.ServerID,
		Subject:     types.IntegrationAWSOIDCSubjectAuth,
		Clock:       a.clock,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	a.AnonymizeAndSubmit(&usagereporter.ExternalAuditStorageAuthenticateEvent{})

	return token, nil
}
