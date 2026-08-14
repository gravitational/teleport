import { delay, http, HttpResponse } from 'msw';
import { useEffect } from 'react';
import { MemoryRouter } from 'react-router';

import ecfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import TeleportEContext from 'e-teleport/teleportContextE';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { DeviceOrigin, type TrustedDevice } from 'teleport/DeviceTrust/types';

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
EmptyNoCTA.beforeEach = ({ msw }) => {
  msw.use(
    http.get(ecfg.getTrustedDevicesUrl({}), () =>
      HttpResponse.json({ items: [] })
    )
  );
};

export function EmptyWithCTA() {
  cfg.entitlements.DeviceTrust = { enabled: true, limit: 10 };
  const ctx = createTeleportContextE();

  return <Component ctx={ctx} />;
}
EmptyWithCTA.beforeEach = ({ msw }) => {
  msw.use(
    http.get(ecfg.getTrustedDevicesUrl({}), () =>
      HttpResponse.json({ items: [] })
    )
  );
};

export function Processing() {
  const ctx = createTeleportContextE();

  return <Component ctx={ctx} />;
}
Processing.beforeEach = ({ msw }) => {
  msw.use(
    http.get(ecfg.getTrustedDevicesUrl({}), async () => {
      await delay('infinite');
    })
  );
};

export function LoadedEnabledAndUnlimited() {
  cfg.entitlements.DeviceTrust = { enabled: true, limit: 0 };
  const ctx = createTeleportContextE();

  return <Component ctx={ctx} />;
}
LoadedEnabledAndUnlimited.beforeEach = ({ msw }) => {
  msw.use(
    http.get(ecfg.getTrustedDevicesUrl({}), () =>
      HttpResponse.json({ items: devices })
    )
  );
};

export function LoadedEnabledAndLimitedCTA() {
  const ctx = createTeleportContextE();
  cfg.entitlements.DeviceTrust = { enabled: true, limit: 10 };

  return <Component ctx={ctx} />;
}
LoadedEnabledAndLimitedCTA.beforeEach = ({ msw }) => {
  msw.use(
    http.get(ecfg.getTrustedDevicesUrl({}), () =>
      HttpResponse.json({ items: devices })
    )
  );
};

export function Failed() {
  const ctx = createTeleportContextE();

  return <Component ctx={ctx} />;
}
Failed.beforeEach = ({ msw }) => {
  msw.use(
    http.get(ecfg.getTrustedDevicesUrl({}), () =>
      HttpResponse.json(
        {
          error: { message: 'Whoops, something went wrong.' },
        },
        { status: 500 }
      )
    )
  );
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
    id: '96e6c8f2-bdd7-4bee-8fe7-8f2737537de2',
    assetTag: 'CSXXXXXXXXX',
    osType: 'macOS',
    enrollStatus: 'enrolled',
    owner: 'mykel',
  },
  {
    id: '51eec618-5367-4dd5-81df-6a30f31e3c46',
    assetTag: 'ESXXXXXXXXX',
    osType: 'Windows',
    enrollStatus: 'enrolled',
    owner: 'yassey',
    // Should be rendered as "Intune" since it's equal to the default name.
    source: {
      origin: DeviceOrigin.Intune,
      name: 'intune',
    },
  },
  {
    id: 'ab323421-9d23-4bcd-9350-bdfe69ee4800',
    assetTag: 'DSXXXXXXXXX',
    osType: 'Linux',
    enrollStatus: 'not enrolled',
    owner: '',
    source: {
      origin: DeviceOrigin.Api,
      name: 'lorem ipsum',
    },
  },
  {
    id: '70b4a18b-1317-4c8d-ac29-c62366b7ef14',
    assetTag: 'FZXXXXXXXXX',
    osType: 'Linux',
    enrollStatus: 'enrolled',
    owner: 'alice',
    // Should be rendered as "jamf-external" since it doesn't match the default name.
    source: {
      origin: DeviceOrigin.Jamf,
      name: 'jamf-external',
    },
  },
  {
    id: '5e7ece68-9e49-42d8-a561-696226ed758d',
    assetTag: 'GYXXXXXXXXX',
    osType: 'macOS',
    enrollStatus: 'enrolled',
    owner: 'lila',
    // Should be rendered as "unknown".
    source: {
      origin: 42 as DeviceOrigin,
      name: 'contoso MDM',
    },
  },
  {
    id: 'de42795b-b5de-4ceb-a4b1-30ed9738047c',
    assetTag: 'HTXXXXXXXXX',
    osType: 'Windows',
    enrollStatus: 'enrolled',
    owner: 'yassey',
    // Should be rendered as "contoso MDM".
    source: {
      origin: 1337 as DeviceOrigin,
      name: 'contoso MDM',
    },
  },
  {
    id: '40facab6-8b54-404b-9345-324afa164c33',
    assetTag: 'ISXXXXXXXXX',
    osType: 'iOS',
    enrollStatus: 'not enrolled',
    owner: '',
  },
  {
    id: 'cff93878-dbb1-4436-8e3b-50cce3db59b9',
    assetTag: 'JRXXXXXXXXX',
    osType: 'iPadOS',
    enrollStatus: 'enrolled',
    owner: 'alice',
  },
];
