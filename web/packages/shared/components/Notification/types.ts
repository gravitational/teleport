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

import { Action } from 'design/Alert';
import { IconProps } from 'design/Icon/Icon';

export type NotificationSeverity =
  | 'info'
  | 'warn'
  | 'error'
  | 'success'
  | 'neutral';

export interface NotificationItem {
  content: NotificationItemContent;
  severity: NotificationSeverity;
  id: string;
}

export type NotificationItemContent = string | NotificationItemObjectContent;

export type NotificationItemObjectContent = {
  title?: string;
  subtitle?: string;
  list?: string[];
  description?: string;
  icon?: React.ComponentType<IconProps>;
  action?: Action;
  /**
   * If defined, isAutoRemovable circumvents the auto-removing logic in the Notification component,
   * which automatically removes 'success', 'info', and 'neutral' notifications.
   */
  isAutoRemovable?: boolean;
};
