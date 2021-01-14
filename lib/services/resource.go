/*
Copyright 2015-2019 Gravitational, Inc.

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
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"
)

// AddOptions adds marshal options and returns a new copy
func AddOptions(opts []MarshalOption, add ...MarshalOption) []MarshalOption {
	out := make([]MarshalOption, len(opts), len(opts)+len(add))
	copy(out, opts)
	return append(opts, add...)
}

// WithResourceID assigns ID to the resource
func WithResourceID(id int64) MarshalOption {
	return func(c *MarshalConfig) error {
		c.ID = id
		return nil
	}
}

// WithExpires assigns expiry value
func WithExpires(expires time.Time) MarshalOption {
	return func(c *MarshalConfig) error {
		c.Expires = expires
		return nil
	}
}

// WithVersion sets marshal version
func WithVersion(v string) MarshalOption {
	return func(c *MarshalConfig) error {
		switch v {
		case V1, V2, V3:
			c.Version = v
			return nil
		default:
			return trace.BadParameter("version '%v' is not supported", v)
		}
	}
}

// PreserveResourceID preserves resource ID when
// marshaling value
func PreserveResourceID() MarshalOption {
	return func(c *MarshalConfig) error {
		c.PreserveResourceID = true
		return nil
	}
}

// SkipValidation is used to disable schema validation.
func SkipValidation() MarshalOption {
	return func(c *MarshalConfig) error {
		c.SkipValidation = true
		return nil
	}
}

// ParseShortcut parses resource shortcut
func ParseShortcut(in string) (string, error) {
	if in == "" {
		return "", trace.BadParameter("missing resource name")
	}
	switch strings.ToLower(in) {
	case KindRole, "roles":
		return KindRole, nil
	case KindNamespace, "namespaces", "ns":
		return KindNamespace, nil
	case KindAuthServer, "auth_servers", "auth":
		return KindAuthServer, nil
	case KindProxy, "proxies":
		return KindProxy, nil
	case KindNode, "nodes":
		return KindNode, nil
	case KindOIDCConnector:
		return KindOIDCConnector, nil
	case KindSAMLConnector:
		return KindSAMLConnector, nil
	case KindGithubConnector:
		return KindGithubConnector, nil
	case KindConnectors, "connector":
		return KindConnectors, nil
	case KindUser, "users":
		return KindUser, nil
	case KindCertAuthority, "cert_authorities", "cas":
		return KindCertAuthority, nil
	case KindReverseTunnel, "reverse_tunnels", "rts":
		return KindReverseTunnel, nil
	case KindTrustedCluster, "tc", "cluster", "clusters":
		return KindTrustedCluster, nil
	case KindClusterAuthPreference, "cluster_authentication_preferences", "cap":
		return KindClusterAuthPreference, nil
	case KindRemoteCluster, "remote_clusters", "rc", "rcs":
		return KindRemoteCluster, nil
	case KindSemaphore, "semaphores", "sem", "sems":
		return KindSemaphore, nil
	case KindKubeService, "kube_services":
		return KindKubeService, nil
	}
	return "", trace.BadParameter("unsupported resource: %q - resources should be expressed as 'type/name', for example 'connector/github'", in)
}

// ParseRef parses resource reference eg daemonsets/ds1
func ParseRef(ref string) (*Ref, error) {
	if ref == "" {
		return nil, trace.BadParameter("missing value")
	}
	parts := strings.FieldsFunc(ref, isDelimiter)
	switch len(parts) {
	case 1:
		shortcut, err := ParseShortcut(parts[0])
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &Ref{Kind: shortcut}, nil
	case 2:
		shortcut, err := ParseShortcut(parts[0])
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &Ref{Kind: shortcut, Name: parts[1]}, nil
	case 3:
		shortcut, err := ParseShortcut(parts[0])
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &Ref{Kind: shortcut, SubKind: parts[1], Name: parts[2]}, nil
	}
	return nil, trace.BadParameter("failed to parse '%v'", ref)
}

// isDelimiter returns true if rune is space or /
func isDelimiter(r rune) bool {
	switch r {
	case '\t', ' ', '/':
		return true
	}
	return false
}

// Ref is a resource reference.  Typically of the form kind/name,
// but sometimes of the form kind/subkind/name.
type Ref struct {
	Kind    string
	SubKind string
	Name    string
}

// IsEmpty checks whether the provided resource name is empty
func (r *Ref) IsEmpty() bool {
	return r.Name == ""
}

// Set sets the name of the resource
func (r *Ref) Set(v string) error {
	out, err := ParseRef(v)
	if err != nil {
		return err
	}
	*r = *out
	return nil
}

func (r *Ref) String() string {
	if r.SubKind == "" {
		return fmt.Sprintf("%s/%s", r.Kind, r.Name)
	}
	return fmt.Sprintf("%s/%s/%s", r.Kind, r.SubKind, r.Name)
}

// Refs is a set of resource references
type Refs []Ref

// ParseRefs parses a comma-separated string of resource references (eg "users/alice,users/bob")
func ParseRefs(refs string) (Refs, error) {
	if refs == "all" {
		return []Ref{{Kind: "all"}}, nil
	}
	var escaped bool
	isBreak := func(r rune) bool {
		brk := false
		switch r {
		case ',':
			brk = true && !escaped
			escaped = false
		case '\\':
			escaped = true && !escaped
		default:
			escaped = false
		}
		return brk
	}
	var parsed []Ref
	split := fieldsFunc(strings.TrimSpace(refs), isBreak)
	for _, s := range split {
		ref, err := ParseRef(strings.ReplaceAll(s, `\,`, `,`))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		parsed = append(parsed, *ref)
	}
	return parsed, nil
}

// Set sets the value of `r` from a comma-separated string of resource
// references (in-place equivalent of `ParseRefs`).
func (r *Refs) Set(v string) error {
	refs, err := ParseRefs(v)
	if err != nil {
		return trace.Wrap(err)
	}
	*r = refs
	return nil
}

// IsAll checks if refs is special wildcard case `all`.
func (r *Refs) IsAll() bool {
	refs := *r
	if len(refs) != 1 {
		return false
	}
	return refs[0].Kind == "all"
}

func (r *Refs) String() string {
	var builder strings.Builder
	for i, ref := range *r {
		if i > 0 {
			builder.WriteRune(',')
		}
		builder.WriteString(ref.String())
	}
	return builder.String()
}

// fieldsFunc is an exact copy of the current implementation of `strings.FieldsFunc`.
// The docs of `strings.FieldsFunc` indicate that future implementations may not call
// `f` on every rune, may not preserve ordering, or may panic if `f` does not return the
// same output for every instance of a given rune.  All of these changes would break
// our implementation of backslash-escaping, so we're using a local copy.
func fieldsFunc(s string, f func(rune) bool) []string {
	// A span is used to record a slice of s of the form s[start:end].
	// The start index is inclusive and the end index is exclusive.
	type span struct {
		start int
		end   int
	}
	spans := make([]span, 0, 32)

	// Find the field start and end indices.
	wasField := false
	fromIndex := 0
	for i, rune := range s {
		if f(rune) {
			if wasField {
				spans = append(spans, span{start: fromIndex, end: i})
				wasField = false
			}
		} else {
			if !wasField {
				fromIndex = i
				wasField = true
			}
		}
	}

	// Last field might end at EOF.
	if wasField {
		spans = append(spans, span{fromIndex, len(s)})
	}

	// Create strings from recorded field indices.
	a := make([]string, len(spans))
	for i, span := range spans {
		a[i] = s[span.start:span.end]
	}

	return a
}
