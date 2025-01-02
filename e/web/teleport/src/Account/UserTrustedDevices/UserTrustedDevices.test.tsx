import { mockIntersectionObserver } from 'jsdom-testing-mocks';

import { act, render, screen } from 'design/utils/testing';

import TeleportContextE from 'e-teleport/teleportContextE';
import { TrustedDevice } from 'teleport/DeviceTrust/types';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

import { UserTrustedDevices } from './UserTrustedDevices';

const mio = mockIntersectionObserver();

const renderComponent = (ctx: TeleportContextE) => {
  return render(
    <TeleportContextProvider ctx={ctx}>
      <UserTrustedDevices />
    </TeleportContextProvider>
  );
};

test('renders empty state', async () => {
  const ctx = new TeleportContextE();

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
  const ctx = new TeleportContextE();

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
  const ctx = new TeleportContextE();

  jest
    .spyOn(ctx.deviceService, 'fetchDevicesByUser')
    .mockRejectedValue(new Error('some error here'));

  renderComponent(ctx);

  await act(mio.enterAll);

  expect(screen.getByTestId('user-trusted-devices-error')).toBeInTheDocument();
  expect(screen.getByText('some error here')).toBeInTheDocument();
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
