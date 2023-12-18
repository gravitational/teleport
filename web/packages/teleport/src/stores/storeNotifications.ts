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

import { Store } from 'shared/libs/stores';
import { assertUnreachable } from 'shared/utils/assertUnreachable';

export enum NotificationKind {
  AccessList = 'access-list',
}

type AccessListNotification = {
  kind: NotificationKind.AccessList;
  resourceName: string;
  route: string;
};

export type Notification = {
  item: AccessListNotification;
  id: string;
  date: Date;
};

// TODO?: based on a feedback, consider representing
// notifications as a collection of maps indexed by id
// which is then converted to a sorted list as needed
// (may be easier to work with)
export type NotificationState = {
  notifications: Notification[];
};

const defaultNotificationState: NotificationState = {
  notifications: [],
};

export class StoreNotifications extends Store<NotificationState> {
  state: NotificationState = defaultNotificationState;

  getNotifications() {
    return this.state.notifications;
  }

  setNotifications(notices: Notification[]) {
    // Sort by earliest dates.
    const sortedNotices = notices.sort((a, b) => {
      return a.date.getTime() - b.date.getTime();
    });
    this.setState({ notifications: [...sortedNotices] });
  }

  updateNotificationsByKind(notices: Notification[], kind: NotificationKind) {
    switch (kind) {
      case NotificationKind.AccessList:
        const filtered = this.state.notifications.filter(
          n => n.item.kind !== NotificationKind.AccessList
        );
        this.setNotifications([...filtered, ...notices]);
        return;
      default:
        assertUnreachable(kind);
    }
  }

  hasNotificationsByKind(kind: NotificationKind) {
    switch (kind) {
      case NotificationKind.AccessList:
        return this.getNotifications().some(
          n => n.item.kind === NotificationKind.AccessList
        );
      default:
        assertUnreachable(kind);
    }
  }
}
