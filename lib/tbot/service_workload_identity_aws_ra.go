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
	"cmp"
	"context"
	"crypto"
	"crypto/x509"
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"
	awsspiffe "github.com/spiffe/aws-spiffe-workload-helper"
	"github.com/spiffe/aws-spiffe-workload-helper/vendoredaws"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity"
)

// WorkloadIdentityAWSRAService is a service that retrieves X.509 certificates
// and exchanges them for AWS credentials using the AWS Roles Anywhere service.
type WorkloadIdentityAWSRAService struct {
	botAuthClient  *authclient.Client
	botCfg         *config.BotConfig
	cfg            *config.WorkloadIdentityAWSRAService
	getBotIdentity getBotIdentityFn
	log            *slog.Logger
	resolver       reversetunnelclient.Resolver
}

// String returns a human-readable description of the service.
func (s *WorkloadIdentityAWSRAService) String() string {
	return fmt.Sprintf("workload-identity-aws-ra (%s)", s.cfg.Destination.String())
}

// OneShot runs the service once, generating the output and writing it to the
// destination, before exiting.
func (s *WorkloadIdentityAWSRAService) OneShot(ctx context.Context) error {
	res, privateKey, err := s.requestSVID(ctx)
	if err != nil {
		return trace.Wrap(err, "requesting SVID")
	}
	err = s.exchangeSVID(ctx, res, privateKey)
	if err != nil {
		return trace.Wrap(err, "exchanging SVID via Roles Anywhere")
	}

	return s.render(ctx)
}

// exchangeSVID will exchange the X.509 SVID for AWS credentials using the
// AWS Roles Anywhere service.
func (s *WorkloadIdentityAWSRAService) exchangeSVID(
	ctx context.Context,
	x509Cred *workloadidentityv1pb.Credential,
	privateKey crypto.Signer,
) (*vendoredaws.CredentialProcessOutput, error) {
	pkcs8, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, trace.Wrap(err, "marshalling private key")
	}
	svid, err := x509svid.ParseRaw(x509Cred.GetX509Svid().Cert, pkcs8)
	if err != nil {
		return nil, trace.Wrap(err, "marshalling private key")
	}
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
		SessionDuration: int(
			cmp.Or(
				s.cfg.CredentialLifetime, s.botCfg.CredentialLifetime,
			).TTL.Seconds()),
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

	roles, err := fetchDefaultRoles(ctx, s.botAuthClient, s.getBotIdentity())
	if err != nil {
		return nil, nil, trace.Wrap(err, "fetching roles")
	}

	id, err := generateIdentity(
		ctx,
		s.botAuthClient,
		s.getBotIdentity(),
		roles,
		cmp.Or(s.cfg.CredentialLifetime, s.botCfg.CredentialLifetime).TTL,
		nil,
	)
	if err != nil {
		return nil, nil, trace.Wrap(err, "generating identity")
	}
	// create a client that uses the impersonated identity, so that when we
	// fetch information, we can ensure access rights are enforced.
	facade := identity.NewFacade(s.botCfg.FIPS, s.botCfg.Insecure, id)
	impersonatedClient, err := clientForFacade(ctx, s.log, s.botCfg, facade, s.resolver)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer impersonatedClient.Close()

	x509Credentials, privateKey, err := workloadidentity.IssueX509WorkloadIdentity(
		ctx,
		s.log,
		impersonatedClient,
		s.cfg.Selector,
		cmp.Or(s.cfg.CredentialLifetime, s.botCfg.CredentialLifetime).TTL,
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

func (s *WorkloadIdentityAWSRAService) render(ctx context.Context) error {

}
