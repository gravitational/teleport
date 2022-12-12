/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package types

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gravitational/teleport/api/defaults"
	vc "github.com/gravitational/teleport/api/versioncontrol"
	"github.com/gravitational/trace"
)

// VersionDirectiveStatus indicates the current status of a version directive in terms
// of its theoretical runnability. Unknown statuses are permitted, but cause the version
// directive to be treated as if it were locked.
type VersionDirectiveStatus string

const (
	// DirectiveStatusNone indicates that a status was not specified. The meaning of this
	// is context-dependent (e.g. a SetVersionDirectiveStatusRequest treats Expect == DirectiveStatusNone
	// to mean that setting the new status is not dependent on the current status).
	DirectiveStatusNone VersionDirectiveStatus = ""

	// DirectiveStatusReady indicates that the directive is safe to promote and enforce.
	DirectiveStatusReady VersionDirectiveStatus = "ready"

	// DirectiveStatusLocked indicates that the directive should not be promoted or enforced right now.
	DirectiveStatusLocked VersionDirectiveStatus = "locked"

	// DirectiveStatusPoisoned is equivalent to 'locked', except that it is permanent. Indicates that
	// something has gone wrong.
	DirectiveStatusPoisoned VersionDirectiveStatus = "poisoned"
)

func (s VersionDirectiveStatus) Check() error {
	// allow unkwnown statuses, but keep allowed characters in-line with
	// the naming conventions of known statuses.
	if !IsStrictKebabCase(string(s)) {
		return trace.BadParameter("invalid version directive status %q", s)
	}

	return nil
}

func (s VersionDirectiveStatus) IsKnownStatus() bool {
	switch s {
	case DirectiveStatusNone:
	case DirectiveStatusReady:
	case DirectiveStatusLocked:
	case DirectiveStatusPoisoned:
	default:
		return false
	}
	return true
}

// VersionDirectivePhase represents the phase that a version directive is in.
type VersionDirectivePhase string

const (
	// DirectivePhaseNone indicates that phase was not specified. The meaning of this is
	// context-dependent (e.g. a filter treats DirectivePhaseNone as 'match all').
	DirectivePhaseNone VersionDirectivePhase = ""

	// DirectivePhaseDraft is the phase in which directives are provisional/mutable. A directive
	// in this phase may be awaiting additional modifications, or may exist for information purposes
	// only.
	DirectivePhaseDraft VersionDirectivePhase = "draft"

	// DirectivePhasePending is the phase in which directives have their parameters frozen, and
	// are assumed to be completed/finalized. A directive in phase 'pending' *may* be promoted to
	// 'active' in the future.
	DirectivePhasePending VersionDirectivePhase = "pending"

	// DirectivePhaseActive is the phase in which a version directive is enforceable. Only one
	// directive is active at a time. Note that being the active directive does not necessarily
	// mean that the directive is *actually* enforced. Directives can be locked/poisoned, and can
	// have their enforcement subjected to scheduling limits.
	DirectivePhaseActive VersionDirectivePhase = "active"
)

func (s VersionDirectivePhase) Check() error {
	switch s {
	case DirectivePhaseNone:
	case DirectivePhaseDraft:
	case DirectivePhasePending:
	case DirectivePhaseActive:
	default:
		return trace.BadParameter("unexpected version directive phase %q", s)
	}
	return nil
}

// Match checks if a given installer matches this filter.
func (f VersionDirectiveFilter) Match(directive VersionDirective) bool {

	if f.Kind != "" && f.Kind != directive.GetSubKind() {
		return false
	}

	if f.Name != "" && f.Name != directive.GetName() {
		return false
	}

	if f.Nonce != 0 && f.Nonce != directive.GetNonce() {
		return false
	}

	return true
}

// Check verifies sanity of filter parameters.
func (f VersionDirectiveFilter) Check() error {
	if err := f.Phase.Check(); err != nil {
		return trace.Wrap(err)
	}

	if f.Name != "" && (f.Kind == "" || f.Phase == DirectivePhaseNone) {
		return trace.BadParameter("cannot resolve version directive %q without phase/kind", f.Name)
	}

	if f.Nonce != 0 && f.Name == "" {
		return trace.BadParameter("cannot assert nonce %d without installer name", f.Nonce)
	}

	return nil
}

// Unique checks if this filter matches exactly one resource (phase, kind, and name are all set).
func (f VersionDirectiveFilter) Unique() bool {
	return f.Phase != DirectivePhaseNone && f.Kind != "" && f.Name != ""
}

// AsMap converts a directive set to a map of the form phase -> directives. Useful for iteration purposes.
func (s *VersionDirectiveSet) AsMap() map[VersionDirectivePhase][]*VersionDirectiveV1 {
	m := make(map[VersionDirectivePhase][]*VersionDirectiveV1)

	if s.Active != nil {
		m[DirectivePhaseActive] = []*VersionDirectiveV1{s.Active}
	}

	if len(s.Pending) != 0 {
		m[DirectivePhasePending] = s.Pending
	}

	if len(s.Draft) != 0 {
		m[DirectivePhaseDraft] = s.Draft
	}

	return m
}

type VersionDirective interface {
	Resource

	// GetDirectivePhase loads the phase of the version directive.
	GetDirectivePhase() VersionDirectivePhase

	// GetDirectiveStatus gets the status of the version directive.
	GetDirectiveStatus() VersionDirectiveStatus

	// GetNonce gets the nonce of the directive.
	GetNonce() uint64

	// WithNonce creates a shallow copy with a new nonce.
	WithNonce(nonce uint64) interface{}

	// OnCreateChecks performs additional checks prior to new resource creation. Useful for
	// enforcing checks that may be removed in future versions, or which are intended to
	// provide a standard/convention rather than being strictly necessary for resource function.
	OnCreateChecks() error
}

func NewVersionDirective(name string, spec VersionDirectiveSpecV1) (VersionDirective, error) {
	directive := &VersionDirectiveV1{
		ResourceHeader: ResourceHeader{
			Metadata: Metadata{
				Name: name,
			},
		},
		Spec: spec,
	}

	if err := directive.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return directive, nil
}

func (d *VersionDirectiveV1) CheckAndSetDefaults() error {
	d.setStaticFields()

	if err := d.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if d.Version != V1 {
		return trace.BadParameter("unsupported version directive resource version: %s", d.Version)
	}

	if d.Kind != KindVersionDirective {
		return trace.BadParameter("unexpected resource kind: %q (expected %s)", d.Kind, KindVersionDirective)
	}

	if d.SubKind == "" {
		d.SubKind = SubKindCustom
	}

	if d.Metadata.Namespace != "" && d.Metadata.Namespace != defaults.Namespace {
		return trace.BadParameter("invalid namespace %q (namespaces are deprecated)", d.Metadata.Namespace)
	}

	if d.Spec.Phase == DirectivePhaseNone {
		d.Spec.Phase = DirectivePhaseDraft
	}

	if err := d.Spec.Phase.Check(); err != nil {
		return trace.Wrap(err)
	}

	if d.Spec.Status == DirectiveStatusNone {
		d.Spec.Status = DirectiveStatusReady
	}

	if err := d.Spec.Status.Check(); err != nil {
		return trace.Wrap(err)
	}

	//d.Spec.Schedule.NotBefore = d.Spec.Schedule.NotBefore.UTC()
	//d.Spec.Schedule.NotAfter = d.Spec.Schedule.NotAfter.UTC()

	for i := range d.Spec.Directives {
		if err := d.Spec.Directives[i].CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (d *VersionDirectiveV1) OnCreateChecks() error {
	// in the future, we'd like to be able to support the convention of using the
	// controller that generated the directive as the directive's
	// sub-kind, allowing direct lookup in the backend via reference strings
	// like `tuf/default` (meaning get the default directive created by the
	// 'tuf' controller). Since we currently only support manually created
	// directives for now, required a standardized sub_kind 'custom'.
	if d.SubKind != SubKindCustom {
		return trace.BadParameter("unexpected sub_kind: %q (expected %q)", d.SubKind, SubKindCustom)
	}

	// disallow custom statuses until we have established guidelines for their use.
	if !d.Spec.Status.IsKnownStatus() {
		return trace.BadParameter("unrecognized version directive status %q", d.Spec.Status)
	}

	// limit origin string to the form 'kebab-case' or 'kebab-kind/kebab-name' for now
	// in order to reserve the ability to make origin meaningful in the future (e.g. allow
	// the origin string 'tuf/default' to cause the controller 'tuf/default' to be checked).
	// only checked on resource creation in order to preserve compatibility with future changes
	// that might relax these restrictions. For now, it is unclear if we need/want origin to be
	// a valid back-reference, and even if we do, we might only need it during the draft phase,
	// which can use `<sub-kind>/<name>` as the backreference via the convention described above.
	if d.Spec.Origin != "" {
		parts := strings.SplitN(d.Spec.Origin, "/", 2)
		for _, p := range parts {
			if !IsStrictKebabCase(p) {
				return trace.BadParameter("invalid version directive origin string %q", d.Spec.Origin)
			}
		}
	}

	for i := range d.Spec.Directives {
		if err := d.Spec.Directives[i].OnCreateChecks(); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (s *VersionDirectiveStanza) CheckAndSetDefaults() error {
	if s.Name == "" {
		return trace.BadParameter("version directive stanza missing required field 'name'")
	}

	for _, target := range s.Targets {
		if !target.Target.Ok() {
			return trace.BadParameter("invalid target version %q in stanza %q, expected go-style semver string (e.g. v1.2.3)", target.Target.Version(), s.Name)
		}
	}

	for _, installer := range s.Installers {
		if !IsStrictKebabCase(string(installer.Kind)) {
			return trace.BadParameter("invdalid installer kind %q in stanza %q, expected kebab-case string (e.g. %q)", installer.Kind, s.Name, InstallerKindLocalScript)
		}
		if !IsStrictKebabCase(installer.Name) {
			return trace.BadParameter("invalid installer name %q in stanza %q, expected kebab-case string", installer.Name, s.Name)
		}
	}

	for _, selector := range s.Selectors {
		for i, service := range selector.Services {
			if service.IsLocalService() {
				continue
			}

			// convert known aliases to their canonical service names (e.g. `node -> Node`).
			if p, ok := ParseTeleportRole(service); ok && p.IsLocalService() {
				selector.Services[i] = p
			}

			// note that we don't actually reject unknown service names here.
			// they are rejected only during creation checks. from that point
			// forward, if the instance's cert has the given service name, *and*
			// its heartbeat advertises the service name, then we consider it acceptable
			// to upgrade it even if we don't know what that service name means.
		}
	}

	return nil
}

// OnCreateChecks checks things that we expect to change in future versions, allowing us to maintain
// partial back-compat in the event of auth downgrades.
func (s *VersionDirectiveStanza) OnCreateChecks() error {
	// only check if installer kind is recognized on create. unknown installer
	// kinds should be ignored for existing directives.
	for _, installer := range s.Installers {
		if err := installer.Kind.Check(); err != nil {
			return trace.Wrap(err)
		}
	}

	for _, selector := range s.Selectors {
		for _, service := range selector.Services {
			if !service.IsLocalService() {
				return trace.BadParameter("unknown service name %q in stanza %q", service, s.Name)
			}
		}

		// limit the allowed label selector syntax so that we can
		// add special syntax in the future without
		// breaking compatibility. we're definitely erring on the side of
		// being too strict here, but its good to keep our options open.
		for key, val := range selector.Labels {
			if key != Wildcard && !IsLabelLiteral(key) {
				return trace.BadParameter("invalid selector key %q in stanza %q, expected wildcard or literal (syntax will be expanded in the future)", key, s.Name)
			}

			if val != Wildcard && !IsLabelLiteral(val) {
				return trace.BadParameter("invalid selector val %q in stanza %q, expected wildcard or literal (syntax will be expanded in the future)", val, s.Name)
			}
		}

		if len(selector.Current) != 0 {
			// instances currently only advertise version, so any other key will cause the selector to match
			// nothing. best to disallow additional keys alltogether until instances can advertise build
			// details (e.g. `arch=amd64`).
			for key := range selector.Current {
				if key != vc.LabelVersion {
					return trace.BadParameter("unexpected selector.current key %q in stanza %q (only the %q key is currently supported)", key, s.Name, vc.LabelVersion)
				}
			}

			// only permit literal version string. Future iterations will add support for wildcard version
			// matching (e.g. `version=v1.2.*`) and matching additional target attributes (e.g. `arch=amd64`).
			if !selector.Current.Ok() {
				return trace.BadParameter("invalid selector.current version %q in stanza %q, expected a go-style semver string (e.g.  v1.2.3)", selector.Current.Version(), s.Name)
			}
		}
	}

	return nil
}

func (d *VersionDirectiveV1) setStaticFields() {
	if d.Version == "" {
		d.Version = V1
	}

	if d.Kind == "" {
		d.Kind = KindVersionDirective
	}
}

func (d *VersionDirectiveV1) GetDirectivePhase() VersionDirectivePhase {
	return d.Spec.Phase
}

func (d *VersionDirectiveV1) GetDirectiveStatus() VersionDirectiveStatus {
	return d.Spec.Status
}

func (d *VersionDirectiveV1) GetNonce() uint64 {
	return d.Spec.Nonce
}

func (d *VersionDirectiveV1) WithNonce(nonce uint64) interface{} {
	shallowCopy := *d
	shallowCopy.Spec.Nonce = nonce
	return &shallowCopy
}

func (w *WrappedInstallTarget) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.Target)
}

func (w *WrappedInstallTarget) UnmarshalJSON(data []byte) error {
	// we regularly use libraries that convery yaml directly to json, and
	// then unmarshal from there, so we need to be able to unmarshal some
	// unexpected types.
	m := make(map[string]interface{})
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	w.Target = make(map[string]string)
	for key, val := range m {
		switch v := val.(type) {
		case string:
			w.Target[key] = v
		case bool:
			if v {
				w.Target[key] = "yes"
			} else {
				w.Target[key] = "no"
			}
		case float64:
			w.Target[key] = strconv.FormatFloat(v, 'f', -1, 64)
		default:
			return trace.BadParameter("unexpected install target value type %T for key %q", val, key)
		}
	}
	return nil
}

func (w WrappedInstallTarget) MarshalYAML() (interface{}, error) {
	// this method needs to use a value receiver instead of the pointer receiver
	// used in other methods because the yaml library doesn't seem to be able
	// to locate marshalling methods in []T if the method accepts *T.
	return w.Target, nil
}

func (w *WrappedInstallTarget) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return unmarshal(&w.Target)
}
