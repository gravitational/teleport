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

import cfg, { UrlNotificationParams } from 'teleport/config';
import api from 'teleport/services/api';

import { FetchNotificationsResponse } from './types';

export class NotificationService {
  fetchNotifications(
    params: UrlNotificationParams
  ): Promise<FetchNotificationsResponse> {
    if (params.userNotificationsStartKey === '') {
      params.userNotificationsStartKey = undefined;
    }
    if (params.globalNotificationsStartKey === '') {
      params.globalNotificationsStartKey = undefined;
    }

    return api.get(cfg.getNotificationsUrl(params)).then(json => {
      return {
        notifications:
          json.notifications.map(notificationJson => {
            const { id, title, subKind, created, clicked } = notificationJson;
            const labels = notificationJson.labels || [];

            return {
              id,
              title,
              subKind,
              createdDate: new Date(created),
              clicked,
              labels,
            };
          }) || [],
        userNotificationsNextKey: json.userNotificationsNextKey || '',
        globalNotificationsNextKey: json.globalNotificationsNextKey || '',
        userLastSeenNotification: json.userLastSeenNotification
          ? new Date(json.userLastSeenNotification)
          : undefined,
      };
    });
  }
}
