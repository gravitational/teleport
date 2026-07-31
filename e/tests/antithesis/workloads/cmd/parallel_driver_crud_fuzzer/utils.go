package main

import (
	"context"
	"log/slog"
	"maps"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/crud"
	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/eventually"
	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/testenv"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	tctlclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

func newClient(ctx context.Context, identity string) (authclient.ClientI, error) {
	ccf := tctlcfg.GlobalCLIFlags{
		AuthServerAddr:   []string{testenv.ProxyAddr()},
		IdentityFilePath: testenv.IdentityPath(identity),
	}
	cfg := servicecfg.MakeDefaultConfig()

	// ignore the clean up func and close the client directly.
	clt, _, err := tctlclient.GetInitFunc(ccf, cfg)(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "creating client")
	}
	return clt, nil
}

func setupOps(ctx context.Context, params *TestCaseParams) (adminOps, ops crud.KindResourceOps, cleanup func(context.Context), err error) {
	adminClt, err := newClient(ctx, adminIdentity)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err, "creating admin client")
	}

	clt, err := newClient(ctx, params.Identity)
	if err != nil {
		adminClt.Close()
		return nil, nil, nil, trace.Wrap(err, "creating client")
	}

	cleanup = func(ctx context.Context) {
		clt.Close()
		adminClt.Close()
	}

	adminOps, err = crud.OpsForKind(adminClt, params.Kind)
	if err != nil {
		cleanup(ctx)
		return nil, nil, nil, trace.Wrap(err, "getting ops for admin")
	}

	ops, err = crud.OpsForKind(clt, params.Kind)
	if err != nil {
		cleanup(ctx)
		return nil, nil, nil, trace.Wrap(err, "getting ops for client")
	}

	return adminOps, ops, cleanup, nil
}

// setupResource is a helper to create a resource for the test case to operate on.
func setupResource(ctx context.Context, ops crud.KindResourceOps, labels map[string]string) (types.Resource153, error) {
	resource, err := ops.NewResource(fuzzerPrefix + "-" + uuid.New().String())
	if err != nil {
		return nil, trace.Wrap(err, "creating resource")
	}
	for key, value := range labels {
		if err := crud.SetStaticLabel(resource, key, value); err != nil {
			return nil, trace.Wrap(err, "setting label")
		}
	}

	if _, err = ops.Create(ctx, resource); err != nil {
		return nil, trace.Wrap(err, "creating fixture")
	}

	err = eventually.Assert(ctx, eventually.AssertParams{
		Message: "Reading a written resources",
		Timeout: writeReplicationTimeout,
		Details: map[string]any{
			"name": resource.GetMetadata().GetName(),
			"kind": ops.Kind(),
		},
		Condition: func(ctx context.Context, _ eventually.AddDetailFunc) (bool, error) {
			r, err := ops.Get153(ctx, resource.GetMetadata().GetName())
			if err != nil {
				return false, trace.Wrap(err, "reading fixture")
			}

			resource = r // update
			return true, nil
		},
	})

	if err != nil {
		return nil, trace.Wrap(err, "reading written resource")
	}

	return resource, nil
}

func equalResourceAfterServerAssignments(ops crud.KindResourceOps, want, got types.Resource153) bool {
	normalized := normalizeResourceAfterServerAssignments(ops, want, got)
	return ops.Equal(normalized, got)
}

func normalizeResourceAfterServerAssignments(ops crud.KindResourceOps, want, got types.Resource153) types.Resource153 {
	normalized := ops.Clone153(want)
	setServerAssignedMetadata(normalized, got)
	setServerAssignedUserFields(normalized, got)
	return normalized
}

func setServerAssignedMetadata(resource, stored types.Resource153) {
	setServerRevision(resource, stored.GetMetadata().GetRevision())
	setServerOrigin(resource, stored.GetMetadata().GetLabels()[types.OriginLabel])
}

func setServerAssignedUserFields(resource, stored types.Resource153) {
	user, ok := unwrapLegacyUser(resource)
	if !ok {
		return
	}
	storedUser, ok := unwrapLegacyUser(stored)
	if !ok {
		return
	}

	if user.GetCreatedBy().IsEmpty() {
		user.SetCreatedBy(storedUser.GetCreatedBy())
	}
	if user.GetPasswordState() == types.PasswordState_PASSWORD_STATE_UNSPECIFIED {
		user.SetPasswordState(storedUser.GetPasswordState())
	}
	if user.GetWeakestDevice() == types.MFADeviceKind_MFA_DEVICE_KIND_UNSPECIFIED {
		user.SetWeakestDevice(storedUser.GetWeakestDevice())
	}
}

func setServerRevision(resource types.Resource153, revision string) {
	if legacy, ok := unwrapLegacyResource(resource); ok {
		legacy.SetRevision(revision)
		return
	}
	resource.GetMetadata().SetRevision(revision)
}

func setServerOrigin(resource types.Resource153, origin string) {
	if legacy, ok := unwrapLegacyResource(resource); ok {
		labeled, ok := legacy.(types.ResourceWithLabels)
		if !ok {
			return
		}
		labels := labelsWithOrigin(labeled.GetStaticLabels(), origin)
		labeled.SetStaticLabels(labels)
		return
	}

	metadata := resource.GetMetadata()
	metadata.SetLabels(labelsWithOrigin(metadata.GetLabels(), origin))
}

func labelsWithOrigin(labels map[string]string, origin string) map[string]string {
	out := maps.Clone(labels)
	if origin == "" {
		delete(out, types.OriginLabel)
		return out
	}
	if out == nil {
		out = make(map[string]string)
	}
	out[types.OriginLabel] = origin
	return out
}

func unwrapLegacyUser(resource types.Resource153) (types.User, bool) {
	legacy, ok := unwrapLegacyResource(resource)
	if !ok {
		return nil, false
	}
	user, ok := legacy.(types.User)
	return user, ok
}

func unwrapLegacyResource(resource types.Resource153) (types.Resource, bool) {
	unwrapper, ok := resource.(interface{ UnwrapT() types.Resource })
	if !ok {
		return nil, false
	}
	return unwrapper.UnwrapT(), true
}

func cleanupResource(ctx context.Context, ops crud.KindResourceOps, resource types.Resource153) {
	if err := ops.Delete(ctx, resource.GetMetadata().GetName()); err != nil && !trace.IsNotFound(err) {
		slog.WarnContext(ctx, "failed to clean up CRUD resource",
			"kind", ops.Kind(),
			"name", resource.GetMetadata().GetName(),
			"error", err)
	}
}
