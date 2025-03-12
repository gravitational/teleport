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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/utils"
)

// ClusterNetworkingConfig defines cluster networking configuration. This is
// a configuration resource, never create more than one instance of it.
type ClusterNetworkingConfig interface {
	ResourceWithOrigin

	// GetClientIdleTimeout returns client idle timeout setting
	GetClientIdleTimeout() time.Duration

	// SetClientIdleTimeout sets client idle timeout setting
	SetClientIdleTimeout(t time.Duration)

	// GetKeepAliveInterval gets the keep-alive interval for server to client
	// connections.
	GetKeepAliveInterval() time.Duration

	// SetKeepAliveInterval sets the keep-alive interval for server to client
	// connections.
	SetKeepAliveInterval(t time.Duration)

	// GetKeepAliveCountMax gets the number of missed keep-alive messages before
	// the server disconnects the client.
	GetKeepAliveCountMax() int64

	// SetKeepAliveCountMax sets the number of missed keep-alive messages before
	// the server disconnects the client.
	SetKeepAliveCountMax(c int64)

	// GetSessionControlTimeout gets the session control timeout.
	GetSessionControlTimeout() time.Duration

	// SetSessionControlTimeout sets the session control timeout.
	SetSessionControlTimeout(t time.Duration)

	// GetClientIdleTimeoutMessage fetches the message to be sent to the client in
	// the event of an idle timeout. An empty string implies no message should
	// be sent.
	GetClientIdleTimeoutMessage() string

	// SetClientIdleTimeoutMessage sets the inactivity timeout disconnection message
	// to be sent to the user.
	SetClientIdleTimeoutMessage(string)

	// GetWebIdleTimeout gets web idle timeout duration.
	GetWebIdleTimeout() time.Duration

	// SetWebIdleTimeout sets the web idle timeout duration.
	SetWebIdleTimeout(time.Duration)

	// GetProxyListenerMode gets the proxy listener mode.
	GetProxyListenerMode() ProxyListenerMode

	// SetProxyListenerMode sets the proxy listener mode.
	SetProxyListenerMode(ProxyListenerMode)

	// Clone performs a deep copy.
	Clone() ClusterNetworkingConfig

	// GetRoutingStrategy gets the routing strategy setting.
	GetRoutingStrategy() RoutingStrategy

	// SetRoutingStrategy sets the routing strategy setting.
	SetRoutingStrategy(strategy RoutingStrategy)

	// GetTunnelStrategy gets the tunnel strategy.
	GetTunnelStrategyType() (TunnelStrategyType, error)

	// GetAgentMeshTunnelStrategy gets the agent mesh tunnel strategy.
	GetAgentMeshTunnelStrategy() *AgentMeshTunnelStrategy

	// GetProxyPeeringTunnelStrategy gets the proxy peering tunnel strategy.
	GetProxyPeeringTunnelStrategy() *ProxyPeeringTunnelStrategy

	// SetTunnelStrategy sets the tunnel strategy.
	SetTunnelStrategy(*TunnelStrategyV1)

	// GetProxyPingInterval gets the proxy ping interval.
	GetProxyPingInterval() time.Duration

	// SetProxyPingInterval sets the proxy ping interval.
	SetProxyPingInterval(time.Duration)

	// GetCaseInsensitiveRouting gets the case-insensitive routing option.
	GetCaseInsensitiveRouting() bool

	// SetCaseInsensitiveRouting sets the case-insenstivie routing option.
	SetCaseInsensitiveRouting(cir bool)

	// GetSSHDialTimeout gets timeout value that should be used for SSH connections.
	GetSSHDialTimeout() time.Duration

	// SetSSHDialTimeout sets the timeout value that should be used for SSH connections.
	SetSSHDialTimeout(t time.Duration)
}

// NewClusterNetworkingConfigFromConfigFile is a convenience method to create
// ClusterNetworkingConfigV2 labeled as originating from config file.
func NewClusterNetworkingConfigFromConfigFile(spec ClusterNetworkingConfigSpecV2) (ClusterNetworkingConfig, error) {
	return newClusterNetworkingConfigWithLabels(spec, map[string]string{
		OriginLabel: OriginConfigFile,
	})
}

// DefaultClusterNetworkingConfig returns the default cluster networking config.
func DefaultClusterNetworkingConfig() ClusterNetworkingConfig {
	config, _ := newClusterNetworkingConfigWithLabels(ClusterNetworkingConfigSpecV2{}, map[string]string{
		OriginLabel: OriginDefaults,
	})
	return config
}

// newClusterNetworkingConfigWithLabels is a convenience method to create
// ClusterNetworkingConfigV2 with a specific map of labels.
func newClusterNetworkingConfigWithLabels(spec ClusterNetworkingConfigSpecV2, labels map[string]string) (ClusterNetworkingConfig, error) {
	c := &ClusterNetworkingConfigV2{
		Metadata: Metadata{
			Labels: labels,
		},
		Spec: spec,
	}
	if err := c.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return c, nil
}

// GetVersion returns resource version.
func (c *ClusterNetworkingConfigV2) GetVersion() string {
	return c.Version
}

// GetName returns the name of the resource.
func (c *ClusterNetworkingConfigV2) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the resource.
func (c *ClusterNetworkingConfigV2) SetName(name string) {
	c.Metadata.Name = name
}

// SetExpiry sets expiry time for the object.
func (c *ClusterNetworkingConfigV2) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting.
func (c *ClusterNetworkingConfigV2) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// GetMetadata returns object metadata.
func (c *ClusterNetworkingConfigV2) GetMetadata() Metadata {
	return c.Metadata
}

// GetRevision returns the revision
func (c *ClusterNetworkingConfigV2) GetRevision() string {
	return c.Metadata.GetRevision()
}

// SetRevision sets the revision
func (c *ClusterNetworkingConfigV2) SetRevision(rev string) {
	c.Metadata.SetRevision(rev)
}

// Origin returns the origin value of the resource.
func (c *ClusterNetworkingConfigV2) Origin() string {
	return c.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (c *ClusterNetworkingConfigV2) SetOrigin(origin string) {
	c.Metadata.SetOrigin(origin)
}

// GetKind returns resource kind.
func (c *ClusterNetworkingConfigV2) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource subkind.
func (c *ClusterNetworkingConfigV2) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind.
func (c *ClusterNetworkingConfigV2) SetSubKind(sk string) {
	c.SubKind = sk
}

// GetClientIdleTimeout returns client idle timeout setting.
func (c *ClusterNetworkingConfigV2) GetClientIdleTimeout() time.Duration {
	return c.Spec.ClientIdleTimeout.Duration()
}

// SetClientIdleTimeout sets client idle timeout setting.
func (c *ClusterNetworkingConfigV2) SetClientIdleTimeout(d time.Duration) {
	c.Spec.ClientIdleTimeout = Duration(d)
}

// GetKeepAliveInterval gets the keep-alive interval.
func (c *ClusterNetworkingConfigV2) GetKeepAliveInterval() time.Duration {
	return c.Spec.KeepAliveInterval.Duration()
}

// SetKeepAliveInterval sets the keep-alive interval.
func (c *ClusterNetworkingConfigV2) SetKeepAliveInterval(t time.Duration) {
	c.Spec.KeepAliveInterval = Duration(t)
}

// GetKeepAliveCountMax gets the number of missed keep-alive messages before
// the server disconnects the client.
func (c *ClusterNetworkingConfigV2) GetKeepAliveCountMax() int64 {
	return c.Spec.KeepAliveCountMax
}

// SetKeepAliveCountMax sets the number of missed keep-alive messages before
// the server disconnects the client.
func (c *ClusterNetworkingConfigV2) SetKeepAliveCountMax(m int64) {
	c.Spec.KeepAliveCountMax = m
}

// GetSessionControlTimeout gets the session control timeout.
func (c *ClusterNetworkingConfigV2) GetSessionControlTimeout() time.Duration {
	return c.Spec.SessionControlTimeout.Duration()
}

// SetSessionControlTimeout sets the session control timeout.
func (c *ClusterNetworkingConfigV2) SetSessionControlTimeout(d time.Duration) {
	c.Spec.SessionControlTimeout = Duration(d)
}

func (c *ClusterNetworkingConfigV2) GetClientIdleTimeoutMessage() string {
	return c.Spec.ClientIdleTimeoutMessage
}

func (c *ClusterNetworkingConfigV2) SetClientIdleTimeoutMessage(msg string) {
	c.Spec.ClientIdleTimeoutMessage = msg
}

// GetWebIdleTimeout gets the web idle timeout.
func (c *ClusterNetworkingConfigV2) GetWebIdleTimeout() time.Duration {
	return c.Spec.WebIdleTimeout.Duration()
}

// SetWebIdleTimeout sets the web idle timeout.
func (c *ClusterNetworkingConfigV2) SetWebIdleTimeout(ttl time.Duration) {
	c.Spec.WebIdleTimeout = Duration(ttl)
}

// GetProxyListenerMode gets the proxy listener mode.
func (c *ClusterNetworkingConfigV2) GetProxyListenerMode() ProxyListenerMode {
	return c.Spec.ProxyListenerMode
}

// SetProxyListenerMode sets the proxy listener mode.
func (c *ClusterNetworkingConfigV2) SetProxyListenerMode(mode ProxyListenerMode) {
	c.Spec.ProxyListenerMode = mode
}

// Clone performs a deep copy.
func (c *ClusterNetworkingConfigV2) Clone() ClusterNetworkingConfig {
	return utils.CloneProtoMsg(c)
}

// setStaticFields sets static resource header and metadata fields.
func (c *ClusterNetworkingConfigV2) setStaticFields() {
	c.Kind = KindClusterNetworkingConfig
	c.Version = V2
	c.Metadata.Name = MetaNameClusterNetworkingConfig
}

// GetRoutingStrategy gets the routing strategy setting.
func (c *ClusterNetworkingConfigV2) GetRoutingStrategy() RoutingStrategy {
	return c.Spec.RoutingStrategy
}

// SetRoutingStrategy sets the routing strategy setting.
func (c *ClusterNetworkingConfigV2) SetRoutingStrategy(strategy RoutingStrategy) {
	c.Spec.RoutingStrategy = strategy
}

// GetTunnelStrategy gets the tunnel strategy type.
func (c *ClusterNetworkingConfigV2) GetTunnelStrategyType() (TunnelStrategyType, error) {
	if c.Spec.TunnelStrategy == nil {
		return "", trace.BadParameter("tunnel strategy is nil")
	}

	switch c.Spec.TunnelStrategy.Strategy.(type) {
	case *TunnelStrategyV1_AgentMesh:
		return AgentMesh, nil
	case *TunnelStrategyV1_ProxyPeering:
		return ProxyPeering, nil
	}

	return "", trace.BadParameter("unknown tunnel strategy type: %T", c.Spec.TunnelStrategy.Strategy)
}

// GetAgentMeshTunnelStrategy gets the agent mesh tunnel strategy.
func (c *ClusterNetworkingConfigV2) GetAgentMeshTunnelStrategy() *AgentMeshTunnelStrategy {
	return c.Spec.TunnelStrategy.GetAgentMesh()
}

// GetProxyPeeringTunnelStrategy gets the proxy peering tunnel strategy.
func (c *ClusterNetworkingConfigV2) GetProxyPeeringTunnelStrategy() *ProxyPeeringTunnelStrategy {
	return c.Spec.TunnelStrategy.GetProxyPeering()
}

// SetTunnelStrategy sets the tunnel strategy.
func (c *ClusterNetworkingConfigV2) SetTunnelStrategy(strategy *TunnelStrategyV1) {
	c.Spec.TunnelStrategy = strategy
}

// CheckAndSetDefaults verifies the constraints for ClusterNetworkingConfig.
func (c *ClusterNetworkingConfigV2) CheckAndSetDefaults() error {
	c.setStaticFields()
	if err := c.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// Make sure origin value is always set.
	if c.Origin() == "" {
		c.SetOrigin(OriginDynamic)
	}

	// Set the keep-alive interval and max missed keep-alives.
	if c.Spec.KeepAliveInterval.Duration() == 0 {
		c.Spec.KeepAliveInterval = NewDuration(defaults.KeepAliveInterval())
	}
	if c.Spec.KeepAliveCountMax == 0 {
		c.Spec.KeepAliveCountMax = int64(defaults.KeepAliveCountMax)
	}

	if c.Spec.TunnelStrategy == nil {
		c.Spec.TunnelStrategy = &TunnelStrategyV1{
			Strategy: DefaultTunnelStrategy(),
		}
	}
	if err := c.Spec.TunnelStrategy.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetProxyPingInterval gets the proxy ping interval.
func (c *ClusterNetworkingConfigV2) GetProxyPingInterval() time.Duration {
	return c.Spec.ProxyPingInterval.Duration()
}

// SetProxyPingInterval sets the proxy ping interval.
func (c *ClusterNetworkingConfigV2) SetProxyPingInterval(interval time.Duration) {
	c.Spec.ProxyPingInterval = Duration(interval)
}

// GetCaseInsensitiveRouting gets the case-insensitive routing option.
func (c *ClusterNetworkingConfigV2) GetCaseInsensitiveRouting() bool {
	return c.Spec.CaseInsensitiveRouting
}

// SetCaseInsensitiveRouting sets the case-insensitive routing option.
func (c *ClusterNetworkingConfigV2) SetCaseInsensitiveRouting(cir bool) {
	c.Spec.CaseInsensitiveRouting = cir
}

// GetSSHDialTimeout returns the timeout to be used for SSH connections.
// If the value is not set, or was intentionally set to zero or a negative value,
// [defaults.DefaultIOTimeout] is returned instead. This is because
// a zero value cannot be distinguished to mean no timeout, or
// that a value had never been set.
func (c *ClusterNetworkingConfigV2) GetSSHDialTimeout() time.Duration {
	if c.Spec.SSHDialTimeout <= 0 {
		return defaults.DefaultIOTimeout
	}

	return c.Spec.SSHDialTimeout.Duration()
}

// SetSSHDialTimeout updates the SSH connection timeout. The value is
// not validated, but will not be respected if zero or negative. See
// the docs on [ClusterNetworkingConfigV2.GetSSHDialTimeout] for more details.
func (c *ClusterNetworkingConfigV2) SetSSHDialTimeout(t time.Duration) {
	c.Spec.SSHDialTimeout = Duration(t)
}

// MarshalYAML defines how a proxy listener mode should be marshaled to a string
func (p ProxyListenerMode) MarshalYAML() (interface{}, error) {
	return strings.ToLower(p.String()), nil
}

// UnmarshalYAML unmarshalls proxy listener mode from YAML value.
func (p *ProxyListenerMode) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var stringVar string
	if err := unmarshal(&stringVar); err != nil {
		return trace.Wrap(err)
	}
	for k, v := range ProxyListenerMode_value {
		if strings.EqualFold(k, stringVar) {
			*p = ProxyListenerMode(v)
			return nil
		}
	}

	available := make([]string, 0, len(ProxyListenerMode_value))
	for k := range ProxyListenerMode_value {
		available = append(available, strings.ToLower(k))
	}
	return trace.BadParameter(
		"proxy listener mode must be one of %s; got %q", strings.Join(available, ","), stringVar)
}

// MarshalYAML defines how a routing strategy should be marshaled to a string
func (s RoutingStrategy) MarshalYAML() (interface{}, error) {
	return strings.ToLower(s.String()), nil
}

// UnmarshalYAML unmarshalls routing strategy from YAML value.
func (s *RoutingStrategy) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var stringVar string
	if err := unmarshal(&stringVar); err != nil {
		return trace.Wrap(err)
	}

	for k, v := range RoutingStrategy_value {
		if strings.EqualFold(k, stringVar) {
			*s = RoutingStrategy(v)
			return nil
		}
	}

	available := make([]string, 0, len(RoutingStrategy_value))
	for k := range RoutingStrategy_value {
		available = append(available, strings.ToLower(k))
	}
	return trace.BadParameter(
		"routing strategy must be one of %s; got %q", strings.Join(available, ","), stringVar)
}
