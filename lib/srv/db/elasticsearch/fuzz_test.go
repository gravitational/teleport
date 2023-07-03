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
	mkEngine := func() *Engine {
		e := &Engine{}
		log := logrus.New()
		log.SetOutput(io.Discard)
		e.Log = log
		return e
	}

	f.Fuzz(func(t *testing.T, contentType string, body []byte) {
		require.NotPanics(t, func() {
			GetQueryFromRequestBody(mkEngine().EngineConfig, contentType, body)
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
