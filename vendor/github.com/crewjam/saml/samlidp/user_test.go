package samlidp

import (
	"net/http"
	"net/http/httptest"
	"strings"

	. "gopkg.in/check.v1"
)

func (test *ServerTest) TestUsersCrud(c *C) {
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "https://idp.example.com/users/", nil)
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusOK)
	c.Assert(string(w.Body.Bytes()), Equals, "{\"users\":[]}\n")

	w = httptest.NewRecorder()
	r, _ = http.NewRequest("PUT", "https://idp.example.com/users/alice",
		strings.NewReader(`{"name": "alice", "password": "hunter2"}`+"\n"))
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusNoContent)

	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "https://idp.example.com/users/alice", nil)
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusOK)
	c.Assert(string(w.Body.Bytes()), Equals, "{\"name\":\"alice\"}\n")

	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "https://idp.example.com/users/", nil)
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusOK)
	c.Assert(string(w.Body.Bytes()), Equals, "{\"users\":[\"alice\"]}\n")

	w = httptest.NewRecorder()
	r, _ = http.NewRequest("DELETE", "https://idp.example.com/users/alice", nil)
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusNoContent)

	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "https://idp.example.com/users/", nil)
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusOK)
	c.Assert(string(w.Body.Bytes()), Equals, "{\"users\":[]}\n")
}
