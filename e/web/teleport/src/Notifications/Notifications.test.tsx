import { subMinutes, subSeconds } from 'date-fns';
import { createMemoryHistory } from 'history';
import { Router } from 'react-router';
import { render, screen, waitFor } from 'design/utils/testing';

import { LayoutContextProvider } from 'teleport/Main/LayoutContext';

import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { getOSSFeatures } from 'teleport/features';
import TeleportContextProvider from 'teleport/TeleportContextProvider';
import TeleportContext from 'teleport/teleportContext';
import {
  LocalNotificationKind,
  NotificationSubKind,
} from 'teleport/services/notifications';

import { Notifications } from 'teleport/Notifications';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

beforeAll(() => {
  jest.useFakeTimers();
  jest.setSystemTime(new Date('2023-01-20'));
});

afterAll(() => {
  jest.useRealTimers();
});

test('notification bell with notifications', async () => {
  const ctx = createTeleportContextE();

  ctx.storeNotifications.state = {
    notifications: [
      {
        item: {
          kind: LocalNotificationKind.AccessList,
          resourceName: 'banana',
          route: '',
        },
        id: 'abc',
        date: new Date('2023-01-25'),
      },
    ],
  };

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
    expect(screen.getByTestId('tb-notifications-badge')).toHaveTextContent('2');
  });

  expect(screen.getByTestId('tb-notifications')).toBeInTheDocument();

  // Expect there to be 2 notifications.
  expect(screen.queryAllByTestId('notification-item')).toHaveLength(2);
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

test('due dates and overdue dates for access list notifications, and that they are shown individually when there are 2 or less of each', async () => {
  const ctx = createTeleportContextE();

  ctx.storeNotifications.state = {
    notifications: [
      // due in 5 days
      {
        item: {
          kind: LocalNotificationKind.AccessList,
          resourceName: 'carrot',
          route: '',
        },
        id: '1',
        date: new Date('2023-01-25'),
      },
      // due in 10 days
      {
        item: {
          kind: LocalNotificationKind.AccessList,
          resourceName: 'carrot',
          route: '',
        },
        id: '2',
        date: new Date('2023-01-30'),
      },
      // overdue by 10 days
      {
        item: {
          kind: LocalNotificationKind.AccessList,
          resourceName: 'carrot',
          route: '',
        },
        id: '3',
        date: new Date('2023-01-10'),
      },
      // overdue by a month
      {
        item: {
          kind: LocalNotificationKind.AccessList,
          resourceName: 'carrot',
          route: '',
        },
        id: '4',
        date: new Date('2022-12-20'),
      },
    ],
  };

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

  await screen.findByTestId('tb-notifications-badge');

  expect(screen.queryAllByTestId('notification-item')).toHaveLength(4);

  expect(screen.getByText(/is overdue by 10 days/i)).toBeInTheDocument();

  expect(screen.getByText(/is overdue by 1 month/i)).toBeInTheDocument();

  expect(
    screen.getByText(/needs your review within 5 days/i)
  ).toBeInTheDocument();

  expect(
    screen.getByText(/needs your review within 10 days/i)
  ).toBeInTheDocument();
});

test('access list notifications should be grouped into one when there are 3 or more of each type', async () => {
  const ctx = createTeleportContextE();

  ctx.storeNotifications.state = {
    notifications: [
      // due in 5 days
      {
        item: {
          kind: LocalNotificationKind.AccessList,
          resourceName: 'carrot',
          route: '',
        },
        id: '1',
        date: new Date('2023-01-25'),
      },
      // due in 10 days
      {
        item: {
          kind: LocalNotificationKind.AccessList,
          resourceName: 'carrot',
          route: '',
        },
        id: '2',
        date: new Date('2023-01-30'),
      },
      // due in 15 days
      {
        item: {
          kind: LocalNotificationKind.AccessList,
          resourceName: 'carrot',
          route: '',
        },
        id: '3',
        date: new Date('2023-02-05'),
      },
      // overdue by 10 days
      {
        item: {
          kind: LocalNotificationKind.AccessList,
          resourceName: 'carrot',
          route: '',
        },
        id: '4',
        date: new Date('2023-01-10'),
      },
      // overdue by a month
      {
        item: {
          kind: LocalNotificationKind.AccessList,
          resourceName: 'carrot',
          route: '',
        },
        id: '5',
        date: new Date('2022-12-20'),
      },
      // overdue by 2 months
      {
        item: {
          kind: LocalNotificationKind.AccessList,
          resourceName: 'carrot',
          route: '',
        },
        id: '6',
        date: new Date('2021-11-20'),
      },
    ],
  };

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

  await screen.findByTestId('tb-notifications-badge');

  expect(screen.queryAllByTestId('notification-item')).toHaveLength(2);

  expect(
    screen.getByText(
      /3 of your access lists require review, the most urgent of which is due in 5 days/i
    )
  ).toBeInTheDocument();

  expect(
    screen.getByText(/3 of your access lists are overdue for review/i)
  ).toBeInTheDocument();
});

const renderNotifications = (ctx: TeleportContext) => {
  return (
    <Router history={createMemoryHistory()}>
      <LayoutContextProvider>
        <TeleportContextProvider ctx={ctx}>
          <FeaturesContextProvider value={getOSSFeatures()}>
            <Notifications />
          </FeaturesContextProvider>
        </TeleportContextProvider>
      </LayoutContextProvider>
    </Router>
  );
};
