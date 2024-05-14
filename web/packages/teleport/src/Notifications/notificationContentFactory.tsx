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

/**
 notificationContentFactory produces the content for notifications for OSS features.
*/
export function notificationContentFactory({
  subKind,
  labels,
  ...notification
}: NotificationType): NotificationContent {
  let notificationContent: NotificationContent;

  switch (subKind) {
    case NotificationSubKind.DefaultInformational:
    case NotificationSubKind.UserCreatedInformational: {
      const textContent = getLabelValue(labels, 'content');
      notificationContent = {
        kind: 'text',
        title: notification.title,
        textContent,
        type: 'informational',
        icon: Icons.Notification,
      };
      break;
    }

    case NotificationSubKind.DefaultWarning:
    case NotificationSubKind.UserCreatedWarning: {
      const textContent = getLabelValue(labels, 'content');
      notificationContent = {
        kind: 'text',
        title: notification.title,
        textContent,
        type: 'warning',
        icon: Icons.Notification,
      };
      break;
    }
    default:
      return null;
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
export function getLabelValue(labels: Label[], key: string): string {
  const label = labels.find(label => {
    return label.name === key;
  });
  return label?.value || '';
}
