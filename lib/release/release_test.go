/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package release

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gravitational/roundtrip"
	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/api/types"
)

func TestNewClient(t *testing.T) {
	// should err when no TLS config is passed in
	_, err := NewClient(ClientConfig{})
	assert.NotNil(t, err)

	// should err when no server addr is passed in
	_, err = NewClient(ClientConfig{
		TLSConfig: &tls.Config{},
	})
	assert.NotNil(t, err)

	// should not err when TLS config and server addr is passed in
	_, err = NewClient(ClientConfig{
		TLSConfig:         &tls.Config{},
		ReleaseServerAddr: "server-addr",
	})
	assert.Nil(t, err)
}

func TestListReleases(t *testing.T) {
	mockClient := Client{}

	// ListReleases should err if client is not initialized
	_, err := mockClient.ListReleases(context.Background())
	assert.NotNil(t, err)

	tt := []struct {
		name               string
		responseStatusCode int
		responseBody       string
		shouldErr          bool
		expected           []*types.Release
	}{
		{
			name:               "empty response",
			responseStatusCode: 200,
			responseBody:       "[]",
			shouldErr:          false,
			expected:           nil,
		},
		{
			name:               "access denied",
			responseStatusCode: 401,
			shouldErr:          true,
		},
		{
			name:               "bad request",
			responseStatusCode: 500,
			shouldErr:          true,
		},
		{
			name:               "OK response",
			responseStatusCode: 200,
			responseBody: `[{
				"notesMd": "notes",
				"product": "teleport",
				"releaseId": "1",
				"status": "released",
				"version": "10.1.3",
				"assets": [
					{
						"arch": "amd64",
						"description": "teleport amd64",
						"name": "Teleport",
						"os": "Linux",
						"sha256": "200fd83c8fbe55df25bfc638e4ee2e746443ff037c78a985005dff0206e103d3",
						"size": 1000000000,
						"releaseIds": ["1", "2"],
						"publicUrl": "example.com/teleport.tar.gz"
					},
					{
						"arch": "amd32",
						"description": "teleport amd32",
						"name": "Teleport",
						"os": "Linux",
						"sha256": "100fd83c8fbe55df25bfc638e4ee2e746443ff037c78a985005dff0206e103d3",
						"size": 1000000000,
						"releaseIds": ["1", "2"],
						"publicUrl": "example.com/teleport.tar.gz"
					}
				]
			},
			{
				"notesMd": "notes",
				"product": "teleport",
				"releaseId": "2",
				"status": "released",
				"version": "10.1.4",
				"assets": [
					{
						"arch": "amd64",
						"description": "teleport amd64",
						"name": "Teleport",
						"os": "Linux",
						"sha256": "200fd83c8fbe55df25bfc638e4ee2e746443ff037c78a985005dff0206e103d3",
						"size": 1000000000,
						"releaseIds": ["1", "2"],
						"publicUrl": "example.com/teleport.tar.gz"
					},
					{
						"arch": "amd32",
						"description": "teleport amd32",
						"name": "Teleport",
						"os": "Linux",
						"sha256": "100fd83c8fbe55df25bfc638e4ee2e746443ff037c78a985005dff0206e103d3",
						"size": 1000000000,
						"releaseIds": ["1", "2"],
						"publicUrl": "example.com/teleport.tar.gz"
					}
				]
			}]`,
			shouldErr: false,
			expected: []*types.Release{
				{
					NotesMD:   "notes",
					Product:   "teleport",
					ReleaseID: "1",
					Status:    "released",
					Version:   "10.1.3",
					Assets: []*types.Asset{
						{
							Arch:        "amd64",
							Description: "teleport amd64",
							Name:        "Teleport",
							OS:          "Linux",
							SHA256:      "200fd83c8fbe55df25bfc638e4ee2e746443ff037c78a985005dff0206e103d3",
							AssetSize:   1000000000,
							DisplaySize: "1.0 GB",
							ReleaseIDs:  []string{"1", "2"},
							PublicURL:   "example.com/teleport.tar.gz",
						},
						{
							Arch:        "amd32",
							Description: "teleport amd32",
							Name:        "Teleport",
							OS:          "Linux",
							SHA256:      "100fd83c8fbe55df25bfc638e4ee2e746443ff037c78a985005dff0206e103d3",
							AssetSize:   1000000000,
							DisplaySize: "1.0 GB",
							ReleaseIDs:  []string{"1", "2"},
							PublicURL:   "example.com/teleport.tar.gz",
						},
					},
				},
				{
					NotesMD:   "notes",
					Product:   "teleport",
					ReleaseID: "2",
					Status:    "released",
					Version:   "10.1.4",
					Assets: []*types.Asset{
						{
							Arch:        "amd64",
							Description: "teleport amd64",
							Name:        "Teleport",
							OS:          "Linux",
							SHA256:      "200fd83c8fbe55df25bfc638e4ee2e746443ff037c78a985005dff0206e103d3",
							AssetSize:   1000000000,
							DisplaySize: "1.0 GB",
							ReleaseIDs:  []string{"1", "2"},
							PublicURL:   "example.com/teleport.tar.gz",
						},
						{
							Arch:        "amd32",
							Description: "teleport amd32",
							Name:        "Teleport",
							OS:          "Linux",
							SHA256:      "100fd83c8fbe55df25bfc638e4ee2e746443ff037c78a985005dff0206e103d3",
							AssetSize:   1000000000,
							DisplaySize: "1.0 GB",
							ReleaseIDs:  []string{"1", "2"},
							PublicURL:   "example.com/teleport.tar.gz",
						},
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case "/teleport-ent":
					w.WriteHeader(tc.responseStatusCode)
					w.Write([]byte(tc.responseBody))
					return
				default:
					http.NotFoundHandler().ServeHTTP(w, r)
				}
			}))

			mockClient.client, err = roundtrip.NewClient(server.URL, "")
			assert.Nil(t, err)

			releases, err := mockClient.ListReleases(context.Background())
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tc.expected, releases)
			}
		})
	}
}
