// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package local

import (
	"strings"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

type identityCenterAccountParser struct {
	baseParser
	prefix backend.Key
}

func newIdentityCenterAccountParser() *identityCenterAccountParser {
	prefix := backend.NewKey(awsResourcePrefix, awsAccountPrefix)
	return &identityCenterAccountParser{
		baseParser: newBaseParser(prefix),
		prefix:     prefix,
	}
}

func (p *identityCenterAccountParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		name := event.Item.Key.TrimPrefix(p.prefix).String()
		if name == "" {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key.String())
		}
		return &types.ResourceHeader{
			Kind:    types.KindIdentityCenterAccount,
			Version: types.V1,
			Metadata: types.Metadata{
				Name:      strings.TrimPrefix(name, backend.SeparatorString),
				Namespace: apidefaults.Namespace,
			},
		}, nil
	case types.OpPut:
		r, err := services.UnmarshalProtoResource[*identitycenterv1.Account](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(r), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

type identityCenterPrincipalAssignmentParser struct {
	baseParser
	prefix backend.Key
}

func newIdentityCenterPrincipalAssignmentParser() *identityCenterPrincipalAssignmentParser {
	prefix := backend.NewKey(awsResourcePrefix, awsPrincipalAssignmentPrefix)
	return &identityCenterPrincipalAssignmentParser{
		baseParser: newBaseParser(prefix),
		prefix:     prefix,
	}
}

func (p *identityCenterPrincipalAssignmentParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		name := event.Item.Key.TrimPrefix(p.prefix).String()
		if name == "" {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key.String())
		}
		return &types.ResourceHeader{
			Kind:    types.KindIdentityCenterPrincipalAssignment,
			Version: types.V1,
			Metadata: types.Metadata{
				Name:      strings.TrimPrefix(name, backend.SeparatorString),
				Namespace: apidefaults.Namespace,
			},
		}, nil

	case types.OpPut:
		r, err := services.UnmarshalProtoResource[*identitycenterv1.PrincipalAssignment](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(r), nil

	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

type identityCenterAccountAssignmentParser struct {
	baseParser
	prefix backend.Key
}

func newIdentityCenterAccountAssignmentParser() *identityCenterAccountAssignmentParser {
	prefix := backend.NewKey(awsResourcePrefix, awsAccountAssignmentPrefix)
	return &identityCenterAccountAssignmentParser{
		baseParser: newBaseParser(prefix),
		prefix:     prefix,
	}
}

func (p *identityCenterAccountAssignmentParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		name := event.Item.Key.TrimPrefix(p.prefix).String()
		if name == "" {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key.String())
		}
		return &types.ResourceHeader{
			Kind:    types.KindIdentityCenterAccountAssignment,
			Version: types.V1,
			Metadata: types.Metadata{
				Name:      strings.TrimPrefix(name, backend.SeparatorString),
				Namespace: apidefaults.Namespace,
			},
		}, nil
	case types.OpPut:
		r, err := services.UnmarshalProtoResource[*identitycenterv1.AccountAssignment](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(r),
			nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}
