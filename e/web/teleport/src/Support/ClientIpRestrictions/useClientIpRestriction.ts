import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { useToastNotifications } from 'shared/components/ToastNotification';

import { SaveClientIpRestrictionRequest } from 'e-teleport/services/clientiprestrictions';
import useTeleportE from 'e-teleport/useTeleportE';

import {
  CirUiState,
  deriveUiState,
  draftPayload,
  enforcePayload,
  pollIntervalFor,
  POLL_INTERVAL_SETTLING_MS,
  POLL_INTERVAL_STABLE_MS,
  STALE_REVISION_MESSAGE,
  testRunPayload,
  writeErrorMessage,
  writeRevision,
} from './utils';

const queryKeyFor = (clusterId: string) =>
  ['clientIpRestriction', clusterId] as const;

/**
 * useClientIpRestriction is the data layer for the IP allowlist panel: it reads
 * and polls the resource, derives the UI state, and provides the write actions.
 *
 * Each action takes the message to toast on success and resolves to whether the
 * write landed, so a caller can keep an editor or a dialog open when it did not.
 * `now` lets the panel re-derive when a test-run deadline elapses.
 */
export function useClientIpRestriction(clusterId: string, now?: number) {
  const ctx = useTeleportE();
  const service = ctx.clientIpRestrictionsService;
  const access = ctx.storeUser.geClientIpRestrictionAccess();
  const queryClient = useQueryClient();
  const toast = useToastNotifications();
  const queryKey = queryKeyFor(clusterId);

  const query = useQuery({
    queryKey,
    queryFn: () => service.fetchClientIpRestriction(clusterId),
    enabled: access.read,
    refetchInterval: ({ state }) =>
      pollIntervalFor(deriveUiState(state.data), {
        settlingMs: POLL_INTERVAL_SETTLING_MS,
        stableMs: POLL_INTERVAL_STABLE_MS,
      }),
  });

  const cir = query.data;
  const uiState: CirUiState = deriveUiState(cir, now);

  const save = useMutation({
    mutationFn: ({
      req,
    }: {
      req: SaveClientIpRestrictionRequest;
      message: string;
      /** Set when the caller renders the failure itself, to avoid saying it twice. */
      quietError?: boolean;
    }) => service.saveClientIpRestriction(clusterId, req),
    onSuccess: (updated, { message }) => {
      queryClient.setQueryData(queryKey, updated);
      toast.add({ severity: 'success', content: message });
      // Refetch to pick up server-derived transitions (e.g. pending -> active).
      return queryClient.invalidateQueries({ queryKey });
    },
    onError: (err, { quietError }) => {
      if (!quietError) {
        toast.add({
          severity: 'error',
          content: {
            title: 'Failed to update the IP allowlist',
            description: writeErrorMessage(err),
          },
        });
      }
      // Resync after a failure, e.g. a stale-revision conflict.
      return queryClient.invalidateQueries({ queryKey });
    },
  });

  const revision = writeRevision(cir?.revision);
  const currentCidrs = cir?.cidrs ?? [];

  // Resolves to false rather than rejecting; the error is already a toast.
  const write = (
    req: SaveClientIpRestrictionRequest,
    message: string,
    opts: { quietError?: boolean } = {}
  ) => {
    // An empty revision would upsert unguarded, so if a poll returned
    // a CIR after editing began, conflict instead of overwriting it.
    if (req.revision === '' && revision !== '') {
      toast.add({
        severity: 'error',
        content: {
          title: 'Failed to update the IP allowlist',
          description: STALE_REVISION_MESSAGE,
        },
      });
      return Promise.resolve(false);
    }
    return save
      .mutateAsync({ req, message, quietError: opts.quietError })
      .then(() => true)
      .catch(() => false);
  };

  const actions = {
    // apply and saveDraft take the revision the edited list was based on, so a
    // save from an editor opened before a background poll conflicts (412) instead
    // of silently replacing what someone else wrote meanwhile. The other actions
    // act on the latest polled resource, so the latest revision is the right one.
    apply: (
      cidrs: string[] = currentCidrs,
      message = 'Enforcement started',
      baseRevision: string | undefined = cir?.revision
    ) => write(enforcePayload(cidrs, writeRevision(baseRevision)), message),
    startTestRun: (
      cidrs: string[] = currentCidrs,
      message = 'Test run started'
    ) => write(testRunPayload(cidrs, revision), message),
    confirm: (message = 'Test run confirmed') =>
      write(enforcePayload(currentCidrs, revision), message),
    // Cancel and Deactivate are the same write; separate so the UI can label them.
    cancel: (message = 'Returning the allowlist to draft') =>
      write(draftPayload(currentCidrs, revision), message),
    // The dialog shows the failure inline, so a toast would say it twice.
    deactivate: (message = 'Returning the allowlist to draft') =>
      write(draftPayload(currentCidrs, revision), message, {
        quietError: true,
      }),
    saveDraft: (
      cidrs: string[] = currentCidrs,
      message = 'Draft saved',
      baseRevision: string | undefined = cir?.revision
    ) => write(draftPayload(cidrs, writeRevision(baseRevision)), message),
  };

  return {
    access,
    cir,
    uiState,
    isLoading: query.isLoading,
    error: query.error,
    refetch: query.refetch,
    saving: save.isPending,
    // Outlives the write that produced it, so a renderer must clear it on open.
    saveError: save.error,
    clearSaveError: save.reset,
    actions,
  };
}
