import React, { useEffect } from 'react';
import { http, HttpResponse, delay } from 'msw';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import ecfg from 'e-teleport/config';

import { DeviceTrust } from './DeviceTrust';

import type { TrustedDevice } from 'teleport/DeviceTrust/types';

const defaultDeviceTrustEntitlement = cfg.entitlements.DeviceTrust;

export default {
  title: 'TeleportE/DeviceTrust',
  decorators: [
    Story => {
      cfg.isEnterprise = true;
      useEffect(() => {
        // Clean up
        return () => {
          cfg.entitlements.DeviceTrust = defaultDeviceTrustEntitlement;
        };
      }, []);
      return <Story />;
    },
  ],
};

export function EmptyNoCTA() {
  cfg.entitlements.DeviceTrust = { enabled: true, limit: 0 };
  const ctx = createTeleportContextE();

  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust />
    </ContextProvider>
  );
}
EmptyNoCTA.parameters = {
  msw: {
    handlers: [
      http.get(ecfg.getTrustedDevicesUrl({}), () =>
        HttpResponse.json({ items: [] })
      ),
    ],
  },
};

export function EmptyWithCTA() {
  cfg.entitlements.DeviceTrust = { enabled: true, limit: 10 };
  const ctx = createTeleportContextE();

  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust />
    </ContextProvider>
  );
}
EmptyWithCTA.parameters = {
  msw: {
    handlers: [
      http.get(ecfg.getTrustedDevicesUrl({}), () =>
        HttpResponse.json({ items: [] })
      ),
    ],
  },
};

export function Processing() {
  const ctx = createTeleportContextE();

  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust />
    </ContextProvider>
  );
}
Processing.parameters = {
  msw: {
    handlers: [
      http.get(ecfg.getTrustedDevicesUrl({}), async () => {
        await delay('infinite');
      }),
    ],
  },
};

export function LoadedEnabledAndUnlimited() {
  cfg.entitlements.DeviceTrust = { enabled: true, limit: 0 };
  const ctx = createTeleportContextE();

  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust />
    </ContextProvider>
  );
}
LoadedEnabledAndUnlimited.parameters = {
  msw: {
    handlers: [
      http.get(ecfg.getTrustedDevicesUrl({}), () =>
        HttpResponse.json({ items: devices })
      ),
    ],
  },
};

export function LoadedEnabledAndLimitedCTA() {
  cfg.entitlements.DeviceTrust = { enabled: true, limit: 10 };
  const ctx = createTeleportContextE();

  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust />
    </ContextProvider>
  );
}
LoadedEnabledAndLimitedCTA.parameters = {
  msw: {
    handlers: [
      http.get(ecfg.getTrustedDevicesUrl({}), () =>
        HttpResponse.json({ items: devices })
      ),
    ],
  },
};

export function Failed() {
  const ctx = createTeleportContextE();

  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust />
    </ContextProvider>
  );
}
Failed.parameters = {
  msw: {
    handlers: [
      http.get(ecfg.getTrustedDevicesUrl({}), () =>
        HttpResponse.json(
          {
            error: { message: 'Whoops, something went wrong.' },
          },
          { status: 500 }
        )
      ),
    ],
  },
};

const devices: TrustedDevice[] = [
  {
    id: 'goteleport.local',
    assetTag: 'CSXXXXXXXXX',
    osType: 'macOS',
    enrollStatus: 'enrolled',
  },
  {
    id: 'goteleport.local',
    assetTag: 'DSXXXXXXXXX',
    osType: 'Linux',
    enrollStatus: 'not enrolled',
  },
  {
    id: 'goteleport.local',
    assetTag: 'ESXXXXXXXXX',
    osType: 'Windows',
    enrollStatus: 'enrolled',
  },
];
