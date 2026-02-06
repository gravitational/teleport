package reversetunnel

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
)

// NewWatcher is a function that creates a new types.Watcher.
type NewWatcher func(ctx context.Context, watch types.Watch) (types.Watcher, error)

// SyncValidatedMFAChallenges monitors for ValidatedMFAChallenge resources in the root cluster and replicates them to
// the leaf cluster if they are intended for it. This is needed to support MFA challenges in leaf clusters, as the
// challenges are created in the root cluster but need to be validated in the leaf cluster.
func (s *leafCluster) SyncValidatedMFAChallenges(ctx context.Context, newWatcher NewWatcher, cfg retryutils.LinearConfig) error {
	retry, err := retryutils.NewLinear(cfg)
	if err != nil {
		return trace.Wrap(err, "create retry for SyncValidatedMFAChallenges")
	}

	return trace.Wrap(retry.For(
		ctx,
		func() error {
			watcher, err := newWatcher(
				ctx,
				types.Watch{
					Kinds: []types.WatchKind{
						{
							Kind:   types.KindValidatedMFAChallenge,
							Filter: nil, // TODO: Filter by target. Need to resolve import cycle to use ValidatedMFAChallengeFilter here!
						},
					},
				},
			)
			if err != nil {
				return trace.Wrap(err, "create ValidatedMFAChallenge resource watcher")
			}
			defer watcher.Close()

			// TODO: Initial sync of existing ValidatedMFAChallenge resources to ensure we don't miss any challenges
			// that were created before the watcher was established. This will require pagination and backoff in
			// case of transient errors, similar to the event loop below.

			for {
				select {
				case <-ctx.Done():
					return nil

				case <-watcher.Done():
					return trace.Wrap(watcher.Error(), "watcher closed")

				case event := <-watcher.Events():
					if err := s.handleValidatedMFAChallengeEvent(ctx, event); err != nil {
						return trace.Wrap(err)
					}
				}
			}
		},
	),
	)
}

func (s *leafCluster) handleValidatedMFAChallengeEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpPut:
		chal, ok := event.Resource.(*validatedMFAChallenge)
		if !ok {
			return trace.BadParameter("event resource should have been *mfav1.ValidatedMFAChallenge (this is a bug)")
		}

		if _, err := s.leafClient.MFAServiceClient().ReplicateValidatedMFAChallenge(
			ctx,
			&mfav1.ReplicateValidatedMFAChallengeRequest{
				Name:          chal.Metadata.GetName(),
				Payload:       chal.Spec.GetPayload(),
				SourceCluster: chal.Spec.GetSourceCluster(),
				TargetCluster: chal.Spec.GetTargetCluster(),
				Username:      chal.Spec.GetUsername(),
			},
		); err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}

	default:
		s.logger.DebugContext(ctx, "ignoring unexpected event", "event_type", event.Type)
	}

	return nil
}

// validatedMFAChallenge is a wrapper around the mfav1.ValidatedMFAChallenge type to implement the types.Resource
// interface. Do not export this type outside of this package to avoid confusion.
// XXX: This is a HACK to avoid an import cycle between the api/types and api/gen/proto packages!!!
type validatedMFAChallenge mfav1.ValidatedMFAChallenge

var _ types.Resource = &validatedMFAChallenge{}

func (r *validatedMFAChallenge) GetKind() string {
	return r.Kind
}

func (r *validatedMFAChallenge) GetSubKind() string {
	return r.SubKind
}

func (r *validatedMFAChallenge) SetSubKind(subkind string) {
	r.SubKind = subkind
}

func (r *validatedMFAChallenge) GetVersion() string {
	return r.Version
}

func (r *validatedMFAChallenge) GetName() string {
	return r.Metadata.Name
}

func (r *validatedMFAChallenge) SetName(name string) {
	r.Metadata.Name = name
}

func (r *validatedMFAChallenge) Expiry() time.Time {
	return r.Metadata.Expiry()
}

func (r *validatedMFAChallenge) SetExpiry(t time.Time) {
	r.Metadata.SetExpiry(t)
}

func (r *validatedMFAChallenge) GetMetadata() types.Metadata {
	if r.Metadata == nil {
		r.Metadata = &types.Metadata{}
	}
	return *r.Metadata
}

func (r *validatedMFAChallenge) GetRevision() string {
	return r.Metadata.Revision
}

func (r *validatedMFAChallenge) SetRevision(rev string) {
	r.Metadata.Revision = rev
}
