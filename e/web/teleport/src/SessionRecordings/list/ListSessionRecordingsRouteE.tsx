import { useMemo } from 'react';

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

  const identitySecurityEnabled = storageService.getAccessGraphEnabled();

  const headerSlot = useMemo(() => {
    if (identitySecurityEnabled && !flags.accessGraph) {
      return null; // the user does not have access to Identity Security
    }

    if (identitySecurityEnabled) {
      return <SessionSummariesStatus />;
    }

    return <SessionSummariesCta />;
  }, [identitySecurityEnabled, flags.accessGraph]);

  return (
    <ListSessionRecordings
      actionComponent={
        cfg.oss.sessionSummarizerEnabled ? ViewSummary : undefined
      }
      headerElement={headerSlot}
    />
  );
}
