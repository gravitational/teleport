/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package local

import (
	"context"
	"errors"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	beamPrefix      = "beams"
	beamAliasPrefix = "beams_alias"
)

// BeamService manages Beam resources in the backend.
type BeamService struct {
	backend backend.Backend
	svc     *generic.ServiceWrapper[*beamsv1.Beam]
}

// NewBeamService creates a new BeamService.
func NewBeamService(b backend.Backend) (*BeamService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*beamsv1.Beam]{
			Backend:       b,
			ResourceKind:  types.KindBeam,
			BackendPrefix: backend.NewKey(beamPrefix),
			MarshalFunc:   services.MarshalProtoResource[*beamsv1.Beam],
			UnmarshalFunc: services.UnmarshalProtoResource[*beamsv1.Beam],
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &BeamService{
		backend: b,
		svc:     service,
	}, nil
}

// GetBeam returns the specified Beam resource.
func (s *BeamService) GetBeam(ctx context.Context, name string) (*beamsv1.Beam, error) {
	item, err := s.svc.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return item, nil
}

// GetBeamByAlias returns the specified Beam resource by alias.
func (s *BeamService) GetBeamByAlias(ctx context.Context, alias string) (*beamsv1.Beam, error) {
	if alias == "" {
		return nil, trace.BadParameter("alias: must be non-empty")
	}

	item, err := s.backend.Get(ctx, beamAliasKey(alias))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("beam %+q doesn't exist", alias)
		}
		return nil, trace.Wrap(err)
	}

	return s.GetBeam(ctx, string(item.Value))
}

// ListBeams returns a paginated list of Beam resources.
func (s *BeamService) ListBeams(ctx context.Context, pageSize int, pageToken string) ([]*beamsv1.Beam, string, error) {
	items, nextKey, err := s.svc.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return items, nextKey, nil
}

// CreateBeam atomically writes the beam and its supporting resources to the
// backend. If the beam's alias is already in-use, or any other resource name
// conflicts, an AlreadyExists error will be returned, and the caller should
// generate a new alias and resource names and try again.
//
// This function should be called before the actual VM is provisioned so that
// if a subsequent operation fails, we maintain a record of it, and can clean
// the VM up later.
func (s *BeamService) CreateBeam(ctx context.Context, p services.CreateBeamParams) (*beamsv1.Beam, error) {
	if err := p.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}

	aliasItem := itemFromBeamAlias(p.Beam)

	beamItem, err := itemFromBeam(p.Beam)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	botUserItem, err := itemFromUser(p.BotUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	botRoleItem, err := itemFromRole(p.BotRole)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	workloadIdentityItem, err := itemFromWorkloadIdentity(p.WorkloadIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	actions := []backend.ConditionalAction{
		{
			Key:       aliasItem.Key,
			Condition: backend.NotExists(),
			Action:    backend.Put(aliasItem),
		},
		{
			Key:       beamItem.Key,
			Condition: backend.Whatever(),
			Action:    backend.Put(*beamItem),
		},
		{
			Key:       botUserItem.Key,
			Condition: backend.Whatever(),
			Action:    backend.Put(*botUserItem),
		},
		{
			Key:       botRoleItem.Key,
			Condition: backend.Whatever(),
			Action:    backend.Put(*botRoleItem),
		},
		{
			Key:       workloadIdentityItem.Key,
			Condition: backend.Whatever(),
			Action:    backend.Put(*workloadIdentityItem),
		},
	}

	tokenActions, err := upsertProvisionTokenActions(p.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	actions = append(actions, tokenActions...)

	rev, err := s.backend.AtomicWrite(ctx, actions)
	switch {
	case errors.Is(err, backend.ErrConditionFailed):
		return nil, trace.AlreadyExists("beam alias or resource name already in-use")
	case err != nil:
		return nil, trace.Wrap(err)
	}

	created := proto.CloneOf(p.Beam)
	created.Metadata.Revision = rev
	return created, nil
}

// UpdateBeamCreateNode atomically writes the beam and node to the backend.
// It is used to "finalize" the creation of the beam.
func (s *BeamService) UpdateBeamCreateNode(ctx context.Context, beam *beamsv1.Beam, node types.Server) (*beamsv1.Beam, error) {
	nodeItem, err := itemFromNode(node)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return s.updateBeam(ctx, beam, []backend.ConditionalAction{
		{
			Key:       backend.NewKey(nodesPrefix, apidefaults.Namespace, node.GetName()),
			Condition: backend.Whatever(),
			Action:    backend.Put(*nodeItem),
		},
	})
}

// UpdateBeamCreateApp atomically writes the beam and app to the backend. It is
// used to "publish" the beam.
func (s *BeamService) UpdateBeamCreateApp(ctx context.Context, beam *beamsv1.Beam, app types.Application) (*beamsv1.Beam, error) {
	appItem, err := itemFromApp(app)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return s.updateBeam(ctx, beam, []backend.ConditionalAction{
		{
			Key:       backend.NewKey(appPrefix, app.GetName()),
			Condition: backend.Whatever(),
			Action:    backend.Put(*appItem),
		},
	})
}

// UpdateBeamDeleteApp atomically writes the beam and deletes its app from
// the backend. It is used to "unpublish" the beam.
func (s *BeamService) UpdateBeamDeleteApp(ctx context.Context, beam *beamsv1.Beam, appName string) (*beamsv1.Beam, error) {
	if appName == "" {
		return nil, trace.BadParameter("app name is required")
	}
	return s.updateBeam(ctx, beam, []backend.ConditionalAction{
		{
			Key:       backend.NewKey(appPrefix, appName),
			Condition: backend.Whatever(),
			Action:    backend.Delete(),
		},
	})
}

func (s *BeamService) updateBeam(ctx context.Context, beam *beamsv1.Beam, actions []backend.ConditionalAction) (*beamsv1.Beam, error) {
	if err := services.ValidateBeam(beam); err != nil {
		return nil, trace.Wrap(err)
	}

	beamItem, err := itemFromBeam(beam)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	actions = append(actions, backend.ConditionalAction{
		Key:       beamKey(beam.GetMetadata().GetName()),
		Condition: backend.Revision(beam.GetMetadata().GetRevision()),
		Action:    backend.Put(*beamItem),
	})

	rev, err := s.backend.AtomicWrite(ctx, actions)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updated := proto.CloneOf(beam)
	updated.Metadata.Revision = rev
	return updated, nil
}

// DeleteBeam atomically deletes the beam and its supporting resources from
// the backend. It should not be called until the VM has been cleaned up.
func (s *BeamService) DeleteBeam(ctx context.Context, name string) error {
	beam, err := s.GetBeam(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(boxofrad): Clean up DelegationSession once #64772 is merged.
	actions := []backend.ConditionalAction{
		{
			Key:       beamAliasKey(beam.GetStatus().GetAlias()),
			Condition: backend.Whatever(),
			Action:    backend.Delete(),
		},
		{
			Key:       beamKey(beam.GetMetadata().GetName()),
			Condition: backend.Revision(beam.GetMetadata().GetRevision()),
			Action:    backend.Delete(),
		},
		{
			Key:       backend.NewKey(tokensPrefix, beam.GetStatus().GetJoinTokenName()),
			Condition: backend.Whatever(),
			Action:    backend.Delete(),
		},
		{
			Key:       backend.NewKey(webPrefix, usersPrefix, services.BotResourceName(beam.GetStatus().GetBotName()), paramsPrefix),
			Condition: backend.Whatever(),
			Action:    backend.Delete(),
		},
		{
			Key:       backend.NewKey(rolesPrefix, services.BotResourceName(beam.GetStatus().GetBotName()), paramsPrefix),
			Condition: backend.Whatever(),
			Action:    backend.Delete(),
		},
		{
			Key:       backend.NewKey(workloadIdentityPrefix, beam.GetStatus().GetWorkloadIdentityName()),
			Condition: backend.Whatever(),
			Action:    backend.Delete(),
		},
	}

	if v := beam.GetStatus().GetNodeId(); v != "" {
		actions = append(actions, backend.ConditionalAction{
			Key:       backend.NewKey(nodesPrefix, apidefaults.Namespace, v),
			Condition: backend.Whatever(),
			Action:    backend.Delete(),
		})
	}

	if v := beam.GetStatus().GetAppName(); v != "" {
		actions = append(actions, backend.ConditionalAction{
			Key:       backend.NewKey(appPrefix, v),
			Condition: backend.Whatever(),
			Action:    backend.Delete(),
		})
	}

	_, err = s.backend.AtomicWrite(ctx, actions)
	return trace.Wrap(err)
}

func beamKey(name string) backend.Key {
	return backend.NewKey(beamPrefix, name)
}

func beamAliasKey(alias string) backend.Key {
	return backend.NewKey(beamAliasPrefix, alias)
}

func itemFromBeamAlias(beam *beamsv1.Beam) backend.Item {
	alias := beam.GetStatus().GetAlias()
	meta := beam.GetMetadata()

	return backend.Item{
		Key:     beamAliasKey(alias),
		Value:   []byte(meta.GetName()),
		Expires: meta.GetExpires().AsTime(),
	}
}

func itemFromBeam(beam *beamsv1.Beam) (*backend.Item, error) {
	meta := beam.GetMetadata()

	value, err := services.MarshalProtoResource(beam)
	if err != nil {
		return nil, err
	}

	return &backend.Item{
		Key:      beamKey(meta.GetName()),
		Value:    value,
		Expires:  meta.GetExpires().AsTime(),
		Revision: meta.GetRevision(),
	}, nil
}
