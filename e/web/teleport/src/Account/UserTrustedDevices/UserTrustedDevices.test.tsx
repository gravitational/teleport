import { userEvent } from '@testing-library/user-event';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';

import {
  act,
  enableMswServer,
  render,
  screen,
  server,
  testQueryClient,
} from 'design/utils/testing';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import TeleportContextE from 'e-teleport/teleportContextE';
import {
  createEnrollPairingSuccess,
  getCurrentEnrollPairingSuccess,
} from 'e-teleport/test/helpers/enrollPairing';
import { TrustedDevice } from 'teleport/DeviceTrust/types';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

import { UserTrustedDevices } from './UserTrustedDevices';

const mio = mockIntersectionObserver();

enableMswServer();

afterEach(async () => {
  await testQueryClient.resetQueries();
});

const renderComponent = (ctx: TeleportContextE) => {
  return render(
    <TeleportContextProvider ctx={ctx}>
      <UserTrustedDevices />
    </TeleportContextProvider>
  );
};

test('renders empty state', async () => {
  const ctx = createTeleportContextE();

  jest.spyOn(ctx.deviceService, 'fetchDevicesByUser').mockResolvedValue({
    startKey: '',
    // @ts-expect-error the resolved value requires the data key `items`
    items: [],
  });

  renderComponent(ctx);

  await act(mio.enterAll);

  expect(screen.getByText(/no devices found/i)).toBeInTheDocument();
});

test('renders users trusted devices with infinite scroll', async () => {
  const ctx = createTeleportContextE();

  jest.spyOn(ctx.deviceService, 'fetchDevicesByUser').mockResolvedValueOnce({
    startKey: '',
    // @ts-expect-error the resolved value requires the data key `items`
    items: devices,
  });

  renderComponent(ctx);

  await act(mio.enterAll);

  expect(screen.getByText('ASSET1')).toBeInTheDocument();
  expect(screen.getByText('ASSET2')).toBeInTheDocument();
});

test('renders error', async () => {
  const ctx = createTeleportContextE();

  jest
    .spyOn(ctx.deviceService, 'fetchDevicesByUser')
    .mockRejectedValue(new Error('some error here'));

  renderComponent(ctx);

  await act(mio.enterAll);

  expect(screen.getByTestId('user-trusted-devices-error')).toBeInTheDocument();
  expect(screen.getByText('some error here')).toBeInTheDocument();
});

test('closing the enroll wizard refreshes the device list', async () => {
  server.use(createEnrollPairingSuccess());
  server.use(getCurrentEnrollPairingSuccess());
  const ctx = createTeleportContextE();
  // deviceService is a module-level singleton, so every context shares it and jest.spyOn hands back
  // the same mock the tests above already called.
  const fetchDevices = jest
    .spyOn(ctx.deviceService, 'fetchDevicesByUser')
    .mockClear()
    .mockResolvedValueOnce({
      startKey: '',
      // The @ts-expect-errors are there, because the declared return type of fetchDevicesByUser
      // doesn't match what the endpoint actually returns and the test mocks the real shape.
      // TODO(ravicious): Fix types for fetchDevicesByUser.
      // @ts-expect-error the resolved value requires the data key `items`
      items: [],
    })
    .mockResolvedValueOnce({
      startKey: '',
      // @ts-expect-error the resolved value requires the data key `items`
      items: devices,
    });
  const user = userEvent.setup();

  renderComponent(ctx);
  await act(mio.enterAll);
  expect(screen.getByText(/no devices found/i)).toBeInTheDocument();

  await user.click(
    screen.getByRole('button', { name: /enroll a mobile device/i })
  );
  await user.click(await screen.findByRole('button', { name: /cancel/i }));

  // The device enrolled on the phone while the wizard was open shows up after the close-triggered
  // refetch.
  expect(await screen.findByText('ASSET1')).toBeInTheDocument();
  expect(fetchDevices).toHaveBeenCalledTimes(2);
  expect(fetchDevices).toHaveBeenNthCalledWith(
    2,
    expect.objectContaining({ startKey: '' }),
    expect.any(AbortSignal)
  );
});

const devices: TrustedDevice[] = [
  {
    id: '123',
    assetTag: 'ASSET1',
    osType: 'macOS',
    enrollStatus: 'enrolled',
    owner: 'avatus',
    createTime: new Date('2024-11-20'),
  },
  {
    id: '234',
    assetTag: 'ASSET2',
    osType: 'macOS',
    enrollStatus: 'enrolled',
    owner: 'avatus',
    createTime: new Date('2024-11-20'),
  },
];
