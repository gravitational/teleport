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

import {
  FetchNotificationsResponse,
  UpsertLastSeenNotificationRequest,
  UpsertNotificationStateRequest,
} from './types';

export class NotificationService {
  fetchNotifications(
    params: UrlNotificationParams
  ): Promise<FetchNotificationsResponse> {
    if (params.startKey === '') {
      params.startKey = undefined;
    }

    return api.get(cfg.getNotificationsUrl(params)).then(json => {
      return {
        notifications: json.notifications
          ? json.notifications.map(notificationJson => {
              const { id, title, subKind, created, clicked, textContent } =
                notificationJson;
              const labels = notificationJson.labels || [];

              return {
                id,
                title,
                subKind,
                createdDate: new Date(created),
                clicked,
                labels,
                textContent,
              };
            })
          : [],
        nextKey: json.nextKey,
        userLastSeenNotification: json.userLastSeenNotification
          ? new Date(json.userLastSeenNotification)
          : undefined,
      };
    });
  }

  upsertLastSeenNotificationTime(
    clusterId: string,
    req: UpsertLastSeenNotificationRequest
  ): Promise<UpsertLastSeenNotificationRequest> {
    return api
      .put(cfg.getNotificationLastSeenUrl(clusterId), req)
      .then((res: UpsertLastSeenNotificationRequest) => ({
        time: new Date(res.time),
      }));
  }

  upsertNotificationState(
    clusterId: string,
    req: UpsertNotificationStateRequest
  ): Promise<UpsertNotificationStateRequest> {
    return api.put(cfg.getNotificationStateUrl(clusterId), req);
  }
}
