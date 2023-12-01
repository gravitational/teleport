import React from 'react';
import { render, screen } from 'design/utils/testing';
import cfg from 'teleport/config';
import { ContextProvider } from 'teleport';

import { deviceService } from 'e-teleport/services/devices';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { Container as DeviceTrust } from './DeviceTrust';

import type { TrustedDevice } from 'teleport/DeviceTrust/types';

describe('test DeviceTrust.tsx', () => {
  const defaultIsTeamFlag = cfg.isTeam;
  const defaultIsEnterpriseFlag = cfg.isEnterprise;
  const defaultIsUsageBasedBillingFlag = cfg.isUsageBasedBilling;
  const defaultIgsFlag = cfg.isIgsEnabled;

  beforeEach(() => {
    cfg.isEnterprise = true;
  });

  afterEach(() => {
    cfg.isTeam = defaultIsTeamFlag;
    cfg.isEnterprise = defaultIsEnterpriseFlag;
    cfg.isUsageBasedBilling = defaultIsUsageBasedBillingFlag;
    cfg.isIgsEnabled = defaultIgsFlag;
    jest.clearAllMocks();
  });

  test('empty team cta', async () => {
    cfg.isTeam = true;
    jest.spyOn(deviceService, 'fetchDevices').mockResolvedValue({ items: [] });

    const { container } = renderComponent();
    await screen.findByText(/register trusted device/i);

    expect(container.firstChild).toMatchSnapshot();
  });

  test('empty EUB cta', async () => {
    cfg.isTeam = false;
    cfg.isUsageBasedBilling = true;
    jest.spyOn(deviceService, 'fetchDevices').mockResolvedValue({ items: [] });

    const { container } = renderComponent();
    await screen.findByText(/register trusted device/i);

    expect(container.firstChild).toMatchSnapshot();
  });

  test('loaded legacy (no CTA)', async () => {
    cfg.isTeam = false;
    cfg.isUsageBasedBilling = false;
    jest
      .spyOn(deviceService, 'fetchDevices')
      .mockResolvedValue({ items: devices });

    const { container } = renderComponent();
    await screen.findByText(/register trusted device/i);

    expect(container.firstChild).toMatchSnapshot();
  });

  test('loaded EUB with IGS (no CTA)', async () => {
    cfg.isTeam = false;
    cfg.isUsageBasedBilling = true;
    cfg.isIgsEnabled = true;
    jest
      .spyOn(deviceService, 'fetchDevices')
      .mockResolvedValue({ items: devices });

    const { container } = renderComponent();
    await screen.findByText(/register trusted device/i);

    expect(container.firstChild).toMatchSnapshot();
  });

  test('loaded EUB without IGS renders CTA', async () => {
    cfg.isTeam = false;
    cfg.isUsageBasedBilling = true;
    jest
      .spyOn(deviceService, 'fetchDevices')
      .mockResolvedValue({ items: devices });

    const { container } = renderComponent();
    await screen.findByText(/register trusted device/i);

    expect(container.firstChild).toMatchSnapshot();
  });

  test('loaded team renders CTA', async () => {
    cfg.isTeam = true;
    jest
      .spyOn(deviceService, 'fetchDevices')
      .mockResolvedValue({ items: devices });

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
