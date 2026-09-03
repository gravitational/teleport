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
	"time"

	"github.com/gravitational/trace"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/utils"
)

const mcpOAuthMutationLockTimeout = 30 * time.Second

// Bindings keep credentials from following an app name to a replacement
// upstream. Registration metadata lets reauthorization reuse the same client.
// Profile permissions protect these secrets from other OS users, not other
// processes running as the same user.
type mcpOAuthCredentials struct {
	ResourceURI  string                   `json:"resource_uri"`
	Issuer       string                   `json:"issuer"`
	ClientID     string                   `json:"client_id"`
	ClientSecret string                   `json:"client_secret,omitempty"`
	RedirectURI  string                   `json:"redirect_uri,omitempty"`
	Scopes       []string                 `json:"scopes,omitempty"`
	Token        mcpclienttransport.Token `json:"token"`
}

func (c *mcpOAuthCredentials) checkResource(resourceURI string) error {
	if c.ResourceURI == "" || c.Issuer == "" {
		return trace.BadParameter("stored MCP OAuth credentials are missing their resource or issuer binding")
	}
	if c.ResourceURI != resourceURI {
		return trace.BadParameter("stored MCP OAuth credentials are bound to a different resource")
	}
	return nil
}

func (c *mcpOAuthCredentials) checkBinding(resourceURI, issuer string) error {
	if err := c.checkResource(resourceURI); err != nil {
		return err
	}
	if c.Issuer != issuer {
		return trace.BadParameter("stored MCP OAuth credentials are bound to a different issuer")
	}
	return nil
}

func mcpOAuthTokenPath(homePath, proxyHost, username, cluster string, appSQN scopes.QualifiedName) (string, error) {
	if err := scopes.StrongValidateResourceName(appSQN.Name); err != nil {
		return "", trace.Wrap(err, "invalid MCP server name")
	}
	if appSQN.Scope != "" {
		if err := scopes.StrongValidate(appSQN.Scope); err != nil {
			return "", trace.Wrap(err, "invalid MCP server scope")
		}
	}
	return keypaths.MCPOAuthCredentialsPath(
		profile.FullProfilePath(homePath), proxyHost, username, cluster, client.ScopedAppName(appSQN),
	), nil
}

func saveMCPOAuthCredentials(path string, creds *mcpOAuthCredentials) error {
	if creds.ResourceURI == "" || creds.Issuer == "" {
		return trace.BadParameter("MCP OAuth credentials are missing their resource or issuer binding")
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return trace.Wrap(err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return trace.ConvertSystemError(err)
	}
	return trace.ConvertSystemError(writeMCPOAuthCredentialsFile(path, data))
}

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

type fileTokenStore struct {
	path             string
	mutationLockPath string
	resourceURI      string
	issuer           string
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
	if err := creds.checkBinding(s.resourceURI, s.issuer); err != nil {
		return nil, trace.Wrap(err)
	}
	return &creds.Token, nil
}

func (s *fileTokenStore) SaveToken(ctx context.Context, token *mcpclienttransport.Token) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return withMCPOAuthMutationLock(ctx, s.mutationLockPath, func() error {
		creds, err := loadMCPOAuthCredentials(s.path)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := creds.checkBinding(s.resourceURI, s.issuer); err != nil {
			return trace.Wrap(err)
		}
		creds.Token = *token
		return trace.Wrap(saveMCPOAuthCredentials(s.path, creds))
	})
}

func withMCPOAuthMutationLock(ctx context.Context, path string, fn func() error) error {
	if path == "" {
		return trace.BadParameter("missing MCP OAuth mutation lock path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return trace.ConvertSystemError(err)
	}
	unlock, err := utils.FSTryWriteLockTimeout(ctx, path, mcpOAuthMutationLockTimeout)
	if err != nil {
		return trace.Wrap(err, "waiting to store MCP OAuth credentials")
	}
	defer unlock()
	return trace.Wrap(fn())
}
