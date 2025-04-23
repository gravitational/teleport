/*
 * Copyright 2023 Gravitational, Inc.
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

	"github.com/gravitational/teleport/api/constants"
)

// UIConfig defines configuration for the web UI served
// by the proxy service. This is a configuration resource,
// never create more than one instance of it.
type UIConfig interface {
	Resource
	// GetShowResources will returns which resources should be shown in the unified resources UI
	GetShowResources() constants.ShowResources
	// GetScrollbackLines returns the amount of scrollback lines the terminal remembers
	GetScrollbackLines() int32
	// SetScrollbackLines sets the amount of scrollback lines the terminal remembers
	SetScrollbackLines(int32)

	String() string
}

func NewUIConfigV1() (*UIConfigV1, error) {
	uiconfig := &UIConfigV1{
		ResourceHeader: ResourceHeader{},
	}
	if err := uiconfig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return uiconfig, nil
}

// CheckAndSetDefaults verifies the constraints for UIConfig.
func (c *UIConfigV1) CheckAndSetDefaults() error {
	c.setStaticFields()
	if err := c.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if c.Spec.ScrollbackLines < 0 {
		return trace.BadParameter("invalid scrollback lines value. Must be greater than or equal to 0.")
	}
	if c.Spec.ShowResources == "" {
		c.Spec.ShowResources = constants.ShowResourcesRequestable
	}
	switch c.Spec.ShowResources {
	case constants.ShowResourcesaccessibleOnly,
		constants.ShowResourcesRequestable:
	default:
		return trace.BadParameter("show resources %q not supported", c.Spec.ShowResources)
	}
	return nil
}

// GetVersion returns resource version.
func (c *UIConfigV1) GetVersion() string {
	return c.Version
}

// GetName returns the name of the resource.
func (c *UIConfigV1) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the resource.
func (c *UIConfigV1) SetName(e string) {
	c.Metadata.Name = e
}

// SetExpiry sets expiry time for the object.
func (c *UIConfigV1) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting.
func (c *UIConfigV1) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// GetMetadata returns object metadata.
func (c *UIConfigV1) GetMetadata() Metadata {
	return c.Metadata
}

// GetKind returns resource kind.
func (c *UIConfigV1) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource subkind.
func (c *UIConfigV1) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind.
func (c *UIConfigV1) SetSubKind(sk string) {
	c.SubKind = sk
}

// GetShowResources will returns which resources should be shown in the unified resources UI
func (c *UIConfigV1) GetShowResources() constants.ShowResources {
	return c.Spec.ShowResources
}

func (c *UIConfigV1) GetScrollbackLines() int32 {
	return c.Spec.ScrollbackLines
}

func (c *UIConfigV1) SetScrollbackLines(lines int32) {
	c.Spec.ScrollbackLines = lines
}

// setStaticFields sets static resource header and metadata fields.
func (c *UIConfigV1) setStaticFields() {
	c.Kind = KindUIConfig
	c.Version = V1
}
