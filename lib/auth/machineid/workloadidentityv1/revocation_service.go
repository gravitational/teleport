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
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
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
	Clock      clockwork.Clock
	Emitter    apievents.Emitter
	Logger     *slog.Logger
	// CertAuthorityGetter is used to fetch the CA for signing the CRL.
	CertAuthorityGetter certAuthorityGetter
	EventsWatcher       eventsWatcher
	// ClusterName is the name of the cluster that the service is running in,
	// used to fetch the correct CA for signing the CRL.
	ClusterName string
	// KeyStore is the key storer used to store and retrieve keys for the
	// signing of the CRL.
	KeyStore KeyStorer

	// RevocationsEventProcessedCh is a channel that will be emitted to when
	// a revocation event has been processed. Used for syncing in tests to avoid
	// flakiness.
	RevocationsEventProcessedCh chan struct{}
}

// RevocationService is the gRPC service for managing workload identity
// revocations.
// It implements the workloadidentityv1pb.WorkloadIdentityRevocationServiceServer
type RevocationService struct {
	workloadidentityv1pb.UnimplementedWorkloadIdentityRevocationServiceServer

	authorizer          authz.Authorizer
	store               workloadIdentityX509RevocationReadWriter
	clock               clockwork.Clock
	emitter             apievents.Emitter
	logger              *slog.Logger
	certAuthorityGetter certAuthorityGetter
	keyStore            KeyStorer
	clusterName         string
	eventsWatcher       eventsWatcher

	crlSigningDebounce time.Duration
	crlFailureBackoff  time.Duration
	crlPeriodicRenewal time.Duration

	// mu protects the signedCRL and notifyNewSignedCRL field.
	mu        sync.Mutex
	signedCRL []byte
	// notifyNewCRL will be closed when a new CRL is available. It is protected
	// by mu.
	notifyNewSignedCRL chan struct{}

	revocationsEventProcessedCh chan struct{}
}

// NewRevocationService returns a new instance of the RevocationService.
func NewRevocationService(cfg *RevocationServiceConfig) (*RevocationService, error) {
	switch {
	case cfg.Store == nil:
		return nil, trace.BadParameter("store service is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	case cfg.ClusterName == "":
		return nil, trace.BadParameter("cluster name is required")
	case cfg.KeyStore == nil:
		return nil, trace.BadParameter("key storer is required")
	case cfg.EventsWatcher == nil:
		return nil, trace.BadParameter("events watcher is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "workload_identity_revocation.service")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	return &RevocationService{
		authorizer:                  cfg.Authorizer,
		store:                       cfg.Store,
		clock:                       cfg.Clock,
		emitter:                     cfg.Emitter,
		logger:                      cfg.Logger,
		clusterName:                 cfg.ClusterName,
		certAuthorityGetter:         cfg.CertAuthorityGetter,
		keyStore:                    cfg.KeyStore,
		eventsWatcher:               cfg.EventsWatcher,
		crlSigningDebounce:          5 * time.Second,
		crlFailureBackoff:           30 * time.Second,
		crlPeriodicRenewal:          10 * time.Minute,
		revocationsEventProcessedCh: cfg.RevocationsEventProcessedCh,

		notifyNewSignedCRL: make(chan struct{}),
	}, nil
}

// GetWorkloadIdentityX509Revocation returns a WorkloadIdentityX509Revocation
// by name. An error is returned if the resource does not exist.
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

// ListWorkloadIdentityX509Revocations returns a list of
// WorkloadIdentityX509Revocation resources. It follows the Google API design
// guidelines for list pagination.
// Implements teleport.workloadidentity.v1.RevocationService/ListWorkloadIdentityX509Revocations
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

// DeleteWorkloadIdentityX509Revocation deletes a WorkloadIdentityX509Revocation
// by name. An error is returned if the resource does not exist.
// Implements teleport.workloadidentity.v1.RevocationService/DeleteWorkloadIdentityX509Revocation
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

	evt := &apievents.WorkloadIdentityX509RevocationDelete{
		Metadata: apievents.Metadata{
			Code: events.WorkloadIdentityX509RevocationDeleteCode,
			Type: events.WorkloadIdentityX509RevocationDeleteEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.Name,
		},
	}
	if err := s.emitter.EmitAuditEvent(ctx, evt); err != nil {
		s.logger.ErrorContext(
			ctx, "Failed to emit audit event for UpsertWorkloadIdentityX509Revocation",
			"error", err,
		)
	}

	return &emptypb.Empty{}, nil
}

// CreateWorkloadIdentityX509Revocation creates a new WorkloadIdentityX509Revocation.
// Implements teleport.workloadidentity.v1.RevocationService/CreateWorkloadIdentityX509Revocation
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

	evt := &apievents.WorkloadIdentityX509RevocationCreate{
		Metadata: apievents.Metadata{
			Code: events.WorkloadIdentityX509RevocationCreateCode,
			Type: events.WorkloadIdentityX509RevocationCreateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: created.GetMetadata().GetName(),
		},
		Reason: created.GetSpec().GetReason(),
	}
	if err := s.emitter.EmitAuditEvent(ctx, evt); err != nil {
		s.logger.ErrorContext(
			ctx, "Failed to emit audit event for CreateWorkloadIdentityX509Revocation",
			"error", err,
		)
	}

	return created, nil
}

// UpdateWorkloadIdentityX509Revocation updates an existing
// WorkloadIdentityX509Revocation.
// Implements teleport.workloadidentity.v1.RevocationService/UpdateWorkloadIdentityX509Revocation
func (s *RevocationService) UpdateWorkloadIdentityX509Revocation(
	ctx context.Context, req *workloadidentityv1pb.UpdateWorkloadIdentityX509RevocationRequest,
) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindWorkloadIdentityX509Revocation, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.store.UpdateWorkloadIdentityX509Revocation(ctx, req.WorkloadIdentityX509Revocation)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	evt := &apievents.WorkloadIdentityX509RevocationUpdate{
		Metadata: apievents.Metadata{
			Code: events.WorkloadIdentityX509RevocationUpdateCode,
			Type: events.WorkloadIdentityX509RevocationUpdateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: created.GetMetadata().GetName(),
		},
		Reason: created.GetSpec().GetReason(),
	}
	if err := s.emitter.EmitAuditEvent(ctx, evt); err != nil {
		s.logger.ErrorContext(
			ctx, "Failed to emit audit event for UpdateWorkloadIdentityX509Revocation",
			"error", err,
		)
	}

	return created, nil
}

// UpsertWorkloadIdentityX509Revocation updates or creates an existing
// WorkloadIdentityX509Revocation.
// Implements teleport.workloadidentity.v1.RevocationService/UpsertWorkloadIdentityX509Revocation
func (s *RevocationService) UpsertWorkloadIdentityX509Revocation(
	ctx context.Context, req *workloadidentityv1pb.UpsertWorkloadIdentityX509RevocationRequest,
) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(
		types.KindWorkloadIdentityX509Revocation, types.VerbCreate, types.VerbUpdate,
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

	evt := &apievents.WorkloadIdentityX509RevocationCreate{
		Metadata: apievents.Metadata{
			Code: events.WorkloadIdentityX509RevocationCreateCode,
			Type: events.WorkloadIdentityX509RevocationCreateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: created.GetMetadata().GetName(),
		},
		Reason: created.GetSpec().GetReason(),
	}
	if err := s.emitter.EmitAuditEvent(ctx, evt); err != nil {
		s.logger.ErrorContext(
			ctx, "Failed to emit audit event for UpsertWorkloadIdentityX509Revocation",
			"error", err,
		)
	}

	return created, nil
}

// StreamSignedCRL streams the signed CRL to the client. If the CRL has not
// yet been signed, the server will wait until it has been signed to send it
// to the client.
// Implements teleport.workloadidentity.v1.RevocationService/StreamSignedCRL
func (s *RevocationService) StreamSignedCRL(
	req *workloadidentityv1pb.StreamSignedCRLRequest,
	srv grpc.ServerStreamingServer[workloadidentityv1pb.StreamSignedCRLResponse],
) error {
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

func (s *RevocationService) RunCRLSigner(ctx context.Context) {
	for {
		err := s.watchAndSign(ctx)
		if err == nil {
			if ctx.Err() != nil {
				return
			}
			err = trace.BadParameter("watchAndSign exited unexpectedly")
		}
		retryAfter := retryutils.HalfJitter(s.crlFailureBackoff)
		if err != nil {
			s.logger.ErrorContext(
				ctx,
				"CRL signer exited with error",
				"error", err,
				"retry_after", retryAfter,
			)
		}

		select {
		case <-ctx.Done():
			return
		case <-s.clock.After(retryAfter):
			s.logger.DebugContext(ctx, "Retry backoff expired, restarting CRL signer")
		}
	}

}

func (s *RevocationService) watchAndSign(ctx context.Context) error {
	s.logger.DebugContext(ctx, "Starting CRL signer")
	w, err := s.eventsWatcher.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{{
			Kind: types.KindWorkloadIdentityX509Revocation,
		}},
	})
	if err != nil {
		return trace.Wrap(err, "creating events watcher")
	}
	defer func() {
		if err := w.Close(); err != nil {
			s.logger.WarnContext(ctx, "Failed to close watcher", "error", err)
		}
	}()

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
			unwrapper, ok := e.Resource.(types.Resource153UnwrapperT[*workloadidentityv1pb.WorkloadIdentityX509Revocation])
			if !ok {
				return false, trace.BadParameter(
					"expected event resource (%s) to implement Resource153Wrapper",
					e.Resource.GetName(),
				)
			}
			revocation := unwrapper.UnwrapT()
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
		return trace.Wrap(err, "signing initial CRL")
	}
	s.publishSignedCRL(crl)
	s.logger.DebugContext(ctx, "Finished initializing CRL signer, watching for revocation events")

	// A short, simple debounce so that we:
	// - Avoid signing the CRL too frequently. This is computationally
	//   expensive and we can afford to wait a few seconds to group together
	//  multiple successive revocations.
	// - Avoid spamming the clients with a rapid succession of CRL updates.
	var debounceCh <-chan time.Time
	for {
		periodic := s.clock.NewTimer(s.crlPeriodicRenewal)
		select {
		case e := <-w.Events():
			triggerSign, err := handleEvent(e)
			if err != nil {
				return trace.Wrap(err, "handling event")
			}
			if triggerSign {
				s.logger.DebugContext(ctx, "Received change to WorkloadIdentityX509Revocation indicating new CRL should be signed", "workload_identity_revocation_name", e.Resource.GetName())
				if debounceCh == nil {
					s.logger.DebugContext(ctx, "Starting debounce timer for signing of new CRL")
					debounceCh = s.clock.After(s.crlSigningDebounce)
				}
			}
			if s.revocationsEventProcessedCh != nil {
				s.revocationsEventProcessedCh <- struct{}{}
			}
			continue
		case <-w.Done():
			if err := w.Error(); err != nil {
				return trace.Wrap(err, "watcher failed")
			}
			return nil
		case <-ctx.Done():
			return nil
		case <-debounceCh:
			// Set debounce channel to nil to indicate that the requested
			// signature has been handled.
			debounceCh = nil

			crl, err := s.signCRL(ctx, revocationsMap)
			if err != nil {
				return trace.Wrap(err, "signing CRL")
			}
			s.publishSignedCRL(crl)
		case <-periodic.Chan():
			revocationsSlice, err := s.fetchAllRevocations(ctx)
			if err != nil {
				return trace.Wrap(err, "initially fetching revocations")
			}
			newRevocationsMap := make(map[string]*workloadidentityv1pb.WorkloadIdentityX509Revocation, len(revocationsSlice))
			for _, revocation := range revocationsSlice {
				newRevocationsMap[revocation.Metadata.Name] = revocation
			}
			revocationsMap = newRevocationsMap
			crl, err := s.signCRL(ctx, revocationsMap)
			if err != nil {
				return trace.Wrap(err, "signing CRL")
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

// signCRL signs a new revocation list for the given set of revocations, and
// returns this as a PKCS.1 DER encoded CRL.
func (s *RevocationService) signCRL(
	ctx context.Context,
	revocations map[string]*workloadidentityv1pb.WorkloadIdentityX509Revocation,
) (_ []byte, err error) {
	ctx, span := tracer.Start(ctx, "RevocationService/signCRL")
	defer func() {
		tracing.EndSpan(span, err)
	}()

	s.logger.InfoContext(ctx, "Starting to generate new CRL")
	ca, err := s.certAuthorityGetter.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SPIFFECA,
		DomainName: s.clusterName,
	}, true)
	if err != nil {
		return nil, trace.Wrap(err, "getting CA")
	}
	tlsCert, tlsSigner, err := s.keyStore.GetTLSCertAndSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err, "getting CA cert and key")
	}
	tlsCA, err := tlsca.FromCertAndSigner(tlsCert, tlsSigner)
	if err != nil {
		return nil, trace.Wrap(err, "creating TLS CA")
	}

	// RFC 5280 Certificate Revocation List
	// Ref: https://datatracker.ietf.org/doc/html/rfc5280#section-5
	tmpl := &x509.RevocationList{
		// Ref: https://www.rfc-editor.org/rfc/rfc5280.html#section-5.1.2.6
		RevokedCertificateEntries: make([]x509.RevocationListEntry, 0, len(revocations)),
		// Ref: https://www.rfc-editor.org/rfc/rfc5280.html#section-5.2.3
		// This is an optional extension we will be omitting for now, at a
		// future date, we may insert a monotonically increasing identifier.
		Number: big.NewInt(s.clock.Now().Unix()),
	}

	for _, revocation := range revocations {
		serial := new(big.Int)
		_, ok := serial.SetString(revocation.Metadata.Name, 16)
		if !ok {
			s.logger.WarnContext(
				ctx,
				"Encountered WorkloadIdentityX509Revocation with unparsable serial number, it will be omitted from the CRL",
				"workload_identity_revocation_name", revocation.Metadata.Name,
			)
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
	s.logger.InfoContext(ctx, "Finished generating new CRL", "revocations", len(revocations))
	return signedCRL, nil
}

func (s *RevocationService) getSignedCRL() ([]byte, chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.signedCRL, s.notifyNewSignedCRL
}

func (s *RevocationService) publishSignedCRL(crl []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signedCRL = crl
	// Close old channel to notify clients that a new CRL is available.
	close(s.notifyNewSignedCRL)
	s.notifyNewSignedCRL = make(chan struct{})
}
