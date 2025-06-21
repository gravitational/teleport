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

package tbot

import (
	"bytes"
	"cmp"
	"context"
	"crypto"
	"crypto/x509"
	"fmt"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	awsspiffe "github.com/spiffe/aws-spiffe-workload-helper"
	"github.com/spiffe/aws-spiffe-workload-helper/vendoredaws"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"gopkg.in/ini.v1"

	apiclient "github.com/gravitational/teleport/api/client"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity"
)

// WorkloadIdentityAWSRAService is a service that retrieves X.509 certificates
// and exchanges them for AWS credentials using the AWS Roles Anywhere service.
type WorkloadIdentityAWSRAService struct {
	botAuthClient      *apiclient.Client
	botIdentityReadyCh <-chan struct{}
	botCfg             *config.BotConfig
	cfg                *config.WorkloadIdentityAWSRAService
	getBotIdentity     getBotIdentityFn
	log                *slog.Logger
	reloadBroadcaster  *channelBroadcaster
	identityGenerator  *identity.Generator
	clientBuilder      *client.Builder
}

// String returns a human-readable description of the service.
func (s *WorkloadIdentityAWSRAService) String() string {
	return fmt.Sprintf("workload-identity-aws-roles-anywhere (%s)", s.cfg.Destination.String())
}

// OneShot runs the service once, generating the output and writing it to the
// destination, before exiting.
func (s *WorkloadIdentityAWSRAService) OneShot(ctx context.Context) error {
	return s.generate(ctx)
}

// Run runs the service in a loop, generating the output and writing it to the
// destination at regular intervals.
func (s *WorkloadIdentityAWSRAService) Run(ctx context.Context) error {
	reloadCh, unsubscribe := s.reloadBroadcaster.subscribe()
	defer unsubscribe()

	err := runOnInterval(ctx, runOnIntervalConfig{
		service:         s.String(),
		name:            "output-renewal",
		f:               s.generate,
		interval:        s.cfg.SessionRenewalInterval,
		retryLimit:      renewalRetryLimit,
		log:             s.log,
		reloadCh:        reloadCh,
		identityReadyCh: s.botIdentityReadyCh,
	})
	return trace.Wrap(err)
}

func (s *WorkloadIdentityAWSRAService) generate(ctx context.Context) error {
	res, privateKey, err := s.requestSVID(ctx)
	if err != nil {
		return trace.Wrap(err, "requesting SVID")
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return trace.Wrap(err, "marshaling private key")
	}
	certWithChain := new(bytes.Buffer)
	_, _ = certWithChain.Write(res.GetX509Svid().GetCert())
	// If external PKI is configured, we need to append the chain to the leaf
	// certificate before calling x509svid.ParseRaw.
	for _, cert := range res.GetX509Svid().GetChain() {
		_, _ = certWithChain.Write(cert)
	}
	svid, err := x509svid.ParseRaw(certWithChain.Bytes(), pkcs8)
	if err != nil {
		return trace.Wrap(err, "parsing x509 svid")
	}

	s.log.InfoContext(
		ctx,
		"Exchanging SVID for AWS credentials",
		"spiffe_id", svid.ID.String(),
		"role_arn", s.cfg.RoleARN,
		"profile_arn", s.cfg.ProfileARN,
		"trust_anchor_arn", s.cfg.TrustAnchorARN,
	)
	creds, err := s.exchangeSVID(svid)
	if err != nil {
		return trace.Wrap(err, "exchanging SVID via Roles Anywhere")
	}
	s.log.InfoContext(
		ctx,
		"Exchanged SVID for AWS credentials",
		"aws_credentials_expiry", creds.Expiration,
	)

	return s.renderAWSCreds(ctx, creds)
}

// exchangeSVID will exchange the X.509 SVID for AWS credentials using the
// AWS Roles Anywhere service.
func (s *WorkloadIdentityAWSRAService) exchangeSVID(
	svid *x509svid.SVID,
) (*vendoredaws.CredentialProcessOutput, error) {
	signer := &awsspiffe.X509SVIDSigner{
		SVID: svid,
	}
	algo, err := signer.SignatureAlgorithm()
	if err != nil {
		return nil, trace.Wrap(err, "getting signature algorithm")
	}

	credentials, err := vendoredaws.GenerateCredentials(&vendoredaws.CredentialsOpts{
		RoleArn:           s.cfg.RoleARN,
		ProfileArnStr:     s.cfg.ProfileARN,
		Region:            s.cfg.Region,
		TrustAnchorArnStr: s.cfg.TrustAnchorARN,
		SessionDuration:   int(s.cfg.SessionDuration.Seconds()),
		Endpoint:          s.cfg.EndpointOverride,
	}, signer, algo)
	if err != nil {
		return nil, trace.Wrap(err, "exchanging credentials")
	}

	return &credentials, nil
}

func (s *WorkloadIdentityAWSRAService) requestSVID(
	ctx context.Context,
) (
	*workloadidentityv1pb.Credential,
	crypto.Signer,
	error,
) {
	ctx, span := tracer.Start(
		ctx,
		"WorkloadIdentityAWSRAService/requestSVID",
	)
	defer span.End()

	id, err := s.identityGenerator.GenerateFacade(ctx,
		// We only need this to issue the X509 SVID, so we don't need the full
		// lifetime.
		identity.WithLifetime(time.Minute*10, 0),
		identity.WithLogger(s.log),
	)
	if err != nil {
		return nil, nil, trace.Wrap(err, "generating identity")
	}
	// create a client that uses the impersonated identity, so that when we
	// fetch information, we can ensure access rights are enforced.
	impersonatedClient, err := s.clientBuilder.Build(ctx, id)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer impersonatedClient.Close()

	x509Credentials, privateKey, err := workloadidentity.IssueX509WorkloadIdentity(
		ctx,
		s.log,
		impersonatedClient,
		s.cfg.Selector,
		// We only use this SVID to exchange for AWS credentials, so we don't
		// need the full lifetime.
		time.Minute*10,
		nil,
	)
	if err != nil {
		return nil, nil, trace.Wrap(err, "generating X509 SVID")
	}
	var x509Credential *workloadidentityv1pb.Credential
	switch len(x509Credentials) {
	case 0:
		return nil, nil, trace.BadParameter("no X509 SVIDs returned")
	case 1:
		x509Credential = x509Credentials[0]
	default:
		// We could eventually implement some kind of hint selection mechanism
		// to pick the "right" one.
		received := make([]string, 0, len(x509Credentials))
		for _, cred := range x509Credentials {
			received = append(received,
				fmt.Sprintf(
					"%s:%s",
					cred.WorkloadIdentityName,
					cred.SpiffeId,
				),
			)
		}
		return nil, nil, trace.BadParameter(
			"multiple X509 SVIDs received: %v", received,
		)
	}

	return x509Credential, privateKey, nil
}

func loadExistingAWSCredentialFile(
	ctx context.Context, dest bot.Destination, artifactName string,
) (*ini.File, error) {
	// Load the existing credential file if it exists so we can merge with
	// it.
	data, err := dest.Read(ctx, artifactName)
	if err != nil {
		if trace.IsNotFound(err) {
			return ini.Empty(), nil
		}
		return nil, trace.Wrap(err, "reading existing credentials")
	}

	f, err := ini.Load(data)
	if err != nil {
		return nil, trace.Wrap(err, "parsing existing credentials")
	}
	return f, nil
}

// render will write the AWS credentials to the AWS CLI configuration file.
// See https://docs.aws.amazon.com/cli/v1/userguide/cli-configure-files.html
func (s *WorkloadIdentityAWSRAService) renderAWSCreds(
	ctx context.Context,
	creds *vendoredaws.CredentialProcessOutput,
) error {
	ctx, span := tracer.Start(
		ctx,
		"renderAWSCreds",
	)
	defer span.End()

	expiresAt, err := time.Parse(time.RFC3339, creds.Expiration)
	if err != nil {
		return fmt.Errorf("parsing expiration time: %w", err)
	}

	artifactName := cmp.Or(s.cfg.ArtifactName, "aws_credentials")

	f := ini.Empty()
	if !s.cfg.OverwriteCredentialFile {
		var err error
		f, err = loadExistingAWSCredentialFile(
			ctx, s.cfg.Destination, artifactName,
		)
		if err != nil {
			return trace.Wrap(err, "loading existing credentials")
		}
	}

	// "default" is the special profile name that the AWS CLI/SDK will read by
	// default.
	profileName := cmp.Or(s.cfg.CredentialProfileName, "default")
	sec := f.Section(profileName)
	sec.Key("aws_secret_access_key").SetValue(creds.SecretAccessKey)
	sec.Key("aws_access_key_id").SetValue(creds.AccessKeyId)
	sec.Key("aws_session_token").SetValue(creds.SessionToken)
	sec.Key("expiration").SetValue(
		fmt.Sprintf("%d", expiresAt.UnixMilli()),
	)

	b := &bytes.Buffer{}
	_, err = f.WriteTo(b)
	if err != nil {
		return trace.Wrap(err, "writing credentials to buffer")
	}

	err = s.cfg.Destination.Write(ctx, artifactName, b.Bytes())
	if err != nil {
		return trace.Wrap(err, "writing credentials to destination")
	}
	return nil
}
