/*
Copyright 2021 Gravitational, Inc.

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

package resource

import (
	"encoding/json"
	"fmt"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// GithubConnectorV3SchemaTemplate is the JSON schema for a Github connector
const GithubConnectorV3SchemaTemplate = `{
    "type": "object",
    "additionalProperties": false,
    "required": ["kind", "spec", "metadata", "version"],
    "properties": {
      "kind": {"type": "string"},
      "version": {"type": "string", "default": "v3"},
      "metadata": %v,
      "spec": %v
    }
  }`

// GithubConnectorSpecV3Schema is the JSON schema for Github connector spec
var GithubConnectorSpecV3Schema = fmt.Sprintf(`{
    "type": "object",
    "additionalProperties": false,
    "required": ["client_id", "client_secret", "redirect_url"],
    "properties": {
      "client_id": {"type": "string"},
      "client_secret": {"type": "string"},
      "redirect_url": {"type": "string"},
      "display": {"type": "string"},
      "teams_to_logins": {
        "type": "array",
        "items": %v
      }
    }
  }`, TeamMappingSchema)

// TeamMappingSchema is the JSON schema for team membership mapping
var TeamMappingSchema = `{
    "type": "object",
    "additionalProperties": false,
    "required": ["organization", "team"],
    "properties": {
      "organization": {"type": "string"},
      "team": {"type": "string"},
      "logins": {
        "type": "array",
        "items": {
            "type": "string"
        }
      },
      "kubernetes_groups": {
        "type": "array",
        "items": {
          "type": "string"
        }
      },
      "kubernetes_users": {
        "type": "array",
        "items": {
          "type": "string"
        }
      }
    }
  }`

// GetGithubConnectorSchema returns schema for Github connector
func GetGithubConnectorSchema() string {
	return fmt.Sprintf(GithubConnectorV3SchemaTemplate, MetadataSchema, GithubConnectorSpecV3Schema)
}

// UnmarshalGithubConnector unmarshals the GithubConnector resource from JSON.
func UnmarshalGithubConnector(bytes []byte) (GithubConnector, error) {
	var h ResourceHeader
	if err := json.Unmarshal(bytes, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case V3:
		var c GithubConnectorV3
		if err := utils.UnmarshalWithSchema(GetGithubConnectorSchema(), &c, bytes); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := c.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		return &c, nil
	}
	return nil, trace.BadParameter(
		"Github connector resource version %q is not supported", h.Version)
}

// MarshalGithubConnector marshals the GithubConnector resource to JSON.
func MarshalGithubConnector(githubConnector GithubConnector, opts ...auth.MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch githubConnector := githubConnector.(type) {
	case *GithubConnectorV3:
		if version := githubConnector.GetVersion(); version != V3 {
			return nil, trace.BadParameter("mismatched github connector version %v and type %T", version, githubConnector)
		}
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *githubConnector
			copy.SetResourceID(0)
			githubConnector = &copy
		}
		return utils.FastMarshal(githubConnector)
	default:
		return nil, trace.BadParameter("unrecognized github connector version %T", githubConnector)
	}
}
