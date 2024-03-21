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

import * as Icons from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';
import React from 'react';

import {
  Notification as NotificationType,
  NotificationSubKind,
} from 'teleport/services/notifications';
import { Label } from 'teleport/types';

export function notificationContentFactory({
  subKind,
  description,
  labels,
  ...notification
}: NotificationType): NotificationContent {
  let notificationContent: NotificationContent;

  switch (subKind) {
    case NotificationSubKind.DefaultInformational:
    case NotificationSubKind.UserCreatedInformational:
      notificationContent = {
        kind: 'text',
        title: notification.title,
        textContent: description,
        type: 'informational',
        icon: Icons.Notification,
      };
      break;

    case NotificationSubKind.DefaultWarning:
    case NotificationSubKind.UserCreatedWarning:
      notificationContent = {
        kind: 'text',
        title: notification.title,
        textContent: description,
        type: 'warning',
        icon: Icons.Notification,
      };
      break;

    case NotificationSubKind.AccessRequestApproved: {
      let title;

      const reviewer = getLabelValue(labels, 'reviewer');
      const requestedResources = getLabelValue(labels, 'requested-resources');
      const numRequestedResources = requestedResources.length
        ? requestedResources.split(',').length
        : 0;

      // Check if it is a resource request or a role request.
      if (numRequestedResources) {
        title = `${reviewer} approved your access request for ${numRequestedResources} resource${
          numRequestedResources > 1 ? 's' : ''
        }.`;
      } else {
        const requestedRole = getLabelValue(labels, 'requested-role');
        title = `${reviewer} approved your access request for the '${requestedRole}' role.`;
      }

      notificationContent = {
        kind: 'redirect',
        title,
        type: 'success',
        icon: Icons.Users,
        redirectRoute: '/', //TODO: rudream - handle enterprise routes
        quickAction: {
          onClick: () => null, //TODO: rudream - handle assuming roles from quick action button
          buttonText: 'Assume Roles',
        },
      };
      break;
    }

    case NotificationSubKind.AccessRequestDenied: {
      let title;

      const reviewer = getLabelValue(labels, 'reviewer');
      const requestedResources = getLabelValue(labels, 'requested-resources');
      const numRequestedResources = requestedResources.length
        ? requestedResources.split(',').length
        : 0;

      // Check if it is a resource request or a role request.
      if (numRequestedResources) {
        title = `${reviewer} denied your access request for ${numRequestedResources} resource${
          numRequestedResources > 1 ? 's' : ''
        }.`;
      } else {
        const requestedRole = getLabelValue(labels, 'requested-role');
        title = `${reviewer} denied your access request for the '${requestedRole}' role.`;
      }

      notificationContent = {
        kind: 'redirect',
        title,
        type: 'failure',
        icon: Icons.Users,
        redirectRoute: '/', //TODO: rudream - handle enterprise routes
      };
      break;
    }

    case NotificationSubKind.AccessRequestPending: {
      let title;

      const requester = getLabelValue(labels, 'requester');
      const requestedResources = getLabelValue(labels, 'requested-resources');
      const numRequestedResources = requestedResources.length
        ? requestedResources.split(',').length
        : 0;

      // Check if it is a resource request or a role request.
      if (numRequestedResources) {
        title = `${requester} requested access to ${numRequestedResources} resource${
          numRequestedResources > 1 ? 's' : ''
        }.`;
      } else {
        const requestedRole = getLabelValue(labels, 'requested-role');
        title = `${requester} requested access to the '${requestedRole}' role.`;
      }

      notificationContent = {
        kind: 'redirect',
        title,
        type: 'informational',
        icon: Icons.UserList,
        redirectRoute: '/', //TODO: rudream - handle enterprise routes
      };
      break;
    }

    case NotificationSubKind.AccessRequestNowAssumable: {
      let title;
      let buttonText;

      const requestedResources = getLabelValue(labels, 'requested-resources');
      const numRequestedResources = requestedResources.length
        ? requestedResources.split(',').length
        : 0;

      // Check if it is a resource request or a role request.
      if (numRequestedResources) {
        if (numRequestedResources === 1) {
          title = `"${requestedResources}" is now available to access.`;
        } else {
          title = `${numRequestedResources} resources are now available to access.`;
        }
        buttonText = 'Access Now';
      } else {
        const requestedRole = getLabelValue(labels, 'requested-role');
        title = `"${requestedRole}" is now ready to assume.`;
        buttonText = 'Assume Role';
      }

      notificationContent = {
        kind: 'redirect',
        title,
        type: 'success-alt',
        icon: Icons.Users,
        redirectRoute: '/', //TODO: rudream - handle enterprise routes
        quickAction: {
          onClick: () => null, //TODO: rudream - handle assuming roles from quick action button
          buttonText: buttonText,
        },
      };
      break;
    }
  }

  return notificationContent;
}

type NotificationContentBase = {
  /** title is the title of the notification. */
  title: string;
  /** type is the type of notification this is, this will determine the style of this notification (color and sub-icon). */
  type: 'success' | 'success-alt' | 'informational' | 'warning' | 'failure';
  /** icon is the icon to render for this notification. This should be an icon from `design/Icon`. */
  icon: React.FC<IconProps>;
};

/** For notifications that are clickable and redirect you to a page, and may also optionally include a quick action. */
type NotificationContentRedirect = NotificationContentBase & {
  kind: 'redirect';
  /** redirectRoute is the route the user should be redirected to when clicking the notification, if any. */
  redirectRoute: string;
  quickAction?: {
    /** onClick is what should be run when the user clicks on the quick action button */
    onClick: () => void;
    /** buttonText is the text that should be shown on the quick action button */
    buttonText: string;
  };
};

/** For notifications that only contain text and are not interactive in any other way. This is used for user-created notifications. */
type NotificationContentText = NotificationContentBase & {
  kind: 'text';
  /** textContent is the text content of the notification, this is used for user-created notifications and will contain the text that will be shown in a modal upon clicking the notification. */
  textContent: string;
};

export type NotificationContent =
  | NotificationContentRedirect
  | NotificationContentText;

// getLabelValue returns the value of a label for a given key.
function getLabelValue(labels: Label[], key: string): string {
  const label = labels.find(label => {
    return label.name === key;
  });
  return label?.value || '';
}
