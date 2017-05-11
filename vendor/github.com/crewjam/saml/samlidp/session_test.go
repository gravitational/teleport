package samlidp

import (
	"net/http"
	"net/http/httptest"
	"strings"

	. "gopkg.in/check.v1"
)

func (test *ServerTest) TestSessionsCrud(c *C) {
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "https://idp.example.com/sessions/", nil)
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusOK)
	c.Assert(string(w.Body.Bytes()), Equals, "{\"sessions\":[]}\n")

	w = httptest.NewRecorder()
	r, _ = http.NewRequest("PUT", "https://idp.example.com/users/alice",
		strings.NewReader(`{"name": "alice", "password": "hunter2"}`+"\n"))
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusNoContent)

	w = httptest.NewRecorder()
	r, _ = http.NewRequest("POST", "https://idp.example.com/login",
		strings.NewReader("user=alice&password=hunter2"))
	r.Header.Set("Content-type", "application/x-www-form-urlencoded")
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusOK)
	c.Assert(w.Header().Get("Set-Cookie"), Equals, "session=AAIEBggKDA4QEhQWGBocHiAiJCYoKiwuMDI0Njg6PD4=; Path=/; Max-Age=3600")
	c.Assert(string(w.Body.Bytes()), Equals,
		"{\"ID\":\"AAIEBggKDA4QEhQWGBocHiAiJCYoKiwuMDI0Njg6PD4=\",\"CreateTime\":\"2015-12-01T01:57:09Z\",\"ExpireTime\":\"2015-12-01T02:57:09Z\",\"Index\":\"40424446484a4c4e50525456585a5c5e60626466686a6c6e70727476787a7c7e\",\"NameID\":\"\",\"Groups\":null,\"UserName\":\"alice\",\"UserEmail\":\"\",\"UserCommonName\":\"\",\"UserSurname\":\"\",\"UserGivenName\":\"\"}\n")

	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "https://idp.example.com/login", nil)
	r.Header.Set("Cookie", "session=AAIEBggKDA4QEhQWGBocHiAiJCYoKiwuMDI0Njg6PD4=")
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusOK)
	c.Assert(string(w.Body.Bytes()), Equals,
		"{\"ID\":\"AAIEBggKDA4QEhQWGBocHiAiJCYoKiwuMDI0Njg6PD4=\",\"CreateTime\":\"2015-12-01T01:57:09Z\",\"ExpireTime\":\"2015-12-01T02:57:09Z\",\"Index\":\"40424446484a4c4e50525456585a5c5e60626466686a6c6e70727476787a7c7e\",\"NameID\":\"\",\"Groups\":null,\"UserName\":\"alice\",\"UserEmail\":\"\",\"UserCommonName\":\"\",\"UserSurname\":\"\",\"UserGivenName\":\"\"}\n")

	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "https://idp.example.com/sessions/AAIEBggKDA4QEhQWGBocHiAiJCYoKiwuMDI0Njg6PD4=", nil)
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusOK)
	c.Assert(string(w.Body.Bytes()), Equals,
		"{\"ID\":\"AAIEBggKDA4QEhQWGBocHiAiJCYoKiwuMDI0Njg6PD4=\",\"CreateTime\":\"2015-12-01T01:57:09Z\",\"ExpireTime\":\"2015-12-01T02:57:09Z\",\"Index\":\"40424446484a4c4e50525456585a5c5e60626466686a6c6e70727476787a7c7e\",\"NameID\":\"\",\"Groups\":null,\"UserName\":\"alice\",\"UserEmail\":\"\",\"UserCommonName\":\"\",\"UserSurname\":\"\",\"UserGivenName\":\"\"}\n")

	w = httptest.NewRecorder()
	r, _ = http.NewRequest("DELETE", "https://idp.example.com/sessions/AAIEBggKDA4QEhQWGBocHiAiJCYoKiwuMDI0Njg6PD4=", nil)
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusNoContent)

	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "https://idp.example.com/sessions/", nil)
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusOK)
	c.Assert(string(w.Body.Bytes()), Equals, "{\"sessions\":[]}\n")

}
