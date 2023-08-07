// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pgcommon

import (
	"context"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"
)

// AzureBeforeConnect will return a pgx BeforeConnect function suitable for
// Azure AD authentication. The returned function will set the password of the
// connection to a token for the relevant scope.
func AzureBeforeConnect(log logrus.FieldLogger) (func(ctx context.Context, config *pgx.ConnConfig) error, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, trace.Wrap(err, "creating Azure credentials")
	}

	beforeConnect := func(ctx context.Context, config *pgx.ConnConfig) error {
		// the [azcore.TokenCredential] returned by the [azidentity] credential
		// functions handle caching and single-flighting for us
		token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
			Scopes: []string{"https://ossrdbms-aad.database.windows.net/.default"},
		})
		if err != nil {
			return trace.Wrap(err, "obtaining Azure authentication token")
		}

		log.WithField("ttl", time.Until(token.ExpiresOn).String()).Debug("Acquired Azure access token.")
		config.Password = token.Token

		return nil
	}

	return beforeConnect, nil
}
