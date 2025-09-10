/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
  createContext,
  FC,
  PropsWithChildren,
  useCallback,
  useContext,
  useState,
} from 'react';

import {
  ToastNotificationItem,
  ToastNotificationItemContent,
  ToastNotificationSeverity,
} from './types';

export type NotificationEntry = {
  severity: ToastNotificationSeverity;
  content: ToastNotificationItemContent;
};

type ToastNotificationContextState = {
  notifications: ToastNotificationItem[];
  /**
   * remove a notification matching id.
   */
  remove(id: string): void;
  /**
   * adds new notification to the beginning of
   * an existing list of notifications.
   */
  add(entry: NotificationEntry): void;
};

const ToastNotificationContext =
  createContext<ToastNotificationContextState>(null);

/**
 * Provider for adding, removing, and listing toast notifications.
 */
export const ToastNotificationProvider: FC<PropsWithChildren> = ({
  children,
}) => {
  const [notifications, setNotifications] = useState<ToastNotificationItem[]>(
    []
  );

  const remove = useCallback((id: string) => {
    setNotifications(n => n.filter(item => item.id !== id));
  }, []);

  const add = useCallback((entry: NotificationEntry) => {
    setNotifications(n => [
      {
        id: crypto.randomUUID(),
        content: entry.content,
        severity: entry.severity,
      },
      ...n,
    ]);
  }, []);

  return (
    <ToastNotificationContext.Provider value={{ notifications, add, remove }}>
      {children}
    </ToastNotificationContext.Provider>
  );
};

/**
 * useToastNotifications allows you to add to or remove from a
 * list of notifications from ToastNotificationContext.
 */
export function useToastNotifications() {
  const context = useContext(ToastNotificationContext);

  if (!context) {
    throw new Error(
      'useToastNotifications must be used within a ToastNotificationProvider'
    );
  }

  return context;
}
