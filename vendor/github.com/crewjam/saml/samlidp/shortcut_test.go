package samlidp

import (
	"net/http"
	"net/http/httptest"
	"strings"

	. "gopkg.in/check.v1"
)

func (test *ServerTest) TestShortcutsCrud(c *C) {
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "https://idp.example.com/shortcuts/", nil)
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusOK)
	c.Assert(string(w.Body.Bytes()), Equals, "{\"shortcuts\":[]}\n")

	w = httptest.NewRecorder()
	r, _ = http.NewRequest("PUT", "https://idp.example.com/shortcuts/bob",
		strings.NewReader("{\"url_suffix_as_relay_state\": true, \"service_provider\": \"https://example.com/saml2/metadata\"}"))
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusNoContent)

	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "https://idp.example.com/shortcuts/bob", nil)
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusOK)
	c.Assert(string(w.Body.Bytes()), Equals, "{\"name\":\"bob\",\"service_provider\":\"https://example.com/saml2/metadata\",\"url_suffix_as_relay_state\":true}\n")

	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "https://idp.example.com/shortcuts/", nil)
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusOK)
	c.Assert(string(w.Body.Bytes()), Equals, "{\"shortcuts\":[\"bob\"]}\n")

	w = httptest.NewRecorder()
	r, _ = http.NewRequest("DELETE", "https://idp.example.com/shortcuts/bob", nil)
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusNoContent)

	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "https://idp.example.com/shortcuts/", nil)
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusOK)
	c.Assert(string(w.Body.Bytes()), Equals, "{\"shortcuts\":[]}\n")
}

func (test *ServerTest) TestShortcut(c *C) {
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("PUT", "https://idp.example.com/shortcuts/bob",
		strings.NewReader("{\"url_suffix_as_relay_state\": true, \"service_provider\": \"https://sp.example.com/saml2/metadata\"}"))
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusNoContent)

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

	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "https://idp.example.com/login/bob/whoami", nil)
	r.Header.Set("Cookie", "session=AAIEBggKDA4QEhQWGBocHiAiJCYoKiwuMDI0Njg6PD4=")
	test.Server.ServeHTTP(w, r)
	c.Assert(w.Code, Equals, http.StatusOK)
	body := string(w.Body.Bytes())
	c.Assert(strings.Contains(body, "<input type=\"hidden\" name=\"RelayState\" value=\"/whoami\" />"), Equals, true)
	c.Assert(strings.Contains(body, "<script>document.getElementById('SAMLResponseForm').submit();</script>"), Equals, true)
}
