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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	testMCPOAuthResourceURI = "mcp+https://mcp.example.com/mcp"
	testMCPOAuthIssuer      = "https://auth.example.com"
)

// Keep dependency upgrades from silently changing persisted credential JSON.
type persistedMCPToken = struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresIn    int64     `json:"expires_in,omitempty"`
	Scope        string    `json:"scope,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

var _ mcpclienttransport.Token = persistedMCPToken{}

func newTestCreds(accessToken string, expiresAt time.Time) *mcpOAuthCredentials {
	return &mcpOAuthCredentials{
		ResourceURI:  testMCPOAuthResourceURI,
		Issuer:       testMCPOAuthIssuer,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:12345/callback",
		Scopes:       []string{"mcp:tools", "mcp:resources"},
		Token: mcpclienttransport.Token{
			AccessToken:  accessToken,
			TokenType:    "bearer",
			RefreshToken: "refresh-" + accessToken,
			ExpiresAt:    expiresAt,
		},
	}
}

func TestMCPOAuthTokenPath(t *testing.T) {
	home := t.TempDir()
	path := func(username string, app scopes.QualifiedName) string {
		path, err := mcpOAuthTokenPath(home, "proxy.example.com", username, "root", app)
		require.NoError(t, err)
		return path
	}

	require.NotEqual(t,
		path("alice", scopes.QualifiedName{Name: "app"}),
		path("bob", scopes.QualifiedName{Name: "app"}),
	)
	require.NotEqual(t,
		path("alice", scopes.QualifiedName{Name: "app", Scope: "/prod"}),
		path("alice", scopes.QualifiedName{Name: "app", Scope: "/dev"}),
	)
	_, err := mcpOAuthTokenPath(home, "proxy.example.com", "alice", "root", scopes.QualifiedName{Name: "../../profile"})
	require.Error(t, err)
}

func TestMCPOAuthCredentialsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cluster", "app.json")
	creds := newTestCreds("token-1", time.Now().Add(time.Hour))

	require.NoError(t, saveMCPOAuthCredentials(path, creds))
	if runtime.GOOS != "windows" {
		require.NoError(t, os.Chmod(path, 0o644))
		require.NoError(t, saveMCPOAuthCredentials(path, creds))
	}

	loaded, err := loadMCPOAuthCredentials(path)
	require.NoError(t, err)
	require.Equal(t, creds.ResourceURI, loaded.ResourceURI)
	require.Equal(t, creds.Issuer, loaded.Issuer)
	require.Equal(t, creds.ClientID, loaded.ClientID)
	require.Equal(t, creds.ClientSecret, loaded.ClientSecret)
	require.Equal(t, creds.RedirectURI, loaded.RedirectURI)
	require.Equal(t, creds.Scopes, loaded.Scopes)
	require.Equal(t, creds.Token.AccessToken, loaded.Token.AccessToken)
	require.Equal(t, creds.Token.RefreshToken, loaded.Token.RefreshToken)

	if runtime.GOOS != "windows" {
		fi, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, "0600", fmt.Sprintf("%04o", fi.Mode().Perm()))
	}
}

func TestFileTokenStore(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "app.json")
	lockPath := filepath.Join(dir, "locks", "mcp_oauth.lock")
	store := &fileTokenStore{
		path:             path,
		mutationLockPath: lockPath,
		resourceURI:      testMCPOAuthResourceURI,
		issuer:           testMCPOAuthIssuer,
	}

	_, err := store.GetToken(ctx)
	require.ErrorIs(t, err, mcpclienttransport.ErrNoToken)

	creds := newTestCreds("old-token", time.Now().Add(-time.Hour))
	creds.Scopes = []string{"mcp:tools"}
	require.NoError(t, saveMCPOAuthCredentials(path, creds))
	token := &mcpclienttransport.Token{AccessToken: "tok", RefreshToken: "ref"}
	require.NoError(t, store.SaveToken(ctx, token))
	require.FileExists(t, lockPath)

	got, err := store.GetToken(ctx)
	require.NoError(t, err)
	require.Equal(t, "tok", got.AccessToken)

	creds, err = loadMCPOAuthCredentials(path)
	require.NoError(t, err)
	require.Equal(t, "test-client-id", creds.ClientID)
	require.Equal(t, "test-client-secret", creds.ClientSecret)
	require.Equal(t, "http://localhost:12345/callback", creds.RedirectURI)
	require.Equal(t, []string{"mcp:tools"}, creds.Scopes)

	creds.Issuer = "https://replacement.example.com"
	require.NoError(t, saveMCPOAuthCredentials(path, creds))
	_, err = store.GetToken(ctx)
	require.ErrorContains(t, err, "bound to a different issuer")
}

func TestFileTokenStoreDoesNotRestoreLoggedOutCredentials(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.json")
	lockPath := filepath.Join(dir, "mcp_oauth.lock")
	require.NoError(t, saveMCPOAuthCredentials(path, newTestCreds("old-token", time.Now().Add(-time.Hour))))

	unlock, err := utils.FSTryWriteLockTimeout(t.Context(), lockPath, time.Second)
	require.NoError(t, err)
	locked := true
	t.Cleanup(func() {
		if locked {
			_ = unlock()
		}
	})

	saved := make(chan error, 1)
	go func() {
		saved <- (&fileTokenStore{
			path:             path,
			mutationLockPath: lockPath,
			resourceURI:      testMCPOAuthResourceURI,
			issuer:           testMCPOAuthIssuer,
		}).SaveToken(t.Context(), &mcpclienttransport.Token{AccessToken: "new-token"})
	}()

	select {
	case err := <-saved:
		require.FailNow(t, "save ignored credential mutation lock", "%v", err)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, os.Remove(path))
	require.NoError(t, unlock())
	locked = false
	require.Error(t, <-saved)
	require.NoFileExists(t, path)
}
