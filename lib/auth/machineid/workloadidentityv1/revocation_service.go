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

package workloadidentityv1

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"log/slog"
	"math/big"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/tlsca"
)

type workloadIdentityX509RevocationReadWriter interface {
	GetWorkloadIdentityX509Revocation(ctx context.Context, name string) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error)
	ListWorkloadIdentityX509Revocations(ctx context.Context, pageSize int, token string) ([]*workloadidentityv1pb.WorkloadIdentityX509Revocation, string, error)
	CreateWorkloadIdentityX509Revocation(ctx context.Context, resource *workloadidentityv1pb.WorkloadIdentityX509Revocation) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error)
	UpdateWorkloadIdentityX509Revocation(ctx context.Context, resource *workloadidentityv1pb.WorkloadIdentityX509Revocation) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error)
	DeleteWorkloadIdentityX509Revocation(ctx context.Context, name string) error
	UpsertWorkloadIdentityX509Revocation(ctx context.Context, resource *workloadidentityv1pb.WorkloadIdentityX509Revocation) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error)
}

type certAuthorityGetter interface {
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
}

type eventsWatcher interface {
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)
}

// RevocationServiceConfig holds configuration options for the RevocationService.
type RevocationServiceConfig struct {
	Authorizer authz.Authorizer
	Store      workloadIdentityX509RevocationReadWriter
	Backend    backend.Backend
	Clock      clockwork.Clock
	Emitter    apievents.Emitter
	Logger     *slog.Logger
	// KeyStorer is used to access the signing keys necessary to sign a CRL.
	KeyStorer KeyStorer
	// CertAuthorityGetter is used to get the certificate authority needed to
	// sign the CRL.
	CertAuthorityGetter certAuthorityGetter
	// ClusterName is the name of the cluster, used to fetch the correct CA.
	ClusterName string

	EventsWatcher eventsWatcher
}

// RevocationService is the gRPC service for managing workload identity
// revocations.
// It implements the workloadidentityv1pb.WorkloadIdentityRevocationServiceServer
type RevocationService struct {
	workloadidentityv1pb.UnimplementedWorkloadIdentityRevocationServiceServer

	authorizer          authz.Authorizer
	backend             backend.Backend
	store               workloadIdentityX509RevocationReadWriter
	clock               clockwork.Clock
	emitter             apievents.Emitter
	logger              *slog.Logger
	keyStorer           KeyStorer
	certAuthorityGetter certAuthorityGetter
	clusterName         string

	eventsWatcher eventsWatcher

	mu        sync.RWMutex
	signedCRL []byte
	// notifyNewCRL will be closed when a new CRL is available. It is protected
	// by mu.
	notifyNewSignedCRL chan struct{}
}

// NewRevocationService returns a new instance of the RevocationService.
func NewRevocationService(cfg *RevocationServiceConfig) (*RevocationService, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("store service is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "workload_identity_revocation.service")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	return &RevocationService{
		authorizer: cfg.Authorizer,
		store:      cfg.Store,
		clock:      cfg.Clock,
		emitter:    cfg.Emitter,
		logger:     cfg.Logger,

		notifyNewSignedCRL: make(chan struct{}),
	}, nil
}

// GetWorkloadIdentity returns a WorkloadIdentity by name.
// Implements teleport.workloadidentity.v1.RevocationService/GetWorkloadIdentityX509Revocation
func (s *RevocationService) GetWorkloadIdentityX509Revocation(
	ctx context.Context, req *workloadidentityv1pb.GetWorkloadIdentityX509RevocationRequest,
) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindWorkloadIdentityX509Revocation, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Name == "" {
		return nil, trace.BadParameter("name: must be non-empty")
	}

	resource, err := s.store.GetWorkloadIdentityX509Revocation(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resource, nil
}

// ListWorkloadIdentities returns a list of WorkloadIdentity resources. It
// follows the Google API design guidelines for list pagination.
// Implements teleport.workloadidentity.v1.ResourceService/ListWorkloadIdentities
func (s *RevocationService) ListWorkloadIdentityX509Revocations(
	ctx context.Context, req *workloadidentityv1pb.ListWorkloadIdentityX509RevocationsRequest,
) (*workloadidentityv1pb.ListWorkloadIdentityX509RevocationsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindWorkloadIdentityX509Revocation, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	resources, nextToken, err := s.store.ListWorkloadIdentityX509Revocations(
		ctx,
		int(req.PageSize),
		req.PageToken,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &workloadidentityv1pb.ListWorkloadIdentityX509RevocationsResponse{
		WorkloadIdentityX509Revocations: resources,
		NextPageToken:                   nextToken,
	}, nil
}

// DeleteWorkloadIdentity deletes a WorkloadIdentity by name.
// Implements teleport.workloadidentity.v1.ResourceService/DeleteWorkloadIdentity
func (s *RevocationService) DeleteWorkloadIdentityX509Revocation(
	ctx context.Context, req *workloadidentityv1pb.DeleteWorkloadIdentityX509RevocationRequest,
) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindWorkloadIdentityX509Revocation, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Name == "" {
		return nil, trace.BadParameter("name: must be non-empty")
	}

	if err := s.store.DeleteWorkloadIdentityX509Revocation(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: Audit event

	return &emptypb.Empty{}, nil
}

// CreateWorkloadIdentity creates a new WorkloadIdentity.
// Implements teleport.workloadidentity.v1.ResourceService/CreateWorkloadIdentity
func (s *RevocationService) CreateWorkloadIdentityX509Revocation(
	ctx context.Context, req *workloadidentityv1pb.CreateWorkloadIdentityX509RevocationRequest,
) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindWorkloadIdentityX509Revocation, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.store.CreateWorkloadIdentityX509Revocation(ctx, req.WorkloadIdentityX509Revocation)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: Audit log

	return created, nil
}

// UpdateWorkloadIdentity updates an existing WorkloadIdentity.
// Implements teleport.workloadidentity.v1.ResourceService/UpdateWorkloadIdentity
func (s *RevocationService) UpdateWorkloadIdentityX509Revocation(
	ctx context.Context, req *workloadidentityv1pb.UpdateWorkloadIdentityX509RevocationRequest,
) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindWorkloadIdentity, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.store.UpdateWorkloadIdentityX509Revocation(ctx, req.WorkloadIdentityX509Revocation)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: Audit log

	return created, nil
}

// UpsertWorkloadIdentity updates or creates an existing WorkloadIdentity.
// Implements teleport.workloadidentity.v1.ResourceService/UpsertWorkloadIdentity
func (s *RevocationService) UpsertWorkloadIdentityX509Revocation(
	ctx context.Context, req *workloadidentityv1pb.UpsertWorkloadIdentityX509RevocationRequest,
) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(
		types.KindWorkloadIdentity, types.VerbCreate, types.VerbUpdate,
	); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.store.UpsertWorkloadIdentityX509Revocation(ctx, req.WorkloadIdentityX509Revocation)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: Audit log

	return created, nil
}

func (s *RevocationService) StreamSignedCRL(req *workloadidentityv1pb.StreamSignedCRLRequest, srv grpc.ServerStreamingServer[workloadidentityv1pb.StreamSignedCRLResponse]) error {
	for {
		crl, notify := s.getSignedCRL()

		// The CRL may not yet have been signed, so, skip straight to waiting
		// for an update.
		if len(crl) != 0 {
			if err := srv.Send(&workloadidentityv1pb.StreamSignedCRLResponse{
				Crl: crl,
			}); err != nil {
				return trace.Wrap(err)
			}
		}

		select {
		case <-notify:
		case <-srv.Context().Done():
			return nil
		}
	}
}

func (s *RevocationService) getSignedCRL() ([]byte, chan struct{}) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.signedCRL, s.notifyNewSignedCRL
}

func (s *RevocationService) publishSignedCRL(crl []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signedCRL = crl
	close(s.notifyNewSignedCRL)
	s.notifyNewSignedCRL = make(chan struct{})
}

func (s *RevocationService) RunCRLSigner(ctx context.Context) {
	for {
		err := s.watchAndSign(ctx)
		if err == nil {
			if ctx.Err() != nil {
				return
			}
			err = trace.BadParameter("watchAndSign exited unexpectedly")
		}
		if err != nil {
			s.logger.Error("CRL signer failed exited with error", "error", err)
		}

		// TODO: Backoff
	}

}

const (
	debounceDuration = time.Second * 5
)

func (s *RevocationService) watchAndSign(ctx context.Context) error {
	w, err := s.eventsWatcher.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{{
			Kind: types.KindWorkloadIdentityX509Revocation,
		}},
	})
	if err != nil {
		return trace.Wrap(err, "creating events watcher")
	}

	// Wait for initial "Init" event to indicate we're now receiving events.
	select {
	case <-w.Done():
		if err := w.Error(); err != nil {
			return trace.Wrap(err, "watcher failed")
		}
		return nil
	case evt := <-w.Events():
		if evt.Type == types.OpInit {
			break
		}
		return trace.BadParameter("expected init event, got %v", evt.Type)
	case <-ctx.Done():
		return nil
	}

	revocationsSlice, err := s.fetchAllRevocations(ctx)
	if err != nil {
		return trace.Wrap(err, "initially fetching revocations")
	}
	revocationsMap := make(map[string]*workloadidentityv1pb.WorkloadIdentityX509Revocation, len(revocationsSlice))
	for _, revocation := range revocationsSlice {
		revocationsMap[revocation.Metadata.Name] = revocation
	}

	handleEvent := func(e types.Event) (bool, error) {
		switch e.Type {
		case types.OpPut:
			unwrapper, ok := e.Resource.(types.Resource153Unwrapper)
			if !ok {
				return false, trace.BadParameter(
					"expected event resource (%s) to implement Resource153Wrapper",
					e.Resource.GetName(),
				)
			}
			unwrapped := unwrapper.Unwrap()
			revocation, ok := unwrapped.(*workloadidentityv1pb.WorkloadIdentityX509Revocation)
			if !ok {
				return false, trace.BadParameter(
					"expected event resource (%s) to be a WorkloadIdentityX509Revocation, but it was %T",
					e.Resource.GetName(),
					unwrapped,
				)
			}
			revocationsMap[revocation.Metadata.Name] = revocation
			return true, nil
		case types.OpDelete:
			delete(revocationsMap, e.Resource.GetName())
			return true, nil
		default:
		}
		return false, nil
	}

	// Perform initial signing of the CRL
	crl, err := s.signCRL(ctx, revocationsMap)
	if err != nil {
		panic(err)
	}
	s.publishSignedCRL(crl)

	for {
		// Perform a short, simple debounce so that we:
		// - Avoid signing the CRL too frequently. This is computationally
		//   expensive and we can afford to wait a few seconds to group together
		//  multiple successive revocations.
		// - Avoid spamming the clients with a rapid succession of CRL updates.
		var debounceCh <-chan time.Time
		select {
		case e := <-w.Events():
			triggerSign, err := handleEvent(e)
			if err != nil {
				return trace.Wrap(err, "handling event")
			}
			if triggerSign {
				debounceCh = s.clock.After(debounceDuration)
			}
			continue
		case <-w.Done():
			if err := w.Error(); err != nil {
				return trace.Wrap(err, "watcher failed")
			}
			return nil
		case <-debounceCh:
			crl, err := s.signCRL(ctx, revocationsMap)
			if err != nil {
				panic(err)
			}
			s.publishSignedCRL(crl)
		}
	}
}

func (s *RevocationService) fetchAllRevocations(ctx context.Context) ([]*workloadidentityv1pb.WorkloadIdentityX509Revocation, error) {
	pageToken := ""
	revocations := []*workloadidentityv1pb.WorkloadIdentityX509Revocation{}
	for {
		res, token, err := s.store.ListWorkloadIdentityX509Revocations(
			ctx, 0, pageToken,
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		revocations = append(revocations, res...)
		if token == "" {
			break
		}
		pageToken = token
	}
	return revocations, nil
}

func (s *RevocationService) signCRL(
	ctx context.Context,
	revocations map[string]*workloadidentityv1pb.WorkloadIdentityX509Revocation,
) ([]byte, error) {
	ca, err := s.certAuthorityGetter.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SPIFFECA,
		DomainName: s.clusterName,
	}, true)
	tlsCert, tlsSigner, err := s.keyStorer.GetTLSCertAndSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err, "getting CA cert and key")
	}
	tlsCA, err := tlsca.FromCertAndSigner(tlsCert, tlsSigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tmpl := &x509.RevocationList{
		// https://www.rfc-editor.org/rfc/rfc5280.html#section-5.1.2.6
		RevokedCertificateEntries: make([]x509.RevocationListEntry, 0, len(revocations)),
		// https://www.rfc-editor.org/rfc/rfc5280.html#section-5.1.2.4
		// This field indicates the issue date of this CRL.  thisUpdate may be
		//  encoded as UTCTime or GeneralizedTime.
		ThisUpdate: time.Now(),
		// https://www.rfc-editor.org/rfc/rfc5280.html#section-5.2.3
		// This is an optional extension we will be omitting for now, at a
		// future date, we may insert a monotonically increasing identifier.
		Number: nil,
	}

	for _, revocation := range revocations {
		serial := new(big.Int)
		_, ok := serial.SetString(revocation.Metadata.Name, 16)
		if !ok {
			// TODO log and skip?
			continue
		}

		tmpl.RevokedCertificateEntries = append(tmpl.RevokedCertificateEntries, x509.RevocationListEntry{
			SerialNumber:   serial,
			RevocationTime: revocation.Spec.RevokedAt.AsTime(),
		})
	}

	signedCRL, err := x509.CreateRevocationList(
		rand.Reader, tmpl, tlsCA.Cert, tlsCA.Signer,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return signedCRL, nil
}
