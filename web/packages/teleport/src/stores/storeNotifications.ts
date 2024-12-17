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

import { LocalNotificationKind } from 'teleport/services/notifications';
import { KeysEnum } from 'teleport/services/storageService';

type AccessListNotification = {
  kind: LocalNotificationKind.AccessList;
  resourceName: string;
  route: string;
};

export type Notification = {
  item: AccessListNotification;
  id: string;
  date: Date;
  clicked?: boolean;
};

// TODO?: based on a feedback, consider representing
// notifications as a collection of maps indexed by id
// which is then converted to a sorted list as needed
// (may be easier to work with)
export type NotificationStoreState = {
  notifications: Notification[];
};

const defaultNotificationStoreState: NotificationStoreState = {
  notifications: [],
};

export type LocalNotificationStates = {
  clicked: string[];
  seen: string[];
};

const defaultLocalNotificationStates: LocalNotificationStates = {
  /** clicked contains the IDs of notifications which have been clicked on. */
  clicked: [],
  /** seen contains the IDs of the notifications which have been seen in the notifications list, even if they were never clicked on.
   *  Opening the notifications list marks all notifications within it as seen.
   */
  seen: [],
};

export class StoreNotifications extends Store<NotificationStoreState> {
  state: NotificationStoreState = defaultNotificationStoreState;

  getNotifications(): Notification[] {
    const allNotifs = this.state.notifications;
    const notifStates = this.getNotificationStates();

    if (allNotifs.length === 0) {
      localStorage.removeItem(KeysEnum.LOCAL_NOTIFICATION_STATES);
      return [];
    }

    return allNotifs.map(notification => {
      // Mark clicked notifications as clicked.
      if (notifStates.clicked.indexOf(notification.id) !== -1) {
        return {
          ...notification,
          clicked: true,
        };
      }
      return notification;
    });
  }

  setNotifications(notices: Notification[]) {
    // Sort by earliest dates.
    const sortedNotices = notices.sort((a, b) => {
      return a.date.getTime() - b.date.getTime();
    });
    this.setState({ notifications: [...sortedNotices] });
  }

  updateNotificationsByKind(
    notices: Notification[],
    kind: LocalNotificationKind
  ) {
    switch (kind) {
      case LocalNotificationKind.AccessList:
        const filtered = this.state.notifications.filter(
          n => n.item.kind !== LocalNotificationKind.AccessList
        );
        this.setNotifications([...filtered, ...notices]);
        return;
      default:
        assertUnreachable(kind);
    }
  }

  hasNotificationsByKind(kind: LocalNotificationKind) {
    switch (kind) {
      case LocalNotificationKind.AccessList:
        return this.getNotifications().some(
          n => n.item.kind === LocalNotificationKind.AccessList
        );
      default:
        assertUnreachable(kind);
    }
  }

  getNotificationStates(): LocalNotificationStates {
    const value = window.localStorage.getItem(
      KeysEnum.LOCAL_NOTIFICATION_STATES
    );

    if (!value) {
      return defaultLocalNotificationStates;
    }

    try {
      return JSON.parse(value) as LocalNotificationStates;
    } catch {
      return defaultLocalNotificationStates;
    }
  }

  markNotificationAsClicked(id: string) {
    const currentStates = this.getNotificationStates();

    // If the notification is already marked as clicked, do nothing.
    if (currentStates.clicked.includes(id)) {
      return;
    }

    const updatedStates: LocalNotificationStates = {
      clicked: [...currentStates.clicked, id],
      seen: currentStates.seen,
    };

    localStorage.setItem(
      KeysEnum.LOCAL_NOTIFICATION_STATES,
      JSON.stringify(updatedStates)
    );
  }

  markNotificationsAsSeen(notificationIds: string[]) {
    const currentStates = this.getNotificationStates();

    // Only add new seen states that aren't already in the state, to prevent duplicates.
    const newSeenStates = notificationIds.filter(
      id => !currentStates.seen.includes(id)
    );

    const updatedStates: LocalNotificationStates = {
      clicked: currentStates.clicked,
      seen: [...currentStates.seen, ...newSeenStates],
    };

    localStorage.setItem(
      KeysEnum.LOCAL_NOTIFICATION_STATES,
      JSON.stringify(updatedStates)
    );
  }

  resetStatesForNotification(notificationId: string) {
    const currentStates = this.getNotificationStates();

    const updatedStates = { ...currentStates };

    // If there is a clicked state for this notification, remove it.
    if (currentStates.clicked.includes(notificationId)) {
      updatedStates.clicked.splice(
        currentStates.clicked.indexOf(notificationId),
        1
      );
    }

    // If there is a seen state for this notification, remove it.
    if (currentStates.seen.includes(notificationId)) {
      updatedStates.seen.splice(currentStates.seen.indexOf(notificationId), 1);
    }

    console.log(updatedStates);

    localStorage.setItem(
      KeysEnum.LOCAL_NOTIFICATION_STATES,
      JSON.stringify(updatedStates)
    );
  }
}
