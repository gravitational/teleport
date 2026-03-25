/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import {
  NotificationContent,
  NotificationsService,
} from './notificationsService';

describe('using a key', () => {
  it('replaces previous notification with same key', () => {
    const service = new NotificationsService();
    const firstNotification: NotificationContent = {
      description: 'bar',
      key: 'foo-1',
    };
    service.notifyInfo(firstNotification);
    expect(service.getNotifications()).toEqual([
      expect.objectContaining({
        content: expect.objectContaining({ description: 'bar' }),
      }),
    ]);

    const secondNotification: NotificationContent = {
      description: 'baz',
      key: ['foo', 1],
    };
    service.notifyWarning(secondNotification);
    expect(service.getNotifications()).toEqual([
      expect.objectContaining({
        content: expect.objectContaining({ description: 'baz' }),
      }),
    ]);
  });
});
