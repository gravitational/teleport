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
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// Equal means two objects are equal
	Equal = iota
	// OnlyTimestampsDifferent is true when only timestamps are different
	OnlyTimestampsDifferent = iota
	// Different means that some fields are different
	Different = iota
)

// CompareServers compares two provided servers.
func CompareServers(a, b types.Resource) int {
	if serverA, ok := a.(types.Server); ok {
		if serverB, ok := b.(types.Server); ok {
			return compareServers(serverA, serverB)
		}
	}
	if appA, ok := a.(types.AppServer); ok {
		if appB, ok := b.(types.AppServer); ok {
			return compareApplicationServers(appA, appB)
		}
	}
	if kubeA, ok := a.(types.KubeServer); ok {
		if kubeB, ok := b.(types.KubeServer); ok {
			return compareKubernetesServers(kubeA, kubeB)
		}
	}
	if dbA, ok := a.(types.DatabaseServer); ok {
		if dbB, ok := b.(types.DatabaseServer); ok {
			return compareDatabaseServers(dbA, dbB)
		}
	}
	if dbServiceA, ok := a.(types.DatabaseService); ok {
		if dbServiceB, ok := b.(types.DatabaseService); ok {
			return compareDatabaseServices(dbServiceA, dbServiceB)
		}
	}
	if winA, ok := a.(types.WindowsDesktopService); ok {
		if winB, ok := b.(types.WindowsDesktopService); ok {
			return compareWindowsDesktopServices(winA, winB)
		}
	}
	return Different
}

func compareServers(a, b types.Server) int {
	if a.GetKind() != b.GetKind() {
		return Different
	}
	if a.GetName() != b.GetName() {
		return Different
	}
	if a.GetAddr() != b.GetAddr() {
		return Different
	}
	if a.GetHostname() != b.GetHostname() {
		return Different
	}
	if a.GetNamespace() != b.GetNamespace() {
		return Different
	}
	if len(a.GetPublicAddrs()) != len(b.GetPublicAddrs()) {
		return Different
	}
	for i := range a.GetPublicAddrs() {
		if a.GetPublicAddrs()[i] != b.GetPublicAddrs()[i] {
			return Different
		}
	}
	r := a.GetRotation()
	if !r.Matches(b.GetRotation()) {
		return Different
	}
	if a.GetUseTunnel() != b.GetUseTunnel() {
		return Different
	}
	if !utils.StringMapsEqual(a.GetStaticLabels(), b.GetStaticLabels()) {
		return Different
	}
	if !cmp.Equal(a.GetCmdLabels(), b.GetCmdLabels()) {
		return Different
	}
	if a.GetTeleportVersion() != b.GetTeleportVersion() {
		return Different
	}
	if !cmp.Equal(a.GetApps(), b.GetApps()) {
		return Different
	}
	if !cmp.Equal(a.GetKubernetesClusters(), b.GetKubernetesClusters()) {
		return Different
	}
	if !cmp.Equal(a.GetProxyIDs(), b.GetProxyIDs()) {
		return Different
	}
	// OnlyTimestampsDifferent check must be after all Different checks.
	if !a.Expiry().Equal(b.Expiry()) {
		return OnlyTimestampsDifferent
	}
	return Equal
}

func compareApplicationServers(a, b types.AppServer) int {
	if a.GetKind() != b.GetKind() {
		return Different
	}
	if a.GetName() != b.GetName() {
		return Different
	}
	if a.GetNamespace() != b.GetNamespace() {
		return Different
	}
	if a.GetTeleportVersion() != b.GetTeleportVersion() {
		return Different
	}
	r := a.GetRotation()
	if !r.Matches(b.GetRotation()) {
		return Different
	}
	if !cmp.Equal(a.GetApp(), b.GetApp()) {
		return Different
	}
	if !cmp.Equal(a.GetProxyIDs(), b.GetProxyIDs()) {
		return Different
	}
	// OnlyTimestampsDifferent check must be after all Different checks.
	if !a.Expiry().Equal(b.Expiry()) {
		return OnlyTimestampsDifferent
	}
	return Equal
}

func compareDatabaseServices(a, b types.DatabaseService) int {
	if a.GetKind() != b.GetKind() {
		return Different
	}
	if a.GetName() != b.GetName() {
		return Different
	}
	if a.GetNamespace() != b.GetNamespace() {
		return Different
	}
	if !cmp.Equal(a.GetResourceMatchers(), b.GetResourceMatchers()) {
		return Different
	}
	if !a.Expiry().Equal(b.Expiry()) {
		return OnlyTimestampsDifferent
	}
	return Equal
}

func compareKubernetesServers(a, b types.KubeServer) int {
	if a.GetKind() != b.GetKind() {
		return Different
	}
	if a.GetName() != b.GetName() {
		return Different
	}
	if a.GetNamespace() != b.GetNamespace() {
		return Different
	}
	if a.GetTeleportVersion() != b.GetTeleportVersion() {
		return Different
	}
	r := a.GetRotation()
	if !r.Matches(b.GetRotation()) {
		return Different
	}
	if !cmp.Equal(a.GetCluster(), b.GetCluster()) {
		return Different
	}
	if !cmp.Equal(a.GetProxyIDs(), b.GetProxyIDs()) {
		return Different
	}
	// OnlyTimestampsDifferent check must be after all Different checks.
	if !a.Expiry().Equal(b.Expiry()) {
		return OnlyTimestampsDifferent
	}
	return Equal
}

func compareDatabaseServers(a, b types.DatabaseServer) int {
	if a.GetKind() != b.GetKind() {
		return Different
	}
	if a.GetName() != b.GetName() {
		return Different
	}
	if a.GetNamespace() != b.GetNamespace() {
		return Different
	}
	if a.GetTeleportVersion() != b.GetTeleportVersion() {
		return Different
	}
	r := a.GetRotation()
	if !r.Matches(b.GetRotation()) {
		return Different
	}
	if !cmp.Equal(a.GetDatabase(), b.GetDatabase()) {
		return Different
	}
	if !cmp.Equal(a.GetProxyIDs(), b.GetProxyIDs()) {
		return Different
	}
	// OnlyTimestampsDifferent check must be after all Different checks.
	if !a.Expiry().Equal(b.Expiry()) {
		return OnlyTimestampsDifferent
	}
	return Equal
}

func compareWindowsDesktopServices(a, b types.WindowsDesktopService) int {
	if a.GetKind() != b.GetKind() {
		return Different
	}
	if a.GetName() != b.GetName() {
		return Different
	}
	if a.GetAddr() != b.GetAddr() {
		return Different
	}
	if a.GetTeleportVersion() != b.GetTeleportVersion() {
		return Different
	}
	if !cmp.Equal(a.GetProxyIDs(), b.GetProxyIDs()) {
		return Different
	}
	// OnlyTimestampsDifferent check must be after all Different checks.
	if !a.Expiry().Equal(b.Expiry()) {
		return OnlyTimestampsDifferent
	}
	return Equal
}

// CommandLabels is a set of command labels
type CommandLabels map[string]types.CommandLabel

// Clone returns copy of the set
func (c *CommandLabels) Clone() CommandLabels {
	out := make(CommandLabels, len(*c))
	for name, label := range *c {
		out[name] = label.Clone()
	}
	return out
}

// SetEnv sets the value of the label from environment variable
func (c *CommandLabels) SetEnv(v string) error {
	if err := json.Unmarshal([]byte(v), c); err != nil {
		return trace.Wrap(err, "can not parse Command Labels")
	}
	return nil
}

// SortedServers is a sort wrapper that sorts servers by name
type SortedServers []types.Server

func (s SortedServers) Len() int {
	return len(s)
}

func (s SortedServers) Less(i, j int) bool {
	return s[i].GetName() < s[j].GetName()
}

func (s SortedServers) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// SortedReverseTunnels sorts reverse tunnels by cluster name
type SortedReverseTunnels []types.ReverseTunnel

func (s SortedReverseTunnels) Len() int {
	return len(s)
}

func (s SortedReverseTunnels) Less(i, j int) bool {
	return s[i].GetClusterName() < s[j].GetClusterName()
}

func (s SortedReverseTunnels) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// GuessProxyHostAndVersion tries to find the first proxy with a public
// address configured and return that public addr and version.
// If no proxies are configured, it will return a guessed value by concatenating
// the first proxy's hostname with default port number, and the first proxy's
// version will also be returned.
//
// Returns empty value if there are no proxies.
func GuessProxyHostAndVersion(proxies []types.Server) (string, string, error) {
	if len(proxies) == 0 {
		return "", "", trace.NotFound("list of proxies empty")
	}

	// Find the first proxy with a public address set and return it.
	for _, proxy := range proxies {
		proxyHost := proxy.GetPublicAddr()
		if proxyHost != "" {
			return proxyHost, proxy.GetTeleportVersion(), nil
		}
	}

	// No proxies have a public address set, return guessed value.
	guessProxyHost := fmt.Sprintf("%v:%v", proxies[0].GetHostname(), defaults.HTTPListenPort)
	return guessProxyHost, proxies[0].GetTeleportVersion(), nil
}

// UnmarshalServer unmarshals the Server resource from JSON.
func UnmarshalServer(bytes []byte, kind string, opts ...MarshalOption) (types.Server, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing server data")
	}

	var s types.ServerV2
	if err := utils.FastUnmarshal(bytes, &s); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	s.Kind = kind
	if err := s.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		s.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		s.SetExpiry(cfg.Expires)
	}
	if s.Metadata.Expires != nil {
		apiutils.UTC(s.Metadata.Expires)
	}
	// Force the timestamps to UTC for consistency.
	// See https://github.com/gogo/protobuf/issues/519 for details on issues this causes for proto.Clone
	apiutils.UTC(&s.Spec.Rotation.Started)
	apiutils.UTC(&s.Spec.Rotation.LastRotated)
	return &s, nil
}

// MarshalServer marshals the Server resource to JSON.
func MarshalServer(server types.Server, opts ...MarshalOption) ([]byte, error) {
	if err := server.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch server := server.(type) {
	case *types.ServerV2:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *server
			copy.SetResourceID(0)
			server = &copy
		}
		return utils.FastMarshal(server)
	default:
		return nil, trace.BadParameter("unrecognized server version %T", server)
	}
}

// UnmarshalServers unmarshals a list of Server resources.
func UnmarshalServers(bytes []byte) ([]types.Server, error) {
	var servers []types.ServerV2

	err := utils.FastUnmarshal(bytes, &servers)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make([]types.Server, len(servers))
	for i, v := range servers {
		out[i] = types.Server(&v)
	}
	return out, nil
}

// MarshalServers marshals a list of Server resources.
func MarshalServers(s []types.Server) ([]byte, error) {
	bytes, err := utils.FastMarshal(s)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return bytes, nil
}

// NodeHasMissedKeepAlives checks if node has missed its keep alive
func NodeHasMissedKeepAlives(s types.Server) bool {
	serverExpiry := s.Expiry()
	return serverExpiry.Before(time.Now().Add(apidefaults.ServerAnnounceTTL - (apidefaults.ServerKeepAliveTTL() * 2)))
}
