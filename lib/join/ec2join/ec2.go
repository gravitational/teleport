// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package ec2join

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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

type EC2Client interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

// ec2ClientFromConfig returns a new ec2 client from the given aws config, or
// may load the client from the passed context if one has been set (for tests).
func ec2ClientFromConfig(cfg aws.Config) EC2Client {
	return ec2.NewFromConfig(cfg, func(o *ec2.Options) {
		o.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
	})
}

// checkEC2AllowRules checks that the iid matches at least one of the allow
// rules of the given token.
func checkEC2AllowRules(ctx context.Context, params *CheckEC2RequestParams, iid *imds.InstanceIdentityDocument) error {
	allowRules := params.ProvisionToken.GetAllowRules()
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
		return trace.Wrap(checkInstanceRunning(ctx, params, iid.InstanceID, iid.Region, rule.AWSRole))
	}
	return trace.AccessDenied("instance %v did not match any allow rules in token %v", iid.InstanceID, params.ProvisionToken.GetName())
}

func checkInstanceRunning(ctx context.Context, params *CheckEC2RequestParams, instanceID, region, IAMRole string) error {
	ec2Client := params.EC2Client
	if ec2Client == nil {
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
		ec2Client = ec2ClientFromConfig(awsClientConfig)
	}

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

func proxyExists(_ context.Context, presence services.Presence, hostID string) (bool, error) {
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

func dbExists(ctx context.Context, presence services.Presence, hostID string) (bool, error) {
	dbs, err := presence.ListResources(ctx, proto.ListResourcesRequest{
		ResourceType:        types.KindDatabaseService,
		PredicateExpression: fmt.Sprintf("resource.metadata.name == %q", hostID),
		Limit:               1,
	})
	if err != nil {
		return false, trace.Wrap(err)
	}

	return len(dbs.Resources) > 0, nil
}

func appExists(ctx context.Context, presence services.Presence, hostID string) (bool, error) {
	apps, err := presence.GetApplicationServers(ctx, defaults.Namespace)
	if err != nil {
		return false, trace.Wrap(err)
	}

	for _, app := range apps {
		if app.GetHostID() == hostID && app.Origin() != types.OriginOkta {
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
		if app.GetHostID() == hostID && app.Origin() == types.OriginOkta {
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

func resourceExists(ctx context.Context, presence services.Presence, role types.SystemRole, hostID string) (bool, error) {
	switch role {
	case types.RoleNode:
		return nodeExists(ctx, presence, hostID)
	case types.RoleProxy:
		return proxyExists(ctx, presence, hostID)
	case types.RoleKube:
		return kubeExists(ctx, presence, hostID)
	case types.RoleApp:
		return appExists(ctx, presence, hostID)
	case types.RoleDatabase:
		return dbExists(ctx, presence, hostID)
	case types.RoleWindowsDesktop:
		return desktopServiceExists(ctx, presence, hostID)
	case types.RoleOkta:
		return oktaExists(ctx, presence, hostID)
	case types.RoleDiscovery, types.RoleMDM:
		// No appropriate check exists for these roles.
		return false, nil
	default:
		return false, trace.BadParameter("role %s is unsupported for the EC2 join method", role)
	}
}

func roleHasPresenceCheck(role types.SystemRole) bool {
	switch role {
	case types.RoleNode, types.RoleProxy, types.RoleKube, types.RoleApp,
		types.RoleDatabase, types.RoleWindowsDesktop, types.RoleOkta:
		return true
	}
	return false
}

// tryToDetectIdentityReuse performs a best-effort check to see if the specified role+id combination
// is already in use by an instance. This will only detect re-use in the case where a recent heartbeat
// clearly shows the combination in use since teleport maintains no long-term per-instance state.
func tryToDetectIdentityReuse(ctx context.Context, params *CheckEC2RequestParams, hostID string) error {
	if params.Role == types.RoleInstance {
		// There is no "Instance" heartbeat, instead this must check if there
		// are any heartbeats for any of the system roles granted by this
		// token.
		if params.SkipInstanceReuseCheck {
			// This check must be skipped if called via the legacy join
			// endpoint, where the Instance role may join after another system
			// role may have already joined and upserted a heartbeat.
			return nil
		}
		for _, role := range params.ProvisionToken.GetRoles() {
			if !roleHasPresenceCheck(role) {
				// There are a couple system roles (Discovery, MDM) that we
				// current allow to join via EC2 join method even though they
				// don't have any presence check.
				continue
			}
			alreadyExists, err := resourceExists(ctx, params.Presence, role, hostID)
			if err != nil {
				return trace.Wrap(err, "checking if %s with ID %s already exists", params.Role, hostID)
			}
			if alreadyExists {
				return trace.AccessDenied("server with host ID %q and role %q already exists", hostID, role)
			}
		}
		return nil
	}
	alreadyExists, err := resourceExists(ctx, params.Presence, params.Role, hostID)
	if err != nil {
		return trace.Wrap(err, "checking if %s with ID %s already exists", params.Role, hostID)
	}
	if alreadyExists {
		return trace.AccessDenied("server with host ID %q and role %q already exists", hostID, params.Role)
	}
	return nil
}

// CheckEC2RequestParams holds parameters for checking an EC2-method join request.
type CheckEC2RequestParams struct {
	// ProvisionToken is the provision token being used.
	ProvisionToken types.ProvisionToken
	// Role is the system role being requested.
	Role types.SystemRole
	// Document is a signed EC2 Instance Identity Document.
	Document []byte
	// RequestedHostID should be set if the joining client explicitly requested
	// a specific host ID.
	RequestedHostID *string
	// Presence is a presence service used to check for existence of resource heartbeats.
	Presence services.Presence
	// EC2Client is an optional client to use to check if EC2 instances are
	// running, if nil a default client will be used.
	EC2Client EC2Client
	// SkipInstanceReuseCheck should be set to true if being called by the
	// legacy join RPC where heartbeat presence checks must be skipped for join
	// attempts with the Instance role.
	SkipInstanceReuseCheck bool
	// Clock is a clock to use for the check.
	Clock clockwork.Clock
}

func (p *CheckEC2RequestParams) checkAndSetDefaults() error {
	switch {
	case p.ProvisionToken == nil:
		return trace.BadParameter("ProvisionToken is required")
	case p.Role == "":
		return trace.BadParameter("Role is required")
	case len(p.Document) == 0:
		return trace.BadParameter("Document is required")
	case p.Presence == nil:
		return trace.BadParameter("Presence service is required")
	case p.Clock == nil:
		p.Clock = clockwork.NewRealClock()
	}
	return nil
}

// CheckEC2Request checks register requests which use EC2 Simplified Node
// Joining. This method checks that:
//  1. The given Instance Identity Document has a valid signature (signed by AWS).
//  2. There is no obvious signs that a node already joined the cluster from this EC2 instance (to
//     reduce the risk of re-use of a stolen Instance Identity Document).
//  3. The signed instance attributes match one of the allow rules for the
//     corresponding token.
//
// Returns the host ID for the node, or an error if any of the checks fail.
func CheckEC2Request(ctx context.Context, params *CheckEC2RequestParams) (string, error) {
	if err := params.checkAndSetDefaults(); err != nil {
		return "", trace.AccessDenied("%s", err.Error())
	}

	iid, err := parseAndVerifyIID(params.Document)
	if err != nil {
		return "", trace.Wrap(err)
	}

	hostID := awsutils.NodeIDFromIID(iid)
	if params.RequestedHostID != nil && *params.RequestedHostID != hostID {
		return "", trace.AccessDenied("requested host ID does not match the Instance Identity Document")
	}

	if err := checkPendingTime(iid, params.ProvisionToken, params.Clock); err != nil {
		return "", trace.Wrap(err)
	}

	if err := tryToDetectIdentityReuse(ctx, params, hostID); err != nil {
		return "", trace.Wrap(err)
	}

	if err := checkEC2AllowRules(ctx, params, iid); err != nil {
		return "", trace.Wrap(err)
	}

	return hostID, nil
}
