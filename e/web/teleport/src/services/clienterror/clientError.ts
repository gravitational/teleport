import cfg from 'e-teleport/config';
import api from 'teleport/services/api';

/** ClientErrorComponent identifies which part of the UI is reporting. */
export type ClientErrorComponent = 'cloud-panel';

/** ClientErrorSource categorizes the kind of failure being reported. */
export type ClientErrorSource = 'network' | 'render';

// FetchError marks a failure to fetch/load a resource as opposed to an error thrown
// while using/rendering something that already loaded successfully
export class FetchError extends Error {}

// Dedup state suppresses repeats of the identical error within the same
// window, so a burst of the same failure doesn't spam our logs.
let lastReportedFingerprint: string | null = null;
let lastReportedAt = 0;

// Throttle state shared across every caller of reportClientError regardless
// of component. This is to avoid spamming the backend with errors.
export const REPORT_WINDOW_MS = 60000;
const REPORT_MAX_CALLS = 5;
let reportTimestamps: number[] = [];

/**
 * reportClientError reports a client-side web UI error to this cluster's
 * proxy, so it shows up in the proxy's own logs.
 * - `component` identifies which part of the UI is reporting
 * - `errorSource` categorizes the failure ("network" or "render")
 * - `errorSignature` identifies the specific error for local deduplication
 */
export function reportClientError(
  component: ClientErrorComponent,
  errorSource: ClientErrorSource,
  errorSignature: string
) {
  if (!cfg.oss.isCloud && !cfg.oss.isUsageBasedBilling) {
    return;
  }

  const now = Date.now();
  const fingerprint = `${component}:${errorSource}:${errorSignature}`;

  if (
    fingerprint === lastReportedFingerprint &&
    now - lastReportedAt < REPORT_WINDOW_MS
  ) {
    return;
  }

  reportTimestamps = reportTimestamps.filter(t => now - t < REPORT_WINDOW_MS);
  if (reportTimestamps.length >= REPORT_MAX_CALLS) {
    return;
  }
  reportTimestamps.push(now);
  lastReportedFingerprint = fingerprint;
  lastReportedAt = now;

  void api.fetch(cfg.api.logClientErrorPath, {
    method: 'POST',
    body: JSON.stringify({ component, error_source: errorSource }),
  });
}
