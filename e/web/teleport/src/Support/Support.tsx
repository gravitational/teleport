import { ScheduledUpgrades } from 'e-teleport/Support/ScheduledUpgrades/ScheduledUpgrades';
import useTeleportE from 'e-teleport/useTeleportE';
import { ExternalAuditStorageCta } from 'teleport/components/ExternalAuditStorageCta';
import cfg from 'teleport/config';
import { Support } from 'teleport/Support';

import { ClientIpRestrictions } from './ClientIpRestrictions';
import { Contacts } from './Contacts';

export default function Container() {
  return (
    <Support>
      <SupportE />
    </Support>
  );
}

export const SupportE = () => {
  const ctx = useTeleportE();

  return (
    <>
      {cfg.isCloud && <ScheduledUpgrades />}
      {(cfg.isCloud || cfg.isDashboard) && (
        <Contacts clusterId={ctx.storeUser.state.cluster.clusterId} />
      )}
      {cfg.isCloud && cfg.entitlements.ClientIPRestrictions.enabled && (
        <ClientIpRestrictions
          clusterId={ctx.storeUser.state.cluster.clusterId}
        />
      )}
      <ExternalAuditStorageCta />
    </>
  );
};
