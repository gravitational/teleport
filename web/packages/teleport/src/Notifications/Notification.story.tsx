/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React from 'react';
import { MemoryRouter } from 'react-router';
import {
  subSeconds,
  subMinutes,
  subHours,
  subDays,
  subWeeks,
  subMonths,
} from 'date-fns';
import { initialize, mswLoader } from 'msw-storybook-addon';
import { rest } from 'msw';

import { Flex } from 'design';

import {
  NotificationSubKind,
  Notification as NotificationType,
} from 'teleport/services/notifications';
import { createTeleportContext } from 'teleport/mocks/contexts';
import cfg from 'teleport/config';

import { ContextProvider } from '..';

import { Notification } from './Notification';
import { Notifications as NotificationsComponent } from './Notifications';

export default {
  title: 'Teleport/Notifications',
  loaders: [mswLoader],
};

initialize();

export const Notifications = () => {
  return (
    <Flex
      css={`
        background: ${props => props.theme.colors.levels.surface};
        padding: 24px;
        width: fit-content;
        height: fit-content;
        flex-direction: column;
        gap: 24px;
      `}
    >
      {mockNotifications.map(notification => {
        return (
          <Notification notification={notification} key={notification.id} />
        );
      })}
    </Flex>
  );
};

const mockNotifications: NotificationType[] = [
  {
    id: '1',
    title: 'joe approved your access request for 5 resources.',
    subKind: NotificationSubKind.AccessRequestApproved,
    createdDate: subSeconds(Date.now(), 30), // 30 seconds ago
    clicked: false,
    labels: [
      {
        name: 'request-id',
        value: '3bd7d71f-64ad-588a-988c-22f3853910fa',
      },
    ],
  },
  {
    id: '2',
    title: `joe approved your access request for the 'auditor' role,`,
    subKind: NotificationSubKind.AccessRequestApproved,
    createdDate: subMinutes(Date.now(), 4), // 4 minutes ago
    clicked: false,
    labels: [
      {
        name: 'request-id',
        value: '3bd7d71f-64ad-588a-988c-22f3853910fa',
      },
    ],
  },
  {
    id: '3',
    title: `joe denied your access request for the 'auditor' role,`,
    subKind: NotificationSubKind.AccessRequestDenied,
    createdDate: subMinutes(Date.now(), 15), // 15 minutes ago
    clicked: false,
    labels: [
      {
        name: 'request-id',
        value: '3bd7d71f-64ad-588a-988c-22f3853910fa',
      },
    ],
  },
  {
    id: '4',
    title: 'bob requested access to 2 resources.',
    subKind: NotificationSubKind.AccessRequestPending,
    createdDate: subHours(Date.now(), 3), // 3 hours ago
    clicked: true,
    labels: [
      {
        name: 'request-id',
        value: '3bd7d71f-64ad-588a-988c-22f3853910fa',
      },
    ],
  },
  {
    id: '5',
    title: `bob requested access to the 'intern' role.`,
    subKind: NotificationSubKind.AccessRequestPending,
    createdDate: subHours(Date.now(), 15), // 15 hours ago
    clicked: true,
    labels: [
      {
        name: 'request-id',
        value: '3bd7d71f-64ad-588a-988c-22f3853910fa',
      },
    ],
  },
  {
    id: '6',
    title: `2 resources are now available to access.`,
    subKind: NotificationSubKind.AccessRequestNowAssumable,
    createdDate: subDays(Date.now(), 1), // 1 day ago
    clicked: true,
    labels: [
      {
        name: 'request-id',
        value: '3bd7d71f-64ad-588a-988c-22f3853910fa',
      },
      { name: 'request-type', value: 'resource' },
    ],
  },
  {
    id: '7',
    title: `"node-5" is now available to access.`,
    subKind: NotificationSubKind.AccessRequestNowAssumable,
    createdDate: subDays(Date.now(), 3), // 3 days ago
    clicked: false,
    labels: [
      {
        name: 'request-id',
        value: '3bd7d71f-64ad-588a-988c-22f3853910fa',
      },
      { name: 'request-type', value: 'resource' },
    ],
  },
  {
    id: '8',
    title: `"auditor" is now ready to assume.`,
    subKind: NotificationSubKind.AccessRequestNowAssumable,
    createdDate: subWeeks(Date.now(), 2), // 2 weeks ago
    clicked: true,
    labels: [
      {
        name: 'request-id',
        value: '3bd7d71f-64ad-588a-988c-22f3853910fa',
      },
      { name: 'request-type', value: 'role' },
    ],
  },
  {
    id: '9',
    title: 'This is an example user-created warning notification',
    subKind: NotificationSubKind.UserCreatedWarning,
    createdDate: subMonths(Date.now(), 3), // 3 months ago
    clicked: true,
    labels: [
      {
        name: 'text-content',
        value: 'This is the text content of a warning notification.',
      },
    ],
  },
];

export const NotificationsList = () => <ListComponent />;
NotificationsList.parameters = {
  msw: {
    handlers: [
      rest.get(cfg.api.notificationsPath, (req, res, ctx) =>
        res.once(ctx.json(mockNotificationsResponseFirstPage))
      ),
      rest.get(cfg.api.notificationsPath, (req, res, ctx) =>
        res(ctx.delay(2000), ctx.json(mockNotificationsResponseSecondPage))
      ),
    ],
  },
};

export const NotificationsListEmpty = () => <ListComponent />;
NotificationsListEmpty.parameters = {
  msw: {
    handlers: [
      rest.get(cfg.api.notificationsPath, (req, res, ctx) =>
        res(
          ctx.json({
            userNotificationsNextKey: '',
            globalNotificationsNextKey: '',
            userLastSeenNotification: subDays(Date.now(), 15).toISOString(), // 15 days ago
            notifications: [],
          })
        )
      ),
    ],
  },
};

export const NotificationsListError = () => <ListComponent />;
NotificationsListError.parameters = {
  msw: {
    handlers: [
      rest.get(cfg.api.notificationsPath, (req, res, ctx) =>
        res(
          ctx.status(403),
          ctx.json({
            message: 'Error encountered: failed to fetch notifications',
          })
        )
      ),
    ],
  },
};

const ListComponent = () => {
  const ctx = createTeleportContext();

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <Flex
          css={`
            width: 100%;
            justify-content: center;
            height: ${p => p.theme.topBarHeight[2]}px;
          `}
        >
          <NotificationsComponent />
        </Flex>
      </ContextProvider>
    </MemoryRouter>
  );
};

const mockNotificationsResponseFirstPage = {
  userNotificationsNextKey: '16',
  globalNotificationsNextKey: '',
  userLastSeenNotification: subMinutes(Date.now(), 12).toISOString(), // 12 minutes ago
  notifications: [
    {
      id: '1',
      title: 'Example notification 1',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subSeconds(Date.now(), 15).toISOString(), // 15 seconds ago
      clicked: true,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
    {
      id: '2',
      title: 'Example notification 2',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subSeconds(Date.now(), 30).toISOString(), // 30 seconds ago
      clicked: true,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
    {
      id: '3',
      title: 'Example notification 3',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subMinutes(Date.now(), 1).toISOString(), // 1 minute ago
      clicked: false,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
    {
      id: '4',
      title: 'Example notification 4',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subMinutes(Date.now(), 5).toISOString(), // 5 minutes ago
      clicked: true,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
    {
      id: '5',
      title: 'Example notification 5',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subMinutes(Date.now(), 10).toISOString(), // 10 minutes ago
      clicked: true,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
    {
      id: '6',
      title: 'Example notification 6',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subHours(Date.now(), 1).toISOString(), // 1 hour ago
      clicked: false,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
    {
      id: '7',
      title: 'Example notification 7',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subDays(Date.now(), 1).toISOString(), // 1 day ago
      clicked: true,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
    {
      id: '8',
      title: 'Example notification 8',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subDays(Date.now(), 2).toISOString(), // 2 days ago
      clicked: true,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
    {
      id: '9',
      title: 'Example notification 9',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subDays(Date.now(), 7).toISOString(), // 7 days ago
      clicked: false,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
    {
      id: '10',
      title: 'Example notification 10',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subDays(Date.now(), 15).toISOString(), // 15 days ago
      clicked: true,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
    {
      id: '11',
      title: 'Example notification 11',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subDays(Date.now(), 30).toISOString(), // 30 days ago
      clicked: true,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
    {
      id: '12',
      title: 'Example notification 12',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subDays(Date.now(), 35).toISOString(), // 35 days ago
      clicked: false,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
    {
      id: '13',
      title: 'Example notification 13',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subDays(Date.now(), 40).toISOString(), // 40 days ago
      clicked: false,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
    {
      id: '14',
      title: 'Example notification 14',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subDays(Date.now(), 45).toISOString(), // 45 days ago
      clicked: true,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
    {
      id: '15',
      title: 'Example notification 15',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subDays(Date.now(), 50).toISOString(), // 50 days ago
      clicked: false,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
  ],
};

const mockNotificationsResponseSecondPage = {
  userNotificationsNextKey: '',
  globalNotificationsNextKey: '',
  userLastSeenNotification: subDays(Date.now(), 60).toISOString(), // 60 days ago
  notifications: [
    {
      id: '16',
      title: 'Example notification 16',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subDays(Date.now(), 55).toISOString(), // 55 days ago
      clicked: false,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
    {
      id: '17',
      title: 'Example notification 17',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subDays(Date.now(), 60).toISOString(), // 60 days ago
      clicked: false,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
    {
      id: '18',
      title: 'Example notification 18',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subDays(Date.now(), 65).toISOString(), // 65 days ago
      clicked: true,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
    {
      id: '19',
      title: 'Example notification 19',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subDays(Date.now(), 70).toISOString(), // 70 days ago
      clicked: false,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
    {
      id: '20',
      title: 'Example notification 20',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subDays(Date.now(), 75).toISOString(), // 75 days ago
      clicked: true,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
    {
      id: '21',
      title: 'Example notification 21',
      subKind: NotificationSubKind.UserCreatedInformational,
      created: subDays(Date.now(), 80).toISOString(), // 80 days ago
      clicked: false,
      labels: [
        {
          name: 'text-content',
          value: 'This is the text content of the notification.',
        },
      ],
    },
  ],
};
