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
  ToastNotificationItem,
  ToastNotificationItemContent,
} from 'shared/components/ToastNotification';

import { ImmutableStore } from 'teleterm/ui/services/immutableStore';
import { unique } from 'teleterm/ui/utils/uid';

export type NotificationContent = ToastNotificationItemContent & {
  key?: NotificationKey;
};
export type NotificationKey = string | (string | number)[];

type State = Map<string, ToastNotificationItem>;

export class NotificationsService extends ImmutableStore<State> {
  state: State = new Map();

  /**
   * Adds a notification with error severity.
   *
   * If key is passed, replaces any previous notification with equal key. If an array is passed as
   * a key, the array is joined with '-'.
   */
  notifyError(rawContent: NotificationContent): string {
    const severity = 'error';
    if (typeof rawContent === 'string') {
      return this.notify({ severity, content: rawContent });
    }

    const { key, ...content } = rawContent;
    return this.notify({ severity, content, key });
  }

  /**
   * Adds a notification with warn severity.
   *
   * If key is passed, replaces any previous notification with equal key. If an array is passed as
   * a key, the array is joined with '-'.
   */
  notifyWarning(rawContent: NotificationContent): string {
    const severity = 'warn';
    if (typeof rawContent === 'string') {
      return this.notify({ severity, content: rawContent });
    }

    const { key, ...content } = rawContent;
    return this.notify({ severity, content, key });
  }

  /**
   * Adds a notification with info severity.
   *
   * If key is passed, replaces any previous notification with equal key. If an array is passed as
   * a key, the array is joined with '-'.
   */
  notifyInfo(rawContent: NotificationContent): string {
    const severity = 'info';
    if (typeof rawContent === 'string') {
      return this.notify({ severity, content: rawContent });
    }

    const { key, ...content } = rawContent;
    return this.notify({ severity, content, key });
  }

  removeNotification(id: string): void {
    if (!id) {
      return;
    }

    if (this.state.size === 0) {
      return;
    }

    this.setState(draftState => {
      draftState.delete(id);
    });
  }

  getNotifications(): ToastNotificationItem[] {
    return [...this.state.values()];
  }

  hasNotification(id: string): boolean {
    return this.state.has(id);
  }

  private notify(
    options: Omit<ToastNotificationItem, 'id'> & { key?: NotificationKey }
  ): string {
    const id = options.key
      ? typeof options.key === 'string'
        ? options.key
        : options.key.join('-')
      : unique();

    this.setState(draftState => {
      draftState.set(id, {
        severity: options.severity,
        content: options.content,
        id,
      });
    });

    return id;
  }
}
