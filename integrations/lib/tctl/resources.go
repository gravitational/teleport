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

package tctl

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/types"
)

func writeResourcesYAML(w io.Writer, resources []types.Resource) error {
	for i, resource := range resources {
		data, err := yaml.Marshal(resource)
		if err != nil {
			return trace.Wrap(err)
		}
		if _, err := w.Write(data); err != nil {
			return trace.Wrap(err)
		}
		if i != len(resources) {
			if _, err := io.WriteString(w, "\n---\n"); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

func readResourcesYAMLOrJSON(r io.Reader) ([]types.Resource, error) {
	var resources []types.Resource
	decoder := kyaml.NewYAMLOrJSONDecoder(r, 32768)
	for {
		var res streamResource
		err := decoder.Decode(&res)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, trace.Wrap(err)
		}
		resources = append(resources, res.Resource)
	}
	return resources, nil
}

type streamResource struct{ types.Resource }

func (res *streamResource) UnmarshalJSON(raw []byte) error {
	var header types.ResourceHeader
	if err := json.Unmarshal(raw, &header); err != nil {
		return trace.Wrap(err)
	}

	var resource types.Resource
	switch header.Kind {
	case types.KindNode:
		switch header.Version {
		case types.V2:
			resource = &types.ServerV2{}
		default:
			return trace.BadParameter("unsupported resource version %s", header.Version)
		}
	case types.KindUser:
		switch header.Version {
		case types.V2:
			resource = &types.UserV2{}
		default:
			return trace.BadParameter("unsupported resource version %s", header.Version)
		}
	case types.KindRole:
		switch header.Version {
		case types.V4, types.V5, types.V6, types.V7, types.V8:
			resource = &types.RoleV6{}
		default:
			return trace.BadParameter("unsupported resource version %s", header.Version)
		}
	case types.KindCertAuthority:
		switch header.Version {
		case types.V2:
			resource = &types.CertAuthorityV2{}
		default:
			return trace.BadParameter("unsupported resource version %s", header.Version)
		}
	default:
		return trace.BadParameter("unsupported resource kind %s", header.Kind)
	}

	if err := json.Unmarshal(raw, resource); err != nil {
		return trace.Wrap(err)
	}

	res.Resource = resource
	return nil
}
