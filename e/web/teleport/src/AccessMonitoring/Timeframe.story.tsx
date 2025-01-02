import { useEffect } from 'react';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { Timeframe } from './Timeframe';

const ctx = createTeleportContext();

const defaultIsEnterprise = cfg.isEnterprise;
const defaultAccessMonitoringEntitlement = cfg.entitlements.AccessMonitoring;

export default {
  title: 'TeleportE/AccessMonitoring',
  decorators: [
    Story => {
      useEffect(() => {
        cfg.isEnterprise = true;
        // Clean up
        return () => {
          cfg.isEnterprise = defaultIsEnterprise;
          cfg.entitlements.AccessMonitoring =
            defaultAccessMonitoringEntitlement;
        };
      }, []);
      return <Story />;
    },
  ],
};

export const TimeframeDropdownWithLimitAndCta = () => {
  cfg.entitlements.AccessMonitoring = { enabled: true, limit: 30 };
  return (
    <ContextProvider ctx={ctx}>
      <Timeframe onChange={() => null} days={30} />
    </ContextProvider>
  );
};

export const TimeframeDropdownUnlimited = () => {
  cfg.entitlements.AccessMonitoring = { enabled: true, limit: 0 };
  return (
    <ContextProvider ctx={ctx}>
      <Timeframe onChange={() => null} days={30} />
    </ContextProvider>
  );
};
