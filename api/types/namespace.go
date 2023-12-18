/*
Copyright 2015-2018 Gravitational, Inc.

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

package types

import (
	"regexp"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
)

// NewNamespace returns new namespace
func NewNamespace(name string) (Namespace, error) {
	n := Namespace{
		Metadata: Metadata{
			Name: name,
		},
	}
	if err := n.CheckAndSetDefaults(); err != nil {
		return Namespace{}, trace.Wrap(err)
	}
	return n, nil
}

// DefaultNamespace returns the default namespace.
func DefaultNamespace() Namespace {
	namespace, _ := NewNamespace(defaults.Namespace)
	return namespace
}

// setStaticFields sets static resource header and metadata fields.
func (n *Namespace) setStaticFields() {
	n.Kind = KindNamespace
	n.Version = V2
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults
func (n *Namespace) CheckAndSetDefaults() error {
	n.setStaticFields()
	if err := n.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if !IsValidNamespace(n.Metadata.Name) {
		return trace.BadParameter("namespace %q is invalid", n.Metadata.Name)
	}

	return nil
}

// GetVersion returns resource version
func (n *Namespace) GetVersion() string {
	return n.Version
}

// GetKind returns resource kind
func (n *Namespace) GetKind() string {
	return n.Kind
}

// GetSubKind returns resource sub kind
func (n *Namespace) GetSubKind() string {
	return n.SubKind
}

// SetSubKind sets resource subkind
func (n *Namespace) SetSubKind(sk string) {
	n.SubKind = sk
}

// GetResourceID returns resource ID
func (n *Namespace) GetResourceID() int64 {
	return n.Metadata.ID
}

// SetResourceID sets resource ID
func (n *Namespace) SetResourceID(id int64) {
	n.Metadata.ID = id
}

// GetRevision returns the revision
func (n *Namespace) GetRevision() string {
	return n.Metadata.GetRevision()
}

// SetRevision sets the revision
func (n *Namespace) SetRevision(rev string) {
	n.Metadata.SetRevision(rev)
}

// GetName returns the name of the cluster.
func (n *Namespace) GetName() string {
	return n.Metadata.Name
}

// SetName sets the name of the cluster.
func (n *Namespace) SetName(e string) {
	n.Metadata.Name = e
}

// Expiry returns object expiry setting
func (n *Namespace) Expiry() time.Time {
	return n.Metadata.Expiry()
}

// SetExpiry sets expiry time for the object
func (n *Namespace) SetExpiry(expires time.Time) {
	n.Metadata.SetExpiry(expires)
}

// GetMetadata returns object metadata
func (n *Namespace) GetMetadata() Metadata {
	return n.Metadata
}

// SortedNamespaces sorts namespaces
type SortedNamespaces []Namespace

// Len returns length of a role list
func (s SortedNamespaces) Len() int {
	return len(s)
}

// Less compares roles by name
func (s SortedNamespaces) Less(i, j int) bool {
	return s[i].Metadata.Name < s[j].Metadata.Name
}

// Swap swaps two roles in a list
func (s SortedNamespaces) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// IsValidNamespace checks if the namespace provided is valid
func IsValidNamespace(s string) bool {
	return validNamespace.MatchString(s)
}

var validNamespace = regexp.MustCompile(`^[A-Za-z0-9]+$`)
