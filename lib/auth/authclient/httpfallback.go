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

package authclient

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/durationpb"

	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// httpfallback.go holds endpoints that have been converted to gRPC
// but still need http fallback logic in the old client.

// GenerateHostCert takes the public key in the OpenSSH “authorized_keys“
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
	if err != nil {
		switch {
		case trace.IsNotImplemented(err):
			// Fall back to HTTP implementation.
			return c.GenerateHostCertHTTP(
				ctx, key, hostID, nodeName, principals, clusterName, role, ttl,
			)
		default:
			return nil, trace.Wrap(err)
		}
	}
	return res.SshCertificate, nil
}

type generateHostCertReq struct {
	Key         []byte            `json:"key"`
	HostID      string            `json:"hostname"`
	NodeName    string            `json:"node_name"`
	Principals  []string          `json:"principals"`
	ClusterName string            `json:"auth_domain"`
	Roles       types.SystemRoles `json:"roles"`
	TTL         time.Duration     `json:"ttl"`
}

// TODO(noah): DELETE IN 16.0.0
func (c *Client) GenerateHostCertHTTP(
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

// TODO(Joerger): DELETE IN 16.0.0
func (c *Client) RotateCertAuthority(ctx context.Context, req types.RotateRequest) error {
	err := c.APIClient.RotateCertAuthority(ctx, req)
	if trace.IsNotImplemented(err) {
		// Fall back to HTTP implementation.
		_, err := c.PostJSON(ctx, c.Endpoint("authorities", string(req.Type), "rotate"), req)
		return trace.Wrap(err)
	}

	return trace.Wrap(err)
}

type rotateExternalCertAuthorityRawReq struct {
	CA json.RawMessage `json:"ca"`
}

// TODO(Joerger): DELETE IN 16.0.0
func (c *Client) RotateExternalCertAuthority(ctx context.Context, ca types.CertAuthority) error {
	err := c.APIClient.RotateExternalCertAuthority(ctx, ca)
	if trace.IsNotImplemented(err) {
		// Fall back to HTTP implementation.
		data, err := services.MarshalCertAuthority(ca)
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = c.PostJSON(ctx, c.Endpoint("authorities", string(ca.GetType()), "rotate", "external"),
			&rotateExternalCertAuthorityRawReq{CA: data})
		return trace.Wrap(err)
	}

	return trace.Wrap(err)
}
