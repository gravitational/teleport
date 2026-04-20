// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package accessgraph

//go:generate go tool oapi-codegen -config ./models/oapi-codegen.cfg.yaml -o ./models/graph/models.gen.go ./openapi/models/graph.yaml
//go:generate go tool oapi-codegen -config ./models/oapi-codegen.cfg.yaml -o ./models/jsondiff/models.gen.go ./openapi/models/json-diff.yaml
//go:generate go tool oapi-codegen -config ./models/oapi-codegen.cfg.yaml -o ./models/logs/models.gen.go ./openapi/models/logs.yaml
//go:generate go tool oapi-codegen -config oapi-codegen.cfg.yaml -o client.gen.go openapi.yaml
