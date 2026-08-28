import { ensureError } from 'shared/utils/error';

import {
  AccessList,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';
import { getPresetRolesFromMetadataLabel } from 'e-teleport/services/accessmanagement/accessmanagement';
import { ApiError } from 'teleport/services/api/parseError';
import { MfaChallengeResponse } from 'teleport/services/mfa';
import ResourceService from 'teleport/services/resources';

// PendingCleanup describes which resources still need deleting.
export type PendingCleanup = {
  /* access list needs deleting */
  accessListId?: string;
  /* roles related to preset access list needs deleting */
  roles?: string[];
  /* any non "not found" error when trying to delete these resources */
  error?: string;
  /* access list was not visible yet, requires retrying to fetch access list */
  verifyAccessListExists?: boolean;
};

function isNotFoundErr(err: unknown) {
  return err instanceof ApiError && err.response.status === 404;
}

function isAccessListAlreadyExistsErr(err: unknown) {
  return (
    err instanceof ApiError &&
    err.response.status === 409 &&
    err.message.includes('access list') &&
    err.message.includes('already exists')
  );
}

// cleanupPresetAcl will delete all resources tied to a "maybe"
// existing access list by newAccessListId. Existing resources
// can exist from a backend partial failure when trying to create a preset
// access list (e.g. access list itself, and any supporting roles
// Teleport tried to auto create for the list)
export async function cleanupPresetAcl(
  newAccessListId: string,
  originalErr: Error,
  mfaResponse: MfaChallengeResponse,
  updateRequiresCleanup: (newCleanup: PendingCleanup) => void
): Promise<PendingCleanup | null> {
  if (isAccessListAlreadyExistsErr(originalErr)) {
    throw originalErr;
  }

  const pendingCleanup = await fetchAndDeletePresetAcl(
    newAccessListId,
    mfaResponse
  );
  if (pendingCleanup) {
    updateRequiresCleanup(pendingCleanup);
  }
  throw originalErr;
}

// tryPresetAclCleanup will delete any remaining resources tied
// to an access list. Remaining resources can exist if initial
// attempt by "cleanupPresetAcl" failed for some reason.
export async function tryPresetAclCleanup(
  newAccessListId: string,
  pendingCleanup: PendingCleanup,
  mfaResponse: MfaChallengeResponse
): Promise<PendingCleanup | null> {
  if (pendingCleanup?.verifyAccessListExists) {
    return fetchAndDeletePresetAcl(newAccessListId, mfaResponse, {
      notFoundMeansDone: true,
    });
  }
  if (pendingCleanup?.accessListId) {
    return deletePresetAclResources({
      mfaResponse,
      accessListToDelete: pendingCleanup.accessListId,
      rolesToDelete: pendingCleanup.roles,
    });
  }
  if (pendingCleanup?.roles?.length > 0) {
    return deletePresetAclResources({
      mfaResponse,
      accessListToDelete: undefined,
      rolesToDelete: pendingCleanup.roles,
    });
  }
  if (pendingCleanup) {
    return fetchAndDeletePresetAcl(newAccessListId, mfaResponse);
  }
  return null; // nothing to do
}

// fetchAndDeleteAcl will ensure the access list to be deleted
// exists before trying to delete it and any of its supporting
// roles.
async function fetchAndDeletePresetAcl(
  newAccessListId: string,
  mfaResponse: MfaChallengeResponse,
  opts?: { notFoundMeansDone?: boolean }
): Promise<PendingCleanup | null> {
  let fetchedAcl: AccessList;
  try {
    fetchedAcl = await accessManagementService.fetchAccessList(newAccessListId);
  } catch (fetchErr) {
    if (isNotFoundErr(fetchErr)) {
      // This can be a false 404 (when trying initial cleanup) since backend
      // fetches access list from cache. Keep cleanup pending so retry can
      // verify again before declaring cleanup complete.
      return opts?.notFoundMeansDone ? null : { verifyAccessListExists: true };
    }
    // Couldn't verify if access list was already created, so cleanup
    // can't be performed at this time.
    const ensuredError = ensureError(fetchErr);
    return { error: ensuredError.message, accessListId: newAccessListId };
  }
  return deletePresetAclResources({
    mfaResponse,
    accessListToDelete: fetchedAcl.id,
    rolesToDelete: getPresetRolesFromMetadataLabel(fetchedAcl.metadata.labels),
  });
}

async function deletePresetAclResources({
  mfaResponse,
  accessListToDelete,
  rolesToDelete = [],
}: {
  mfaResponse: MfaChallengeResponse;
  accessListToDelete?: string;
  rolesToDelete?: string[];
}): Promise<PendingCleanup | null> {
  if (accessListToDelete) {
    // Delete the access list.
    // Its success response will return any roles that may need to be deleted.
    try {
      rolesToDelete = await accessManagementService.deleteAccessListWithPreset(
        accessListToDelete,
        mfaResponse
      );
    } catch (err) {
      if (isNotFoundErr(err)) {
        accessListToDelete = undefined;
      } else {
        const ensuredError = ensureError(err);
        return {
          accessListId: accessListToDelete,
          error: ensuredError.message,
          roles: rolesToDelete,
        };
      }
    }
  }

  // Try deleting all roles, recording failed
  // attempts only if it wasn't a not found error.
  const failedRoles: string[] = [];
  let deleteError; // Only store one of the error.
  if (rolesToDelete?.length > 0) {
    const resourceSvc = new ResourceService();
    for (const role of rolesToDelete) {
      try {
        await resourceSvc.deleteRole(role, mfaResponse);
      } catch (roleDeleteErr) {
        if (!isNotFoundErr(roleDeleteErr)) {
          const ensuredError = ensureError(roleDeleteErr);
          deleteError = ensuredError.message;
          failedRoles.push(role);
        }
      }
    }
  }

  if (failedRoles.length > 0) {
    return { roles: failedRoles, error: deleteError };
  }

  return null; // cleanup finished
}
