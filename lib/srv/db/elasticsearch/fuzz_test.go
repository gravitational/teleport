/*
Copyright 2022 Gravitational, Inc.

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

package elasticsearch

import (
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func FuzzGetQueryFromRequestBody(f *testing.F) {
	// unit test examples
	f.Add([]byte("{\"query\":{\"bool\":{\"must\":{\"term\":{\"user.id\":\"pam\"}}," +
		"\"filter\":{\"term\":{\"tags\":\"production\"}}}}}"))
	f.Add([]byte("{\n  \"query\": \"SELECT * FROM library ORDER BY page_count DESC LIMIT 5\"\n}"))
	f.Add([]byte("{\"knn\":{\"field\":\"image_vector\",\"query_vector\":[0.3,0.1,1.2]," +
		"\"k\":10,\"num_candidates\":100},\"_source\":[\"name\",\"file_type\"]}"))
	f.Add([]byte("_source:\n- name\n- file_type\n" +
		"knn:\n  field: image_vector\n  k: 10\n  num_candidates: 100\n  query_vector:\n  - 0.3\n  - 0.1\n  - 1.2"))
	f.Add([]byte("query:\n  bool:\n    filter:\n      term:\n        tags: production\n    must:\n      term:\n        user.id: pam"))
	f.Add([]byte("query: SELECT * FROM library ORDER BY page_count DESC LIMIT 5"))
	f.Add([]byte("{ \"query\": \"SELECT 42\" }"))

	mkEngine := func() *Engine {
		e := &Engine{}
		log := logrus.New()
		log.SetOutput(io.Discard)
		e.Log = log
		return e
	}

	f.Fuzz(func(t *testing.T, body []byte) {
		require.NotPanics(t, func() {
			GetQueryFromRequestBody(mkEngine().EngineConfig, "application/yaml", body)
			GetQueryFromRequestBody(mkEngine().EngineConfig, "application/json", body)
		})
	})
}

func FuzzPathToMatcher(f *testing.F) {
	f.Add("/_security/foo")
	f.Add("/_ssl/asd")
	f.Add("/_search/")
	f.Add("/_async_search/")
	f.Add("/_pit/")
	f.Add("/_msearch/")
	f.Add("/_render/")
	f.Add("/_field_caps/")
	f.Add("/_sql/")
	f.Add("/_eql/")

	f.Add("/target/_search")
	f.Add("/target/_async_search")
	f.Add("/target/_pit")
	f.Add("/target/_knn_search")
	f.Add("/target/_msearch")
	f.Add("/target/_search_shards")
	f.Add("/target/_count")
	f.Add("/target/_validate")
	f.Add("/target/_terms_enum")
	f.Add("/target/_explain")
	f.Add("/target/_field_caps")
	f.Add("/target/_rank_eval")
	f.Add("/target/_mvt")

	f.Fuzz(func(t *testing.T, path string) {
		require.NotPanics(t, func() {
			parsePath(path)
		})
	})
}
