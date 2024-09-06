/*
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
	"time"

	"github.com/gravitational/trace"
)

// Installer is an installer script resource
type Installer interface {
	Resource

	// GetScript returns the contents of the installer script
	GetScript() string
	// SetScript sets the installer script
	SetScript(string)

	String() string
}

// NewInstallerV1 returns a new installer resource
func NewInstallerV1(name, script string) (*InstallerV1, error) {
	installer := &InstallerV1{
		Metadata: Metadata{
			Name: name,
		},
		Spec: InstallerSpecV1{
			Script: script,
		},
	}
	if err := installer.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return installer, nil
}

// MustNewInstallerV1 creates a new installer resource from the provided script.
//
// Panics in case of any error when creating the resource.
func MustNewInstallerV1(name, script string) *InstallerV1 {
	inst, err := NewInstallerV1(name, script)
	if err != nil {
		panic(err)
	}
	return inst
}

// CheckAndSetDefaults implements Installer
func (c *InstallerV1) CheckAndSetDefaults() error {
	c.setStaticFields()
	return trace.Wrap(c.Metadata.CheckAndSetDefaults())
}

// GetVersion returns resource version.
func (c *InstallerV1) GetVersion() string {
	return c.Version
}

// GetName returns the name of the resource.
func (c *InstallerV1) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the resource.
func (c *InstallerV1) SetName(e string) {
	c.Metadata.Name = e
}

// SetExpiry sets expiry time for the object.
func (c *InstallerV1) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting.
func (c *InstallerV1) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// GetMetadata returns object metadata.
func (c *InstallerV1) GetMetadata() Metadata {
	return c.Metadata
}

// GetRevision returns the revision
func (c *InstallerV1) GetRevision() string {
	return c.Metadata.GetRevision()
}

// SetRevision sets the revision
func (c *InstallerV1) SetRevision(rev string) {
	c.Metadata.SetRevision(rev)
}

// GetKind returns resource kind.
func (c *InstallerV1) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource subkind.
func (c *InstallerV1) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind.
func (c *InstallerV1) SetSubKind(sk string) {
	c.SubKind = sk
}

func (c *InstallerV1) GetScript() string {
	return c.Spec.Script
}

func (c *InstallerV1) SetScript(s string) {
	c.Spec.Script = s
}

// setStaticFields sets static resource header and metadata fields.
func (c *InstallerV1) setStaticFields() {
	c.Kind = KindInstaller
	c.Version = V1
}
