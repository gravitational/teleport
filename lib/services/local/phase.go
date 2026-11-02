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
	"iter"
	"strings"

	"github.com/gravitational/trace"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const phasePrefix = "phases"

// PhaseService stores Phase resources in the backend.
type PhaseService struct {
	backend backend.Backend
	svc     *generic.ServiceWrapper[*beamsv1.Phase]
}

// NewPhaseService creates a Phase storage service.
func NewPhaseService(b backend.Backend) (*PhaseService, error) {
	svc, err := generic.NewServiceWrapper(generic.ServiceConfig[*beamsv1.Phase]{
		Backend:       b,
		ResourceKind:  types.KindPhase,
		BackendPrefix: backend.NewKey(phasePrefix),
		MarshalFunc:   services.MarshalProtoResource[*beamsv1.Phase],
		UnmarshalFunc: services.UnmarshalProtoResource[*beamsv1.Phase],
		ValidateFunc:  services.ValidatePhase,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &PhaseService{
		backend: b,
		svc:     svc,
	}, nil
}

// CreatePhase persists a new Phase and advances the graph revision.
func (s *PhaseService) CreatePhase(ctx context.Context, phase *beamsv1.Phase) (*beamsv1.Phase, error) {
	return s.writePhaseGraph(ctx, phase, backend.NotExists(), backend.Whatever())
}

// Outside the resource prefix: the guard is not a listed or watched Phase.
var phaseGraphKey = backend.NewKey("phase_graph")

// GetPhaseGraphRevision reads the optimistic concurrency guard for graph mutations.
// Read it before loading the Phases used to validate a graph change. Commit that
// change with the same revision so a concurrent graph mutation forces the caller
// to reread and revalidate. The graph and resource revisions advance atomically.
// Ordinary build status updates only advance the affected resource's revision.
func (s *PhaseService) GetPhaseGraphRevision(ctx context.Context) (string, error) {
	item, err := s.backend.Get(ctx, phaseGraphKey)
	if trace.IsNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", trace.Wrap(err)
	}
	return item.Revision, nil
}

func phaseGraphCondition(revision string) backend.Condition {
	if revision == "" {
		return backend.NotExists()
	}
	return backend.Revision(revision)
}

func phaseGraphAction(condition backend.Condition) backend.ConditionalAction {
	return backend.ConditionalAction{
		Key:       phaseGraphKey,
		Condition: condition,
		Action: backend.Put(backend.Item{
			Key:   phaseGraphKey,
			Value: []byte("1"),
		}),
	}
}

// CreatePhaseWithGraphRevision creates a Phase only if the graph has not changed.
func (s *PhaseService) CreatePhaseWithGraphRevision(ctx context.Context, phase *beamsv1.Phase, revision string) (*beamsv1.Phase, error) {
	return s.writePhaseGraph(ctx, phase, backend.NotExists(), phaseGraphCondition(revision))
}

// UpdatePhaseWithGraphRevision checks both resource and graph revisions before updating.
func (s *PhaseService) UpdatePhaseWithGraphRevision(ctx context.Context, phase *beamsv1.Phase, revision string) (*beamsv1.Phase, error) {
	return s.writePhaseGraph(ctx, phase, backend.Revision(phase.GetMetadata().GetRevision()), phaseGraphCondition(revision))
}

func (s *PhaseService) writePhaseGraph(ctx context.Context, phase *beamsv1.Phase, resourceCondition, graphCondition backend.Condition) (*beamsv1.Phase, error) {
	if err := services.ValidatePhase(phase); err != nil {
		return nil, trace.Wrap(err)
	}
	item, err := s.svc.MakeBackendItem(phase)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	revision, err := s.backend.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       item.Key,
			Condition: resourceCondition,
			Action:    backend.Put(item),
		},
		phaseGraphAction(graphCondition),
	})
	if errors.Is(err, backend.ErrConditionFailed) &&
		resourceCondition.Kind == backend.KindNotExists {
		if _, getErr := s.GetPhase(ctx, phase.GetMetadata().GetName()); getErr == nil {
			return nil, trace.AlreadyExists("Phase %q already exists", phase.GetMetadata().GetName())
		}
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	phase.GetMetadata().SetRevision(revision)
	return phase, nil
}

// GetPhase loads a Phase by name.
func (s *PhaseService) GetPhase(ctx context.Context, name string) (*beamsv1.Phase, error) {
	phase, err := s.svc.GetResource(ctx, name)
	return phase, trace.Wrap(err)
}

// ListPhases returns a page of Phases matching the supplied filters.
func (s *PhaseService) ListPhases(ctx context.Context, pageSize int, pageToken string, opts *services.ListPhasesRequestOptions) ([]*beamsv1.Phase, string, error) {
	phases, next, err := s.svc.ListResourcesWithFilter(ctx, pageSize, pageToken, services.MakePhaseFilterFunc(opts))
	return phases, next, trace.Wrap(err)
}

// RangePhases yields Phases starting at the supplied pagination token.
func (s *PhaseService) RangePhases(ctx context.Context, startToken string) iter.Seq2[*beamsv1.Phase, error] {
	return s.svc.Resources(ctx, startToken, "")
}

// UpdatePhase updates operation progress using the resource revision.
func (s *PhaseService) UpdatePhase(ctx context.Context, phase *beamsv1.Phase) (*beamsv1.Phase, error) {
	// Routine operation progress uses the resource revision only. Graph-changing
	// request claims must use UpdatePhaseWithGraphRevision after validation.
	updated, err := s.svc.ConditionalUpdateResource(ctx, phase)
	return updated, trace.Wrap(err)
}

// DeletePhase removes a Phase and advances the graph revision.
func (s *PhaseService) DeletePhase(ctx context.Context, name string) error {
	err := s.deletePhaseGraph(ctx, name, backend.Exists())
	if errors.Is(err, backend.ErrConditionFailed) {
		return trace.NotFound("Phase %q does not exist", name)
	}
	return trace.Wrap(err)
}

// DeletePhaseRevision removes a Phase only if its resource revision still matches.
func (s *PhaseService) DeletePhaseRevision(ctx context.Context, name, revision string) error {
	return s.deletePhaseGraph(ctx, name, backend.Revision(revision))
}

func (s *PhaseService) deletePhaseGraph(ctx context.Context, name string, condition backend.Condition) error {
	if name == "" {
		return trace.BadParameter("name is required")
	}
	_, err := s.backend.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       backend.NewKey(phasePrefix, name),
			Condition: condition,
			Action:    backend.Delete(),
		},
		phaseGraphAction(backend.Whatever()),
	})
	return trace.Wrap(err)
}

func newPhaseParser() *phaseParser {
	return &phaseParser{baseParser: newBaseParser(backend.NewKey(phasePrefix).ExactKey())}
}

type phaseParser struct{ baseParser }

func (p *phaseParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		name := strings.TrimPrefix(event.Item.Key.TrimPrefix(backend.NewKey(phasePrefix)).String(), backend.SeparatorString)
		if name == "" {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key.String())
		}
		return &types.ResourceHeader{
			Kind:     types.KindPhase,
			Version:  types.V1,
			Metadata: types.Metadata{Name: name},
		}, nil
	case types.OpPut:
		resource, err := services.UnmarshalProtoResource[*beamsv1.Phase](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err, "unmarshalling Phase event")
		}
		return types.Resource153ToLegacy(resource), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}
