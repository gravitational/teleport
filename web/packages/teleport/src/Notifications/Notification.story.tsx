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

import { Flex } from 'design';

import {
  NotificationSubKind,
  Notification as NotificationType,
} from 'teleport/services/notifications';

import { Notification } from './Notification';

export default {
  title: 'Teleport/Notifications',
};

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
    title: '',
    description: '',
    subKind: NotificationSubKind.AccessRequestApproved,
    createdDate: new Date(Date.now() - 30 * 1000), // 30 seconds ago
    clicked: false,
    labels: [
      {
        name: 'requested-resources',
        value: 'node-1,node-2,db-1,db-2,db-3',
      },
      { name: 'reviewer', value: 'joe' },
    ],
  },
  {
    id: '2',
    title: '',
    description: '',
    subKind: NotificationSubKind.AccessRequestApproved,
    createdDate: new Date(Date.now() - 4 * 60 * 1000), // 4 minutes ago
    clicked: false,
    labels: [
      {
        name: 'requested-role',
        value: 'auditor',
      },
      { name: 'reviewer', value: 'joe' },
    ],
  },
  {
    id: '3',
    title: '',
    description: '',
    subKind: NotificationSubKind.AccessRequestDenied,
    createdDate: new Date(Date.now() - 15 * 60 * 1000), // 15 minutes ago
    clicked: false,
    labels: [
      {
        name: 'requested-role',
        value: 'auditor',
      },
      { name: 'reviewer', value: 'joe' },
    ],
  },
  {
    id: '4',
    title: '',
    description: '',
    subKind: NotificationSubKind.AccessRequestPending,
    createdDate: new Date(Date.now() - 3 * 60 * 60 * 1000), // 3 hours ago
    clicked: true,
    labels: [
      {
        name: 'requested-resources',
        value: 'db-2,node-5',
      },
      { name: 'requester', value: 'bob' },
    ],
  },
  {
    id: '5',
    title: '',
    description: '',
    subKind: NotificationSubKind.AccessRequestPending,
    createdDate: new Date(Date.now() - 15 * 60 * 60 * 1000), // 15 hours ago
    clicked: true,
    labels: [
      {
        name: 'requested-role',
        value: 'intern',
      },
      { name: 'requester', value: 'bob' },
    ],
  },
  {
    id: '6',
    title: '',
    description: '',
    subKind: NotificationSubKind.AccessRequestNowAssumable,
    createdDate: new Date(Date.now() - 24 * 60 * 60 * 1000), // 1 day ago
    clicked: true,
    labels: [
      {
        name: 'requested-resources',
        value: 'db-2,node-5',
      },
      { name: 'requester', value: 'bob' },
    ],
  },
  {
    id: '7',
    title: '',
    description: '',
    subKind: NotificationSubKind.AccessRequestNowAssumable,
    createdDate: new Date(Date.now() - 3 * 24 * 60 * 60 * 1000), // 3 days ago
    clicked: false,
    labels: [
      {
        name: 'requested-resources',
        value: 'node-5',
      },
      { name: 'requester', value: 'bob' },
    ],
  },
  {
    id: '8',
    title: '',
    description: '',
    subKind: NotificationSubKind.AccessRequestNowAssumable,
    createdDate: new Date(Date.now() - 2 * 7 * 24 * 60 * 60 * 1000), // 2 weeks ago
    clicked: true,
    labels: [
      {
        name: 'requested-role',
        value: 'auditor',
      },
      { name: 'requester', value: 'bob' },
    ],
  },
  {
    id: '9',
    title: 'This is an example user-created warning notification',
    description: 'This is the text content of a warning notification.',
    subKind: NotificationSubKind.UserCreatedWarning,
    createdDate: new Date(Date.now() - 93 * 24 * 60 * 60 * 1000), // 3 months ago
    clicked: true,
    labels: [],
  },
];
