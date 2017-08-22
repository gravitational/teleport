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

package roundtrip

import (
	"net/http"
	"net/url"

	"github.com/gravitational/trace"

	. "gopkg.in/check.v1"
)

var _ = Suite(&CredsSuite{})

type CredsSuite struct {
	c *testClient
}

func (s *CredsSuite) TestBasicAuth(c *C) {
	var creds *AuthCreds
	var credsErr error
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		creds, credsErr = ParseAuthHeaders(r)
	})
	defer srv.Close()

	clt := newC(srv.URL, "v1", BasicAuth("user", "pass"))
	_, err := clt.Get(clt.Endpoint("test"), url.Values{})
	c.Assert(err, IsNil)

	c.Assert(credsErr, IsNil)
	c.Assert(creds, DeepEquals, &AuthCreds{Type: AuthBasic, Username: "user", Password: "pass"})
}

func (s *CredsSuite) TestTokenAuth(c *C) {
	var creds *AuthCreds
	var credsErr error
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		creds, credsErr = ParseAuthHeaders(r)
	})
	defer srv.Close()

	clt := newC(srv.URL, "v1", BearerAuth("token1"))
	_, err := clt.Get(clt.Endpoint("test"), url.Values{})
	c.Assert(err, IsNil)

	c.Assert(credsErr, IsNil)
	c.Assert(creds, DeepEquals, &AuthCreds{Type: AuthBearer, Password: "token1"})
}

func (s *CredsSuite) TestTokenURIAuth(c *C) {
	var creds *AuthCreds
	var credsErr error
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		creds, credsErr = ParseAuthHeaders(r)
	})
	defer srv.Close()

	clt := newC(srv.URL, "v1")
	_, err := clt.Get(clt.Endpoint("test"), url.Values{AccessTokenQueryParam: []string{"token2"}})
	c.Assert(err, IsNil)

	c.Assert(credsErr, IsNil)
	c.Assert(creds, DeepEquals, &AuthCreds{Type: AuthBearer, Password: "token2"})
}

func (s *CredsSuite) TestGarbage(c *C) {
	type tc struct {
		Headers map[string][]string
		Error   error
	}
	testCases := []tc{
		// missing auth requests
		{
			Headers: map[string][]string{"Authorization": []string{""}},
			Error:   &AccessDeniedError{},
		},
		{
			Headers: map[string][]string{"Authorisation": []string{"Bearer blabla"}},
			Error:   &AccessDeniedError{},
		},
		// corrupted auth requests
		{
			Headers: map[string][]string{"Authorization": []string{"WAT? blabla"}},
			Error:   &ParameterError{},
		},
		{
			Headers: map[string][]string{"Authorization": []string{"Basic bad"}},
			Error:   &ParameterError{},
		},
		{
			Headers: map[string][]string{"Authorization": []string{"Bearer"}},
			Error:   &ParameterError{},
		},
	}

	for i, tc := range testCases {
		var credsErr error
		srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
			_, credsErr = ParseAuthHeaders(r)
		})
		defer srv.Close()

		comment := Commentf("test %v", i)

		req, err := http.NewRequest("GET", srv.URL, nil)
		c.Assert(err, IsNil, comment)
		for key, vals := range tc.Headers {
			for _, val := range vals {
				req.Header.Add(key, val)
			}
		}
		_, err = http.DefaultClient.Do(req)
		c.Assert(err, IsNil, comment)

		c.Assert(credsErr, NotNil, comment)
		origErr := credsErr.(trace.Error)

		c.Assert(origErr.OrigError(), FitsTypeOf, tc.Error, comment)
	}

}
