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

import type {
  NotificationItem,
  NotificationItemContent,
} from 'shared/components/Notification';
import { useStore } from 'shared/libs/stores';

import { ImmutableStore } from 'teleterm/ui/services/immutableStore';
import { unique } from 'teleterm/ui/utils/uid';

export class NotificationsService extends ImmutableStore<NotificationItem[]> {
  state: NotificationItem[] = [];

  notifyError(content: NotificationItemContent): string {
    return this.notify({ severity: 'error', content });
  }

  notifyWarning(content: NotificationItemContent): string {
    return this.notify({ severity: 'warn', content });
  }

  notifyInfo(content: NotificationItemContent): string {
    return this.notify({ severity: 'info', content });
  }

  removeNotification(id: string): void {
    this.setState(draftState =>
      draftState.filter(stateItem => stateItem.id !== id)
    );
  }

  getNotifications(): NotificationItem[] {
    return this.state;
  }

  useState(): NotificationItem[] {
    return useStore(this).state;
  }

  private notify(options: Omit<NotificationItem, 'id'>): string {
    const id = unique();

    this.setState(draftState => {
      draftState.push({
        severity: options.severity,
        content: options.content,
        id,
      });
    });

    return id;
  }
}
