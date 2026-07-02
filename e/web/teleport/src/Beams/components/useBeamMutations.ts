import {
  QueryClient,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query';
import { useEffect, useRef } from 'react';

import { useToastNotifications } from 'shared/components/ToastNotification';

import { beamsService } from 'e-teleport/services/beams';
import { Beam, BeamPublish } from 'e-teleport/services/beams/types';

import { isProvisioning } from './constants';

// Defaults the backend applies when a publish spec omits them. We send them
// explicitly so the request body matches the response shape.
const DEFAULT_PUBLISH: BeamPublish = { port: 8080, protocol: 'http' };

const REFETCH_DELAYS_MS = [850, 3000];

// Schedules a list refetch at a fixed delay after a mutation succeeds.
function scheduleListRefetch(
  queryClient: QueryClient,
  delay: number,
  shouldFire: () => boolean = () => true
): () => void {
  const id = window.setTimeout(() => {
    if (!shouldFire()) return;
    queryClient.removeQueries({ queryKey: ['beams'], type: 'inactive' });
    queryClient.invalidateQueries({ queryKey: ['beams'] });
  }, delay);
  return () => clearTimeout(id);
}

// Manages a set of in-flight refetch timers. Schedules cancel automatically
// on unmount and on any subsequent schedule call (so rapid mutations don't
// stack up).
function useDelayedListRefetch() {
  const queryClient = useQueryClient();
  const cancelRef = useRef<(() => void) | null>(null);

  useEffect(() => () => cancelRef.current?.(), []);

  return (opts: { retryIf: (beams: Beam[]) => boolean }) => {
    cancelRef.current?.();
    const [first, second] = REFETCH_DELAYS_MS;
    const shouldFireSecond = () => {
      const lists = queryClient.getQueriesData<{ items?: Beam[] }>({
        queryKey: ['beams'],
      });
      const beams = lists.flatMap(([, data]) => data?.items ?? []);
      return opts.retryIf(beams);
    };
    const cancelFirst = scheduleListRefetch(queryClient, first);
    // This second fetch is usually never fired because most mutations only need the first refetch
    // This works as a safety net for slower cache updates.
    const cancelSecond = scheduleListRefetch(
      queryClient,
      second,
      shouldFireSecond
    );
    cancelRef.current = () => {
      cancelFirst();
      cancelSecond();
    };
  };
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
  });
}

// Single mutation path that handles both single and bulk deletes — bulk
// deletes are dispatched in parallel via Promise.all over the existing
// per-beam deleteBeam API. Callers pass a Beam[] with length >= 1.
export function useDeleteBeams(
  clusterId: string,
  opts: { onSuccess?: (beams: Beam[]) => void } = {}
) {
  const toast = useToastNotifications();
  const scheduleRefetches = useDelayedListRefetch();
  return useMutation({
    mutationFn: async (beams: Beam[]) => {
      await Promise.all(
        beams.map(b => beamsService.deleteBeam({ clusterId, name: b.name }))
      );
      return beams;
    },
    onSuccess: beams => {
      const single = beams.length === 1;
      toast.add({
        severity: 'success',
        content: {
          title: single ? 'Removed beam' : `Removed ${beams.length} beams`,
          description: single
            ? `"${beams[0].alias || beams[0].name}" has been deleted.`
            : undefined,
        },
      });
      const names = new Set(beams.map(b => b.name));
      scheduleRefetches({
        retryIf: items => items.some(b => names.has(b.name)),
      });
      opts.onSuccess?.(beams);
    },
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
    mutationFn: () =>
      beamsService.updateBeam(
        { clusterId, name: beam.name },
        { ...beam, publish: DEFAULT_PUBLISH }
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
  });
}
