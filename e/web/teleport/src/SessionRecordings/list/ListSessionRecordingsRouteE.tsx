import { useMemo } from 'react';

import cfg from 'e-teleport/config';
import { useTeleport } from 'teleport';
import { ListSessionRecordings } from 'teleport/SessionRecordings/list/ListSessionRecordingsRoute';
import { SessionSummariesCta } from 'teleport/SessionRecordings/list/SessionSummariesCta';

import { SessionSummariesStatus } from './SessionSummariesStatus';
import { ViewSummary } from './ViewSummary';

export function ListSessionRecordingsRouteE() {
  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();

  const identitySecurityEnabled = cfg.oss.identitySecurity.licensed;

  const headerSlot = useMemo(() => {
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
