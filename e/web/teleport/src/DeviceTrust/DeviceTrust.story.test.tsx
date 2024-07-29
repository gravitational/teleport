import React from 'react';
import { render, screen } from 'design/utils/testing';
import cfg from 'teleport/config';
import { ContextProvider } from 'teleport';

import { deviceService } from 'e-teleport/services/devices';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { DeviceTrust } from './DeviceTrust';

import type { TrustedDevice } from 'teleport/DeviceTrust/types';

describe('test DeviceTrust.tsx', () => {
  const defaultDeviceTrustEntitlement = cfg.entitlements.DeviceTrust;

  beforeEach(() => {
    cfg.isEnterprise = true;
  });

  afterEach(() => {
    cfg.entitlements.DeviceTrust = defaultDeviceTrustEntitlement;
    jest.clearAllMocks();
  });

  test('empty list with cta', async () => {
    cfg.entitlements.DeviceTrust = { enabled: true, limit: 1 };
    jest
      .spyOn(deviceService, 'fetchDevices')
      .mockResolvedValue({ items: [], startKey: '' });

    const { container } = renderComponent();
    await screen.findByText(/register trusted device/i);

    expect(container.firstChild).toMatchSnapshot();
  });

  test('enabled no limit - no CTA', async () => {
    cfg.entitlements.DeviceTrust = { enabled: true, limit: 0 };
    jest
      .spyOn(deviceService, 'fetchDevices')
      .mockResolvedValue({ items: devices, startKey: '' });

    const { container } = renderComponent();
    await screen.findByText(/register trusted device/i);

    expect(container.firstChild).toMatchSnapshot();
  });

  test('enabled with limit - renders CTA', async () => {
    cfg.entitlements.DeviceTrust = { enabled: true, limit: 1 };
    jest
      .spyOn(deviceService, 'fetchDevices')
      .mockResolvedValue({ items: devices, startKey: '' });

    const { container } = renderComponent();
    await screen.findByText(/register trusted device/i);

    expect(container.firstChild).toMatchSnapshot();
  });
});

function renderComponent() {
  return render(
    <ContextProvider ctx={createTeleportContextE()}>
      <DeviceTrust />
    </ContextProvider>
  );
}

const devices: TrustedDevice[] = [
  {
    id: 'goteleport.local',
    assetTag: 'CSXXXXXXXXX',
    osType: 'macOS',
    enrollStatus: 'enrolled',
  },
];
