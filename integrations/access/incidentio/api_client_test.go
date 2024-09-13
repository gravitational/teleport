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

package incidentio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSchedule(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/v2/schedules/someRequestID" {
			res.WriteHeader(http.StatusOK)
		} else {
			res.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer func() { testServer.Close() }()

	c, err := NewAPIClient(ClientConfig{
		APIEndpoint: testServer.URL,
		APIKey:      "someAPIKey",
		ClusterName: "someClusterName",
	})
	assert.NoError(t, err)

	_, err = c.GetOnCall(context.Background(), "someRequestID")
	assert.NoError(t, err)
}

func TestHealthCheck(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/v2/schedules" {
			res.WriteHeader(http.StatusOK)
		} else {
			res.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer func() { testServer.Close() }()

	c, err := NewAPIClient(ClientConfig{
		APIEndpoint: testServer.URL,
		APIKey:      "someAPIKey",
		ClusterName: "someClusterName",
	})
	assert.NoError(t, err)

	err = c.CheckHealth(context.Background())
	assert.NoError(t, err)
}
