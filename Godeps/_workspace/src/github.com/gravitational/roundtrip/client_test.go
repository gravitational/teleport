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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1" // note that we don't vendor libraries dependencies, only end daemons deps are vendored
)

func TestClient(t *testing.T) { TestingT(t) }

type ClientSuite struct {
	c *testClient
}

var _ = Suite(&ClientSuite{})

func (s *ClientSuite) TestPostForm(c *C) {
	var u *url.URL
	var form url.Values
	var method string
	var user, pass string
	var ok bool
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok = r.BasicAuth()
		u = r.URL
		c.Assert(r.ParseForm(), IsNil)
		form = r.Form
		method = r.Method
		io.WriteString(w, "hello back")
	})
	defer srv.Close()

	clt := newC(srv.URL, "v1", BasicAuth("user", "pass"))
	values := url.Values{"a": []string{"b"}}
	out, err := clt.PostForm(clt.Endpoint("a", "b"), values)

	c.Assert(err, IsNil)
	c.Assert(string(out.Bytes()), Equals, "hello back")
	c.Assert(u.String(), DeepEquals, "/v1/a/b")
	c.Assert(form, DeepEquals, values)
	c.Assert(method, Equals, "POST")
	c.Assert(user, DeepEquals, "user")
	c.Assert(pass, DeepEquals, "pass")
}

func (s *ClientSuite) TestAddAuth(c *C) {
	var user, pass string
	var ok bool
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok = r.BasicAuth()
		io.WriteString(w, "hello back")
	})
	defer srv.Close()

	clt := newC(srv.URL, "v1", BasicAuth("user", "pass"))
	req, err := http.NewRequest("GET", clt.Endpoint("a", "b"), nil)
	c.Assert(err, IsNil)
	clt.SetAuthHeader(req.Header)
	_, err = clt.HTTPClient().Do(req)
	c.Assert(err, IsNil)

	c.Assert(user, DeepEquals, "user")
	c.Assert(pass, DeepEquals, "pass")
}

func (s *ClientSuite) TestPostJSON(c *C) {
	var data interface{}
	var user, pass string
	var ok bool
	var method string
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		user, pass, ok = r.BasicAuth()
		err := json.NewDecoder(r.Body).Decode(&data)
		c.Assert(err, IsNil)
	})
	defer srv.Close()

	clt := newC(srv.URL, "v1", BasicAuth("user", "pass"))

	values := map[string]interface{}{"hello": "there"}
	_, err := clt.PostJSON(clt.Endpoint("a", "b"), values)

	c.Assert(err, IsNil)
	c.Assert(method, Equals, "POST")
	c.Assert(user, DeepEquals, "user")
	c.Assert(pass, DeepEquals, "pass")
	c.Assert(data, DeepEquals, values)

	values = map[string]interface{}{"hello": "there, put"}
	_, err = clt.PutJSON(clt.Endpoint("a", "b"), values)

	c.Assert(err, IsNil)
	c.Assert(method, Equals, "PUT")
	c.Assert(user, DeepEquals, "user")
	c.Assert(pass, DeepEquals, "pass")
	c.Assert(data, DeepEquals, values)
}

func (s *ClientSuite) TestDelete(c *C) {
	var method string
	var user, pass string
	var ok bool
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok = r.BasicAuth()
		method = r.Method
	})
	defer srv.Close()

	clt := newC(srv.URL, "v1", BasicAuth("user", "pass"))
	re, err := clt.Delete(clt.Endpoint("a", "b"))
	c.Assert(err, IsNil)
	c.Assert(method, Equals, "DELETE")
	c.Assert(re.Code(), Equals, http.StatusOK)
	c.Assert(user, DeepEquals, "user")
	c.Assert(pass, DeepEquals, "pass")
}

func (s *ClientSuite) TestGet(c *C) {
	var method string
	var query url.Values
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		query = r.URL.Query()
	})
	defer srv.Close()

	clt := newC(srv.URL, "v1")
	values := url.Values{"q": []string{"1", "2"}}
	clt.Get(clt.Endpoint("a", "b"), values)
	c.Assert(method, Equals, "GET")
	c.Assert(query, DeepEquals, values)
}

func (s *ClientSuite) TestGetFile(c *C) {
	fileName := filepath.Join(c.MkDir(), "file.txt")
	err := ioutil.WriteFile(fileName, []byte("hello there"), 0666)
	c.Assert(err, IsNil)
	var user, pass string
	var ok bool
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok = r.BasicAuth()
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%v`, "file.txt"))
		http.ServeFile(w, r, fileName)
	})
	defer srv.Close()

	clt := newC(srv.URL, "v1", BasicAuth("user", "pass"))
	f, err := clt.GetFile(clt.Endpoint("download"), url.Values{})
	c.Assert(err, IsNil)
	defer f.Close()
	data, err := ioutil.ReadAll(f.Body())
	c.Assert(err, IsNil)
	c.Assert(string(data), Equals, "hello there")
	c.Assert(f.FileName(), Equals, "file.txt")
	c.Assert(user, Equals, "user")
	c.Assert(pass, Equals, "pass")
}

func (s *ClientSuite) TestReplyNotFound(c *C) {
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		ReplyJSON(w, http.StatusNotFound, map[string]interface{}{"msg": "not found"})
	})
	defer srv.Close()

	clt := newC(srv.URL, "v1")
	re, err := clt.Get(clt.Endpoint("a"), url.Values{})
	c.Assert(err, IsNil)
	c.Assert(re.Code(), Equals, http.StatusNotFound)
	c.Assert(re.Headers().Get("Content-Type"), Equals, "application/json")
}

func (s *ClientSuite) TestCustomClient(c *C) {
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
	})
	defer srv.Close()

	clt, err := NewClient(srv.URL, "v1", HTTPClient(&http.Client{Timeout: time.Millisecond}))
	c.Assert(err, IsNil)

	_, err = clt.Get(clt.Endpoint("a"), url.Values{})
	c.Assert(err, NotNil)
}

func (s *ClientSuite) TestPostMultipartForm(c *C) {
	var u *url.URL
	var params url.Values
	var method string
	var data []string
	var user, pass string
	var ok bool
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok = r.BasicAuth()
		u = r.URL
		c.Assert(r.ParseMultipartForm(64<<20), IsNil)
		params = r.Form
		method = r.Method

		c.Assert(r.MultipartForm, NotNil)
		c.Assert(len(r.MultipartForm.File["a"]), Not(Equals), 0)

		fhs := r.MultipartForm.File["a"]
		for _, fh := range fhs {
			f, err := fh.Open()
			c.Assert(err, IsNil)
			val, err := ioutil.ReadAll(f)
			c.Assert(err, IsNil)
			data = append(data, string(val))
		}

		io.WriteString(w, "hello back")

	})
	defer srv.Close()

	clt := newC(srv.URL, "v1", BasicAuth("user", "pass"))
	values := url.Values{"a": []string{"b"}}
	out, err := clt.PostForm(
		clt.Endpoint("a", "b"),
		values,
		File{
			Name:     "a",
			Filename: "a.json",
			Reader:   strings.NewReader("file 1")},
		File{
			Name:     "a",
			Filename: "a.json",
			Reader:   strings.NewReader("file 2")},
	)

	c.Assert(err, IsNil)
	c.Assert(string(out.Bytes()), Equals, "hello back")
	c.Assert(u.String(), DeepEquals, "/v1/a/b")

	c.Assert(method, Equals, "POST")
	c.Assert(params, DeepEquals, values)
	c.Assert(data, DeepEquals, []string{"file 1", "file 2"})

	c.Assert(user, Equals, "user")
	c.Assert(pass, Equals, "pass")
}

func (s *ClientSuite) TestGetBasicAuth(c *C) {
	var user, pass string
	var ok bool
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok = r.BasicAuth()
	})
	defer srv.Close()

	clt := newC(srv.URL, "v1", BasicAuth("user", "pass"))
	clt.Get(clt.Endpoint("a", "b"), url.Values{})
	c.Assert(user, DeepEquals, "user")
	c.Assert(pass, DeepEquals, "pass")
}

func newC(addr, version string, params ...ClientParam) *testClient {
	c, err := NewClient(addr, version, params...)
	if err != nil {
		panic(err)
	}
	return &testClient{*c}
}

type testClient struct {
	Client
}

func serveHandler(f http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(f))
}
