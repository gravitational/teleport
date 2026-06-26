/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package credentials

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gravitational/trace"
)

// RevokeGitHubTokenGrant revokes a GitHub App's grant for a user by calling
// DELETE /applications/{client_id}/grant with the access token.
// https://docs.github.com/en/rest/apps/oauth-applications#delete-an-app-authorization
func RevokeGitHubTokenGrant(ctx context.Context, clientID, clientSecret, accessToken string) error {
	body := fmt.Sprintf(`{"access_token":%q}`, accessToken)
	url := fmt.Sprintf("https://api.github.com/applications/%s/grant", clientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, strings.NewReader(body))
	if err != nil {
		return trace.Wrap(err)
	}
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	return trace.Errorf("GitHub token revocation returned %d", resp.StatusCode)
}
