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

package auth

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/durationpb"

	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/types"
)

// httpfallback.go holds endpoints that have been converted to gRPC
// but still need http fallback logic in the old client.

// GenerateHostCert takes the public key in the Open SSH “authorized_keys“
// plain text format, signs it using Host Certificate Authority private key and
// returns the resulting certificate.
// TODO(noah): DELETE IN 16.0.0
func (c *Client) GenerateHostCert(
	ctx context.Context,
	key []byte,
	hostID, nodeName string,
	principals []string,
	clusterName string,
	role types.SystemRole,
	ttl time.Duration,
) ([]byte, error) {
	res, err := c.APIClient.TrustClient().GenerateHostCert(ctx, &trustpb.GenerateHostCertRequest{
		Key:         key,
		HostId:      hostID,
		NodeName:    nodeName,
		Principals:  principals,
		ClusterName: clusterName,
		Role:        string(role),
		Ttl:         durationpb.New(ttl),
	})
	if err == nil {
		return res.SshCertificate, nil
	} else if !trace.IsNotImplemented(err) {
		return nil, trace.Wrap(err)
	}

	// Fall back to HTTP implementation.
	return c.generateHostCertHTTP(
		ctx, key, hostID, nodeName, principals, clusterName, role, ttl,
	)
}

// TODO(noah): DELETE IN 16.0.0
func (c *Client) generateHostCertHTTP(
	ctx context.Context,
	key []byte,
	hostID, nodeName string,
	principals []string,
	clusterName string,
	role types.SystemRole,
	ttl time.Duration,
) ([]byte, error) {
	out, err := c.PostJSON(ctx, c.Endpoint("ca", "host", "certs"),
		generateHostCertReq{
			Key:         key,
			HostID:      hostID,
			NodeName:    nodeName,
			Principals:  principals,
			ClusterName: clusterName,
			Roles:       types.SystemRoles{role},
			TTL:         ttl,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var cert string
	if err := json.Unmarshal(out.Bytes(), &cert); err != nil {
		return nil, err
	}
	return []byte(cert), nil
}
