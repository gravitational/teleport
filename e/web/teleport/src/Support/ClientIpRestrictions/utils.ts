import {
  ClientIpRestriction,
  SaveClientIpRestrictionRequest,
} from 'e-teleport/services/clientiprestrictions';
import { ApiError } from 'teleport/services/api/parseError';

export type CirUiState =
  | 'notConfigured'
  | 'draft'
  | 'pending'
  | 'returningToDraft'
  | 'testRunApplying'
  | 'testRunActive'
  | 'testRunEnding'
  | 'expired'
  | 'active'
  | 'unknown';

const NIL_REVISION = '00000000-0000-0000-0000-000000000000';

export function writeRevision(revision: string | undefined): string {
  return !revision || revision === NIL_REVISION ? '' : revision;
}

export const TEST_RUN_DURATION_MS = 30 * 60 * 1000;

export const POLL_INTERVAL_SETTLING_MS = 5_000;
export const POLL_INTERVAL_STABLE_MS = 30_000;

/**
 * deriveUiState maps the raw resource ({status, mode, expires}) to the UI state.
 *
 * The server never rewrites the spec, so `expires` only means a test run is
 * running while it is still ahead.
 */
export function deriveUiState(
  cir: ClientIpRestriction | undefined,
  now: number = Date.now()
): CirUiState {
  if (!cir) {
    return 'unknown';
  }
  const hasExpiry = !!cir.expires;
  const testRunLive = getRemainingMs(cir.expires, now) > 0;
  switch (cir.status) {
    case 'draft':
      return 'draft';
    case 'expired':
      return 'expired';
    case 'pending':
      if (cir.mode === 'draft') {
        return 'returningToDraft';
      }
      return testRunLive
        ? 'testRunApplying'
        : hasExpiry
          ? 'testRunEnding'
          : 'pending';
    case 'active':
      return testRunLive
        ? 'testRunActive'
        : hasExpiry
          ? 'testRunEnding'
          : 'active';
    // Statuses were migrated, so anything outside the four above is either a
    // resource that was never saved or a server we cannot read.
    default:
      return isNotConfigured(cir) ? 'notConfigured' : 'unknown';
  }
}

/** What Cloud returns for a tenant where no allowlist was ever saved. */
function isNotConfigured(cir: ClientIpRestriction): boolean {
  const noStatus = !cir.status || cir.status === 'unknown';
  return noStatus && !cir.mode && !cir.expires && cir.cidrs.length === 0;
}

export function isSettling(state: CirUiState): boolean {
  return (
    state === 'pending' ||
    state === 'returningToDraft' ||
    state === 'testRunApplying' ||
    state === 'testRunActive' ||
    state === 'testRunEnding'
  );
}

export function pollIntervalFor(
  state: CirUiState,
  intervals: { settlingMs: number; stableMs: number }
): number {
  return isSettling(state) ? intervals.settlingMs : intervals.stableMs;
}

export function getRemainingMs(
  expires: string | undefined,
  now: number = Date.now()
): number {
  if (!expires) {
    return 0;
  }
  const end = parseExpiresMs(expires);
  if (Number.isNaN(end)) {
    return 0;
  }
  return Math.max(0, end - now);
}

/** `expires` is always UTC; without a zone JavaScript would read it as local. */
function parseExpiresMs(expires: string): number {
  const hasZone = /(?:Z|[+-]\d{2}:?\d{2})$/i.test(expires);
  return new Date(hasZone ? expires : `${expires}Z`).getTime();
}

// Writes fully replace the resource, so every payload carries the whole CIDR list
// and the last-seen revision.

export function enforcePayload(
  cidrs: string[],
  revision: string
): SaveClientIpRestrictionRequest {
  return { cidrs, mode: 'enforced', revision };
}

export function testRunPayload(
  cidrs: string[],
  revision: string,
  now: number = Date.now()
): SaveClientIpRestrictionRequest {
  return {
    cidrs,
    mode: 'enforced',
    expires: new Date(now + TEST_RUN_DURATION_MS).toISOString(),
    revision,
  };
}

export function draftPayload(
  cidrs: string[],
  revision: string
): SaveClientIpRestrictionRequest {
  return { cidrs, mode: 'draft', revision };
}

/** What the server sends when the revision is stale: CompareFailed, so a 412. */
const STALE_REVISION_STATUS = 412;

export const STALE_REVISION_MESSAGE =
  'Someone else changed the allowlist. It has been refreshed, so review your changes and try again.';

/**
 * The message to show for a failed write. A stale revision is expected in a panel
 * that polls, and the write is already followed by a refetch, so it gets copy of
 * its own instead of the server's "created|modified|deleted" wording.
 */
export function writeErrorMessage(err: Error): string {
  const stale =
    err instanceof ApiError && err.response?.status === STALE_REVISION_STATUS;
  return stale ? STALE_REVISION_MESSAGE : err.message || 'Please try again.';
}
