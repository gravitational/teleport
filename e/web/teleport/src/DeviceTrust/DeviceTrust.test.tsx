import { MemoryRouter } from 'react-router';
import { render, screen, waitFor } from 'design/utils/testing';
import { ContextProvider } from 'teleport';
import api from 'teleport/services/api';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import TeleportEContext from 'e-teleport/teleportContextE';
import { makeDevices } from 'e-teleport/services/devices/makeDevices';

import { DeviceTrust } from './DeviceTrust';
import { fakeItems } from './EmptyList/EmptyList';

describe('DeviceTrust', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  test('renders the empty list when no devices exist', async () => {
    jest.spyOn(api, 'get').mockResolvedValue({ items: [] });

    render(<Component />);

    await waitFor(() => {
      expect(screen.getByTestId('devices-empty-state')).toBeInTheDocument();
    });

    expect(screen.queryByTestId('devices-cta')).not.toBeInTheDocument();
  });

  test('renders empty list and cta', async () => {
    const ctx = createTeleportContextE();
    jest.spyOn(api, 'get').mockResolvedValue({ items: [] });
    ctx.entitlements.DeviceTrust.limit = 5;

    render(<Component ctx={ctx} />);

    await waitFor(() => {
      expect(screen.getByTestId('devices-empty-state')).toBeInTheDocument();
    });

    expect(screen.getByTestId('devices-cta')).toBeInTheDocument();
  });

  test('renders device list when devices are present', async () => {
    jest
      .spyOn(api, 'get')
      .mockResolvedValue({ items: fakeItems.map(makeDevices) });

    render(<Component />);

    await waitFor(() => {
      expect(
        screen.queryByTestId('devices-empty-state')
      ).not.toBeInTheDocument();
    });
    expect(screen.getByTestId('devices-list')).toBeInTheDocument();
  });

  test('renders device list when devices are present and a CTA', async () => {
    const ctx = createTeleportContextE();
    jest
      .spyOn(api, 'get')
      .mockResolvedValue({ items: fakeItems.map(makeDevices) });
    ctx.entitlements.DeviceTrust.limit = 5;

    render(<Component ctx={ctx} />);

    await waitFor(() => {
      expect(
        screen.queryByTestId('devices-empty-state')
      ).not.toBeInTheDocument();
    });

    expect(screen.getByTestId('devices-list')).toBeInTheDocument();
    expect(screen.getByTestId('devices-cta')).toBeInTheDocument();
  });
});

const Component = ({ ctx }: { ctx?: TeleportEContext }) => {
  const defaultCtx = createTeleportContextE();
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx || defaultCtx}>
        <DeviceTrust />
      </ContextProvider>
    </MemoryRouter>
  );
};
