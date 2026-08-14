import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query';

import { useToastNotifications } from 'shared/components/ToastNotification';

import { beamsService } from 'e-teleport/services/beams';
import {
  Beam,
  BeamsSortField,
  Protocol,
} from 'e-teleport/services/beams/types';

import { isProvisioning, notifyError, SortDir } from './constants';

const PUBLISH_PORT = 8080;

const REFETCH_DELAY_MS = 850;
const SAFETY_NET_DELAY_MS = 3000;

function useDelayedListRefetch() {
  const queryClient = useQueryClient();

  const invalidateList = () => {
    queryClient.removeQueries({ queryKey: ['beams'], type: 'inactive' });
    queryClient.invalidateQueries({ queryKey: ['beams'] });
  };

  // Backend changes take time to appear in the list, so an immediate refetch may
  // return stale data. The first timeout waits for the expected propagation
  // delay. The second acts as a fallback, re-invalidating if the list still
  // hasn't reached the expected state (e.g. provisioning isn't complete or the
  // publish status hasn't updated yet).
  return (opts: { retryIf: (beams: Beam[]) => boolean }) => {
    setTimeout(invalidateList, REFETCH_DELAY_MS);
    setTimeout(() => {
      const lists = queryClient.getQueriesData<{ items?: Beam[] }>({
        queryKey: ['beams'],
      });
      const beams = lists.flatMap(([, data]) => data?.items ?? []);
      if (opts.retryIf(beams)) invalidateList();
    }, SAFETY_NET_DELAY_MS);
  };
}

export function useListBeams({
  clusterId,
  pageSize,
  pageToken,
  sortField,
  sortDir,
  users,
  enabled,
}: {
  clusterId: string;
  pageSize: number;
  pageToken: string;
  sortField: BeamsSortField;
  sortDir: SortDir;
  users: string[] | undefined;
  enabled: boolean;
}) {
  return useQuery({
    enabled,
    queryKey: ['beams', clusterId, pageToken, sortField, sortDir, users],
    queryFn: ({ signal }) =>
      beamsService.listBeams(
        {
          pageSize,
          pageToken,
          sortField,
          sortDir: sortDir === 'DESC' ? 'desc' : 'asc',
          users,
        },
        clusterId,
        signal
      ),
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });
}

export function useCreateBeam(
  clusterId: string,
  opts: { onSuccess?: (beam: Beam) => void } = {}
) {
  const toast = useToastNotifications();
  const scheduleRefetches = useDelayedListRefetch();
  return useMutation({
    mutationFn: () => beamsService.createBeam(clusterId),
    onSuccess: beam => {
      toast.add({
        severity: 'success',
        content: {
          title: 'New beam created',
          description: `Your new beam "${beam.alias || beam.name}" is provisioning and will appear in the list once it is ready.`,
        },
      });
      scheduleRefetches({ retryIf: beams => beams.some(isProvisioning) });
      opts.onSuccess?.(beam);
    },
    onError: err => notifyError(toast, 'Failed to create beam', err),
  });
}

// Single mutation path that handles both single and bulk deletes — bulk
// deletes are dispatched via Promise.allSettled over the existing per-beam
// deleteBeam API. Callers pass a Beam[] with length >= 1. Using allSettled
// ensures partial failures don't discard the results of successful deletes.
export function useDeleteBeams(
  clusterId: string,
  opts: { onSuccess?: (beams: Beam[]) => void } = {}
) {
  const toast = useToastNotifications();
  const scheduleRefetches = useDelayedListRefetch();
  return useMutation({
    mutationFn: async (beams: Beam[]) => {
      const results = await Promise.allSettled(
        beams.map(b => beamsService.deleteBeam({ clusterId, name: b.name }))
      );
      const failed = results.filter(r => r.status === 'rejected');
      if (failed.length === beams.length) {
        throw failed[0].reason;
      }
      if (failed.length > 0) {
        const succeeded = beams.filter(
          (_, i) => results[i].status === 'fulfilled'
        );
        return { succeeded, partialFailureCount: failed.length };
      }
      return { succeeded: beams, partialFailureCount: 0 };
    },
    onSuccess: ({ succeeded, partialFailureCount }) => {
      const single = succeeded.length === 1;
      const title = single
        ? 'Removed beam'
        : `Removed ${succeeded.length} beams`;
      const description = partialFailureCount
        ? `${partialFailureCount} beam(s) could not be deleted.`
        : single
          ? `"${succeeded[0].alias || succeeded[0].name}" has been deleted.`
          : undefined;
      toast.add({
        severity: partialFailureCount ? 'warn' : 'success',
        content: { title, description },
      });
      const names = new Set(succeeded.map(b => b.name));
      scheduleRefetches({
        retryIf: items => items.some(b => names.has(b.name)),
      });
      opts.onSuccess?.(succeeded);
    },
    onError: (err, beams) =>
      notifyError(
        toast,
        beams.length === 1
          ? 'Failed to remove beam'
          : `Failed to remove ${beams.length} beams`,
        err
      ),
  });
}

export function usePublishBeam(
  clusterId: string,
  beam: Beam,
  opts: { onSuccess?: (beam: Beam) => void } = {}
) {
  const toast = useToastNotifications();
  const scheduleRefetches = useDelayedListRefetch();
  return useMutation({
    mutationFn: (protocol: Protocol) =>
      beamsService.updateBeam(
        { clusterId, name: beam.name },
        { ...beam, publish: { port: PUBLISH_PORT, protocol } }
      ),
    onSuccess: updated => {
      toast.add({
        severity: 'success',
        content: {
          title: 'App published',
          description: `"${beam.alias || beam.name}" is now reachable via its app URL.`,
        },
      });
      scheduleRefetches({
        retryIf: beams => {
          const b = beams.find(x => x.name === beam.name);
          return !b || !b.publish;
        },
      });
      opts.onSuccess?.(updated);
    },
    onError: err => notifyError(toast, 'Failed to publish app', err),
  });
}

export function useUnpublishBeam(
  clusterId: string,
  beam: Beam,
  opts: { onSuccess?: () => void } = {}
) {
  const toast = useToastNotifications();
  const scheduleRefetches = useDelayedListRefetch();
  return useMutation({
    mutationFn: () =>
      beamsService.updateBeam(
        { clusterId, name: beam.name },
        { ...beam, publish: undefined }
      ),
    onSuccess: () => {
      toast.add({
        severity: 'success',
        content: { title: 'App unpublished' },
      });
      scheduleRefetches({
        retryIf: beams => {
          const b = beams.find(x => x.name === beam.name);
          return !b || !!b.publish;
        },
      });
      opts.onSuccess?.();
    },
    onError: err => notifyError(toast, 'Failed to unpublish app', err),
  });
}
