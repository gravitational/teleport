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
		RequestAnnotations: map[string][]string{
			ReqAnnotationRespondersKey: {"responder@teleport.com"},
		},
	})
	assert.NoError(t, err)

	expected := AlertBody{
		Message:     "Access request from someUser",
		Alias:       "teleport-access-request/someRequestID",
		Description: "someUser requested permissions for roles role1, role2 on Teleport at 01 Jan 01 00:00 UTC.\nReason: someReason\n\n",
		Responders:  []Responder{{Type: "schedule", ID: "responder@teleport.com"}},
		Priority:    "somePriority",
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
