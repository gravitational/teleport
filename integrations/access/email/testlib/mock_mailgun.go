/*
Copyright 2015-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific languap governing permissions and
limitations under the License.
*/

package testlib

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
)

const (
	// multipartFormBufSize is a buffer size for ParseMultipartForm
	multipartFormBufSize = 8192
)

// mockMailgunMessage is a mock mailgun message
type mockMailgunMessage struct {
	ID         string
	Sender     string
	Recipient  string
	Subject    string
	Body       string
	References string
}

// mockMailgun is a mock mailgun server
type mockMailgunServer struct {
	server     *httptest.Server
	chMessages chan mockMailgunMessage
}

// NewMockMailgun creates unstarted mock mailgun server instance.
// Standard server from mailgun-go does not catch message texts.
func newMockMailgunServer(concurrency int) *mockMailgunServer {
	mg := &mockMailgunServer{
		chMessages: make(chan mockMailgunMessage, concurrency*50),
	}

	s := httptest.NewUnstartedServer(func(mg *mockMailgunServer) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseMultipartForm(multipartFormBufSize); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			id := uuid.New().String()

			// The testmode flag is only used during health check.
			// Do no create message when in testmode.
			if r.PostFormValue("o:testmode") == "yes" {
				fmt.Fprintf(w, `{"id": "%v"}`, id)
				return
			}

			message := mockMailgunMessage{
				ID:         id,
				Sender:     r.PostFormValue("from"),
				Recipient:  r.PostFormValue("to"),
				Subject:    r.PostFormValue("subject"),
				Body:       r.PostFormValue("text"),
				References: r.PostFormValue("references"),
			}

			mg.chMessages <- message

			fmt.Fprintf(w, `{"id": "%v"}`, id)
		}
	}(mg))

	mg.server = s

	return mg
}

// Start starts server
func (m *mockMailgunServer) Start() {
	m.server.Start()
}

// GetURL returns server url
func (m *mockMailgunServer) GetURL() string {
	return m.server.URL + "/v4"
}

// GetMessage gets the new Mailgun message from a queue
func (m *mockMailgunServer) GetMessage(ctx context.Context) (mockMailgunMessage, error) {
	select {
	case message := <-m.chMessages:
		return message, nil
	case <-ctx.Done():
		return mockMailgunMessage{}, trace.Wrap(ctx.Err())
	}
}

// Close stops servers
func (m *mockMailgunServer) Stop() {
	m.server.Close()
}
