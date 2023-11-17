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

package services

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// MarshalNamespace marshals the Namespace resource to JSON.
func MarshalNamespace(resource types.Namespace, opts ...MarshalOption) ([]byte, error) {
	if err := resource.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cfg.PreserveResourceID {
		// avoid modifying the original object
		// to prevent unexpected data races
		copy := resource
		copy.SetResourceID(0)
		copy.SetRevision("")
		resource = copy
	}
	return utils.FastMarshal(resource)
}

// UnmarshalNamespace unmarshals the Namespace resource from JSON.
func UnmarshalNamespace(data []byte, opts ...MarshalOption) (*types.Namespace, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing namespace data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// always skip schema validation on namespaces unmarshal
	// the namespace is always created by teleport now
	var namespace types.Namespace
	if err := utils.FastUnmarshal(data, &namespace); err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	if err := namespace.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		namespace.Metadata.ID = cfg.ID
	}
	if cfg.Revision != "" {
		namespace.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		namespace.Metadata.Expires = &cfg.Expires
	}

	return &namespace, nil
}
