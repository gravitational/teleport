package form

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1" // note that we don't vendor libraries dependencies, only end daemons deps are vendored
)

func TestForm(t *testing.T) { TestingT(t) }

type FormSuite struct {
}

var _ = Suite(&FormSuite{})

func (s *FormSuite) TestFormOK(c *C) {
	var err error
	var str string
	var i int
	var d time.Duration
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		err = Parse(r,
			String("svar", &str),
			Int("ivar", &i),
			Duration("dvar", &d),
		)
	})
	defer srv.Close()

	http.PostForm(srv.URL, url.Values{
		"svar": []string{"hello"},
		"ivar": []string{"77"},
		"dvar": []string{"100s"},
	})

	c.Assert(err, IsNil)
	c.Assert(str, Equals, "hello")
	c.Assert(i, Equals, 77)
	c.Assert(d, Equals, 100*time.Second)
}

func (s *FormSuite) TestStringRequiredMissing(c *C) {
	var err error
	var str string
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		err = Parse(r, String("var", &str, Required()))
	})
	defer srv.Close()

	http.PostForm(srv.URL, url.Values{})

	c.Assert(err, FitsTypeOf, &MissingParameterError{})
}

func (s *FormSuite) TestIntInvalidFormat(c *C) {
	var err error
	var i int
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		err = Parse(r, Int("var", &i))
	})
	defer srv.Close()

	http.PostForm(srv.URL, url.Values{"var": []string{"hello"}})

	c.Assert(err, FitsTypeOf, &BadParameterError{})
}

func (s *FormSuite) TestDurationInvalidFormat(c *C) {
	var err error
	var d time.Duration
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		err = Parse(r, Duration("var", &d))
	})
	defer srv.Close()

	http.PostForm(srv.URL, url.Values{"var": []string{"hello"}})

	c.Assert(err, FitsTypeOf, &BadParameterError{})
}

func (s *FormSuite) TestMultipartFormOK(c *C) {
	var err error
	var str string
	var i int
	var d time.Duration
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		err = Parse(r,
			String("svar", &str),
			Int("ivar", &i),
			Duration("dvar", &d),
		)
	})
	defer srv.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	writer.WriteField("svar", "hello")
	writer.WriteField("ivar", "77")
	writer.WriteField("dvar", "100s")
	boundary := writer.Boundary()
	c.Assert(writer.Close(), IsNil)

	r, err := http.NewRequest("POST", srv.URL, body)
	c.Assert(err, IsNil)
	r.Header.Set("Content-Type", fmt.Sprintf(`multipart/form-data;boundary="%v"`, boundary))

	_, err = http.DefaultClient.Do(r)
	c.Assert(err, IsNil)

	c.Assert(str, Equals, "hello")
	c.Assert(i, Equals, 77)
	c.Assert(d, Equals, 100*time.Second)
}

func (s *FormSuite) TestStringSliceOK(c *C) {
	var err error
	var slice []string
	var empty []string
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		err = Parse(r,
			StringSlice("slice", &slice),
			StringSlice("empty", &empty),
		)
	})
	defer srv.Close()

	http.PostForm(srv.URL, url.Values{
		"slice": []string{"hello1", "hello2"},
	})

	c.Assert(err, IsNil)
	c.Assert(slice, DeepEquals, []string{"hello1", "hello2"})
	c.Assert(empty, DeepEquals, []string{})
}

func (s *FormSuite) TestFileSliceOK(c *C) {
	var err error
	var files Files
	var values []string
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		err = Parse(r,
			FileSlice("file", &files),
		)
		c.Assert(err, IsNil)
		values = make([]string, len(files))
		for i, f := range files {
			out, err := ioutil.ReadAll(f)
			c.Assert(err, IsNil)
			values[i] = string(out)
		}
	})
	defer srv.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// upload multiple files with the same name
	for _, data := range []string{"file 1", "file 2"} {
		w, err := writer.CreateFormFile("file", "file.json")
		c.Assert(err, IsNil)
		_, err = io.WriteString(w, data)
		c.Assert(err, IsNil)
	}
	boundary := writer.Boundary()
	c.Assert(writer.Close(), IsNil)

	req, err := http.NewRequest("POST", srv.URL, body)
	req.Header.Set("Content-Type",
		fmt.Sprintf(`multipart/form-data;boundary="%v"`, boundary))
	c.Assert(err, IsNil)
	_, err = http.DefaultClient.Do(req)
	c.Assert(err, IsNil)

	c.Assert(values, DeepEquals, []string{"file 1", "file 2"})
}

func serveHandler(f http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(f))
}
