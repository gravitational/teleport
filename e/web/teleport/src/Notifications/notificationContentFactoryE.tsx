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

import * as Icons from 'design/Icon';
import Logger from 'shared/libs/logger';

const logger = Logger.create('Notifications');

import {
  Notification as NotificationType,
  NotificationSubKind,
} from 'teleport/services/notifications';
import {
  notificationContentFactory,
  NotificationContent,
  getLabelValue,
} from 'teleport/Notifications/notificationContentFactory';

import cfg from 'e-teleport/config';

/**
 notificationContentFactoryE produces the content for notifications for enterprise-only features.
 */
export function notificationContentFactoryE(
  notification: NotificationType
): NotificationContent {
  let notificationContent: NotificationContent;
  const { labels, subKind } = notification;

  // If it's an OSS notification, the OSS notification content factory can process it.
  notificationContent = notificationContentFactory(notification);
  if (notificationContent) {
    return notificationContent;
  }

  switch (subKind) {
    case NotificationSubKind.AccessRequestApproved: {
      const requestId = getLabelValue(labels, 'request-id');

      notificationContent = {
        kind: 'redirect',
        title: notification.title,
        type: 'success',
        icon: Icons.Users,
        redirectRoute: cfg.getAccessRequestRoute(requestId),
        quickAction: {
          onClick: () => null, //TODO: rudream - handle assuming roles from quick action button
          buttonText: 'Assume Roles',
        },
      };
      break;
    }

    case NotificationSubKind.AccessRequestDenied: {
      const requestId = getLabelValue(labels, 'request-id');

      notificationContent = {
        kind: 'redirect',
        title: notification.title,
        type: 'failure',
        icon: Icons.Users,
        redirectRoute: cfg.getAccessRequestRoute(requestId),
      };
      break;
    }

    case NotificationSubKind.AccessRequestPending: {
      const requestId = getLabelValue(labels, 'request-id');

      notificationContent = {
        kind: 'redirect',
        title: notification.title,
        type: 'informational',
        icon: Icons.UserList,
        redirectRoute: cfg.getAccessRequestRoute(requestId),
      };
      break;
    }

    case NotificationSubKind.AccessRequestNowAssumable: {
      let buttonText;

      const accessRequestType = getLabelValue(labels, 'request-type');

      if (accessRequestType === 'resource') {
        buttonText = 'Access Now';
      } else {
        buttonText = 'Assume Role';
      }

      const requestId = getLabelValue(labels, 'request-id');

      notificationContent = {
        kind: 'redirect',
        title: notification.title,
        type: 'success-alt',
        icon: Icons.Users,
        redirectRoute: cfg.getAccessRequestRoute(requestId),
        quickAction: {
          onClick: () => null, //TODO: rudream - handle assuming roles from quick action button
          buttonText: buttonText,
        },
      };
      break;
    }

    default:
      // If neither the OSS content factory nor this one was able to process it, there is a bug.
      logger.error(
        `Notification with title "${notification.title}" was unable to be processed.`
      );
      return null;
  }

  return notificationContent;
}
