/*
Copyright 2016 Gravitational, Inc.

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

package httplib

import (
	"net/http"
	"testing"

	"github.com/julienschmidt/httprouter"
	. "gopkg.in/check.v1"
)

func TestHTTP(t *testing.T) { TestingT(t) }

type HTTPSuite struct {
}

var _ = Suite(&HTTPSuite{})

func (s *HTTPSuite) TestRewritePaths(c *C) {
	RewritePaths(newTestHandler(), Rewrite("/v1/sessions/([^/]+)/stream", "/v1/namespaces/default/sessions/1/stream"))
}

type testHandler struct {
	httprouter.Router
	capturedNamespace string
	capturedID        string
}

func newTestHandler() http.Handler {
	h := &testHandler{}
	h.Router = *httprouter.New()
	h.POST("/v1/sessions/:id/stream", MakeHandler(h.postSessionChunkOriginal))
	h.POST("/v1/namespaces/:namespace/sessions/:id/stream", MakeHandler(h.postSessionChunkNamespace))
	return h
}

func (h *testHandler) postSessionChunkOriginal(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	return "ok", nil
}

func (h *testHandler) postSessionChunkNamespace(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	h.capturedNamespace = p.ByName("namespace")
	h.capturedID = p.ByName("id")
	return "ok", nil
}
