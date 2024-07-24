import React, { useEffect } from 'react';
import { http, HttpResponse, delay } from 'msw';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import ecfg from 'e-teleport/config';

import { DeviceTrust } from './DeviceTrust';

import type { TrustedDevice } from 'teleport/DeviceTrust/types';

const defaultTrustedDevicesFlag = cfg.trustedDevices;
const defaultIsEnterpriseFlag = cfg.isEnterprise;
const defaultIsUsageBasedBillingFlag = cfg.isUsageBasedBilling;
const defaultIgsFlag = cfg.isIgsEnabled;

export default {
  title: 'TeleportE/DeviceTrust',
  decorators: [
    Story => {
      cfg.isEnterprise = true;
      useEffect(() => {
        // Clean up
        return () => {
          cfg.trustedDevices = defaultTrustedDevicesFlag;
          cfg.isEnterprise = defaultIsEnterpriseFlag;
          cfg.isUsageBasedBilling = defaultIsUsageBasedBillingFlag;
          cfg.isIgsEnabled = defaultIgsFlag;
        };
      }, []);
      return <Story />;
    },
  ],
};

export function Empty() {
  cfg.trustedDevices = true;
  const ctx = createTeleportContextE();

  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust />
    </ContextProvider>
  );
}
Empty.parameters = {
  msw: {
    handlers: [
      http.get(ecfg.getTrustedDevicesUrl({}), () =>
        HttpResponse.json({ items: [] })
      ),
    ],
  },
};

export function EmptyCta() {
  cfg.trustedDevices = false;
  const ctx = createTeleportContextE();

  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust />
    </ContextProvider>
  );
}
EmptyCta.parameters = {
  msw: {
    handlers: [
      http.get(ecfg.getTrustedDevicesUrl({}), () =>
        HttpResponse.json({ items: [] })
      ),
    ],
  },
};

export function EmptyEubCta() {
  cfg.trustedDevices = true;
  cfg.isUsageBasedBilling = true;
  const ctx = createTeleportContextE();

  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust />
    </ContextProvider>
  );
}
EmptyEubCta.parameters = {
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

export function LoadedLegacy() {
  cfg.trustedDevices = true;
  cfg.isUsageBasedBilling = false;
  const ctx = createTeleportContextE();

  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust />
    </ContextProvider>
  );
}
LoadedLegacy.parameters = {
  msw: {
    handlers: [
      http.get(ecfg.getTrustedDevicesUrl({}), () =>
        HttpResponse.json({ items: devices })
      ),
    ],
  },
};

export function LoadedEubWithIgs() {
  cfg.trustedDevices = true;
  cfg.isUsageBasedBilling = true;
  cfg.isIgsEnabled = true;
  const ctx = createTeleportContextE();

  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust />
    </ContextProvider>
  );
}
LoadedEubWithIgs.parameters = {
  msw: {
    handlers: [
      http.get(ecfg.getTrustedDevicesUrl({}), () =>
        HttpResponse.json({ items: devices })
      ),
    ],
  },
};

export function LoadedCta() {
  cfg.trustedDevices = false;
  const ctx = createTeleportContextE();

  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust />
    </ContextProvider>
  );
}
LoadedCta.parameters = {
  msw: {
    handlers: [
      http.get(ecfg.getTrustedDevicesUrl({}), () =>
        HttpResponse.json({ items: devices })
      ),
    ],
  },
};

export function LoadedEubCta() {
  cfg.trustedDevices = true;
  cfg.isUsageBasedBilling = true;
  const ctx = createTeleportContextE();

  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust />
    </ContextProvider>
  );
}
LoadedEubCta.parameters = {
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
