import { useCallback, useMemo } from 'react';

import cfg from 'e-teleport/config';
import { useTeleport } from 'teleport';
import { storageService } from 'teleport/services/storageService';
import { ListSessionRecordings } from 'teleport/SessionRecordings/list/ListSessionRecordingsRoute';
import { SessionSummariesCta } from 'teleport/SessionRecordings/list/SessionSummariesCta';

import { SessionSummariesStatus } from './SessionSummariesStatus';
import { ViewSummary } from './ViewSummary';

export function ListSessionRecordingsRouteE() {
  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();

  const hasIdentitySecurity =
    storageService.getAccessGraphEnabled() && flags.accessGraph;

  const actionSlot = useCallback(
    (sessionId: string) =>
      cfg.oss.sessionSummarizerEnabled ? (
        <ViewSummary sessionId={sessionId} />
      ) : null,
    []
  );

  const headerSlot = useMemo(() => {
    if (hasIdentitySecurity) {
      return <SessionSummariesStatus />;
    }

    return <SessionSummariesCta />;
  }, [hasIdentitySecurity]);

  return (
    <ListSessionRecordings actionSlot={actionSlot} headerSlot={headerSlot} />
  );
}
