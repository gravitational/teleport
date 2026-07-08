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

package common

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"

	"github.com/gravitational/teleport/api/profile"
)

// mcpOAuthCredentials is everything tsh needs to authorize requests to an
// OAuth-protected MCP server: the token itself plus the client ID it was
// issued to, which is required to refresh the token later.
type mcpOAuthCredentials struct {
	ClientID string                   `json:"client_id"`
	Token    mcpclienttransport.Token `json:"token"`
}

// mcpOAuthTokenPath returns where the OAuth credentials for the
// (proxy, cluster, app) combination are stored, e.g.
// ~/.tsh/mcp_tokens/example.com/mycluster/linear.json.
func mcpOAuthTokenPath(homePath, proxyHost, cluster, appName string) string {
	return filepath.Join(profile.FullProfilePath(homePath), "mcp_tokens", proxyHost, cluster, appName+".json")
}

// saveMCPOAuthCredentials writes the credentials with owner-only permissions.
// TODO(mcp-oauth): atomic write + file locking, for concurrent tsh processes
// sharing one rotating refresh token.
func saveMCPOAuthCredentials(path string, creds *mcpOAuthCredentials) error {
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return trace.Wrap(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return trace.ConvertSystemError(err)
	}
	return trace.ConvertSystemError(os.WriteFile(path, data, 0o600))
}

// loadMCPOAuthCredentials returns trace.NotFound when no credentials are
// stored at the path.
func loadMCPOAuthCredentials(path string) (*mcpOAuthCredentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	var creds mcpOAuthCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, trace.Wrap(err)
	}
	return &creds, nil
}

// fileTokenStore exposes the credentials file through mcp-go's TokenStore
// interface, so that its OAuthHandler reads the current token on every
// request and writes refreshed tokens back to the same file.
type fileTokenStore struct {
	path     string
	clientID string
}

func (s *fileTokenStore) GetToken(ctx context.Context) (*mcpclienttransport.Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	creds, err := loadMCPOAuthCredentials(s.path)
	if trace.IsNotFound(err) {
		return nil, mcpclienttransport.ErrNoToken
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &creds.Token, nil
}

func (s *fileTokenStore) SaveToken(ctx context.Context, token *mcpclienttransport.Token) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return trace.Wrap(saveMCPOAuthCredentials(s.path, &mcpOAuthCredentials{
		ClientID: s.clientID,
		Token:    *token,
	}))
}
