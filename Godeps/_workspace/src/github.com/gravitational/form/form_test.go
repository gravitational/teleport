package form

import (
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

func serveHandler(f http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(f))
}
