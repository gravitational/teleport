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

export type Notification = {
  /** id is the uuid of this notification */
  id: string;
  /* subKind is a string which represents which type of notification this is, ie. "access-request-approved" */
  subKind: NotificationSubKind;
  /** createdDate is when the notification was created. */
  createdDate: Date;
  /** clicked is whether this notification has been clicked on by this user. */
  clicked: boolean;
  /** labels are this notification's labels, this is where the notification's metadata is stored.*/
  labels: Label[];
  /** title is the title of this notification. It is preferred to not use this and instead construct a title dynamically using metadata from the labels. */
  title: string;
  /** description is the description of this notification. It is preferred to not use this and instead construct a description dynamically using metadata from the labels. */
  description: string;
};

/** NotificationSubKind is the subkind of notifications, these should be kept in sync with TBD (TODO: rudream - add backend counterpart location here) */
export enum NotificationSubKind {
  DefaultInformational = 'default-informational',
  DefaultWarning = 'default-warning',
  UserCreatedInformational = 'user-created-informational',
  UserCreatedWarning = 'user-created-warning',
  AccessRequestPending = 'access-request-pending',
  AccessRequestApproved = 'access-request-approved',
  AccessRequestDenied = 'access-request-denied',
  /** AccessRequestNowAssumable is the notification for when an approved access request that was scheduled for a later date is now assumable. */
  AccessRequestNowAssumable = 'access-request-now-assumable',
}
