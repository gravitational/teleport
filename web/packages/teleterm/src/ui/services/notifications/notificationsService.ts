/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useStore } from 'shared/libs/stores';

import { ImmutableStore } from 'teleterm/ui/services/immutableStore';
import { unique } from 'teleterm/ui/utils/uid';

import type {
  NotificationItem,
  NotificationItemContent,
} from 'shared/components/Notification';

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
