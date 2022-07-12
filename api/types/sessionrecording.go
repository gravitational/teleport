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

package types

import (
	"strings"
	"time"

	"github.com/gravitational/teleport/api/utils"

	"github.com/gravitational/trace"
)

// SessionRecordingConfig defines session recording configuration. This is
// a configuration resource, never create more than one instance of it.
type SessionRecordingConfig interface {
	ResourceWithOrigin

	// GetMode gets the session recording mode.
	GetMode() string

	// SetMode sets the session recording mode.
	SetMode(string)

	// GetProxyChecksHostKeys gets if the proxy will check host keys.
	GetProxyChecksHostKeys() bool

	// SetProxyChecksHostKeys sets if the proxy will check host keys.
	SetProxyChecksHostKeys(bool)
}

// NewSessionRecordingConfigFromConfigFile is a convenience method to create
// SessionRecordingConfigV2 labeled as originating from config file.
func NewSessionRecordingConfigFromConfigFile(spec SessionRecordingConfigSpecV2) (SessionRecordingConfig, error) {
	return newSessionRecordingConfigWithLabels(spec, map[string]string{
		OriginLabel: OriginConfigFile,
	})
}

// DefaultSessionRecordingConfig returns the default session recording configuration.
func DefaultSessionRecordingConfig() SessionRecordingConfig {
	config, _ := newSessionRecordingConfigWithLabels(SessionRecordingConfigSpecV2{}, map[string]string{
		OriginLabel: OriginDefaults,
	})
	return config
}

// newSessionRecordingConfigWithLabels is a convenience method to create
// SessionRecordingConfigV2 with a specific map of labels.
func newSessionRecordingConfigWithLabels(spec SessionRecordingConfigSpecV2, labels map[string]string) (SessionRecordingConfig, error) {
	recConfig := &SessionRecordingConfigV2{
		Metadata: Metadata{
			Labels: labels,
		},
		Spec: spec,
	}
	if err := recConfig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return recConfig, nil
}

// GetVersion returns resource version.
func (c *SessionRecordingConfigV2) GetVersion() string {
	return c.Version
}

// GetName returns the name of the resource.
func (c *SessionRecordingConfigV2) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the resource.
func (c *SessionRecordingConfigV2) SetName(e string) {
	c.Metadata.Name = e
}

// SetExpiry sets expiry time for the object.
func (c *SessionRecordingConfigV2) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting.
func (c *SessionRecordingConfigV2) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// GetMetadata returns object metadata.
func (c *SessionRecordingConfigV2) GetMetadata() Metadata {
	return c.Metadata
}

// GetResourceID returns resource ID.
func (c *SessionRecordingConfigV2) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID.
func (c *SessionRecordingConfigV2) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// Origin returns the origin value of the resource.
func (c *SessionRecordingConfigV2) Origin() string {
	return c.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (c *SessionRecordingConfigV2) SetOrigin(origin string) {
	c.Metadata.SetOrigin(origin)
}

// GetKind returns resource kind.
func (c *SessionRecordingConfigV2) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource subkind.
func (c *SessionRecordingConfigV2) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind.
func (c *SessionRecordingConfigV2) SetSubKind(sk string) {
	c.SubKind = sk
}

// GetMode gets the cluster's session recording mode.
func (c *SessionRecordingConfigV2) GetMode() string {
	return c.Spec.Mode
}

// SetMode sets the cluster's session recording mode.
func (c *SessionRecordingConfigV2) SetMode(m string) {
	c.Spec.Mode = m
}

// GetProxyChecksHostKeys gets if the proxy will check host keys.
func (c *SessionRecordingConfigV2) GetProxyChecksHostKeys() bool {
	return c.Spec.ProxyChecksHostKeys.Value
}

// SetProxyChecksHostKeys sets if the proxy will check host keys.
func (c *SessionRecordingConfigV2) SetProxyChecksHostKeys(t bool) {
	c.Spec.ProxyChecksHostKeys = NewBoolOption(t)
}

// setStaticFields sets static resource header and metadata fields.
func (c *SessionRecordingConfigV2) setStaticFields() {
	c.Kind = KindSessionRecordingConfig
	c.Version = V2
	c.Metadata.Name = MetaNameSessionRecordingConfig
}

// CheckAndSetDefaults verifies the constraints for SessionRecordingConfig.
func (c *SessionRecordingConfigV2) CheckAndSetDefaults() error {
	c.setStaticFields()
	if err := c.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// Make sure origin value is always set.
	if c.Origin() == "" {
		c.SetOrigin(OriginDynamic)
	}

	if c.Spec.Mode == "" {
		c.Spec.Mode = RecordAtNode
	}
	if c.Spec.ProxyChecksHostKeys == nil {
		c.Spec.ProxyChecksHostKeys = NewBoolOption(true)
	}

	// Check that the session recording mode is set to a valid value.
	if !utils.SliceContainsStr(SessionRecordingModes, c.Spec.Mode) {
		return trace.BadParameter("session recording mode must be one of %v; got %q", strings.Join(SessionRecordingModes, ","), c.Spec.Mode)
	}

	return nil
}
