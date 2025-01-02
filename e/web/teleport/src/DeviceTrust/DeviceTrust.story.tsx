import { delay, http, HttpResponse } from 'msw';
import { useEffect } from 'react';
import { MemoryRouter } from 'react-router';

import ecfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import TeleportEContext from 'e-teleport/teleportContextE';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import type { TrustedDevice } from 'teleport/DeviceTrust/types';

import { DeviceTrust } from './DeviceTrust';

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

  return <Component ctx={ctx} />;
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

  return <Component ctx={ctx} />;
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

  return <Component ctx={ctx} />;
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

  return <Component ctx={ctx} />;
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
  const ctx = createTeleportContextE();
  cfg.entitlements.DeviceTrust = { enabled: true, limit: 10 };

  return <Component ctx={ctx} />;
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

  return <Component ctx={ctx} />;
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

const Component = ({ ctx }: { ctx: TeleportEContext }) => {
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <DeviceTrust />
      </ContextProvider>
    </MemoryRouter>
  );
};

const devices: TrustedDevice[] = [
  {
    id: 'goteleport.local',
    assetTag: 'CSXXXXXXXXX',
    osType: 'macOS',
    enrollStatus: 'enrolled',
    owner: 'mykel',
  },
  {
    id: 'goteleport.local',
    assetTag: 'DSXXXXXXXXX',
    osType: 'Linux',
    enrollStatus: 'not enrolled',
    owner: 'lila',
  },
  {
    id: 'goteleport.local',
    assetTag: 'ESXXXXXXXXX',
    osType: 'Windows',
    enrollStatus: 'enrolled',
    owner: 'yassey',
  },
];
