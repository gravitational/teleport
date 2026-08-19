import { SortItem } from 'shared/components/Controls/SortMenu';
import { useToastNotifications } from 'shared/components/ToastNotification';

import {
  Beam,
  BeamsSortField,
  ComputeStatus,
} from 'e-teleport/services/beams/types';

type Toaster = ReturnType<typeof useToastNotifications>;

export const PAGE_SIZE = 20;

export type SortDir = 'ASC' | 'DESC';

const VALID_SORT_FIELDS: readonly BeamsSortField[] = [
  'name',
  'alias',
  'user',
  'expires',
];

export function coerceSortField(raw: string): BeamsSortField | null {
  return VALID_SORT_FIELDS.find(f => f === raw) ?? null;
}

export const BEAM_SORT_ITEMS: SortItem[] = [
  {
    key: 'alias',
    label: 'Beam',
    ascendingLabel: 'Beam, A - Z',
    descendingLabel: 'Beam, Z - A',
    ascendingOptionLabel: 'Alphabetical, A - Z',
    descendingOptionLabel: 'Alphabetical, Z - A',
    defaultOrder: 'ASC',
  },
  {
    key: 'user',
    label: 'Created by',
    ascendingLabel: 'Created by, A - Z',
    descendingLabel: 'Created by, Z - A',
    ascendingOptionLabel: 'Alphabetical, A - Z',
    descendingOptionLabel: 'Alphabetical, Z - A',
    defaultOrder: 'ASC',
  },
  {
    key: 'expires',
    label: 'Expiration',
    ascendingLabel: 'Expiration, Soonest',
    descendingLabel: 'Expiration, Latest',
    ascendingOptionLabel: 'Soonest',
    descendingOptionLabel: 'Latest',
    defaultOrder: 'ASC',
  },
];

const sortLabelByKey = new Map(BEAM_SORT_ITEMS.map(i => [i.key, i.label]));

export function beamSortLabel(key: string): string {
  return sortLabelByKey.get(key) ?? key;
}

export function resolveBeamName(beam: Beam): string {
  return beam.alias || beam.name;
}

const PROVISION_COMPLETE: ComputeStatus = 'provision_complete';

// The SSH login used to connect to a beam.
export const BEAM_SSH_LOGIN = 'beams';

// A beam is provisioning until both its compute status reports complete and
// the node id has been populated. Connect/publish/open are gated on this.
export function isProvisioning(beam: Beam): boolean {
  return beam.compute_status !== PROVISION_COMPLETE || !beam.node_id;
}

export function isBeamOwner(beam: Beam, currentUsername: string): boolean {
  return beam.user === currentUsername;
}

export function notifyError(toast: Toaster, title: string, err: unknown): void {
  toast.add({
    severity: 'error',
    content: {
      title,
      description:
        err instanceof Error && err.message ? err.message : 'Please try again.',
    },
  });
}

// Builds the public URL Teleport assigns to a published beam app.
// URL parsing normalises the host so the default https port (443) is stripped
// while non-default ports (e.g. 3080 on dev clusters) are preserved.
export function publishedBeamUrl(beam: Beam, clusterPublicUrl: string): string {
  if (!beam.app_name || !beam.publish) return '';
  const host = new URL(`https://${clusterPublicUrl}`).host;
  if (beam.publish.protocol === 'tcp') {
    return `tcp://${beam.app_name}.${host}:${beam.publish.port}`;
  }
  return `https://${beam.app_name}.${host}`;
}
