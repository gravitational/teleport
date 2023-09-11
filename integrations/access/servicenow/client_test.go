/*
Copyright 2015-2023 Gravitational, Inc.

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
		ShortDescription: "Access request from someUser",
		Description:      "someUser requested permissions for roles role1, role2 on Teleport at 01 Jan 01 00:00 UTC.\nReason: someReason\n\n",
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
	})
	require.NoError(t, err)

	err = c.ResolveIncident(context.Background(), "someIncidentID", Resolution{
		CloseCode: "approved",
		Reason:    "someReason",
		State:     "6",
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
