/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/stretchr/testify/require"
)

func testCreds(refreshToken string, expiresAt time.Time) *Credentials {
	return &Credentials{
		Token: &mcpclienttransport.Token{
			AccessToken:  "access-1",
			TokenType:    "bearer",
			RefreshToken: refreshToken,
			ExpiresAt:    expiresAt,
		},
		ClientID:      "client-1",
		TokenEndpoint: "http://example.com/token",
		Resource:      "http://example.com/mcp",
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return NewStore(filepath.Join(dir, "app.json"), filepath.Join(dir, "app.json.lock"), nil)
}

func TestStoreSaveLoad(t *testing.T) {
	store := newTestStore(t)

	_, err := store.Load()
	require.True(t, errors.Is(err, ErrLoginRequired), "missing file must map to ErrLoginRequired, got %v", err)

	want := testCreds("refresh-1", time.Now().Add(time.Hour).UTC())
	require.NoError(t, store.Save(want))

	info, err := os.Stat(store.tokenPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	got, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, want.Token.AccessToken, got.Token.AccessToken)
	require.Equal(t, want.Token.RefreshToken, got.Token.RefreshToken)
	require.Equal(t, want.ClientID, got.ClientID)
	require.Equal(t, want.TokenEndpoint, got.TokenEndpoint)
	require.Equal(t, want.Resource, got.Resource)
	require.True(t, want.Token.ExpiresAt.Equal(got.Token.ExpiresAt))
}

func TestStoreSaveConcurrent(t *testing.T) {
	// Saves are temp-file+rename; concurrent writers must never produce a torn file.
	store := newTestStore(t)
	require.NoError(t, store.Save(testCreds("r", time.Now().Add(time.Hour))))

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			_ = store.Save(testCreds("r", time.Now().Add(time.Hour)))
		}
	}()
	for i := 0; i < 50; i++ {
		_, err := store.Load()
		require.NoError(t, err, "Load must never observe a partial write")
	}
	<-done
}

// fakeTokenEndpoint is a rotation-enforcing refresh endpoint: it accepts the
// current refresh token exactly once, rotates it, and rejects reuse with
// invalid_grant: the behavior that wedges a second concurrent refresher.
type fakeTokenEndpoint struct {
	mu             sync.Mutex
	currentRefresh string
	generation     int
	refreshCalls   atomic.Int64
	srv            *httptest.Server
}

func newFakeTokenEndpoint(t *testing.T, initialRefresh string) *fakeTokenEndpoint {
	t.Helper()
	f := &fakeTokenEndpoint{currentRefresh: initialRefresh}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("client_id") != "client-1" {
			http.Error(w, "bad refresh request", http.StatusBadRequest)
			return
		}
		f.refreshCalls.Add(1)

		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Form.Get("refresh_token") != f.currentRefresh {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		f.generation++
		f.currentRefresh = fmt.Sprintf("refresh-%d", f.generation+1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  fmt.Sprintf("access-%d", f.generation+1),
			"token_type":    "Bearer",
			"refresh_token": f.currentRefresh,
			"expires_in":    3600,
		})
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func newRefreshableStore(t *testing.T, endpoint string, expiresAt time.Time) *Store {
	t.Helper()
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "app.json"), filepath.Join(dir, "app.json.lock"), nil)
	creds := testCreds("refresh-1", expiresAt)
	creds.TokenEndpoint = endpoint
	require.NoError(t, store.Save(creds))
	return store
}

func TestGetValidAuthHeaderFastPath(t *testing.T) {
	// Fresh token: no lock, no network. The bogus endpoint proves no request is made.
	store := newRefreshableStore(t, "http://127.0.0.1:1/token", time.Now().Add(time.Hour))
	header, err := store.GetValidAuthHeader(context.Background())
	require.NoError(t, err)
	require.Equal(t, "Bearer access-1", header)
}

func TestGetValidAuthHeaderUpstreamMismatch(t *testing.T) {
	store := newRefreshableStore(t, "http://127.0.0.1:1/token", time.Now().Add(time.Hour))
	creds, err := store.Load()
	require.NoError(t, err)
	creds.UpstreamURL = "https://old.example.com/mcp"
	require.NoError(t, store.Save(creds))

	store.SetExpectedUpstreamURL("https://new.example.com/mcp")
	_, err = store.GetValidAuthHeader(context.Background())
	require.True(t, errors.Is(err, ErrLoginRequired), "upstream mismatch must force re-login, got %v", err)
}

func TestGetValidAuthHeaderRefresh(t *testing.T) {
	fake := newFakeTokenEndpoint(t, "refresh-1")
	store := newRefreshableStore(t, fake.srv.URL, time.Now().Add(-time.Minute))

	header, err := store.GetValidAuthHeader(context.Background())
	require.NoError(t, err)
	require.Equal(t, "Bearer access-2", header)
	require.Equal(t, int64(1), fake.refreshCalls.Load())

	creds, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, "refresh-2", creds.Token.RefreshToken)
}

func TestGetValidAuthHeaderSingleFlight(t *testing.T) {
	// N stores (own fds, like N processes) race on one expired token file.
	// The rotation-enforcing endpoint fails the test unless refresh is
	// single-flight: any second real refresh attempt gets invalid_grant.
	fake := newFakeTokenEndpoint(t, "refresh-1")
	store := newRefreshableStore(t, fake.srv.URL, time.Now().Add(-time.Minute))

	const n = 8
	headers := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := NewStore(store.tokenPath, store.lockPath, nil)
			headers[i], errs[i] = s.GetValidAuthHeader(context.Background())
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		require.NoError(t, errs[i])
		require.Equal(t, "Bearer access-2", headers[i])
	}
	require.Equal(t, int64(1), fake.refreshCalls.Load(), "stampede must collapse to exactly one refresh")
}

func TestGetValidAuthHeaderRefreshDead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
	}))
	defer srv.Close()
	store := newRefreshableStore(t, srv.URL, time.Now().Add(-time.Minute))

	_, err := store.GetValidAuthHeader(context.Background())
	require.True(t, errors.Is(err, ErrLoginRequired), "dead refresh token must map to ErrLoginRequired, got %v", err)
}

func TestGetValidAuthHeaderNoRefreshToken(t *testing.T) {
	store := newRefreshableStore(t, "http://127.0.0.1:1/token", time.Now().Add(-time.Minute))
	creds, err := store.Load()
	require.NoError(t, err)
	creds.Token.RefreshToken = ""
	require.NoError(t, store.Save(creds))

	_, err = store.GetValidAuthHeader(context.Background())
	require.True(t, errors.Is(err, ErrLoginRequired))
}
