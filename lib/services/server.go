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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/google/go-cmp/cmp"
)

const (
	// Equal means two objects are equal
	Equal = iota
	// OnlyTimestampsDifferent is true when only timestamps are different
	OnlyTimestampsDifferent = iota
	// Different means that some fields are different
	Different = iota
)

// Compare compares two provided resources.
func Compare(a, b Resource) int {
	if serverA, ok := a.(Server); ok {
		if serverB, ok := b.(Server); ok {
			return CompareServers(serverA, serverB)
		}
	}
	if dbA, ok := a.(types.DatabaseServer); ok {
		if dbB, ok := b.(types.DatabaseServer); ok {
			return CompareDatabaseServers(dbA, dbB)
		}
	}
	return Different
}

// CompareServers returns difference between two server
// objects, Equal (0) if identical, OnlyTimestampsDifferent(1) if only timestamps differ, Different(2) otherwise
func CompareServers(a, b Server) int {
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
	if a.GetPublicAddr() != b.GetPublicAddr() {
		return Different
	}
	r := a.GetRotation()
	if !r.Matches(b.GetRotation()) {
		return Different
	}
	if a.GetUseTunnel() != b.GetUseTunnel() {
		return Different
	}
	if !utils.StringMapsEqual(a.GetLabels(), b.GetLabels()) {
		return Different
	}
	if !CmdLabelMapsEqual(a.GetCmdLabels(), b.GetCmdLabels()) {
		return Different
	}
	if !a.Expiry().Equal(b.Expiry()) {
		return OnlyTimestampsDifferent
	}
	if a.GetTeleportVersion() != b.GetTeleportVersion() {
		return Different
	}

	// If this server is proxying applications, compare them to make sure they match.
	if a.GetKind() == KindAppServer {
		return CompareApps(a.GetApps(), b.GetApps())
	}

	if !cmp.Equal(a.GetKubernetesClusters(), b.GetKubernetesClusters()) {
		return Different
	}

	return Equal
}

// CompareApps compares two slices of apps and returns if they are equal or
// different.
func CompareApps(a []*App, b []*App) int {
	if len(a) != len(b) {
		return Different
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			return Different
		}
		if a[i].URI != b[i].URI {
			return Different
		}
		if a[i].PublicAddr != b[i].PublicAddr {
			return Different
		}
		if !utils.StringMapsEqual(a[i].StaticLabels, b[i].StaticLabels) {
			return Different
		}
		if !CmdLabelMapsEqual(
			V2ToLabels(a[i].DynamicLabels),
			V2ToLabels(b[i].DynamicLabels)) {
			return Different
		}
		if (a[i].Rewrite == nil && b[i].Rewrite != nil) ||
			(a[i].Rewrite != nil && b[i].Rewrite == nil) {
			return Different
		}
		if a[i].Rewrite != nil && b[i].Rewrite != nil {
			if !utils.StringSlicesEqual(a[i].Rewrite.Redirect, b[i].Rewrite.Redirect) {
				return Different
			}
		}
	}
	return Equal
}

// CompareDatabaseServers returns whether the two provided database servers
// are equal or different.
func CompareDatabaseServers(a, b types.DatabaseServer) int {
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
	if !utils.StringMapsEqual(a.GetStaticLabels(), b.GetStaticLabels()) {
		return Different
	}
	if !CmdLabelMapsEqual(a.GetDynamicLabels(), b.GetDynamicLabels()) {
		return Different
	}
	if !a.Expiry().Equal(b.Expiry()) {
		return OnlyTimestampsDifferent
	}
	if a.GetProtocol() != b.GetProtocol() {
		return Different
	}
	if a.GetURI() != b.GetURI() {
		return Different
	}
	return Equal
}

// CmdLabelMapsEqual compares two maps with command labels,
// returns true if label sets are equal
func CmdLabelMapsEqual(a, b map[string]CommandLabel) bool {
	if len(a) != len(b) {
		return false
	}
	for key, val := range a {
		val2, ok := b[key]
		if !ok {
			return false
		}
		if !val.Equals(val2) {
			return false
		}
	}
	return true
}

// CommandLabels is a set of command labels
type CommandLabels map[string]CommandLabel

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
type SortedServers []Server

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
type SortedReverseTunnels []ReverseTunnel

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
func GuessProxyHostAndVersion(proxies []Server) (string, string, error) {
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
