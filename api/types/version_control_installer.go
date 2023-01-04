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
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/trace"
)

// IsStrictKebabCase checks if the given string meets a fairly strict definition of
// kebab case (no dots, dashes, caps, etc). Useful for strings that might need to be
// included in filenames.
var IsStrictKebabCase = regexp.MustCompile(`^[a-z0-9\-]+$`).MatchString

// IsLabelLiteral matches the subset of allowed label characters that never have a special
// meaning.
var IsLabelLiteral = regexp.MustCompile(`^[a-zA-Z/.0-9_:-]+$`).MatchString

func (e *ExecScript) Check() error {
	if !IsStrictKebabCase(e.Type) {
		// type name is used in a filename, so we need to be strict
		// about its allowed characters.
		return trace.BadParameter("invalid type %q in exec-script message", e.Type)
	}

	if e.Script == "" {
		return trace.BadParameter("missing required field 'script' in exec-script message")
	}

	for name := range e.Env {
		if !isValidEnvVarName(name) {
			return trace.BadParameter("invalid env var name %q in exec-script message", name)
		}
	}

	for _, name := range e.EnvPassthrough {
		if !isValidEnvVarName(name) {
			return trace.BadParameter("invalid env passthrough var name %q in exec-script message", name)
		}
	}

	return nil
}

// VersionControlInstallerKind is the kind of a version control installer.
type VersionControlInstallerKind string

const (
	// InstallerKindNone indicates that kind was not specified. The meaning of this is
	// context-dependent (e.g. a filter treats InstallerKindNone as 'match all').
	InstallerKindNone VersionControlInstallerKind = ""

	// InstallerKindLocalScript is the local script installer kind.
	InstallerKindLocalScript VersionControlInstallerKind = "local-script"
)

func (t VersionControlInstallerKind) Check() error {
	switch t {
	case InstallerKindNone, InstallerKindLocalScript:
		return nil
	default:
		return trace.BadParameter("unknown version control installer kind %q", t)
	}
}

type VersionControlInstallerPreference struct {
	kind string
	name string
}

func ParseVersionControlInstallerPreference(s string) VersionControlInstallerPreference {
	parts := strings.SplitN(s, "/", 2)
	kind := parts[0]
	switch kind {
	case SubKindLocalScript:
		kind = string(InstallerKindLocalScript)
	}

	var name string
	if len(parts) == 2 {
		name = parts[1]
	}

	return VersionControlInstallerPreference{
		kind: kind,
		name: name,
	}
}

func (p *VersionControlInstallerPreference) MatchKind(kind VersionControlInstallerKind) bool {
	if p.kind == Wildcard {
		return true
	}

	return p.kind == string(kind)
}

func (p *VersionControlInstallerPreference) MatchName(name string) bool {
	if p.name == Wildcard || p.name == "" {
		return true
	}

	return p.name == name
}

// Iter iterates all installers in the set.
func (s *VersionControlInstallerSet) Iter(fn func(i VersionControlInstaller)) {
	for _, installer := range s.LocalScript {
		fn(installer)
	}
}

// VersionControlInstallerStatus indicates the current status of an installer in terms of
// its theoretical runnability.
type VersionControlInstallerStatus string

const (
	// InstallerStatusNone indicates that a status was not specified. The meaning of this
	// is context-dependent.
	InstallerStatusNone VersionControlInstallerStatus = ""

	// InstallerStatusOk indicates that the installer can be used.
	InstallerStatusOk VersionControlInstallerStatus = "ok"

	// InstallerStatusLocked indicates that the installer should not be used.
	InstallerStatusLocked VersionControlInstallerStatus = "locked"
)

func (s VersionControlInstallerStatus) Check() error {
	// allow unknown statuses, but keep allowed characters in-line with
	// the naming conventions of known statuses.
	if !IsStrictKebabCase(string(s)) {
		return trace.BadParameter("invalid version control installer status %q", s)
	}

	return nil
}

func (s VersionControlInstallerStatus) IsKnownStatus() bool {
	switch s {
	case InstallerStatusNone, InstallerStatusOk, InstallerStatusLocked:
		return true
	default:
		return false
	}
}

// Match checks if a given installer matches this filter.
func (f *VersionControlInstallerFilter) Match(installer VersionControlInstaller) bool {
	if f.Kind != InstallerKindNone && f.Kind != installer.GetInstallerKind() {
		return false
	}

	if f.Name != "" && f.Name != installer.GetName() {
		return false
	}

	if f.Nonce != 0 && f.Nonce != installer.GetNonce() {
		return false
	}

	return true
}

// Check verifies sanity of filter parameters.
func (f *VersionControlInstallerFilter) Check() error {
	if err := f.Kind.Check(); err != nil {
		return trace.Wrap(err)
	}

	if f.Name != "" && f.Kind == InstallerKindNone {
		return trace.BadParameter("cannot resolve installer %q without an installer kind", f.Name)
	}

	if f.Nonce != 0 && f.Name == "" {
		return trace.BadParameter("cannot assert nonce %d without installer name", f.Nonce)
	}

	return nil
}

// Unique checks if this filter matches exactly one resource (kind and name are both set).
func (f *VersionControlInstallerFilter) Unique() bool {
	return f.Kind != InstallerKindNone && f.Name != ""
}

// VersionControlInstaller abstracts over the common methods of all version conrol installers.
type VersionControlInstaller interface {
	Resource

	// GetInstallerKind gets the kind of the installer.
	GetInstallerKind() VersionControlInstallerKind

	// GetInstallerStatus gets the status of the installer.
	GetInstallerStatus() VersionControlInstallerStatus

	// GetNonce gets the nonce of the installer.
	GetNonce() uint64

	// WithNonce creates a shallow copy with a new nonce.
	WithNonce(nonce uint64) any

	// OnCreateChecks performs additional checks prior to new resource creation. Useful for
	// enforcing checks that may be removed in future versions, or which are intended to
	// provide a standard/convention rather than being strictly necessary for resource function.
	OnCreateChecks() error
}

// LocalScriptInstaller is used by the version control system to install
// new teleport versions via user-provided scripts.
type LocalScriptInstaller interface {
	VersionControlInstaller

	// BaseInstallMsg builds the base exec message for the install.sh script. The returned
	// value requires additional configuration to be valid (id, variable interpolation, etc...).
	BaseInstallMsg() ExecScript

	// BaseRestartMsg builds the base exec message for the restart.sh script. The returned
	// value requires additional configuration to be valid (id, variable interpolation, etc...).
	BaseRestartMsg() ExecScript

	// Clone performs a deep copy.
	Clone() LocalScriptInstaller
}

// NewLocalScriptInstaller constructs a new LocalScriptInstaller from the provided spec.
func NewLocalScriptInstaller(name string, spec LocalScriptInstallerSpecV1) (LocalScriptInstaller, error) {
	installer := &LocalScriptInstallerV1{
		ResourceHeader: ResourceHeader{
			Metadata: Metadata{
				Name: name,
			},
		},
		Spec: spec,
	}

	if err := installer.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return installer, nil
}

func (i *LocalScriptInstallerV1) GetNonce() uint64 {
	return i.Spec.Nonce
}

func (i *LocalScriptInstallerV1) WithNonce(nonce uint64) any {
	shallowCopy := *i
	shallowCopy.Spec.Nonce = nonce
	return &shallowCopy
}

func (i *LocalScriptInstallerV1) GetInstallerKind() VersionControlInstallerKind {
	return InstallerKindLocalScript
}

func (i *LocalScriptInstallerV1) GetInstallerStatus() VersionControlInstallerStatus {
	return i.Spec.Status
}

func (i *LocalScriptInstallerV1) CheckAndSetDefaults() error {
	i.setStaticFields()

	if err := i.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if i.Version != V1 {
		return trace.BadParameter("unsupported local script installer version: %s", i.Version)
	}

	if i.Kind != KindVersionControlInstaller {
		// accept kebab-case kind value
		if strings.ReplaceAll(i.Kind, "-", "_") != KindVersionControlInstaller {
			return trace.BadParameter("unexpected resource kind: %q (expected %s)", i.Kind, KindVersionControlInstaller)
		}
		i.Kind = KindVersionControlInstaller
	}

	if i.SubKind != SubKindLocalScript {
		// accept kebab-case sub kind value
		if strings.ReplaceAll(i.SubKind, "-", "_") != SubKindLocalScript {
			return trace.BadParameter("unexpected resource sub_kind: %q (expected %s)", i.SubKind, SubKindLocalScript)
		}
		i.SubKind = SubKindLocalScript
	}

	if i.Metadata.Namespace != "" && i.Metadata.Namespace != defaults.Namespace {
		return trace.BadParameter("invalid namespace %q (namespaces are deprecated)", i.Metadata.Namespace)
	}

	if i.Spec.Status == InstallerStatusNone {
		i.Spec.Status = InstallerStatusOk
	}

	if err := i.Spec.Status.Check(); err != nil {
		return trace.Wrap(err)
	}

	if i.Spec.InstallScript == "" {
		return trace.BadParameter("missing required field 'install.sh' in local script installer")
	}

	if i.Spec.RestartScript == "" {
		return trace.BadParameter("missing required field 'restart.sh' in local script installer")
	}

	for name := range i.Spec.Env {
		if !isValidEnvVarName(name) {
			return trace.BadParameter("invalid env var name %q in local script installer", name)
		}
	}

	for _, name := range i.Spec.EnvPassthrough {
		if !isValidEnvVarName(name) {
			return trace.BadParameter("invalid env passthrough var name %q in local script installer", name)
		}
	}

	if i.Spec.Shell != "" {
		// verify shell directive w/ optional shebang-style arg
		parts := strings.SplitN(strings.TrimSpace(i.Spec.Shell), " ", 2)
		if !filepath.IsAbs(parts[0]) {
			return trace.BadParameter("non-absolute shell path %q in local script installer", parts[0])
		}

		for _, arg := range parts[1:] {
			// some shebang impls bundle all space separated args after the executable
			// path into a single argument, and some allow for multiple args. the former
			// is more common, but the latter is generally regarded as superior. we sidestep
			// the issue for now by simply disallowing additional spaces. this will allow
			// us to adopt either behavior in the future w/o breaking user-facing compatibility
			// (though care will need to be taken to ensure auth<->node compat).
			for _, c := range arg {
				if unicode.IsSpace(c) {
					return trace.BadParameter("invalid argument %q for shell of local script installer", arg)
				}
			}
		}
	}

	return nil
}

func (i *LocalScriptInstallerV1) OnCreateChecks() error {
	if !IsStrictKebabCase(i.Metadata.Name) {
		// name requirements may be loosened in the future, so we only check
		// them on initial creation.
		return trace.BadParameter("invalid local script installer name %q (must be kebab-case)", i.Metadata.Name)
	}

	// disallow custom statuses until we have established guidelines for their use.
	if !i.Spec.Status.IsKnownStatus() {
		return trace.BadParameter("unrecognized local script installer status %q", i.Spec.Status)
	}

	return nil
}

// isValidEnvVarName checks if the given name is valid for use in script installers.
func isValidEnvVarName(name string) bool {
	if name == "" {
		return false
	}

	for _, c := range name {
		if c == '=' || unicode.IsSpace(c) {
			return false
		}
	}

	return true
}

func (i *LocalScriptInstallerV1) setStaticFields() {
	if i.Version == "" {
		i.Version = V1
	}

	if i.Kind == "" {
		i.Kind = KindVersionControlInstaller
	}

	if i.SubKind == "" {
		i.SubKind = SubKindLocalScript
	}
}

func (i *LocalScriptInstallerV1) BaseInstallMsg() ExecScript {
	msg := i.baseExecMsg()
	msg.Script = i.Spec.InstallScript
	return msg
}

func (i *LocalScriptInstallerV1) BaseRestartMsg() ExecScript {
	msg := i.baseExecMsg()
	msg.Script = i.Spec.RestartScript
	return msg
}

func (i *LocalScriptInstallerV1) baseExecMsg() ExecScript {
	return ExecScript{
		Env:            i.Spec.Env,
		EnvPassthrough: i.Spec.EnvPassthrough,
		Shell:          i.Spec.Shell,
	}
}

func (i *LocalScriptInstallerV1) Clone() LocalScriptInstaller {
	return proto.Clone(i).(*LocalScriptInstallerV1)
}
