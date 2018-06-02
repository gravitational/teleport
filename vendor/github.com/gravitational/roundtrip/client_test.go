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
	"bytes"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "gopkg.in/check.v1" // note that we don't vendor libraries dependencies, only end daemons deps are vendored
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
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		user, pass, ok = r.BasicAuth()
		c.Assert(ok, Equals, true)
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
	c.Assert(method, Equals, http.MethodPost)
	c.Assert(user, DeepEquals, "user")
	c.Assert(pass, DeepEquals, "pass")
}

func (s *ClientSuite) TestAddAuth(c *C) {
	var user, pass string
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		user, pass, ok = r.BasicAuth()
		c.Assert(ok, Equals, true)
		io.WriteString(w, "hello back")
	})
	defer srv.Close()

	clt := newC(srv.URL, "v1", BasicAuth("user", "pass"))
	req, err := http.NewRequest(http.MethodGet, clt.Endpoint("a", "b"), nil)
	c.Assert(err, IsNil)
	clt.SetAuthHeader(req.Header)
	_, err = clt.HTTPClient().Do(req)
	c.Assert(err, IsNil)

	c.Assert(user, DeepEquals, "user")
	c.Assert(pass, DeepEquals, "pass")
}

func (s *ClientSuite) TestPostPutPatchJSON(c *C) {
	var data interface{}
	var user, pass string
	var method string
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		method = r.Method
		user, pass, ok = r.BasicAuth()
		c.Assert(ok, Equals, true)
		err := json.NewDecoder(r.Body).Decode(&data)
		c.Assert(err, IsNil)
	})
	defer srv.Close()

	clt := newC(srv.URL, "v1", BasicAuth("user", "pass"))

	values := map[string]interface{}{"hello": "there"}
	_, err := clt.PostJSON(clt.Endpoint("a", "b"), values)

	c.Assert(err, IsNil)
	c.Assert(method, Equals, http.MethodPost)
	c.Assert(user, DeepEquals, "user")
	c.Assert(pass, DeepEquals, "pass")
	c.Assert(data, DeepEquals, values)

	values = map[string]interface{}{"hello": "there, put"}
	_, err = clt.PutJSON(clt.Endpoint("a", "b"), values)

	c.Assert(err, IsNil)
	c.Assert(method, Equals, http.MethodPut)
	c.Assert(user, DeepEquals, "user")
	c.Assert(pass, DeepEquals, "pass")
	c.Assert(data, DeepEquals, values)

	values = map[string]interface{}{"hello": "there,patch"}
	_, err = clt.PatchJSON(clt.Endpoint("a", "b"), values)

	c.Assert(err, IsNil)
	c.Assert(method, Equals, http.MethodPatch)
	c.Assert(user, DeepEquals, "user")
	c.Assert(pass, DeepEquals, "pass")
	c.Assert(data, DeepEquals, values)
}

func (s *ClientSuite) TestDelete(c *C) {
	var method string
	var user, pass string
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ = r.BasicAuth()
		method = r.Method
	})
	defer srv.Close()

	clt := newC(srv.URL, "v1", BasicAuth("user", "pass"))
	re, err := clt.Delete(clt.Endpoint("a", "b"))
	c.Assert(err, IsNil)
	c.Assert(method, Equals, http.MethodDelete)
	c.Assert(re.Code(), Equals, http.StatusOK)
	c.Assert(user, DeepEquals, "user")
	c.Assert(pass, DeepEquals, "pass")
}

func (s *ClientSuite) TestDeleteP(c *C) {
	var method string
	var user, pass string
	var query url.Values
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ = r.BasicAuth()
		method = r.Method
		query = r.URL.Query()
	})
	defer srv.Close()

	clt := newC(srv.URL, "v1", BasicAuth("user", "pass"))
	values := url.Values{"force": []string{"true"}}
	re, err := clt.DeleteWithParams(clt.Endpoint("a", "b"), values)
	c.Assert(err, IsNil)
	c.Assert(method, Equals, http.MethodDelete)
	c.Assert(re.Code(), Equals, http.StatusOK)
	c.Assert(user, DeepEquals, "user")
	c.Assert(pass, DeepEquals, "pass")
	c.Assert(query, DeepEquals, values)
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
	c.Assert(method, Equals, http.MethodGet)
	c.Assert(query, DeepEquals, values)
}

func (s *ClientSuite) TestTracer(c *C) {
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
	})
	defer srv.Close()

	out := &bytes.Buffer{}

	clt := newC(srv.URL, "v1", Tracer(func() RequestTracer {
		return NewWriterTracer(out)
	}))
	clt.Get(clt.Endpoint("a", "b"), url.Values{"q": []string{"1", "2"}})
	c.Assert(out.String(), Matches, ".*a/b.*")
}

func (s *ClientSuite) TestGetFile(c *C) {
	fileName := filepath.Join(c.MkDir(), "file.txt")
	err := ioutil.WriteFile(fileName, []byte("hello there"), 0666)
	c.Assert(err, IsNil)
	var user, pass string
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		user, pass, ok = r.BasicAuth()
		c.Assert(ok, Equals, true)
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

func createFile(size int64) (*os.File, string) {
	randomStream, err := os.Open("/dev/urandom")
	if err != nil {
		panic(err)
	}

	out, err := ioutil.TempFile("", "")
	if err != nil {
		panic(err)
	}

	h := sha512.New()

	_, err = io.CopyN(io.MultiWriter(out, h), randomStream, size)
	if err != nil {
		panic(err)
	}

	_, err = out.Seek(0, 0)
	if err != nil {
		panic(err)
	}

	return out, fmt.Sprintf("%x", h.Sum(nil))
}

func hashOfReader(r io.Reader) string {
	h := sha512.New()
	tr := io.TeeReader(r, h)
	_, _ = io.Copy(ioutil.Discard, tr)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (s *ClientSuite) TestOpenFile(c *C) {
	var fileSize int64 = 32*1024*3 + 7
	file, hash := createFile(fileSize) // that's 3 default io.Copy buffer + some nice number to make it less aligned
	defer os.RemoveAll(file.Name())

	now := time.Now().UTC()
	var user, pass string
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		user, pass, ok = r.BasicAuth()
		c.Assert(ok, Equals, true)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%v`, file.Name()))
		http.ServeContent(w, r, file.Name(), now, file)
	})
	defer srv.Close()

	clt := newC(srv.URL, "v1", BasicAuth("user", "pass"))
	reader, err := clt.OpenFile(clt.Endpoint("download"), url.Values{})
	c.Assert(err, IsNil)
	c.Assert(hashOfReader(reader), Equals, hash)

	// seek and read again
	_, err = reader.Seek(0, 0)
	c.Assert(err, IsNil)
	c.Assert(hashOfReader(reader), Equals, hash)

	// seek to half size, concat and test resulting hash
	buf := &bytes.Buffer{}
	_, err = reader.Seek(0, 0)
	c.Assert(err, IsNil)
	_, err = io.Copy(buf, io.LimitReader(reader, fileSize/2))
	c.Assert(err, IsNil)
	_, err = reader.Seek(fileSize/2, 0)
	c.Assert(err, IsNil)
	_, err = io.Copy(buf, reader)
	c.Assert(err, IsNil)
	c.Assert(hashOfReader(buf), Equals, hash)

	c.Assert(reader.Close(), IsNil)
	// make sure that double close does not result in error
	c.Assert(reader.Close(), IsNil)
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
	files := []File{
		File{
			Name:     "a",
			Filename: "a.json",
			Reader:   strings.NewReader("file 1"),
		},
		File{
			Name:     "a",
			Filename: "b.json",
			Reader:   strings.NewReader("file 2"),
		},
	}
	expected := [][]byte{[]byte("file 1"), []byte("file 2")}
	s.testPostMultipartForm(c, files, expected)
}

func (s *ClientSuite) TestPostMultipartFormLargeFile(c *C) {
	buffer := make([]byte, 1024<<10)
	rand.Read(buffer)
	files := []File{
		File{
			Name:     "a",
			Filename: "a.json",
			Reader:   strings.NewReader("file 1"),
		},
		File{
			Name:     "a",
			Filename: "b.json",
			Reader:   strings.NewReader("file 2"),
		},
		File{
			Name:     "a",
			Filename: "c",
			Reader:   bytes.NewReader(buffer),
		},
	}
	expected := [][]byte{[]byte("file 1"), []byte("file 2"), buffer}
	s.testPostMultipartForm(c, files, expected)
}

func (s *ClientSuite) testPostMultipartForm(c *C, files []File, expected [][]byte) {
	var u *url.URL
	var params url.Values
	var method string
	var data [][]byte
	var user, pass string
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		user, pass, ok = r.BasicAuth()
		c.Assert(ok, Equals, true)
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
			data = append(data, val)
		}

		io.WriteString(w, "hello back")

	})
	defer srv.Close()

	clt := newC(srv.URL, "v1", BasicAuth("user", "pass"))
	values := url.Values{"a": []string{"b"}}
	out, err := clt.PostForm(
		clt.Endpoint("a", "b"),
		values,
		files...,
	)

	c.Assert(err, IsNil)
	c.Assert(string(out.Bytes()), Equals, "hello back")
	c.Assert(u.String(), DeepEquals, "/v1/a/b")

	c.Assert(method, Equals, http.MethodPost)
	c.Assert(params, DeepEquals, values)
	c.Assert(data, DeepEquals, expected)

	c.Assert(user, Equals, "user")
	c.Assert(pass, Equals, "pass")
}

func (s *ClientSuite) TestGetBasicAuth(c *C) {
	var user, pass string
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		user, pass, ok = r.BasicAuth()
		c.Assert(ok, Equals, true)
	})
	defer srv.Close()

	clt := newC(srv.URL, "v1", BasicAuth("user", "pass"))
	clt.Get(clt.Endpoint("a", "b"), url.Values{})
	c.Assert(user, DeepEquals, "user")
	c.Assert(pass, DeepEquals, "pass")
}

func (s *ClientSuite) TestCookies(c *C) {
	var capturedRequestCookies []*http.Cookie
	responseCookies := []*http.Cookie{
		{
			Name:  "session",
			Value: "howdy",
			Path:  "/",
		},
	}
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestCookies = r.Cookies()
		for _, c := range responseCookies {
			http.SetCookie(w, c)
		}
	})
	defer srv.Close()

	jar, err := cookiejar.New(nil)
	c.Assert(err, IsNil)
	clt := newC(srv.URL, "v1", CookieJar(jar))

	requestCookies := []*http.Cookie{
		{
			Name:  "hello",
			Value: "here?",
			Path:  "/",
		},
	}
	u, err := url.Parse(srv.URL)
	c.Assert(err, IsNil)
	jar.SetCookies(u, requestCookies)

	re, err := clt.Get(clt.Endpoint("test"), url.Values{})
	c.Assert(err, IsNil)

	c.Assert(len(capturedRequestCookies), Equals, len(requestCookies))
	c.Assert(capturedRequestCookies[0].Name, DeepEquals, requestCookies[0].Name)
	c.Assert(capturedRequestCookies[0].Value, DeepEquals, requestCookies[0].Value)

	c.Assert(len(re.Cookies()), DeepEquals, len(responseCookies))
	c.Assert(re.Cookies()[0].Name, DeepEquals, responseCookies[0].Name)
	c.Assert(re.Cookies()[0].Value, DeepEquals, responseCookies[0].Value)
}

func (s *ClientSuite) TestEndpoint(c *C) {
	client := newC("http://localhost", "v1")
	c.Assert(client.Endpoint("api", "resource"), Equals, "http://localhost/v1/api/resource")
	client = newC("http://localhost", "")
	c.Assert(client.Endpoint("api", "resource"), Equals, "http://localhost/api/resource")
}

func (s *ClientSuite) TestLimitsWrites(c *C) {
	var buf bytes.Buffer
	w := &limitWriter{&buf, 10}
	input := []byte("The quick brown fox jumps over the lazy dog")
	r := bytes.NewReader(input)
	_, err := io.Copy(w, r)
	c.Assert(err, Equals, errShortWrite)
	c.Assert(buf.Bytes(), DeepEquals, input[:10])
	out, err := ioutil.ReadAll(r)
	c.Assert(out, DeepEquals, input[10:], Commentf("expected %q but got %q", input[10:], out))
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
