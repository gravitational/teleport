/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { subSeconds, subMinutes, subHours, subMonths } from 'date-fns';

import { NotificationSubKind } from 'teleport/services/notifications';
import { Notification } from 'teleport/services/notifications';

export const notifications: Notification[] = [
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
  {
    id: '7',
    title: `joe promoted your access request to long-term access.`,
    subKind: NotificationSubKind.AccessRequestPromoted,
    createdDate: subMinutes(Date.now(), 4), // 4 minutes ago
    clicked: false,
    labels: [
      {
        name: 'request-id',
        value: '3bd7d71f-64ad-588a-988c-22f3853910fa',
      },
    ],
  },
];
