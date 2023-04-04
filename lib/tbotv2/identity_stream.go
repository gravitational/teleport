package tbotv2

import (
	"context"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

type IdentityStreamManager struct {
	// mu protects identityStreams
	mu              sync.Mutex
	identityStreams map[*IdentityStream]struct{}

	wg sync.WaitGroup

	// TODO: Allow client to be updated... Wouldn't it be nice if we supported
	// dynamic credentials on this.
	client auth.ClientI

	clock clockwork.Clock

	close chan struct{}

	log logrus.FieldLogger
}

func (ism *IdentityStreamManager) fetchIdentity(ctx context.Context, req IdentityRequest) (*proto.Certs, error) {
	_, publicKey, err := native.GenerateKeyPair()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certs, err := ism.client.GenerateUserCerts(
		ctx,
		proto.UserCertsRequest{
			PublicKey:       publicKey,
			Username:        "",
			Expires:         ism.clock.Now().Add(req.TTL),
			RouteToCluster:  "",
			UseRoleRequests: true,
			RoleRequests:    req.Roles,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return certs, nil
}

// TODO: What repercussion does this have? How can we ensure we don't
// stampeding herd an auth server?
// Can we limit number of concurrent renewals as a sane option?
func (ism *IdentityStreamManager) RenewAll() {
	ism.mu.Lock()
	defer ism.mu.Unlock()
	for is := range ism.identityStreams {
		is.ForceRenew()
	}
}

func (ism *IdentityStreamManager) Run() {
	// TODO: Does this actually even need a Run() lol ?
	// We will if we switch to a worker goroutine model.
}

func (ism *IdentityStreamManager) Close() {
	// Ensure double closure does not cause panic
	select {
	case <-ism.close:
		return
	default:
	}
	close(ism.close)
}

type IdentityRequest struct {
	// Roles are the roles requested for the identity
	Roles []string

	TTL     time.Duration
	Refresh time.Duration
}

type IdentityStream struct {
	owner *IdentityStreamManager
	req   IdentityRequest

	closeCh        chan struct{}
	forceRenewalCh chan struct{}

	Identity chan Identity
}

func (is *IdentityStream) ForceRenew() {
	// If a forced renewal is already queued, queueing another would be
	// redundant.
	select {
	case is.forceRenewalCh <- struct{}{}:
	default:
	}
}

func (is *IdentityStream) Close() {
	// Ensure double closure does not cause panic
	select {
	case <-is.closeCh:
		return
	default:
	}

	close(is.closeCh)
}

func (is *IdentityStream) run() {
	defer func() {
		close(is.Identity)
		is.Close()
		is.owner.wg.Done()
	}()
	ctx := context.Background()

	certs, err := is.owner.fetchIdentity(ctx, is.req)
	if err != nil {
		panic(err) // TODO: Requeue after X
	}
	is.Identity <- Identity{certs: certs}

	// If refresh is disabled, all work is complete.
	if is.req.Refresh == 0 {
		return
	}
	// Main loop for IS
	for {
		func() {
			timer := is.owner.clock.NewTimer(is.req.Refresh)
			defer timer.Stop()

			select {
			case <-is.owner.close:
			case <-is.closeCh:
				// TODO: Should we push an error out to indicate owner has closed
				return
			case <-is.forceRenewalCh:
			case <-timer.Chan():
				// TODO: Timeout context
				certs, err := is.owner.fetchIdentity(ctx, is.req)
				if err != nil {
					panic(err) // TODO: Requeue after X
				}
				is.Identity <- Identity{certs: certs}
			}
		}()
	}
}

type Identity struct {
	certs *proto.Certs
}

func (ism *IdentityStreamManager) StreamIdentity(req IdentityRequest) (*IdentityStream, error) {
	is := &IdentityStream{
		owner:          ism,
		req:            req,
		Identity:       make(chan Identity),
		forceRenewalCh: make(chan struct{}),
		closeCh:        make(chan struct{}),
	}

	ism.wg.Add(1)
	go is.run()

	return is, nil
}

func (ism *IdentityStreamManager) Identity(ctx context.Context, req IdentityRequest) (*Identity, error) {
	req.Refresh = 0 // 0 refresh to indicate not necessary.
	ids, err := ism.StreamIdentity(req)
	if err != nil {
		return nil, err
	}
	defer ids.Close()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case id := <-ids.Identity:
		return &id, nil
	}
}
