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
package local

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// nonceViolation is an internal error value, used to signal a nonce mismatch. it should be converted
// to a more actionable error before being returned.
var nonceViolation = trace.CompareFailed("nonce-violation")

// fastGetRange is a generic helper for getting a range of resources and unmarshaling them with FastUnmarshal.
func fastGetRange[T types.Resource](ctx context.Context, s *VersionControlService, key []byte) ([]T, error) {
	result, err := s.Backend.GetRange(ctx, key, backend.RangeEnd(key), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	values := make([]T, 0, len(result.Items))
	for _, item := range result.Items {
		var value T
		if err := utils.FastUnmarshal(item.Value, &value); err != nil {
			s.log.Warnf("Skipping resource at %s, failed to unmarshal: %v", item.Key, err)
			continue
		}
		value.SetExpiry(item.Expires.UTC())
		values = append(values, value)
	}
	return values, nil
}

func fastGetResource[T types.Resource](ctx context.Context, s *VersionControlService, key []byte) (T, error) {
	var value T

	item, err := s.Backend.Get(ctx, key)
	if err != nil {
		return value, trace.Wrap(err)
	}

	if err := utils.FastUnmarshal(item.Value, &value); err != nil {
		return value, trace.Errorf("failed to unmarshal resource at %s: %v", key, err)
	}

	value.SetExpiry(item.Expires.UTC())
	return value, nil
}

type VersionControlService struct {
	backend.Backend
	log *logrus.Entry
}

// NewVersionControlService returns new version control service instance
func NewVersionControlService(b backend.Backend) *VersionControlService {
	return &VersionControlService{
		log:     logrus.WithFields(logrus.Fields{trace.Component: "version-control"}),
		Backend: b,
	}
}

// GetVersionControlInstallers loads version control installers, sorted by type.
func (s *VersionControlService) GetVersionControlInstallers(ctx context.Context, filter types.VersionControlInstallerFilter) (types.VersionControlInstallerSet, error) {
	var set types.VersionControlInstallerSet

	if err := filter.Check(); err != nil {
		return set, trace.Wrap(err)
	}

	// we only actually support one installer kind right now.
	if filter.Kind != types.InstallerKindNone && filter.Kind != types.InstallerKindLocalScript {
		return set, trace.BadParameter("unsupported installer kind %q", filter.Kind)
	}

	// note: below logic will need to be reworked once other installer kinds are
	// introduced.

	if filter.Unique() {
		// special case: filter matches exactly zero or one resources

		installer, err := fastGetResource[*types.LocalScriptInstallerV1](ctx, s, versionControlInstallerKey(types.InstallerKindLocalScript, filter.Name))
		if err != nil {
			if trace.IsNotFound(err) {
				return set, nil
			}
			return set, trace.Wrap(err)
		}

		if !filter.Match(installer) {
			return set, nil
		}
		set.LocalScript = append(set.LocalScript, installer)
		return set, nil
	}

	var err error
	installers, err := fastGetRange[*types.LocalScriptInstallerV1](ctx, s, versionControlInstallerKey(types.InstallerKindLocalScript, ""))
	if err != nil {
		return set, trace.Wrap(err)
	}

	// filter in place
	set.LocalScript = installers[:0]
	for _, installer := range installers {
		if !filter.Match(installer) {
			continue
		}

		set.LocalScript = append(set.LocalScript, installer)
	}

	return set, nil
}

// UpsertVersionControlInstaller creates or updates a version control installer (nonce safety is
// enforced).
func (s *VersionControlService) UpsertVersionControlInstaller(ctx context.Context, installer types.VersionControlInstaller) error {
	if err := installer.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if installer.GetNonce() == 0 {
		if err := installer.OnCreateChecks(); err != nil {
			return trace.Wrap(err)
		}
	}

	switch installer.(type) {
	case *types.LocalScriptInstallerV1:
		err := fastUpsertNonceProtectedResource(ctx, s, versionControlInstallerKey(types.InstallerKindLocalScript, installer.GetName()), installer)
		if err != nil {
			if err == nonceViolation {
				return trace.CompareFailed("local script installer %q was concurrently modified, please work from latest state", installer.GetName())
			}
			return trace.Wrap(err)
		}
		return nil
	default:
		return trace.BadParameter("unexpected version control installer type %T", installer)
	}
}

// DeleteVersionControlInstaller deletes a single version control installer if it matches the supplied
// filter. Filters that match multiple installers are rejected.
func (s *VersionControlService) DeleteVersionControlInstaller(ctx context.Context, filter types.VersionControlInstallerFilter) error {
	if err := filter.Check(); err != nil {
		return trace.Wrap(err)
	}

	if !filter.Unique() {
		return trace.BadParameter("delete is ambiguous, kind and name must be specified")
	}

	if filter.Kind != types.InstallerKindLocalScript {
		return trace.BadParameter("unsupported installer kind %q", filter.Kind)
	}

	if filter.Nonce != 0 {
		return trace.BadParameter("nonce-conditional deletes are not currently supported")
	}

	if err := s.Backend.Delete(ctx, versionControlInstallerKey(filter.Kind, filter.Name)); err != nil {
		if trace.IsNotFound(err) {
			return nil
		}

		return trace.Wrap(err)
	}

	return nil
}

// GetVersionDirectives gets one or more version directives, sorted by state (draft|pending|active).
func (s *VersionControlService) GetVersionDirectives(ctx context.Context, filter types.VersionDirectiveFilter) (types.VersionDirectiveSet, error) {
	var set types.VersionDirectiveSet

	if err := filter.Check(); err != nil {
		return set, trace.Wrap(err)
	}

	if filter.Unique() {
		// special case: filter matches exactly zero or one resources

		// ensure we know how to sort the directive before trying to load it
		var setter func(*types.VersionDirectiveV1)
		switch filter.Phase {
		case types.DirectivePhaseDraft:
			setter = func(d *types.VersionDirectiveV1) {
				set.Draft = append(set.Draft, d)
			}
		case types.DirectivePhasePending:
			setter = func(d *types.VersionDirectiveV1) {
				set.Pending = append(set.Pending, d)
			}
		case types.DirectivePhaseActive:
			// technically, loading the active directive *is* a single-resource query, but
			// the standard loading logic handles it as such already.
			return set, trace.BadParameter("active version directive cannot be queried by kind/name")
		default:
			return set, trace.BadParameter("unrecognized directive phase %q", filter.Phase)
		}

		directive, err := fastGetResource[*types.VersionDirectiveV1](ctx, s, versionDirectiveKey(filter.Phase, filter.Kind, filter.Name))
		if err != nil {
			if trace.IsNotFound(err) {
				return set, nil
			}

			return set, trace.Wrap(err)
		}

		if !filter.Match(directive) {
			return set, nil
		}

		// place directive in appropriate slot
		setter(directive)
		return set, nil
	}

	var knownPhase bool

	if filter.Phase == types.DirectivePhaseNone || filter.Phase == types.DirectivePhaseDraft {
		knownPhase = true

		directives, err := fastGetRange[*types.VersionDirectiveV1](ctx, s, versionDirectiveRangeKey(types.DirectivePhaseDraft))
		if err != nil {
			return set, trace.Wrap(err)
		}

		set.Draft = directives[:0]
		for _, directive := range directives {
			if !filter.Match(directive) {
				continue
			}
			set.Draft = append(set.Draft, directive)
		}
	}

	if filter.Phase == types.DirectivePhaseNone || filter.Phase == types.DirectivePhasePending {
		knownPhase = true

		directives, err := fastGetRange[*types.VersionDirectiveV1](ctx, s, versionDirectiveRangeKey(types.DirectivePhasePending))
		if err != nil {
			return set, trace.Wrap(err)
		}

		set.Pending = directives[:0]
		for _, directive := range directives {
			if !filter.Match(directive) {
				continue
			}
			set.Pending = append(set.Pending, directive)
		}
	}

	if filter.Phase == types.DirectivePhaseNone || filter.Phase == types.DirectivePhaseActive {
		knownPhase = true

		directive, err := fastGetResource[*types.VersionDirectiveV1](ctx, s, activeDirectiveKey())
		if err != nil && !trace.IsNotFound(err) {
			return set, trace.Wrap(err)
		}

		if err == nil && filter.Match(directive) {
			set.Active = directive
		}
	}

	if !knownPhase {
		return set, trace.BadParameter("unrecognized directive phase %q", filter.Phase)
	}

	return set, nil
}

// UpsertVersionDirective creates or updates a draft phase version directive.
func (s *VersionControlService) UpsertVersionDirective(ctx context.Context, directive types.VersionDirective) error {
	if err := directive.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if directive.GetNonce() == 0 {
		if err := directive.OnCreateChecks(); err != nil {
			return trace.Wrap(err)
		}
	}

	if p := directive.GetDirectivePhase(); p != types.DirectivePhaseDraft {
		return trace.BadParameter("cannot upsert directive in phase %q, directives can only be upsert in phase %q", p, types.DirectivePhaseDraft)
	}

	err := fastUpsertNonceProtectedResource(ctx, s, versionDirectiveKey(types.DirectivePhaseDraft, directive.GetSubKind(), directive.GetName()), directive)

	if err != nil {
		if err == nonceViolation {
			if directive.GetNonce() == 0 {
				return trace.CompareFailed("version directive %s/%s already exists, please delete the old version or work from the latest state", directive.GetSubKind(), directive.GetName())
			}
			return trace.CompareFailed("version directive %s/%s was concurrently modified, please work from the latest state", directive.GetSubKind(), directive.GetName())
		}

		return trace.Wrap(err)
	}

	return nil
}

// DeleteVersionDirective deletes a single version directive if it matches the supplied
// filter. Filters that match multiple directives are rejected.
func (s *VersionControlService) DeleteVersionDirective(ctx context.Context, filter types.VersionDirectiveFilter) error {
	if err := filter.Check(); err != nil {
		return trace.Wrap(err)
	}

	if !filter.Unique() && filter.Phase != types.DirectivePhaseActive {
		return trace.BadParameter("delete is ambiguous, phase/kind/name must all be specified")
	}

	if filter.Nonce != 0 {
		return trace.BadParameter("nonce-conditional deletes are not currently supported")
	}

	key := versionDirectiveKey(filter.Phase, filter.Kind, filter.Name)
	if filter.Phase == types.DirectivePhaseActive {
		key = activeDirectiveKey()
	}

	if err := s.Backend.Delete(ctx, key); err != nil {
		if trace.IsNotFound(err) {
			return nil
		}

		return trace.Wrap(err)
	}

	return nil
}

// PromoteVersionDirective attempts to promote a version directive (allowed phase transitions
// are draft -> pending, and pending -> active).
func (s *VersionControlService) PromoteVersionDirective(ctx context.Context, req proto.PromoteVersionDirectiveRequest) (proto.PromoteVersionDirectiveResponse, error) {
	var rsp proto.PromoteVersionDirectiveResponse
	if err := req.Ref.Check(); err != nil {
		return rsp, trace.Wrap(err)
	}

	if !req.Ref.Unique() {
		return rsp, trace.BadParameter("promote is ambiguous, phase/kind/name must all be specified")
	}

	if req.ToPhase == types.DirectivePhaseNone {
		return rsp, trace.BadParameter("directive promotion target phase must be specified")
	}

	switch req.Ref.Phase {
	case types.DirectivePhaseDraft:
		if req.ToPhase != types.DirectivePhasePending {
			return rsp, trace.BadParameter("invalid phase transition %s -> %s", req.Ref.Phase, req.ToPhase)
		}
		// promote from draft -> pending

		directive, err := fastGetResource[*types.VersionDirectiveV1](ctx, s, versionDirectiveKey(req.Ref.Phase, req.Ref.Kind, req.Ref.Name))
		if err != nil {
			return rsp, trace.Wrap(err)
		}

		if !req.Ref.Match(directive) {
			return rsp, trace.CompareFailed("promote aborted, target directive didn't match supplied filter")
		}

		if s := directive.GetDirectiveStatus(); s != types.DirectiveStatusReady {
			return rsp, trace.Errorf("cannot promote directive with status %q, must be %q", s, types.DirectiveStatusReady)
		}

		directive.Spec.Phase = types.DirectivePhasePending

		// use origin to preserve original name if unset.
		if directive.Spec.Origin == "" {
			directive.Spec.Origin = fmt.Sprintf("%s/%s", directive.GetSubKind(), directive.GetName())
		}

		newID := uuid.New().String()

		directive.Metadata.Name = newID

		directive.Spec.Nonce = 0

		directive.SetExpiry(time.Now().Add(time.Hour * 24).UTC())

		err = fastUpsertNonceProtectedResource(ctx, s, versionDirectiveKey(types.DirectivePhasePending, directive.GetSubKind(), directive.GetName()), directive)
		if err != nil {
			if err == nonceViolation {
				// this is incredibly improbable unless we're dealing with a bug
				return rsp, trace.CompareFailed("collision creating pending directive %s/%s (this might be a bug)", directive.GetSubKind(), directive.GetName())
			}

			return rsp, trace.Wrap(err)
		}

		rsp.NewRef = types.VersionDirectiveFilter{
			Phase: types.DirectivePhasePending,
			Kind:  directive.GetSubKind(),
			Name:  directive.GetName(),
		}

		return rsp, nil

	case types.DirectivePhasePending:
		if req.ToPhase != types.DirectivePhaseActive {
			return rsp, trace.BadParameter("invalid phase transition %s -> %s", req.Ref.Phase, req.ToPhase)
		}
		// promote from pending -> active

		pendingKey := versionDirectiveKey(req.Ref.Phase, req.Ref.Kind, req.Ref.Name)

		directive, err := fastGetResource[*types.VersionDirectiveV1](ctx, s, pendingKey)
		if err != nil {
			return rsp, trace.Wrap(err)
		}

		if !req.Ref.Match(directive) {
			return rsp, trace.CompareFailed("promote aborted, target directive didn't match supplied filter")
		}

		if s := directive.GetDirectiveStatus(); s != types.DirectiveStatusReady {
			return rsp, trace.Errorf("cannot promote directive with status %q, must be %q", s, types.DirectiveStatusReady)
		}

		directive.Spec.Phase = types.DirectivePhaseActive

		directive.Spec.Nonce = 0
		directive.SetExpiry(time.Time{})

		val, err := utils.FastMarshal(directive)
		if err != nil {
			return rsp, trace.Errorf("failed to marshal pending directive %s/%s for promotion to active: %v", directive.GetSubKind(), directive.GetName(), err)
		}
		item := backend.Item{
			Key:   activeDirectiveKey(),
			Value: val,
		}

		_, err = s.Backend.Put(ctx, item)
		if err != nil {
			return rsp, trace.Errorf("failed to write directive %s/%s to active slot: %v", directive.GetSubKind(), directive.GetName(), err)
		}

		// promote successful. attempt to clean up the pending entry.
		if err := s.Backend.Delete(ctx, pendingKey); err != nil {
			s.log.Warnf("Failed to clean up pending directive %s/%s after promotion: %v", directive.GetSubKind(), directive.GetName(), err)
		}

		rsp.NewRef = types.VersionDirectiveFilter{
			Phase: types.DirectivePhaseActive,
		}

		return rsp, nil
	case types.DirectivePhaseActive:
		return rsp, trace.BadParameter("invalid phase transition %s -> %s", req.Ref.Phase, req.ToPhase)

	default:
		return rsp, trace.BadParameter("unrecognized directive phase: %s", req.Ref.Phase)
	}
}

// SetVersionDirectiveStatus attempts to update the status of a version directive.
func (s *VersionControlService) SetVersionDirectiveStatus(ctx context.Context, req proto.SetVersionDirectiveStatusRequest) (proto.SetVersionDirectiveStatusResponse, error) {
	var rsp proto.SetVersionDirectiveStatusResponse
	if err := req.Ref.Check(); err != nil {
		return rsp, trace.Wrap(err)
	}

	if !req.Ref.Unique() && req.Ref.Phase != types.DirectivePhaseActive {
		return rsp, trace.BadParameter("status update is ambiguous, phase/kind/name must be specified for non-active directives")
	}

	if req.Status == types.DirectiveStatusNone {
		return rsp, trace.BadParameter("desired directive status must be specified")
	}

	key := versionDirectiveKey(req.Ref.Phase, req.Ref.Kind, req.Ref.Name)
	if req.Ref.Phase == types.DirectivePhaseActive {
		key = activeDirectiveKey()
	}

	directive, err := fastGetResource[*types.VersionDirectiveV1](ctx, s, key)
	if err != nil {
		return rsp, trace.Wrap(err)
	}

	if !req.Ref.Match(directive) {
		return rsp, trace.CompareFailed("status update aborted, target directive didn't match supplied filter")
	}

	if req.Expect != types.DirectiveStatusNone && req.Expect != directive.GetDirectiveStatus() {
		return rsp, trace.CompareFailed("expected directive status %q, got %q", req.Expect, directive.GetDirectiveStatus())
	}

	if directive.GetDirectiveStatus() == types.DirectiveStatusPoisoned {
		// the poisoned status is permanent.
		msg := directive.Spec.StatusMsg
		if msg == "" {
			msg = "permanently disabled"
		}
		return rsp, trace.Errorf("cannot update directive status: %q (poisoned)", msg)
	}

	rsp.PreviousStatus = directive.GetDirectiveStatus()

	directive.Spec.Status = req.Status
	directive.Spec.StatusMsg = req.Message

	if err := fastUpsertNonceProtectedResource(ctx, s, key, directive); err != nil {
		if err == nonceViolation {
			return rsp, trace.CompareFailed("status update aborted, directive was concurrently modified")
		}

		return rsp, trace.Wrap(err)
	}

	return rsp, nil
}

// nonceProtectedResourceShim is a helper for quickly extracting the nonce
type nonceProtectedResourceShim struct {
	Spec struct {
		Nonce uint64 `json:"nonce"`
	} `json:"spec"`
}

type nonceProtectedResource interface {
	Expiry() time.Time
	GetNonce() uint64
	WithNonce(uint64) interface{}
}

func fastUpsertNonceProtectedResource[T nonceProtectedResource](ctx context.Context, s *VersionControlService, key []byte, resource T) error {
	upsert := resource.GetNonce() == math.MaxUint64
	newNonce := resource.GetNonce() + 1
	if upsert {
		newNonce = 1
	}
	val, err := utils.FastMarshal(resource.WithNonce(newNonce))
	if err != nil {
		return trace.Errorf("failed to marshal resource at %s: %v", key, err)
	}
	item := backend.Item{
		Key:     key,
		Value:   val,
		Expires: resource.Expiry(),
	}

	if upsert {
		_, err := s.Backend.Put(ctx, item)
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	}

	if resource.GetNonce() == 0 {
		_, err := s.Backend.Create(ctx, item)
		if err != nil {
			if trace.IsAlreadyExists(err) {
				return nonceViolation
			}
			return trace.Wrap(err)
		}

		return nil
	}

	prev, err := s.Get(ctx, item.Key)
	if err != nil {
		if trace.IsNotFound(err) {
			return nonceViolation
		}
		return trace.Wrap(err)
	}

	var shim nonceProtectedResourceShim
	if err := utils.FastUnmarshal(prev.Value, &shim); err != nil {
		return trace.Errorf("failed to read nonce of resource at %q", item.Key)
	}

	if shim.Spec.Nonce != resource.GetNonce() {
		return nonceViolation
	}

	_, err = s.Backend.CompareAndSwap(ctx, *prev, item)
	if err != nil {
		if trace.IsCompareFailed(err) {
			return nonceViolation
		}

		return trace.Wrap(err)
	}

	return nil
}

// upsertNonceProtectedResource is a helper for creating/updated a resource that uses a nonce to protect itself
// from concurrent updated.
func (s *VersionControlService) upsertNonceProtectedResource(ctx context.Context, expect uint64, item backend.Item) error {
	if expect == 0 {
		_, err := s.Backend.Create(ctx, item)
		if err != nil {
			if trace.IsAlreadyExists(err) {
				return nonceViolation
			}
			return trace.Wrap(err)
		}
	}

	prev, err := s.Get(ctx, item.Key)
	if err != nil {
		if trace.IsNotFound(err) {
			return nonceViolation
		}
		return trace.Wrap(err)
	}

	var shim nonceProtectedResourceShim
	if err := utils.FastUnmarshal(prev.Value, &shim); err != nil {
		return trace.Errorf("failed to read nonce of resource at %q", item.Key)
	}

	if shim.Spec.Nonce != expect {
		return nonceViolation
	}

	_, err = s.Backend.CompareAndSwap(ctx, *prev, item)
	if err != nil {
		if trace.IsCompareFailed(err) {
			return nonceViolation
		}

		return trace.Wrap(err)
	}

	return nil
}

func versionControlInstallerKey(kind types.VersionControlInstallerKind, name string) []byte {
	return backend.Key("version-control-installers", string(kind), name)
}

const versionDirectivePrefix = "version-directives"

func versionDirectiveKey(phase types.VersionDirectivePhase, kind string, name string) []byte {
	return backend.Key(versionDirectivePrefix, string(phase), kind, name)
}

func activeDirectiveKey() []byte {
	// we could just store the active directive at version-directives/active, but the consistency of
	// storing all directives at <prefix>/<kind>/<name> seems desirable. At the very least, it leaves
	// us the option to seamlessly migrate to a model with multiple active directives in the future.
	return versionDirectiveKey(types.DirectivePhaseActive, "default", "default")
}

func versionDirectiveRangeKey(phase types.VersionDirectivePhase) []byte {
	return backend.Key(versionDirectivePrefix, string(phase), "")
}
