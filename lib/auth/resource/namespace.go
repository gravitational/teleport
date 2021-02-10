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
	"fmt"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// NamespaceSpecSchema is JSON schema for NameSpace resource spec
const NamespaceSpecSchema = `{
	"type": "object",
	"additionalProperties": false,
	"default": {}
  }`

// NamespaceSchemaTemplate is JSON schema for the Namespace resource
const NamespaceSchemaTemplate = `{
	"type": "object",
	"additionalProperties": false,
	"default": {},
	"required": ["kind", "spec", "metadata"],
	"properties": {
	  "kind": {"type": "string"},
	  "version": {"type": "string", "default": "v1"},
	  "metadata": %v,
	  "spec": %v
	}
  }`

// GetNamespaceSchema returns Namespace schema
func GetNamespaceSchema() string {
	return fmt.Sprintf(NamespaceSchemaTemplate, MetadataSchema, NamespaceSpecSchema)
}

// MarshalNamespace marshals the Namespace resource to JSON.
func MarshalNamespace(resource Namespace, opts ...auth.MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cfg.PreserveResourceID {
		// avoid modifying the original object
		// to prevent unexpected data races
		copy := resource
		copy.SetResourceID(0)
		resource = copy
	}
	return utils.FastMarshal(resource)
}

// UnmarshalNamespace unmarshals the Namespace resource from JSON.
func UnmarshalNamespace(data []byte, opts ...auth.MarshalOption) (*Namespace, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing namespace data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// always skip schema validation on namespaces unmarshal
	// the namespace is always created by teleport now
	var namespace Namespace
	if err := utils.FastUnmarshal(data, &namespace); err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	if err := namespace.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		namespace.Metadata.ID = cfg.ID
	}
	if !cfg.Expires.IsZero() {
		namespace.Metadata.Expires = &cfg.Expires
	}

	return &namespace, nil
}
