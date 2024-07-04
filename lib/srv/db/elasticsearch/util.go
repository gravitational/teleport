/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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

	log := e.Log.With("content_type", contentType)

	switch contentType {
	// CBOR and Smile are officially supported by Elasticsearch:
	// https://www.elastic.co/guide/en/elasticsearch/reference/master/api-conventions.html#_content_type_requirements
	// We don't support introspection of these content types, at least for now.
	case "application/cbor":
		log.WarnContext(e.Context, "Content type not supported.")
		return ""

	case "application/smile":
		log.WarnContext(e.Context, "Content type not supported.")
		return ""

	case "application/yaml":
		if len(body) == 0 {
			log.InfoContext(e.Context, "Empty request body.")
			return ""
		}
		err := yaml.Unmarshal(body, &q)
		if err != nil {
			log.WarnContext(e.Context, "Error decoding request body.", "error", err)
			return ""
		}

	case "application/json":
		if len(body) == 0 {
			log.InfoContext(e.Context, "Empty request body.")
			return ""
		}
		err := json.Unmarshal(body, &q)
		if err != nil {
			log.WarnContext(e.Context, "Error decoding request body.", "error", err)
			return ""
		}

	default:
		log.WarnContext(e.Context, "Unknown or missing 'Content-Type', assuming 'application/json'.")
		if len(body) == 0 {
			log.InfoContext(e.Context, "Empty request body.")
			return ""
		}

		err := json.Unmarshal(body, &q)
		if err != nil {
			log.WarnContext(e.Context, "Error decoding request body.", "error", err)
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
			log.WarnContext(e.Context, "Error encoding query to json.", "body", body, "error", err)
			return ""
		}
		return string(marshal)
	}
}
