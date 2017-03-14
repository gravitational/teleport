/*
Copyright 2015 Gravitational, Inc.

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

package utils

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"time"

	"gopkg.in/check.v1"
)

// TimeoutSuite helps us to test ObeyTimeout mechanism. We use HTTP server/client
// machinery to test timeouts
type TimeoutSuite struct {
	lastRequest *http.Request
	server      *httptest.Server
}

var _ = check.Suite(&TimeoutSuite{})

func (s *TimeoutSuite) SetUpSuite(c *check.C) {
	//
	// set up an HTTP server which listens and responds to queries
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		// GET /slow?delay=10ms sleeps for a given delay, then returns word "slow"
		case "/slow":
			delay, err := time.ParseDuration(r.URL.Query().Get("delay"))
			c.Assert(err, check.IsNil)
			time.Sleep(delay)
			fmt.Fprintf(w, "slow")

		// GET /ping returns "pong"
		case "/ping":
			fmt.Fprintf(w, "pong")
		}
	}))
}

func (s *TimeoutSuite) TearDownSuite(c *check.C) {
	s.server.Close()
}

func (s *TimeoutSuite) TestSlowOperation(c *check.C) {
	client := newClient(time.Millisecond * 5)
	_, err := client.Get(s.server.URL + "/slow?delay=20ms")
	// must fail with I/O timeout
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Matches, "^.*i/o timeout$")
}

func (s *TimeoutSuite) TestNormalOperation(c *check.C) {
	client := newClient(time.Millisecond * 5)
	resp, err := client.Get(s.server.URL + "/ping")
	c.Assert(err, check.IsNil)
	c.Assert(bodyText(resp), check.Equals, "pong")
}

// newClient helper returns HTTP client configured to use a connection
// wich drops itself after N idle time
func newClient(timeout time.Duration) *http.Client {
	var t http.Transport
	t.Dial = func(network string, addr string) (net.Conn, error) {
		conn, err := net.Dial(network, addr)
		if err != nil {
			return nil, err
		}
		return ObeyIdleTimeout(conn, timeout, "test"), nil
	}
	return &http.Client{Transport: &t}
}

// bodyText helper returns a body string from an http response
func bodyText(resp *http.Response) string {
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return string(bytes)
}
