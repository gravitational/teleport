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

package workloadidentity

import (
	"context"
	"crypto"
	"crypto/x509"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/durationpb"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/tbot/config"
)

// WorkloadIdentityLogValue returns a slog.Value for a given
// *workloadidentityv1pb.Credential
func WorkloadIdentityLogValue(credential *workloadidentityv1pb.Credential) slog.Value {
	return slog.GroupValue(
		slog.String("name", credential.GetWorkloadIdentityName()),
		slog.String("revision", credential.GetWorkloadIdentityRevision()),
		slog.String("spiffe_id", credential.GetSpiffeId()),
		slog.String("serial_number", credential.GetX509Svid().GetSerialNumber()),
	)
}

// WorkloadIdentitiesLogValue returns []slog.Value for a slice of
// *workloadidentityv1.Credential
func WorkloadIdentitiesLogValue(credentials []*workloadidentityv1pb.Credential) []slog.Value {
	values := make([]slog.Value, 0, len(credentials))
	for _, credential := range credentials {
		values = append(values, WorkloadIdentityLogValue(credential))
	}
	return values
}

// IssueX509WorkloadIdentity uses a given client and selector to issue a single
// or multiple X509 workload identity credentials.
func IssueX509WorkloadIdentity(
	ctx context.Context,
	log *slog.Logger,
	clt *authclient.Client,
	workloadIdentity config.WorkloadIdentitySelector,
	ttl time.Duration,
	attest *workloadidentityv1pb.WorkloadAttrs,
) ([]*workloadidentityv1pb.Credential, crypto.Signer, error) {
	ctx, span := tracer.Start(
		ctx,
		"issueX509WorkloadIdentity",
	)
	defer span.End()
	privateKey, err := cryptosuites.GenerateKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(clt),
		cryptosuites.BotSVID)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	switch {
	case workloadIdentity.Name != "":
		log.DebugContext(
			ctx,
			"Requesting issuance of X509 workload identity credential using name of WorkloadIdentity resource",
			"name", workloadIdentity.Name,
		)
		// When using the "name" based selector, we either get a single WIC back,
		// or an error. We don't need to worry about selecting the right one.
		res, err := clt.WorkloadIdentityIssuanceClient().IssueWorkloadIdentity(ctx,
			&workloadidentityv1pb.IssueWorkloadIdentityRequest{
				Name: workloadIdentity.Name,
				Credential: &workloadidentityv1pb.IssueWorkloadIdentityRequest_X509SvidParams{
					X509SvidParams: &workloadidentityv1pb.X509SVIDParams{
						PublicKey: pubBytes,
					},
				},
				RequestedTtl:  durationpb.New(ttl),
				WorkloadAttrs: attest,
			},
		)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		log.DebugContext(
			ctx,
			"Received X509 workload identity credential",
			"credential", WorkloadIdentityLogValue(res.Credential),
		)
		return []*workloadidentityv1pb.Credential{res.Credential}, privateKey, nil
	case len(workloadIdentity.Labels) > 0:
		labelSelectors := make([]*workloadidentityv1pb.LabelSelector, 0, len(workloadIdentity.Labels))
		for k, v := range workloadIdentity.Labels {
			labelSelectors = append(labelSelectors, &workloadidentityv1pb.LabelSelector{
				Key:    k,
				Values: v,
			})
		}
		log.DebugContext(
			ctx,
			"Requesting issuance of X509 workload identity credentials using labels",
			"labels", labelSelectors,
		)
		res, err := clt.WorkloadIdentityIssuanceClient().IssueWorkloadIdentities(ctx,
			&workloadidentityv1pb.IssueWorkloadIdentitiesRequest{
				LabelSelectors: labelSelectors,
				Credential: &workloadidentityv1pb.IssueWorkloadIdentitiesRequest_X509SvidParams{
					X509SvidParams: &workloadidentityv1pb.X509SVIDParams{
						PublicKey: pubBytes,
					},
				},
				RequestedTtl:  durationpb.New(ttl),
				WorkloadAttrs: attest,
			},
		)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		log.DebugContext(
			ctx,
			"Received X509 workload identity credentials",
			"credentials", WorkloadIdentitiesLogValue(res.Credentials),
		)
		return res.Credentials, privateKey, nil
	default:
		return nil, nil, trace.BadParameter("no valid selector configured")
	}
}
