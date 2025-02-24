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

package auth

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/tracing/smithyoteltracing"
	"github.com/digitorus/pkcs7"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"go.opentelemetry.io/otel"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

type ec2Client interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

type ec2ClientKey struct{}

func ec2ClientFromContext(ctx context.Context) (ec2Client, bool) {
	ec2Client, ok := ctx.Value(ec2ClientKey{}).(ec2Client)
	return ec2Client, ok
}

// ec2ClientFromConfig returns a new ec2 client from the given aws config, or
// may load the client from the passed context if one has been set (for tests).
func ec2ClientFromConfig(ctx context.Context, cfg aws.Config) ec2Client {
	ec2Client, ok := ec2ClientFromContext(ctx)
	if ok {
		return ec2Client
	}
	return ec2.NewFromConfig(cfg, func(o *ec2.Options) {
		o.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
	})
}

// checkEC2AllowRules checks that the iid matches at least one of the allow
// rules of the given token.
func checkEC2AllowRules(ctx context.Context, iid *imds.InstanceIdentityDocument, provisionToken types.ProvisionToken) error {
	allowRules := provisionToken.GetAllowRules()
	for _, rule := range allowRules {
		// if this rule specifies an AWS account, the IID must match
		if len(rule.AWSAccount) > 0 {
			if rule.AWSAccount != iid.AccountID {
				continue
			}
		}
		// if this rule specifies any AWS regions, the IID must match one of them
		if len(rule.AWSRegions) > 0 {
			if !slices.Contains(rule.AWSRegions, iid.Region) {
				continue
			}
		}
		// iid matches this allow rule. Check if it is running.
		return trace.Wrap(checkInstanceRunning(ctx, iid.InstanceID, iid.Region, rule.AWSRole))
	}
	return trace.AccessDenied("instance %v did not match any allow rules in token %v", iid.InstanceID, provisionToken.GetName())
}

func checkInstanceRunning(ctx context.Context, instanceID, region, IAMRole string) error {
	awsClientConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	awsClientConfig.Region = region

	// assume the configured IAM role if necessary
	if IAMRole != "" {
		stsClient := stsutils.NewFromConfig(awsClientConfig, func(o *sts.Options) {
			o.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
		})
		creds := stscreds.NewAssumeRoleProvider(stsClient, IAMRole)
		awsClientConfig.Credentials = aws.NewCredentialsCache(creds)
	}

	ec2Client := ec2ClientFromConfig(ctx, awsClientConfig)

	output, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if len(output.Reservations) == 0 || len(output.Reservations[0].Instances) == 0 {
		return trace.AccessDenied("failed to get instance state")
	}
	instance := output.Reservations[0].Instances[0]
	if instance.InstanceId == nil || *instance.InstanceId != instanceID {
		return trace.AccessDenied("failed to get instance state")
	}
	if instance.State == nil || instance.State.Name != ec2types.InstanceStateNameRunning {
		return trace.AccessDenied("instance is not running")
	}
	return nil
}

// parseAndVerifyIID parses the given Instance Identity Document and checks that
// the signature is valid.
func parseAndVerifyIID(iidBytes []byte) (*imds.InstanceIdentityDocument, error) {
	sigPEM := fmt.Sprintf("-----BEGIN PKCS7-----\n%s\n-----END PKCS7-----", string(iidBytes))
	sigBER, _ := pem.Decode([]byte(sigPEM))
	if sigBER == nil {
		return nil, trace.AccessDenied("unable to decode Instance Identity Document")
	}

	p7, err := pkcs7.Parse(sigBER.Bytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var iid imds.InstanceIdentityDocument
	if err := json.Unmarshal(p7.Content, &iid); err != nil {
		return nil, trace.Wrap(err)
	}

	rawCert, ok := awsRSA2048CertBytes[iid.Region]
	if !ok {
		return nil, trace.AccessDenied("unsupported EC2 region: %q", iid.Region)
	}
	certPEM, _ := pem.Decode(rawCert)
	cert, err := x509.ParseCertificate(certPEM.Bytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p7.Certificates = []*x509.Certificate{cert}
	if err = p7.Verify(); err != nil {
		return nil, trace.AccessDenied("invalid signature")
	}

	return &iid, nil
}

func checkPendingTime(iid *imds.InstanceIdentityDocument, provisionToken types.ProvisionToken, clock clockwork.Clock) error {
	timeSinceInstanceStart := clock.Since(iid.PendingTime)
	// Sanity check IID is not from the future. Allow 1 minute of clock drift.
	if timeSinceInstanceStart < -1*time.Minute {
		return trace.AccessDenied("Instance Identity Document PendingTime appears to be in the future")
	}
	ttl := time.Duration(provisionToken.GetAWSIIDTTL())
	if timeSinceInstanceStart > ttl {
		return trace.AccessDenied("Instance Identity Document with PendingTime %v is older than configured TTL of %v", iid.PendingTime, ttl)
	}
	return nil
}

func nodeExists(ctx context.Context, presence services.Presence, hostID string) (bool, error) {
	_, err := presence.GetNode(ctx, defaults.Namespace, hostID)
	switch {
	case trace.IsNotFound(err):
		return false, nil
	case err != nil:
		return false, trace.Wrap(err)
	default:
		return true, nil
	}
}

func proxyExists(ctx context.Context, presence services.Presence, hostID string) (bool, error) {
	proxies, err := presence.GetProxies()
	if err != nil {
		return false, trace.Wrap(err)
	}
	for _, proxy := range proxies {
		if proxy.GetName() == hostID {
			return true, nil
		}
	}
	return false, nil
}

func kubeExists(ctx context.Context, presence services.Presence, hostID string) (bool, error) {
	kubes, err := presence.GetKubernetesServers(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	for _, kube := range kubes {
		if kube.GetHostID() == hostID {
			return true, nil
		}
	}
	return false, nil
}

func appExists(ctx context.Context, presence services.Presence, hostID string) (bool, error) {
	apps, err := presence.GetApplicationServers(ctx, defaults.Namespace)
	if err != nil {
		return false, trace.Wrap(err)
	}

	for _, app := range apps {
		if app.GetName() == hostID {
			return true, nil
		}
	}

	return false, nil
}

func dbExists(ctx context.Context, presence services.Presence, hostID string) (bool, error) {
	dbs, err := presence.GetDatabaseServers(ctx, defaults.Namespace)
	if err != nil {
		return false, trace.Wrap(err)
	}

	for _, db := range dbs {
		if db.GetName() == hostID {
			return true, nil
		}
	}

	return false, nil
}

func oktaExists(ctx context.Context, presence services.Presence, hostID string) (bool, error) {
	apps, err := presence.GetApplicationServers(ctx, defaults.Namespace)
	if err != nil {
		return false, trace.Wrap(err)
	}
	for _, app := range apps {
		if app.GetName() == hostID && app.Origin() == types.OriginOkta {
			return true, nil
		}
	}

	return false, nil
}

func desktopServiceExists(ctx context.Context, presence services.Presence, hostID string) (bool, error) {
	svcs, err := presence.GetWindowsDesktopServices(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}

	for _, wds := range svcs {
		if wds.GetName() == hostID {
			return true, nil
		}
	}
	return false, nil
}

// tryToDetectIdentityReuse performs a best-effort check to see if the specified role+id combination
// is already in use by an instance. This will only detect re-use in the case where a recent heartbeat
// clearly shows the combination in use since teleport maintains no long-term per-instance state.
func (a *Server) tryToDetectIdentityReuse(ctx context.Context, req *types.RegisterUsingTokenRequest, iid *imds.InstanceIdentityDocument) error {
	requestedHostID := req.HostID
	expectedHostID := utils.NodeIDFromIID(iid)
	if requestedHostID != expectedHostID {
		return trace.AccessDenied("invalid host ID %q, expected %q", requestedHostID, expectedHostID)
	}

	var instanceExists bool
	var err error

	switch req.Role {
	case types.RoleNode:
		instanceExists, err = nodeExists(ctx, a, req.HostID)
	case types.RoleProxy:
		instanceExists, err = proxyExists(ctx, a, req.HostID)
	case types.RoleKube:
		instanceExists, err = kubeExists(ctx, a, req.HostID)
	case types.RoleApp:
		instanceExists, err = appExists(ctx, a, req.HostID)
	case types.RoleDatabase:
		instanceExists, err = dbExists(ctx, a, req.HostID)
	case types.RoleWindowsDesktop:
		instanceExists, err = desktopServiceExists(ctx, a, req.HostID)
	case types.RoleOkta:
		instanceExists, err = oktaExists(ctx, a, req.HostID)
	case types.RoleInstance:
		// no appropriate check exists for the Instance role
		instanceExists = false
	case types.RoleDiscovery:
		// no appropriate check exists for the Discovery role
		instanceExists = false
	case types.RoleMDM:
		// no appropriate check exists for the MDM role
		instanceExists = false
	default:
		return trace.BadParameter("unsupported role: %q", req.Role)
	}

	if err != nil {
		return trace.Wrap(err)
	}
	if instanceExists {
		const msg = "Server is attempting to join the cluster with a Simplified Node Joining request, but" +
			" a server with this ID is already present in the cluster"
		a.logger.WarnContext(ctx, msg,
			"host_id", req.HostID,
			"role", req.Role,
		)
		return trace.AccessDenied("server with host ID %q and role %q already exists", req.HostID, req.Role)
	}
	return nil
}

// checkEC2JoinRequest checks register requests which use EC2 Simplified Node
// Joining. This method checks that:
//  1. The given Instance Identity Document has a valid signature (signed by AWS).
//  2. There is no obvious signs that a node already joined the cluster from this EC2 instance (to
//     reduce the risk of re-use of a stolen Instance Identity Document).
//  3. The signed instance attributes match one of the allow rules for the
//     corresponding token.
//
// If the request does not include an Instance Identity Document, and the
// token does not include any allow rules, this method returns nil and the
// normal token checking logic resumes.
func (a *Server) checkEC2JoinRequest(ctx context.Context, req *types.RegisterUsingTokenRequest) error {
	tokenName := req.Token
	provisionToken, err := a.GetToken(ctx, tokenName)
	if err != nil {
		return trace.Wrap(err)
	}

	a.logger.DebugContext(ctx, "Received Simplified Node Joining request", "host_id", req.HostID)

	if len(req.EC2IdentityDocument) == 0 {
		return trace.AccessDenied("this token is only valid for the EC2 join " +
			"method but the node has not included an EC2 Instance Identity " +
			"Document, make sure your node is configured to use the EC2 join method")
	}

	iid, err := parseAndVerifyIID(req.EC2IdentityDocument)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := checkPendingTime(iid, provisionToken, a.clock); err != nil {
		return trace.Wrap(err)
	}

	if err := a.tryToDetectIdentityReuse(ctx, req, iid); err != nil {
		return trace.Wrap(err)
	}

	if err := checkEC2AllowRules(ctx, iid, provisionToken); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
