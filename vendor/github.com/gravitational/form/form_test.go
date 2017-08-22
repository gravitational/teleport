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

	. "gopkg.in/check.v1" // note that we don't vendor libraries dependencies, only end daemons deps are vendored
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
	var t time.Time
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		err = Parse(r,
			String("svar", &str),
			Int("ivar", &i),
			Duration("dvar", &d),
			Time("tvar", &t),
		)
	})
	defer srv.Close()

	dt := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	bytes, err := dt.MarshalText()
	c.Assert(err, IsNil)
	http.PostForm(srv.URL, url.Values{
		"svar": []string{"hello"},
		"ivar": []string{"77"},
		"dvar": []string{"100s"},
		"tvar": []string{string(bytes)},
	})

	c.Assert(err, IsNil)
	c.Assert(str, Equals, "hello")
	c.Assert(i, Equals, 77)
	c.Assert(d, Equals, 100*time.Second)
	c.Assert(t, Equals, dt)
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
	var names []string
	srv := serveHandler(func(w http.ResponseWriter, r *http.Request) {
		err = Parse(r,
			FileSlice("file", &files),
		)
		c.Assert(err, IsNil)
		values = make([]string, len(files))
		names = make([]string, len(files))
		for i, f := range files {
			out, err := ioutil.ReadAll(f)
			c.Assert(err, IsNil)
			values[i] = string(out)
			names[i] = f.Name()
		}
	})
	defer srv.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// upload multiple files with the same name
	for i, data := range []string{"file 1", "file 2"} {
		w, err := writer.CreateFormFile("file", fmt.Sprintf("file%v.json", i+1))
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
	c.Assert(names, DeepEquals, []string{"file1.json", "file2.json"})
}

func serveHandler(f http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(f))
}
