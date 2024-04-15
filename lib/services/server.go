/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ec2v1 "github.com/aws/aws-sdk-go/service/ec2"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	libaws "github.com/gravitational/teleport/lib/cloud/aws"
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

	// defaultSSHPort is the default port for the OpenSSH Service.
	defaultSSHPort = "22"
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
	if cfg.Revision != "" {
		s.SetRevision(cfg.Revision)
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
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch server := server.(type) {
	case *types.ServerV2:
		if err := server.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoResourceID(cfg.PreserveResourceID, server))
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
	for i := range servers {
		out[i] = types.Server(&servers[i])
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

// NewAWSNodeFromEC2Instance creates a Node resource from an EC2 Instance.
// It has a pre-populated spec which contains info that is not available in the ec2.Instance object.
func NewAWSNodeFromEC2Instance(instance ec2types.Instance, awsCloudMetadata *types.AWSInfo) (types.Server, error) {
	labels := libaws.TagsToLabels(instance.Tags)
	if labels == nil {
		labels = make(map[string]string)
	}
	libaws.AddMetadataLabels(labels, awsCloudMetadata.AccountID, awsCloudMetadata.Region)

	instanceID := aws.ToString(instance.InstanceId)
	labels[types.AWSInstanceIDLabel] = instanceID
	labels[types.AWSAccountIDLabel] = awsCloudMetadata.AccountID

	awsCloudMetadata.InstanceID = instanceID
	awsCloudMetadata.VPCID = aws.ToString(instance.VpcId)
	awsCloudMetadata.SubnetID = aws.ToString(instance.SubnetId)

	if aws.ToString(instance.PrivateIpAddress) == "" {
		return nil, trace.BadParameter("private ip address is required from ec2 instance")
	}
	// Address requires the Port.
	// We use the default port for the OpenSSH daemon.
	addr := net.JoinHostPort(aws.ToString(instance.PrivateIpAddress), defaultSSHPort)

	server, err := types.NewEICENode(
		types.ServerSpecV2{
			Hostname: aws.ToString(instance.PrivateDnsName),
			Addr:     addr,
			CloudMetadata: &types.CloudMetadata{
				AWS: awsCloudMetadata,
			},
		},
		labels,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return server, nil
}

// NewAWSNodeFromEC2v1Instance creates a Node resource from an EC2 Instance.
// It has a pre-populated spec which contains info that is not available in the ec2.Instance object.
// Uses AWS SDK Go V1
func NewAWSNodeFromEC2v1Instance(instance ec2v1.Instance, awsCloudMetadata *types.AWSInfo) (types.Server, error) {
	server, err := NewAWSNodeFromEC2Instance(ec2InstanceV1ToV2(instance), awsCloudMetadata)
	return server, trace.Wrap(err)
}

func ec2InstanceV1ToV2(instance ec2v1.Instance) ec2types.Instance {
	tags := make([]ec2types.Tag, 0, len(instance.Tags))
	for _, tag := range instance.Tags {
		tags = append(tags, ec2types.Tag{
			Key:   tag.Key,
			Value: tag.Value,
		})
	}

	return ec2types.Instance{
		InstanceId:       instance.InstanceId,
		VpcId:            instance.VpcId,
		SubnetId:         instance.SubnetId,
		PrivateIpAddress: instance.PrivateIpAddress,
		PrivateDnsName:   instance.PrivateDnsName,
		Tags:             tags,
	}
}
