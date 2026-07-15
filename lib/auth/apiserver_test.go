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

package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/auth"
)

// TestTokensRegisterReturnsTooOldError verifies that the re-added /tokens/register
// route returns an explicit "too old" error rather than a 404, so that outdated
// clients using the deprecated legacy join endpoint get an actionable message.
func TestTokensRegisterReturnsTooOldError(t *testing.T) {
	t.Parallel()

	handler, err := auth.NewAPIServer(&auth.APIConfig{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/tokens/register", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusForbidden, rr.Code)
	require.Contains(t, rr.Body.String(), "this client is too old to join the cluster")
}
