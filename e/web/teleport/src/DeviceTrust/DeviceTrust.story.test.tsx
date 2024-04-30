import React from 'react';
import { render, screen } from 'design/utils/testing';
import cfg from 'teleport/config';
import { ContextProvider } from 'teleport';

import { deviceService } from 'e-teleport/services/devices';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { DeviceTrust } from './DeviceTrust';

import type { TrustedDevice } from 'teleport/DeviceTrust/types';

describe('test DeviceTrust.tsx', () => {
  const defaultTrustedDevicesFlag = cfg.trustedDevices;
  const defaultIsEnterpriseFlag = cfg.isEnterprise;
  const defaultIsUsageBasedBillingFlag = cfg.isUsageBasedBilling;
  const defaultIgsFlag = cfg.isIgsEnabled;

  beforeEach(() => {
    cfg.isEnterprise = true;
  });

  afterEach(() => {
    cfg.trustedDevices = defaultTrustedDevicesFlag;
    cfg.isEnterprise = defaultIsEnterpriseFlag;
    cfg.isUsageBasedBilling = defaultIsUsageBasedBillingFlag;
    cfg.isIgsEnabled = defaultIgsFlag;
    jest.clearAllMocks();
  });

  test('empty cta', async () => {
    cfg.trustedDevices = false;
    jest
      .spyOn(deviceService, 'fetchDevices')
      .mockResolvedValue({ items: [], startKey: '' });

    const { container } = renderComponent();
    await screen.findByText(/register trusted device/i);

    expect(container.firstChild).toMatchSnapshot();
  });

  test('empty EUB cta', async () => {
    cfg.isUsageBasedBilling = true;
    jest
      .spyOn(deviceService, 'fetchDevices')
      .mockResolvedValue({ items: [], startKey: '' });

    const { container } = renderComponent();
    await screen.findByText(/register trusted device/i);

    expect(container.firstChild).toMatchSnapshot();
  });

  test('loaded legacy (no CTA)', async () => {
    cfg.trustedDevices = true;
    cfg.isUsageBasedBilling = false;
    jest
      .spyOn(deviceService, 'fetchDevices')
      .mockResolvedValue({ items: devices, startKey: '' });

    const { container } = renderComponent();
    await screen.findByText(/register trusted device/i);

    expect(container.firstChild).toMatchSnapshot();
  });

  test('loaded EUB with IGS (no CTA)', async () => {
    cfg.trustedDevices = true;
    cfg.isUsageBasedBilling = true;
    cfg.isIgsEnabled = true;
    jest
      .spyOn(deviceService, 'fetchDevices')
      .mockResolvedValue({ items: devices, startKey: '' });

    const { container } = renderComponent();
    await screen.findByText(/register trusted device/i);

    expect(container.firstChild).toMatchSnapshot();
  });

  test('loaded EUB without IGS renders CTA', async () => {
    cfg.trustedDevices = true;
    cfg.isUsageBasedBilling = true;
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
