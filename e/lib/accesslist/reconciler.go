package accesslist

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/services"
)

// AccessListsService describes the subset of services.AccessListsMembers
// required by the reconciler
type AccessListMembers interface {
	UpsertAccessListMember(context.Context, *accesslist.AccessListMember) (*accesslist.AccessListMember, error)
	UpdateAccessListMember(context.Context, *accesslist.AccessListMember) (*accesslist.AccessListMember, error)
	DeleteAccessListMember(ctx context.Context, accessList string, memberName string) error
}

// Static assertion that AccessListMembers is a subset of services.AccessListMembers
var _ AccessListMembers = (services.AccessListMembers)(nil)

// MemberReconcilerConfig holds the configuration parameters for a
// MemberReconciler
type MemberReconcilerConfig struct {
	// AccessListMembers is a mandatory handle to an AccessListMembers CRUD
	// service
	AccessListMembers AccessListMembers

	// Log is an optional logger. A default logger will be created if not set.
	Logger *slog.Logger

	// Matcher is an optional predicate for selecting  resources to reconcile.
	// Defaults to matching all supplied resources
	Matcher services.Matcher[*accesslist.AccessListMember]

	// OnUpsert is an optional event handler to be called after an
	// AccessListMember is successfully created or updated. The MemberReconciler
	// handles  the actual back-end update; you only need supply an event handler
	// if you have further processing to do on an upsert.
	OnUpsert func(ctx context.Context, member *accesslist.AccessListMember) error

	// OnDelete is an optional event handler to be called after an
	// AccessListMember is successfully deleted. The MemberReconciler handles
	// the actual back-end update; you only need supply an event handler if you
	// have further processing to do on a deletion.
	OnDelete func(ctx context.Context, member *accesslist.AccessListMember) error
}

// CheckAndSetDefaults validates the MemberReconcilerConfig values and applies
// any defaults.
func (cfg *MemberReconcilerConfig) CheckAndSetDefaults() error {
	if cfg.AccessListMembers == nil {
		return trace.BadParameter("must provide access list service")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, componentAccessListService)
	}

	if cfg.Matcher == nil {
		cfg.Matcher = func(*accesslist.AccessListMember) bool { return true }
	}

	return nil
}

// MemberReconciler reconciles two lists of AccessListMember records,
// automatically updating the cluster back end as necessary
type MemberReconciler struct {
	accessListMembers AccessListMembers
	backend           *services.Reconciler[*accesslist.AccessListMember]
	logger            *slog.Logger

	// onUpsert is an optional event handler to be invoked after a member record
	// has been created or updated. May be nil.
	onUpsert func(ctx context.Context, member *accesslist.AccessListMember) error

	// onDelete is an optional event handler to be invoked after a member record
	// has been deleted. May be nil.
	onDelete func(ctx context.Context, member *accesslist.AccessListMember) error

	// newMembers holds the list of new member records during the reconciliation.
	// Will only be non-nil during a reconciliation.
	newMembers map[string]*accesslist.AccessListMember

	// currentMembers holds the list of existing member records during the
	// reconciliation. Will only be non-nil during a reconciliation.
	currentMembers map[string]*accesslist.AccessListMember
}

// NewMemberReconciler constructs a new MemberReconciler, wiring up the underlying
// reconciler and data callbacks
func NewMemberReconciler(cfg MemberReconcilerConfig) (*MemberReconciler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	reconciler := &MemberReconciler{
		accessListMembers: cfg.AccessListMembers,
		logger:            cfg.Logger,
		onUpsert:          cfg.OnUpsert,
		onDelete:          cfg.OnDelete,
	}

	var err error
	reconciler.backend, err = services.NewReconciler(services.ReconcilerConfig[*accesslist.AccessListMember]{
		Matcher:             cfg.Matcher,
		GetCurrentResources: reconciler.getCurrentMembers,
		CompareResources: func(alm1, alm2 *accesslist.AccessListMember) int {
			if alm1.Spec.Name == alm2.Spec.Name &&
				alm1.Spec.AccessList == alm2.Spec.AccessList {
				return services.Equal
			}

			return services.Different
		},
		GetNewResources: reconciler.getNewMembers,
		OnCreate:        reconciler.upsertMember,
		OnUpdate:        reconciler.updateMember,
		OnDelete:        reconciler.deleteMember,
		Logger:          cfg.Logger,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return reconciler, nil
}

// Reconcile runs a reconciliation on the supplied member lists, automatically
// updating the cluster AccessLists service wth new, updated or deleted records.
//
// The MemberReconciler makes no effort to be thread safe. It's up to the caller
// to ensure that no-one else is changing the input maps during a reconciliation.
func (r *MemberReconciler) Reconcile(ctx context.Context, newMembers, currentMembers map[string]*accesslist.AccessListMember) error {
	// Stash the input maps where the reconciler's feeder callbacks can find
	// them, making sure to clean up as we exit.
	r.newMembers = newMembers
	r.currentMembers = currentMembers
	defer func() {
		r.newMembers = nil
		r.currentMembers = nil
	}()

	// Run the underlying reconciliation
	if err := r.backend.Reconcile(ctx); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// getNewMembers feeds the new-member data map into the underlying reconciler
func (r *MemberReconciler) getNewMembers() map[string]*accesslist.AccessListMember {
	return r.newMembers
}

// getCurrentMembers feeds the current-member data map into the underlying reconciler
func (r *MemberReconciler) getCurrentMembers() map[string]*accesslist.AccessListMember {
	return r.currentMembers
}

// updateMember is invoked by the reconciler when an AccessListMember has been
// modified and needs to be updated
func (r *MemberReconciler) updateMember(ctx context.Context, member, _ *accesslist.AccessListMember) error {
	return r.upsertMember(ctx, member)
}

// upsertMember is invoked by the reconciler, either directly when an AccessListMember
// has been created, or indirectly when a AccessListMember has been updated.
// Updates the cluster AccessLists service with the new record.
func (r *MemberReconciler) upsertMember(ctx context.Context, member *accesslist.AccessListMember) error {
	if _, err := r.accessListMembers.UpsertAccessListMember(ctx, member); err != nil {
		return trace.Wrap(err)
	}

	if r.onUpsert != nil {
		if err := r.onUpsert(ctx, member); err != nil {
			return trace.Wrap(err, "invoking OnUpsert event handler")
		}
	}
	return nil
}

// deleteMember is invoked by the underlying reconciler when an AccessListMember
// record needs to be deleted.
func (r *MemberReconciler) deleteMember(ctx context.Context, member *accesslist.AccessListMember) error {
	// If the enclosing AccessList has been deleted it is possible that we will
	// try to delete a membership that no longer exists.  If we're attempting to
	// delete something that can't be found, then we'll consider that a success.
	if err := r.accessListMembers.DeleteAccessListMember(ctx, member.Spec.AccessList, member.GetName()); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	if r.onDelete != nil {
		if err := r.onDelete(ctx, member); err != nil {
			return trace.Wrap(err, "invoking OnDelete event handler")
		}
	}

	return nil
}
