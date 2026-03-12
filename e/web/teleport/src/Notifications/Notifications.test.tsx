import { subMinutes, subSeconds } from 'date-fns';
import { MemoryRouter } from 'react-router';

import { render, screen, waitFor } from 'design/utils/testing';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { getOSSFeatures } from 'teleport/features';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { LayoutContextProvider } from 'teleport/Main/LayoutContext';
import { Notifications } from 'teleport/Notifications';
import { NotificationSubKind } from 'teleport/services/notifications';
import TeleportContext from 'teleport/teleportContext';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

beforeAll(() => {
  jest.useFakeTimers();
  jest.setSystemTime(new Date('2023-01-20'));
});

afterAll(() => {
  jest.useRealTimers();
});

test('notification bell with notifications', async () => {
  const ctx = createTeleportContextE();

  jest.spyOn(ctx.notificationService, 'fetchNotifications').mockResolvedValue({
    nextKey: '',
    userLastSeenNotification: subMinutes(Date.now(), 12), // 12 minutes ago
    notifications: [
      {
        id: '1',
        title: 'Example notification 1',
        subKind: NotificationSubKind.UserCreatedInformational,
        createdDate: subSeconds(Date.now(), 15), // 15 seconds ago
        clicked: false,
        labels: [
          {
            name: 'text-content',
            value: 'This is the text content of the notification.',
          },
        ],
      },
    ],
  });

  jest
    .spyOn(ctx.notificationService, 'upsertLastSeenNotificationTime')
    .mockResolvedValue({
      time: new Date(),
    });

  render(renderNotifications(ctx));

  await screen.findByTestId('tb-notifications-badge');

  await waitFor(() => {
    expect(screen.getByTestId('tb-notifications-badge')).toHaveTextContent('1');
  });

  expect(screen.getByTestId('tb-notifications')).toBeInTheDocument();

  // Expect there to be 2 notifications.
  expect(screen.queryAllByTestId('notification-item')).toHaveLength(1);
});

test('notification bell with no notifications', async () => {
  const ctx = createTeleportContextE();
  jest.spyOn(ctx.notificationService, 'fetchNotifications').mockResolvedValue({
    nextKey: '',
    userLastSeenNotification: subMinutes(Date.now(), 12), // 12 minutes ago
    notifications: [],
  });

  jest
    .spyOn(ctx.notificationService, 'upsertLastSeenNotificationTime')
    .mockResolvedValue({
      time: new Date(),
    });

  render(renderNotifications(ctx));

  await screen.findByText(/you currently have no notifications/i);

  expect(screen.queryByTestId('notification-item')).not.toBeInTheDocument();
});

const renderNotifications = (ctx: TeleportContext) => {
  return (
    <MemoryRouter initialEntries={['/']}>
      <LayoutContextProvider>
        <TeleportContextProvider ctx={ctx}>
          <FeaturesContextProvider value={getOSSFeatures()}>
            <Notifications />
          </FeaturesContextProvider>
        </TeleportContextProvider>
      </LayoutContextProvider>
    </MemoryRouter>
  );
};
