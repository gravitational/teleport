import { ImmutableStore } from 'teleterm/ui/services/immutableStore';
import {
  NotificationItem,
  NotificationItemContent,
} from 'teleterm/ui/components/Notifcations';
import { useStore } from 'shared/libs/stores';
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
