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

package servicenow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestCreateIncident(t *testing.T) {
	recievedReq := ""
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			t.Error(err)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		recievedReq = string(bodyBytes)
	}))
	defer func() { testServer.Close() }()

	c, err := NewClient(ClientConfig{
		APIEndpoint: testServer.URL,
	})
	require.NoError(t, err)

	_, err = c.CreateIncident(context.Background(), "someRequestID", RequestData{
		User:          "someUser",
		Roles:         []string{"role1", "role2"},
		RequestReason: "someReason",
	})
	assert.NoError(t, err)

	expected := Incident{
		ShortDescription: "Teleport access request from user someUser",
		Description:      "Teleport user someUser submitted access request for roles role1, role2 on Teleport cluster .\nReason: someReason\n\n",
		Caller:           "someUser",
	}
	var got Incident
	err = json.Unmarshal([]byte(recievedReq), &got)
	assert.NoError(t, err)

	assert.Equal(t, expected, got)
}

func TestPostReviewNote(t *testing.T) {
	recievedReq := ""
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			t.Error(err)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		recievedReq = string(bodyBytes)
	}))
	defer func() { testServer.Close() }()

	c, err := NewClient(ClientConfig{
		APIEndpoint: testServer.URL,
	})
	require.NoError(t, err)

	err = c.PostReviewNote(context.Background(), "someIncidentID", types.AccessReview{
		ProposedState: types.RequestState_APPROVED,
		Author:        "someUser",
		Reason:        "someReason",
	})
	assert.NoError(t, err)

	expected := Incident{
		WorkNotes: "someUser reviewed the request at 01 Jan 01 00:00 UTC.\nResolution: APPROVED.\nReason: someReason.",
	}
	var got Incident
	err = json.Unmarshal([]byte(recievedReq), &got)
	assert.NoError(t, err)

	assert.Equal(t, expected, got)
}

func TestResolveIncident(t *testing.T) {
	recievedReq := ""
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			t.Error(err)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		recievedReq = string(bodyBytes)
	}))
	defer func() { testServer.Close() }()

	c, err := NewClient(ClientConfig{
		APIEndpoint: testServer.URL,
		CloseCode:   "approved",
	})
	require.NoError(t, err)

	err = c.ResolveIncident(context.Background(), "someIncidentID", Resolution{
		Reason: "someReason",
		State:  "6",
	})
	assert.NoError(t, err)

	expected := Incident{
		CloseNotes:    "Access request has been approved\nReason: someReason",
		CloseCode:     "approved",
		IncidentState: "6",
	}
	var got Incident
	err = json.Unmarshal([]byte(recievedReq), &got)
	assert.NoError(t, err)

	assert.Equal(t, expected, got)

}

func TestCreateIncidentError(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusForbidden)
	}))
	defer func() { testServer.Close() }()

	c, err := NewClient(ClientConfig{
		APIEndpoint: testServer.URL,
	})
	require.NoError(t, err)

	_, err = c.CreateIncident(context.Background(), "someRequestID", RequestData{})
	assert.True(t, trace.IsAccessDenied(err))
}
