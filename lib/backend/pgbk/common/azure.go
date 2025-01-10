/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package pgcommon

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
)

// AzureBeforeConnect will return a pgx BeforeConnect function suitable for
// Azure AD authentication. The returned function will set the password of the
// connection to a token for the relevant scope.
func AzureBeforeConnect(ctx context.Context, logger *slog.Logger) (func(ctx context.Context, config *pgx.ConnConfig) error, error) {
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

		logger.DebugContext(ctx, "Acquired Azure access token.", "ttl", time.Until(token.ExpiresOn).String())
		config.Password = token.Token

		return nil
	}

	return beforeConnect, nil
}
