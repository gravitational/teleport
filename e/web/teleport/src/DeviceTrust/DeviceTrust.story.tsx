import React, { useEffect } from 'react';
import { initialize, mswLoader } from 'msw-storybook-addon';
import { rest } from 'msw';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import ecfg from 'e-teleport/config';

import { Container as DeviceTrust } from './DeviceTrust';

import type { TrustedDevice } from 'teleport/DeviceTrust/types';

const defaultIsTeamFlag = cfg.isTeam;
const defaultIsEnterpriseFlag = cfg.isEnterprise;
const defaultIsUsageBasedBillingFlag = cfg.isUsageBasedBilling;
const defaultIgsFlag = cfg.isIgsEnabled;

export default {
  title: 'TeleportE/DeviceTrust',
  loaders: [mswLoader],
  decorators: [
    Story => {
      cfg.isEnterprise = true;
      useEffect(() => {
        // Clean up
        return () => {
          cfg.isTeam = defaultIsTeamFlag;
          cfg.isEnterprise = defaultIsEnterpriseFlag;
          cfg.isUsageBasedBilling = defaultIsUsageBasedBillingFlag;
          cfg.isIgsEnabled = defaultIgsFlag;
        };
      }, []);
      return <Story />;
    },
  ],
};

initialize();

export function Empty() {
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
      rest.get(ecfg.api.devices, (req, res, ctx) =>
        res(ctx.json({ items: [] }))
      ),
    ],
  },
};

export function EmptyTeamCta() {
  cfg.isTeam = true;
  const ctx = createTeleportContextE();

  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust />
    </ContextProvider>
  );
}
EmptyTeamCta.parameters = {
  msw: {
    handlers: [
      rest.get(ecfg.api.devices, (req, res, ctx) =>
        res(ctx.json({ items: [] }))
      ),
    ],
  },
};

export function EmptyEubCta() {
  cfg.isTeam = false;
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
      rest.get(ecfg.api.devices, (req, res, ctx) =>
        res(ctx.json({ items: [] }))
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
      rest.get(ecfg.api.devices, (req, res, ctx) => res(ctx.delay('infinite'))),
    ],
  },
};

export function LoadedLegacy() {
  cfg.isTeam = false;
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
      rest.get(ecfg.api.devices, (req, res, ctx) =>
        res(ctx.json({ items: devices }))
      ),
    ],
  },
};

export function LoadedEubWithIgs() {
  cfg.isTeam = false;
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
      rest.get(ecfg.api.devices, (req, res, ctx) =>
        res(ctx.json({ items: devices }))
      ),
    ],
  },
};

export function LoadedTeamCta() {
  cfg.isTeam = true;
  const ctx = createTeleportContextE();

  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust />
    </ContextProvider>
  );
}
LoadedTeamCta.parameters = {
  msw: {
    handlers: [
      rest.get(ecfg.api.devices, (req, res, ctx) =>
        res(ctx.json({ items: devices }))
      ),
    ],
  },
};

export function LoadedEubCta() {
  cfg.isTeam = false;
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
      rest.get(ecfg.api.devices, (req, res, ctx) =>
        res(ctx.json({ items: devices }))
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
      rest.get(ecfg.api.devices, (req, res, ctx) =>
        res(ctx.status(404), ctx.json({ message: 'some error message' }))
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
