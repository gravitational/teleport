// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azidentity

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
)

const okPage = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8" />
    <title>Login Succeeded</title>
</head>
<body>
    <h4>You have logged into Microsoft Azure!</h4>
    <p>You can now close this window.</p>
</body>
</html>
`

const failPage = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8" />
    <title>Login Failed</title>
</head>
<body>
    <h4>An error occurred during authentication</h4>
    <p>Please open an issue in the <a href="https://github.com/azure/go-autorest/issues">Go Autorest repo</a> for assistance.</p>
</body>
</html>
`

type server struct {
	done chan struct{}
	s    *http.Server
	code string
	err  error
}

// NewServer creates an object that satisfies the Server interface.
func newServer() *server {
	rs := &server{
		done: make(chan struct{}),
		s:    &http.Server{},
	}
	return rs
}

// Start starts the local HTTP server on a separate go routine.
// The return value is the full URL plus port number.
func (s *server) Start(reqState string, port int) string {
	if port == 0 {
		port = rand.Intn(600) + 8400
	}
	s.s.Addr = fmt.Sprintf(":%d", port)
	s.s.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			s.done <- struct{}{}

			page := okPage
			if s.err != nil {
				page = failPage
			}
			w.Write([]byte(page)) // nolint:errcheck
		}()

		qp := r.URL.Query()
		if reqState != qp.Get("state") {
			s.err = errors.New("mismatched OAuth state")
			return
		}
		if err := qp.Get("error"); err != "" {
			errMsg := fmt.Sprintf("authentication error: %s", err)
			if detail := qp.Get("error_description"); detail != "" {
				errMsg = fmt.Sprintf("%s; description: %s", errMsg, detail)
			}
			s.err = fmt.Errorf("%s", errMsg)
			return
		}

		code := qp.Get("code")
		if code == "" {
			s.err = errors.New("authorization code missing in query string")
			return
		}
		s.code = code
	})
	go s.s.ListenAndServe() // nolint:errcheck
	return fmt.Sprintf("http://localhost:%d", port)
}

// Stop will shut down the local HTTP server.
func (s *server) Stop() {
	close(s.done)
	s.s.Shutdown(context.Background()) // nolint:errcheck
}

// WaitForCallback will wait until Azure interactive login has called us back with an authorization code or error.
func (s *server) WaitForCallback(ctx context.Context) error {
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// AuthorizationCode returns the authorization code or error result from the interactive login.
func (s *server) AuthorizationCode() (string, error) {
	return s.code, s.err
}
