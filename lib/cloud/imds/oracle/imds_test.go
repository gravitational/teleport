// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package oracle

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const defaultInstanceID = "ocid1.instance.oc1.phx.12345678"

func mockIMDSServer(t *testing.T, status int, data any) string {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		if data == nil {
			return
		}
		body, err := json.Marshal(data)
		if !assert.NoError(t, err) {
			return
		}
		w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func TestIsAvailable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		imdsStatus   int
		imdsResponse any
		assert       assert.BoolAssertionFunc
	}{
		{
			name:       "ok",
			imdsStatus: http.StatusOK,
			imdsResponse: instance{
				ID: defaultInstanceID,
			},
			assert: assert.True,
		},
		{
			name:       "not available",
			imdsStatus: http.StatusNotFound,
			assert:     assert.False,
		},
		{
			name:       "not on oci",
			imdsStatus: http.StatusOK,
			imdsResponse: instance{
				ID: "notavalidocid",
			},
			assert: assert.False,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			imdsURL := mockIMDSServer(t, tc.imdsStatus, tc.imdsResponse)
			clt := &InstanceMetadataClient{baseIMDSAddr: imdsURL}
			tc.assert(t, clt.IsAvailable(context.Background()))
		})
	}

	t.Run("don't hang on connection", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		t.Cleanup(cancel)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-time.After(10 * time.Second):
				data, err := json.Marshal(instance{
					ID: defaultInstanceID,
				})
				if !assert.NoError(t, err) {
					return
				}
				w.Write(data)
			case <-ctx.Done():
			}
		}))
		t.Cleanup(server.Close)

		clt := &InstanceMetadataClient{baseIMDSAddr: server.URL}
		assert.False(t, clt.IsAvailable(ctx))
	})
}

func TestGetTags(t *testing.T) {
	t.Parallel()
	serverURL := mockIMDSServer(t, http.StatusOK, instance{
		DefinedTags: map[string]map[string]string{
			"my-namespace": {
				"foo": "bar",
			},
		},
		FreeformTags: map[string]string{
			"baz": "quux",
		},
	})
	clt := &InstanceMetadataClient{baseIMDSAddr: serverURL}
	tags, err := clt.GetTags(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"my-namespace/foo": "bar",
		"baz":              "quux",
	}, tags)

}
