export interface NotificationItem {
  content: NotificationItemContent;
  severity: 'info' | 'warn' | 'error';
  id: string;
}

export type NotificationItemContent =
  | string
  | { title: string; description: string }