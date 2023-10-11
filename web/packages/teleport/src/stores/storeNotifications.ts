/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
