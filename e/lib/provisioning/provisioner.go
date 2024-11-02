package provisioning

import (
	"context"
	"iter"
	"log/slog"

	scimSchema "github.com/elimity-com/scim/schema"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type ExternalIDGetter interface {
	GetExternalID(context.Context, services.ProvisioningStateID) (ExternalID, error)
}

type resourceType struct {
	name       string
	pathSuffix string
	schemas    []string
}

type externalIDUpdateHandler func(context.Context, *provisioningv1.PrincipalState)

// provisioner is the actual process that attempts to make the downstream consumer
// match the resource
type provisioner struct {
	log                 *slog.Logger
	stateSvc            services.DownstreamProvisioningStates
	externalIDCache     ExternalIDGetter
	usersSvc            UsersService
	accessListSvc       AccessListsService
	locksSvc            services.LockGetter
	clock               clockwork.Clock
	scimClient          scimsdk.Client
	resourceTypes       utils.SyncMap[provisioningv1.PrincipalType, resourceType]
	maxConcurrency      int
	onExternalIDUpdated externalIDUpdateHandler
}

type provisionerConfig struct {
	log            *slog.Logger
	stateSvc       services.DownstreamProvisioningStates
	usersSvc       UsersService
	accessListsSvc AccessListsService
	locksSvc       services.LockGetter
	scimClient     scimsdk.Client
	clock          clockwork.Clock

	// maxConcurrency defines the maximum number of provisioning operations that
	// can happen concurrently.
	maxConcurrency int

	onExternalIDUpdated externalIDUpdateHandler
}

func (cfg *provisionerConfig) CheckAndSetDefaults() error {
	if cfg.stateSvc == nil {
		return trace.BadParameter("must supply provisioning state service")
	}
	if cfg.usersSvc == nil {
		return trace.BadParameter("must supply Users service")
	}
	if cfg.accessListsSvc == nil {
		return trace.BadParameter("must supply Access Lists service")
	}
	if cfg.locksSvc == nil {
		return trace.BadParameter("must supply Locks service")
	}
	if cfg.scimClient == nil {
		return trace.BadParameter("must supply configured scim client")
	}
	if cfg.clock == nil {
		cfg.clock = clockwork.NewRealClock()
	}
	if cfg.log == nil {
		cfg.log = slog.With(teleport.ComponentKey, provisioningComponent)
	}
	if cfg.maxConcurrency == 0 {
		cfg.maxConcurrency = defaultProvisioningConcurrency
	}
	if cfg.onExternalIDUpdated == nil {
		cfg.onExternalIDUpdated = func(context.Context, *provisioningv1.PrincipalState) {}
	}
	return nil
}

func newProvisioner(cfg provisionerConfig) (*provisioner, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	p := &provisioner{
		log:            cfg.log,
		clock:          cfg.clock,
		stateSvc:       cfg.stateSvc,
		usersSvc:       cfg.usersSvc,
		accessListSvc:  cfg.accessListsSvc,
		locksSvc:       cfg.locksSvc,
		scimClient:     cfg.scimClient,
		maxConcurrency: cfg.maxConcurrency,
	}

	// TODO(tcsc): query the /Resources SCIM end point and unpack into here
	//             rather than hardcoding. See `refreshResourceTypes()`.
	p.resourceTypes.Set(map[provisioningv1.PrincipalType]resourceType{
		provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER: {
			name:       scimsdk.ResourceTypeUser,
			pathSuffix: "Users",
			schemas:    []string{scimSchema.UserSchema},
		},
		provisioningv1.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST: {
			name:       scimsdk.ResourceTypeGroup,
			pathSuffix: "Groups",
			schemas:    []string{scimSchema.GroupSchema},
		},
	})

	return p, nil
}

// Provision provisions an arbitrary principal into the configured downstream
// service. Takes care of updating the resource records and suchlike internally.
func (p *provisioner) Provision(ctx context.Context, state *provisioningv1.PrincipalState) error {
	log := p.log.With(principalStateAttr(state))
	log.DebugContext(ctx, "Provisioning", "state", state.Status.ProvisioningState)

	switch state.Status.ProvisioningState {
	case provisioningv1.ProvisioningState_PROVISIONING_STATE_STALE:
		var provisioningErr error

		switch state.Spec.PrincipalType {
		case provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER:
			_, provisioningErr = p.provisionUser(ctx, state)

		case provisioningv1.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST:
			_, provisioningErr = p.provisionAccessList(ctx, state)

		default:
			return trace.BadParameter("Unsupported principal type %v", state.Spec.PrincipalType)
		}

		if provisioningErr != nil {
			_, err := markStateInError(ctx, p.stateSvc, state, provisioningErr, log)
			return trace.Wrap(err)
		}

		return trace.Wrap(provisioningErr, "provisioning principal")

	case provisioningv1.ProvisioningState_PROVISIONING_STATE_DELETED:
		err := p.deprovisionPrincipal(ctx, state, log)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := p.stateSvc.DeleteProvisioningState(ctx, getDownstreamID(state), getID(state)); err != nil {
			return trace.Wrap(err, "deleting provisioning state")
		}
		return nil

	case provisioningv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED:
		return nil

	default:
		return trace.BadParameter("unexpected provisioning status: %v", state.Status.ProvisioningState)
	}
}

// ProvisionAll concurrently provisions a number of principals into the
// downstream system. A provisioning failure in any given principal will be
// logged, but not effect the provisioning of other principals in the batch
func (p *provisioner) ProvisionAll(ctx context.Context, states iter.Seq[*provisioningv1.PrincipalState]) error {
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(p.maxConcurrency)

	for s := range states {
		group.Go(func() error {
			if err := p.Provision(groupCtx, s); err != nil {
				p.log.WarnContext(groupCtx, "Failed provisioning resource",
					"principal_type", s.Spec.PrincipalType,
					"principal_id", s.Spec.PrincipalId,
					"error", err)
			}
			return nil
		})
	}

	group.Wait()

	return nil
}

func (p *provisioner) deprovisionPrincipal(ctx context.Context, state *provisioningv1.PrincipalState, log *slog.Logger) error {
	log.InfoContext(ctx, "Deprovisioning principal")

	// If the record was never actually provisioned...
	if state.Status.ExternalId == "" {
		log.DebugContext(ctx, "Principal was never provisioned")
		return nil
	}

	var err error
	switch state.Spec.PrincipalType {
	case provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER:
		err = p.scimClient.DeleteUser(ctx, state.Status.ExternalId)

	case provisioningv1.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST:
		err = p.scimClient.DeleteGroup(ctx, state.Status.ExternalId)

	default:
		return trace.BadParameter("unsupported principal type: %s", state.Spec.PrincipalType)
	}

	return trace.Wrap(err, "deprovisioning principal")
}

func getAccessListWithMembers(ctx context.Context, name string, aclSvc AccessListsService) (*accesslist.AccessList, []*accesslist.AccessListMember, error) {
	// TODO(tcsc): handle nested access lists
	acl, err := aclSvc.GetAccessList(ctx, name)
	if err != nil {
		return nil, nil, trace.Wrap(err, "fetching ACL for provisioning")
	}

	var allMembers []*accesslist.AccessListMember
	var pageToken string

	for {
		members, nextPage, err := aclSvc.ListAccessListMembers(ctx, name, defaultUserPageSize, pageToken)
		if err != nil {
			return nil, nil, trace.Wrap(err, "fetching access list members for provisioning")
		}

		allMembers = append(allMembers, members...)

		if nextPage == "" {
			break
		}

		pageToken = nextPage
	}

	return acl, allMembers, nil
}
