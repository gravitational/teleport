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

import { Label } from 'teleport/types';

export type FetchNotificationsResponse = {
  /**
   * notifications is the list of notification items.
   */
  notifications: Notification[];
  /**
   * nextKey is the next page keys for both lists (user-specific notifications & global notifications)
   * separated by a comma, ie. "<user-specific notifications nextKey>,<global notifications nextKey>".
   * If either one of these nextKeys is "end", it means we have reached the end of that list.
   */
  nextKey: string;
  /**
   * userLastSeenNotification is  the timestamp of the last notification the  user has seen.
   */
  userLastSeenNotification: Date;
};

/**
 * UpsertLastSeenNotificationRequest is the request to upsert the timestamp of the latest notification that
 * the user has seen.
 */
export type UpsertLastSeenNotificationRequest = {
  /**
   * time is the timestamp of the last seen notification.
   */
  time: Date;
};

/**
 * UpsertNotificationStateRequest is the request made when a user updates a notification's state by marking it
 * as "clicked" or "dismissed".
 */
export type UpsertNotificationStateRequest = {
  /**
   * notificationId is the id of the notification.
   */
  notificationId: string;
  /**
   * notificationState is the state to upsert, either "CLICKED" or "DISMISSED".
   */
  notificationState: NotificationState;
};

export type Notification = {
  /** id is the uuid of this notification */
  id: string;
  /* subKind is a string which represents which type of notification this is, ie. "access-request-approved"*/
  subKind: NotificationSubKind;
  /** createdDate is when the notification was created. */
  createdDate: Date;
  /** clicked is whether this notification has been clicked on by this user. */
  clicked: boolean;
  /** labels are this notification's labels, this is where the notification's metadata is stored.*/
  labels: Label[];
  /** title is the title of this notification. This can be overwritten in notificationContentFactory if needed. */
  title: string;
  /** textContent is the text content of this notification if it is merely a text notification (such as one created via `tctl notifications create`).
   * This is the text that will be displayed in a dialog upon clicking the notification.
   */
  textContent?: string;
};

/** NotificationSubKind is the subkind of notifications, these should be kept in sync with the values in api/types/constants.go */
export enum NotificationSubKind {
  DefaultInformational = 'default-informational',
  DefaultWarning = 'default-warning',

  UserCreatedInformational = 'user-created-informational',
  UserCreatedWarning = 'user-created-warning',

  AccessRequestPending = 'access-request-pending',
  AccessRequestApproved = 'access-request-approved',
  AccessRequestDenied = 'access-request-denied',
  AccessRequestPromoted = 'access-request-promoted',

  NotificationAccessListReviewDue14d = 'access-list-review-due-14d',
  NotificationAccessListReviewDue7d = 'access-list-review-due-7d',
  NotificationAccessListReviewDue3d = 'access-list-review-due-3d',
  NotificationAccessListReviewDue0d = 'access-list-review-due-0d',
  NotificationAccessListReviewOverdue3d = 'access-list-review-overdue-3d',
  NotificationAccessListReviewOverdue7d = 'access-list-review-overdue-7d',
}

/**
 * NotificationState the state of a notification for a user. This can represent either "clicked" or "dismissed".
 *
 * This should be kept in sync with teleport.notifications.v1.NotificationState.
 */
export enum NotificationState {
  UNSPECIFIED = 0,
  /**
   * NOTIFICATION_STATE_CLICKED marks this notification as having been clicked on by the user.
   */
  CLICKED = 1,
  /**
   * NOTIFICATION_STATE_DISMISSED marks this notification as having been dismissed by the user.
   */
  DISMISSED = 2,
}
