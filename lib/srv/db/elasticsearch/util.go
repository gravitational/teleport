// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package elasticsearch

import (
	"encoding/json"

	"github.com/ghodss/yaml"

	"github.com/gravitational/teleport/lib/srv/db/common"
)

// GetQueryFromRequestBody attempts to find the actual query from the request body, to be shown to the interested user.
func GetQueryFromRequestBody(e common.EngineConfig, contentType string, body []byte) string {
	// Elasticsearch APIs have no shared schema, but the ones we support have the query either
	// as 'query' or as 'knn'.
	// We will attempt to deserialize the query as 'q' to discover these fields.
	// The type for those is 'any': both strings and objects can be found.
	var q struct {
		Query any `json:"query" yaml:"query"`
		Knn   any `json:"knn" yaml:"knn"`
	}

	switch contentType {
	// CBOR and Smile are officially supported by Elasticsearch:
	// https://www.elastic.co/guide/en/elasticsearch/reference/master/api-conventions.html#_content_type_requirements
	// We don't support introspection of these content types, at least for now.
	case "application/cbor":
		e.Log.Warnf("Content type not supported: %q.", contentType)
		return ""

	case "application/smile":
		e.Log.Warnf("Content type not supported: %q.", contentType)
		return ""

	case "application/yaml":
		if len(body) == 0 {
			e.Log.WithField("content-type", contentType).Infof("Empty request body.")
			return ""
		}
		err := yaml.Unmarshal(body, &q)
		if err != nil {
			e.Log.WithError(err).Warnf("Error decoding request body as %q.", contentType)
			return ""
		}

	case "application/json":
		if len(body) == 0 {
			e.Log.WithField("content-type", contentType).Infof("Empty request body.")
			return ""
		}
		err := json.Unmarshal(body, &q)
		if err != nil {
			e.Log.WithError(err).Warnf("Error decoding request body as %q.", contentType)
			return ""
		}

	default:
		e.Log.Warnf("Unknown or missing 'Content-Type': %q, assuming 'application/json'.", contentType)
		if len(body) == 0 {
			e.Log.WithField("content-type", contentType).Infof("Empty request body.")
			return ""
		}

		err := json.Unmarshal(body, &q)
		if err != nil {
			e.Log.WithError(err).Warnf("Error decoding request body as %q.", contentType)
			return ""
		}
	}

	result := q.Query
	if result == nil {
		result = q.Knn
	}

	if result == nil {
		return ""
	}

	switch qt := result.(type) {
	case string:
		return qt
	default:
		marshal, err := json.Marshal(result)
		if err != nil {
			e.Log.WithError(err).Warnf("Error encoding query to json; body: %x, content type: %v.", body, contentType)
			return ""
		}
		return string(marshal)
	}
}
