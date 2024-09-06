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

package opsgenie

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/api/types"
)

func TestCreateAlert(t *testing.T) {
	recievedReq := ""
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/v2/alerts" {
			return
		}
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			log.Fatal(err)
		}
		recievedReq = string(bodyBytes)
	}))
	defer func() { testServer.Close() }()

	c, err := NewClient(ClientConfig{
		APIEndpoint: testServer.URL,
		Priority:    "somePriority",
		ClusterName: "someClusterName",
	})
	assert.NoError(t, err)

	_, err = c.CreateAlert(context.Background(), "someRequestID", RequestData{
		User:          "someUser",
		Roles:         []string{"role1", "role2"},
		RequestReason: "someReason",
		SystemAnnotations: types.Labels{
			types.TeleportNamespace + types.ReqAnnotationNotifySchedulesLabel: {"responder@example.com", "bb4d9938-c3c2-455d-aaab-727aa701c0d8"},
			types.TeleportNamespace + types.ReqAnnotationTeamsLabel:           {"MyOpsGenieTeam", "aee8a0de-c80f-4515-a232-501c0bc9d715"},
		},
	})
	assert.NoError(t, err)

	expected := AlertBody{
		Message:     "Access request from someUser",
		Alias:       "teleport-access-request/someRequestID",
		Description: "someUser requested permissions for roles role1, role2 on Teleport at 01 Jan 01 00:00 UTC.\nReason: someReason\n\n",
		Responders: []Responder{
			{Type: "schedule", Name: "responder@example.com"},
			{Type: "schedule", ID: "bb4d9938-c3c2-455d-aaab-727aa701c0d8"},
			{Type: "team", Name: "MyOpsGenieTeam"},
			{Type: "team", ID: "aee8a0de-c80f-4515-a232-501c0bc9d715"},
		},
		Priority: "somePriority",
	}
	var got AlertBody
	err = json.Unmarshal([]byte(recievedReq), &got)
	assert.NoError(t, err)

	assert.Equal(t, expected, got)
}

func TestPostReviewNote(t *testing.T) {
	recievedReq := ""
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			log.Fatal(err)
		}
		recievedReq = string(bodyBytes)
	}))
	defer func() { testServer.Close() }()

	c, err := NewClient(ClientConfig{
		APIEndpoint: testServer.URL,
		Priority:    "somePriority",
		ClusterName: "someClusterName",
	})
	assert.NoError(t, err)

	err = c.PostReviewNote(context.Background(), "someAlertID", types.AccessReview{
		ProposedState: types.RequestState_APPROVED,
		Author:        "someUser",
		Reason:        "someReason",
	})
	assert.NoError(t, err)

	expected := AlertNote{
		Note: "someUser reviewed the request at 01 Jan 01 00:00 UTC.\nResolution: APPROVED.\nReason: someReason.",
	}
	var got AlertNote
	err = json.Unmarshal([]byte(recievedReq), &got)
	assert.NoError(t, err)

	assert.Equal(t, expected, got)
}

func TestResolveAlert(t *testing.T) {
	recievedReq := ""
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			log.Fatal(err)
		}
		recievedReq = string(bodyBytes)
	}))
	defer func() { testServer.Close() }()

	c, err := NewClient(ClientConfig{
		APIEndpoint: testServer.URL,
		Priority:    "somePriority",
		ClusterName: "someClusterName",
	})
	assert.NoError(t, err)

	err = c.ResolveAlert(context.Background(), "someAlertID", Resolution{
		Tag:    ResolvedApproved,
		Reason: "someReason",
	})
	assert.NoError(t, err)

	expected := AlertNote{
		Note: "Access request has been approved\nReason: someReason",
	}
	var got AlertNote
	err = json.Unmarshal([]byte(recievedReq), &got)
	assert.NoError(t, err)

	assert.Equal(t, expected, got)

}

func TestCreateAlertError(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusForbidden)
	}))
	defer func() { testServer.Close() }()

	c, err := NewClient(ClientConfig{
		APIEndpoint: testServer.URL,
	})
	assert.NoError(t, err)

	_, err = c.CreateAlert(context.Background(), "someRequestID", RequestData{})
	assert.True(t, trace.IsAccessDenied(err))
}
