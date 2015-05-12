package roundtrip

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		u = r.URL
		c.Assert(r.ParseForm(), IsNil)
		form = r.Form
		method = r.Method
		io.WriteString(w, "hello back")
	})
	defer srv.Close()

	clt := newC(srv.URL, "v1")
	values := url.Values{"a": []string{"b"}}
	out, err := clt.PostForm(clt.Endpoint("a", "b"), values)

	c.Assert(err, IsNil)
	c.Assert(string(out.Bytes()), Equals, "hello back")
	c.Assert(u.String(), DeepEquals, "/v1/a/b")
	c.Assert(form, DeepEquals, values)
	c.Assert(method, Equals, "POST")
}

func (s *ClientSuite) TestDelete(c *C) {
	var method string
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
	})
	defer srv.Close()

	clt := newC(srv.URL, "v1")
	re, err := clt.Delete(clt.Endpoint("a", "b"))
	c.Assert(err, IsNil)
	c.Assert(method, Equals, "DELETE")
	c.Assert(re.Code(), Equals, http.StatusOK)
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
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
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

	clt := newC(srv.URL, "v1")
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
}

func newC(addr, version string) *testClient {
	c, err := NewClient(addr, version)
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
