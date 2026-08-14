import { MemoryRouter } from 'react-router';

import { render, screen, waitFor } from 'design/utils/testing';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { makeDevices } from 'e-teleport/services/devices/makeDevices';
import TeleportEContext from 'e-teleport/teleportContextE';
import { ContextProvider } from 'teleport';
import { fakeItems } from 'teleport/DeviceTrust/EmptyList';
import { allAccessAcl, noAccess } from 'teleport/mocks/contexts';
import api from 'teleport/services/api';

import { DeviceTrust } from './DeviceTrust';

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
  });

  test('renders permission error', async () => {
    const ctx = createTeleportContextE();
    ctx.storeUser.setState({ acl: { ...allAccessAcl, deviceTrust: noAccess } });
    jest.spyOn(api, 'get').mockResolvedValue({ items: [] });

    render(<Component ctx={ctx} />);

    await waitFor(() => {
      expect(screen.getByTestId('devices-empty-state')).toBeInTheDocument();
    });

    expect(screen.getByText(/you do not have permission/i)).toBeInTheDocument();
  });

  test('renders device list when devices are present', async () => {
    jest
      .spyOn(api, 'get')
      .mockResolvedValue({ items: fakeItems.map(makeDevices) });

    render(<Component />);

    await waitFor(() => {
      expect(screen.getByTestId('devices-list')).toBeInTheDocument();
    });
    expect(screen.queryByTestId('devices-empty-state')).not.toBeInTheDocument();
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
