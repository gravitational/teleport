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

package awsoidc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	signer "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
)

var (
	// validEC2Ports contains the available EC2 ports to use with EC2 Instance Connect Endpoint.
	validEC2Ports = []string{"22", "3389"}

	// filterEC2InstanceConnectEndpointStateKey is the filter key for filtering EC2 Instance Connection Endpoint by their state.
	filterEC2InstanceConnectEndpointStateKey = "state"
	// filterEC2InstanceConnectEndpointVPCIDKey is the filter key for filtering EC2 Instance Connection Endpoint by their VPC ID.
	filterEC2InstanceConnectEndpointVPCIDKey = "vpc-id"
)

const (
	// hashForGetRequests is the SHA-256 for an empty element.
	// PresignHTTP requires the hash of the body, but this is a GET request and has no body.
	// https://docs.aws.amazon.com/AmazonS3/latest/API/sig-v4-header-based-auth.html
	hashForGetRequests = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

// OpenTunnelEC2Request contains the required fields to open a tunnel to an EC2 instance.
// This will create a TCP socket that forwards incoming connections to the EC2's private IP address.
type OpenTunnelEC2Request struct {
	// Region is the AWS Region.
	Region string

	// VPCID is the VPC where the EC2 Instance is located.
	// Used to look for the EC2 Instance Connect Endpoint.
	// Each VPC ID can only have one EC2 Instance Connect Endpoint.
	VPCID string

	// EC2Address is the address to connect to in the EC2 Instance.
	// Eg, ip-172-31-32-234.eu-west-2.compute.internal:22
	EC2Address string

	// EC2InstanceID is the EC2 Instance ID.
	EC2InstanceID string

	// ec2OpenSSHPort is the port to connect to in the EC2 Instance.
	// This value is parsed from EC2Address.
	// Possible values: 22, 3389.
	ec2OpenSSHPort string

	// ec2PrivateHostname is the private hostname of the EC2 Instance.
	// This value is parsed from EC2Address.
	ec2PrivateHostname string

	// websocketCustomCA is a x509.Certificate to trust when trying to connect to the websocket.
	// For testing purposes only.
	websocketCustomCA *x509.Certificate
}

// CheckAndSetDefaults checks if the required fields are present.
func (r *OpenTunnelEC2Request) CheckAndSetDefaults() error {
	var err error

	if r.Region == "" {
		return trace.BadParameter("region is required")
	}

	if r.VPCID == "" {
		return trace.BadParameter("vpcid is required")
	}

	if r.EC2InstanceID == "" {
		return trace.BadParameter("ec2 instance id is required")
	}

	if r.EC2Address == "" {
		return trace.BadParameter("ec2 address required")
	}

	r.ec2PrivateHostname, r.ec2OpenSSHPort, err = net.SplitHostPort(r.EC2Address)
	if err != nil {
		return trace.BadParameter("ec2 address is invalid: %v", err)
	}

	if !slices.Contains(validEC2Ports, r.ec2OpenSSHPort) {
		return trace.BadParameter("invalid ec2 address port %s, possible values: %v", r.ec2OpenSSHPort, validEC2Ports)
	}

	return nil
}

// OpenTunnelEC2Response contains the response for creating a Tunnel to an EC2 Instance.
// It returns the listening address and the SSH Private Key (PEM encoded).
type OpenTunnelEC2Response struct {
	// Tunnel is a net.Conn that is connected to the EC2 instance.
	// The SSH Client must use this connection to connect to it.
	Tunnel net.Conn
}

// OpenTunnelEC2Client describes the required methods to Open a Tunnel to an EC2 Instance using
// EC2 Instance Connect Endpoint.
type OpenTunnelEC2Client interface {
	// DescribeInstanceConnectEndpoints describes the specified EC2 Instance Connect Endpoints or all EC2 Instance
	// Connect Endpoints.
	DescribeInstanceConnectEndpoints(ctx context.Context, params *ec2.DescribeInstanceConnectEndpointsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceConnectEndpointsOutput, error)

	// Retrieve returns nil if it successfully retrieved the value.
	// Error is returned if the value were not obtainable, or empty.
	Retrieve(ctx context.Context) (aws.Credentials, error)
}

type defaultOpenTunnelEC2Client struct {
	*ec2.Client
	awsCredentialsProvider aws.CredentialsProvider
	ec2icClient            *ec2instanceconnect.Client
}

// Retrieve returns nil if it successfully retrieved the value.
// Error is returned if the value were not obtainable, or empty.
func (d defaultOpenTunnelEC2Client) Retrieve(ctx context.Context) (aws.Credentials, error) {
	return d.awsCredentialsProvider.Retrieve(ctx)
}

// NewOpenTunnelEC2Client creates a OpenTunnelEC2Client using AWSClientRequest.
func NewOpenTunnelEC2Client(ctx context.Context, clientReq *AWSClientRequest) (OpenTunnelEC2Client, error) {
	ec2Client, err := newEC2Client(ctx, clientReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ec2instanceconnectClient, err := newEC2InstanceConnectClient(ctx, clientReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	awsCredProvider, err := NewAWSCredentialsProvider(ctx, clientReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultOpenTunnelEC2Client{
		Client:                 ec2Client,
		awsCredentialsProvider: awsCredProvider,
		ec2icClient:            ec2instanceconnectClient,
	}, nil
}

// OpenTunnelEC2 creates a tunnel to an ec2 instance using its private IP.
// Ref:
// - https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/connect-using-eice.html
// - https://github.com/aws/aws-cli/blob/f6c820e89d8b566ab54ab9d863754ec4b713fd6a/awscli/customizations/ec2instanceconnect/opentunnel.py
//
// High level archictecture:
// - does a lookup for an EC2 Instance Connect Endpoint available (create-complete state) for the target VPC
// - connects to it (websockets) and returns the connection
// - the connection can be used to access the EC2 instance directly (tcp stream)
func OpenTunnelEC2(ctx context.Context, clt OpenTunnelEC2Client, req OpenTunnelEC2Request) (*OpenTunnelEC2Response, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	eice, err := fetchEC2InstanceConnectEndpoint(ctx, clt, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ec2Conn, err := dialEC2InstanceUsingEICE(ctx, dialEC2InstanceUsingEICERequest{
		credsProvider:    clt,
		awsRegion:        req.Region,
		endpointId:       aws.ToString(eice.InstanceConnectEndpointId),
		endpointHost:     aws.ToString(eice.DnsName),
		privateIPAddress: req.ec2PrivateHostname,
		remotePort:       req.ec2OpenSSHPort,
		customCA:         req.websocketCustomCA,
		subnetID:         aws.ToString(eice.SubnetId),
		ec2InstanceID:    req.EC2InstanceID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &OpenTunnelEC2Response{
		Tunnel: ec2Conn,
	}, nil
}

// fetchEC2InstanceConnectEndpoint returns an EC2InstanceConnectEndpoint for the given VPC and whose state is ready to use ("create-complete").
func fetchEC2InstanceConnectEndpoint(ctx context.Context, clt OpenTunnelEC2Client, req OpenTunnelEC2Request) (*ec2types.Ec2InstanceConnectEndpoint, error) {
	describe, err := clt.DescribeInstanceConnectEndpoints(ctx, &ec2.DescribeInstanceConnectEndpointsInput{
		Filters: []ec2types.Filter{
			{
				Name:   &filterEC2InstanceConnectEndpointVPCIDKey,
				Values: []string{req.VPCID},
			},
			{
				Name:   &filterEC2InstanceConnectEndpointStateKey,
				Values: []string{string(ec2types.Ec2InstanceConnectEndpointStateCreateComplete)},
			},
		},
	})
	if err != nil {
		return nil, trace.BadParameter("failed to list EC2 Instance Connect Endpoint for VPC %q (region=%q): %v", req.VPCID, req.Region, err)
	}

	if len(describe.InstanceConnectEndpoints) == 0 {
		return nil, trace.BadParameter("no EC2 Instance Connect Endpoint for VPC %q (region=%q), please create one", req.VPCID, req.Region)
	}

	return &describe.InstanceConnectEndpoints[0], nil
}

// dialEC2InstanceUsingEICERequest is a request to dial into an EC2 Instance Connect Endpoint.
type dialEC2InstanceUsingEICERequest struct {
	credsProvider    aws.CredentialsProvider
	awsRegion        string
	endpointId       string
	customCA         *x509.Certificate
	endpointHost     string
	privateIPAddress string
	remotePort       string
	subnetID         string
	ec2InstanceID    string
}

// dialEC2InstanceUsingEICE dials into an EC2 instance port using an EC2 Instance Connect Endpoint.
// Returns a net.Conn that transparently proxies the connection to the EC2 instance.
func dialEC2InstanceUsingEICE(ctx context.Context, req dialEC2InstanceUsingEICERequest) (net.Conn, error) {
	// There's no official documentation on how to connect to the EC2 Instance Connect Endpoint.
	// So, we had to rely on the awscli implementation, which you can find here:
	// https://github.com/aws/aws-cli/blob/f6c820e89d8b566ab54ab9d863754ec4b713fd6a/awscli/customizations/ec2instanceconnect/opentunnel.py
	//
	// The lack of documentation means this implementation is a risk, however, by following awscli implementation we are confident that
	// it will work for the foreseable future (aws will *probably* not break old awscli versions).
	q := url.Values{}
	q.Set("instanceConnectEndpointId", req.endpointId)
	q.Set("maxTunnelDuration", "3600") // 1 hour (max allowed)
	q.Set("privateIpAddress", req.privateIPAddress)
	q.Set("remotePort", req.remotePort)

	openTunnelURL := url.URL{
		Scheme:   "wss",
		Host:     req.endpointHost,
		Path:     "openTunnel",
		RawQuery: q.Encode(),
	}

	r, err := http.NewRequest(http.MethodGet, openTunnelURL.String(), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	creds, err := req.credsProvider.Retrieve(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := signer.NewSigner()
	signed, _, err := s.PresignHTTP(ctx, creds, r, hashForGetRequests, "ec2-instance-connect", req.awsRegion, time.Now())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	websocketDialer := websocket.DefaultDialer

	// For testing purposes only. Adds the httpTestServer CA
	if req.customCA != nil {
		if !strings.HasPrefix(req.endpointHost, "127.0.0.1:") {
			return nil, trace.BadParameter("custom CA can only be used for testing and the websocket address must be localhost: %v", req.endpointHost)
		}
		websocketDialer.TLSClientConfig = &tls.Config{
			RootCAs: x509.NewCertPool(),
		}
		websocketDialer.TLSClientConfig.RootCAs.AddCert(req.customCA)
	}

	conn, resp, err := websocketDialer.DialContext(ctx, signed, http.Header{})
	if err != nil {
		if errors.Is(err, websocket.ErrBadHandshake) {
			defer resp.Body.Close()
			// resp is non-nil in this case, so we can read the http response body
			httpResponseBody, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return nil, trace.BadParameter("websocket bad handshake: %s", string(httpResponseBody))
		}

		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	return &eicedConn{
		Conn:          conn,
		r:             websocket.JoinMessages(conn, ""),
		eiceID:        req.endpointId,
		ec2InstanceID: req.ec2InstanceID,
		subnetID:      req.subnetID,
	}, nil
}

// eicedConn is a net.Conn implementation that reads from reader r and writes into a websocket.Conn
type eicedConn struct {
	*websocket.Conn
	r io.Reader

	ec2InstanceID string
	eiceID        string
	subnetID      string
}

// Reads from the reader into b and returns the number of read bytes.
func (i *eicedConn) Read(b []byte) (n int, err error) {
	n, err = i.r.Read(b)
	if err != nil {
		return 0, i.handleIOError(err)
	}

	return n, nil
}

// Write writes into the websocket connection the contents of b.
// Returns how many bytes were written.
func (i *eicedConn) Write(b []byte) (int, error) {
	err := i.Conn.WriteMessage(websocket.BinaryMessage, b)
	if err != nil {
		return 0, i.handleIOError(err)
	}
	return len(b), trace.Wrap(err)
}

func (i *eicedConn) handleIOError(err error) error {
	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) {
		return trace.ConnectionProblem(err,
			"Could not connect to %s via EC2 Instance Connect Endpoint %s. "+
				"Please ensure the instance's SecurityGroups allow inbound TCP traffic on port 22 from %s",
			i.ec2InstanceID,
			i.eiceID,
			i.subnetID,
		)
	}
	return trace.Wrap(err)
}

// SetDeadline sets the websocket read and write deadline.
func (i *eicedConn) SetDeadline(t time.Time) error {
	i.SetReadDeadline(t)
	i.SetWriteDeadline(t)
	return nil
}
