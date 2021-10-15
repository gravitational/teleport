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
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/jonboulle/clockwork"
	"go.mozilla.org/pkcs7"
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
	return ec2.NewFromConfig(cfg)
}

// NodeIDFromIID returns the node ID that must be used for nodes joining with
// the given Instance Identity Document.
func NodeIDFromIID(iid *imds.InstanceIdentityDocument) string {
	return iid.AccountID + "-" + iid.InstanceID
}

// checkEC2AllowRules checks that the iid matches at least one of the allow
// rules of the given token.
func checkEC2AllowRules(ctx context.Context, iid *imds.InstanceIdentityDocument, provisionToken types.ProvisionToken) error {
	allowRules := provisionToken.GetAllowRules()
	for _, rule := range allowRules {
		// If this rule specifies an AWS account, the IID must match
		if len(rule.AWSAccount) > 0 {
			if rule.AWSAccount != iid.AccountID {
				continue
			}
		}
		// If this rule specifies any AWS regions, the IID must match one of them
		if len(rule.AWSRegions) > 0 {
			if !utils.SliceContainsStr(rule.AWSRegions, iid.Region) {
				continue
			}
		}
		// iid matches this allow rule. Check if it is running.
		return trace.Wrap(checkInstanceRunning(ctx, iid.InstanceID, iid.Region, rule.AWSRole))
	}
	return trace.AccessDenied("instance did not match any allow rules")
}

func checkInstanceRunning(ctx context.Context, instanceID, region, IAMRole string) error {
	awsClientConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	awsClientConfig.Region = region

	// assume the configured IAM role if necessary
	if IAMRole != "" {
		stsClient := sts.NewFromConfig(awsClientConfig)
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
	namespaces, err := presence.GetNamespaces()
	if err != nil {
		return false, trace.Wrap(err)
	}
	for _, namespace := range namespaces {
		_, err := presence.GetNode(ctx, namespace.GetName(), hostID)
		if trace.IsNotFound(err) {
			continue
		} else if err != nil {
			return false, trace.Wrap(err)
		} else {
			return true, nil
		}
	}
	return false, nil
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
	kubes, err := presence.GetKubeServices(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	for _, kube := range kubes {
		if kube.GetName() == hostID {
			return true, nil
		}
	}
	return false, nil
}

func appExists(ctx context.Context, presence services.Presence, hostID string) (bool, error) {
	namespaces, err := presence.GetNamespaces()
	if err != nil {
		return false, trace.Wrap(err)
	}
	for _, namespace := range namespaces {
		apps, err := presence.GetApplicationServers(ctx, namespace.GetName())
		if err != nil {
			return false, trace.Wrap(err)
		}
		for _, app := range apps {
			if app.GetName() == hostID {
				return true, nil
			}
		}
	}
	return false, nil
}

func dbExists(ctx context.Context, presence services.Presence, hostID string) (bool, error) {
	namespaces, err := presence.GetNamespaces()
	if err != nil {
		return false, trace.Wrap(err)
	}
	for _, namespace := range namespaces {
		dbs, err := presence.GetDatabaseServers(ctx, namespace.GetName())
		if err != nil {
			return false, trace.Wrap(err)
		}
		for _, db := range dbs {
			if db.GetName() == hostID {
				return true, nil
			}
		}
	}
	return false, nil
}

// checkInstanceUnique makes sure the instance which sent the request has not
// already joined the cluster with the same role. Tokens should be limited to
// only allow the roles which will actually be used by all expected instances so
// that a stolen IID could not be used to join the cluster with a different
// role.
func (a *Server) checkInstanceUnique(ctx context.Context, req types.RegisterUsingTokenRequest, iid *imds.InstanceIdentityDocument) error {
	requestedHostID := req.HostID
	expectedHostID := NodeIDFromIID(iid)
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
	default:
		return trace.BadParameter("unsupported role: %q", req.Role)
	}

	if err != nil {
		return trace.Wrap(err)
	}
	if instanceExists {
		log.Warnf("Server with ID %q and role %q is attempting to join the cluster with a Simplified Node Joining request, but"+
			" a server with this ID is already present in the cluster.", req.HostID, req.Role)
		return trace.AccessDenied("server with host ID %q and role %q already exists", req.HostID, req.Role)
	}
	return nil
}

// CheckEC2Request checks register requests which use EC2 Simplified Node
// Joining. This method checks that:
// 1. The given Instance Identity Document has a valid signature (signed by AWS).
// 2. A node has not already joined the cluster from this EC2 instance (to
//    prevent re-use of a stolen Instance Identity Document).
// 3. The signed instance attributes match one of the allow rules for the
//    corresponding token.
// If the request does not include an Instance Identity Document, and the
// token does not include any allow rules, this method returns nil and the
// normal token checking logic resumes.
func (a *Server) CheckEC2Request(ctx context.Context, req types.RegisterUsingTokenRequest) error {
	requestIncludesIID := req.EC2IdentityDocument != nil
	tokenName := req.Token
	provisionToken, err := a.GetCache().GetToken(ctx, tokenName)
	if err != nil {
		if trace.IsNotFound(err) && !requestIncludesIID {
			// This is not a Simplified Node Joining request, pass on to the
			// regular token checking logic in case this is a static token.
			return nil
		}
		return trace.Wrap(err)
	}
	if err = provisionToken.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if provisionToken.GetJoinMethod() != types.JoinMethodEC2 {
		if requestIncludesIID {
			return trace.BadParameter("an EC2 Identity Document is included in a register request for a token which does not expect it")
		}
		// not a simplified node joining request, pass on to the regular token
		// checking logic
		return nil
	}

	log.Debugf("Received Simplified Node Joining request for host %q", req.HostID)

	iid, err := parseAndVerifyIID(req.EC2IdentityDocument)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := checkPendingTime(iid, provisionToken, a.clock); err != nil {
		return trace.Wrap(err)
	}

	if err := a.checkInstanceUnique(ctx, req, iid); err != nil {
		return trace.Wrap(err)
	}

	if err := checkEC2AllowRules(ctx, iid, provisionToken); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
