package tbotv2

import (
	"context"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/jonboulle/clockwork"
	"time"
)

type IdentityStreamManager struct {
	stop        chan struct{}
	updateQueue chan *IdentityStream
	client      auth.ClientI

	clock clockwork.Clock
}

func (b *IdentityStreamManager) queueWorker() {
	for {
		select {
		case <-b.stop:
			return
		case ids := <-b.updateQueue:
			ctx := context.TODO() // Enforce timeout action here ?

			privateKey, publicKey, err := native.GenerateKeyPair()
			if err != nil {
				panic(err)
			}

			certs, err := b.client.GenerateUserCerts(
				ctx,
				proto.UserCertsRequest{
					PublicKey:       publicKey,
					Username:        "",
					Expires:         b.clock.Now().Add(ids.req.TTL),
					RouteToCluster:  "",
					UseRoleRequests: true,
					RoleRequests:    ids.req.Roles,
				})
			if err != nil {
				panic(err)
			}
			ids.Stream <- &Identity{
				certs,
			}
		}
	}
}

func (b *IdentityStreamManager) Run() {
	b.queueWorker()
}

func (b *IdentityStreamManager) closeStream(ids *IdentityStream) {
	// does nothing rn
}

type IdentityRequest struct {
	// Roles are the roles requested for the identity
	Roles []string

	TTL     time.Duration
	Refresh time.Duration
}

type IdentityStream struct {
	identityStreamManager *IdentityStreamManager
	req                   IdentityRequest
	Stream                chan *Identity
}

func (ids *IdentityStream) Close() {
	ids.identityStreamManager.closeStream(ids)
}

type Identity struct {
	certs *proto.Certs
}

func (b *IdentityStreamManager) StreamIdentity(req IdentityRequest) (*IdentityStream, error) {
	ids := &IdentityStream{
		identityStreamManager: b,
		req:                   req,
		Stream:                make(chan *Identity),
	}

	b.updateQueue <- ids

	return ids, nil
}

func (b *IdentityStreamManager) Identity(ctx context.Context, req IdentityRequest) (*Identity, error) {
	req.Refresh = 0
	ids, err := b.StreamIdentity(req)
	if err != nil {
		return nil, err
	}
	defer ids.Close()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case id := <-ids.Stream:
		return id, nil
	}
}

type DestinationFile struct {
	IdentityRequest
	Path string
}

// IdentityStreamManager -> IdentityStream -> Formatter -> Destination ??
// Seperation of formatter and destination ?
// Destination - file, memory(?), kubernetes ??
// Formatter - SSH Config, Identity File??
// Will too much seperation/concept of pipeline be confusing ?

// How to handle bots's own identity ? Can IdentityStream mechanism be
// re-used with a join mechanism ? Or can we write a similar, but separate slice
// of code for this.

// BotIdentityManager -> Client -> IdentityStreamManager
//  ^                           -> CAWatcher --^
//  ------------------------------<|
//
// CAWatcher requires valid client, BotIdentityManager requires knowledge
// of CA rotations. This produces a cyclic dependency. CAW must also inform
// ISM of CA rotations so all outstanding IdentityStreams can be generated.

// Do we completely remove the concept of destinations from the core and make
// these a thing assembled by the command/config parser ? How should
// destinations plug with IdentityStream() ?
