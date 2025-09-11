import { useMemo } from 'react';

import cfg from 'e-teleport/config';
import { RECORDING_TYPES_WITH_SUMMARIES } from 'e-teleport/services/recordings/recordings';
import { useTeleport } from 'teleport';
import type { RecordingType } from 'teleport/services/recordings';
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

  const actionSlot = useMemo(() => {
    if (!cfg.oss.sessionSummarizerEnabled) {
      return;
    }

    return (sessionId: string, type: RecordingType) =>
      RECORDING_TYPES_WITH_SUMMARIES.includes(type) ? (
        <ViewSummary sessionId={sessionId} />
      ) : null;
  }, []);

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
