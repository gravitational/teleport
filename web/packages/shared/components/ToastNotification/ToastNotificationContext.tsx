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
  useContext,
  useMemo,
  useState,
} from 'react';

import {
  ToastNotificationItem,
  ToastNotificationItemContent,
  ToastNotificationSeverity,
} from './types';

type ToastNotificationContextState = {
  notifications: ToastNotificationItem[];
  removeNotification(id: string): void;
  addNotification(
    severity: ToastNotificationSeverity,
    content: ToastNotificationItemContent
  ): void;
};

const ToastNotificationContext =
  createContext<ToastNotificationContextState>(null);

export const ToastNotificationProvider: FC<PropsWithChildren> = ({
  children,
}) => {
  const [notifications, setNotifications] = useState<ToastNotificationItem[]>(
    []
  );

  function removeNotification(id: string) {
    setNotifications(n => n.filter(item => item.id !== id));
  }

  function addNotification(
    severity: ToastNotificationSeverity,
    content: ToastNotificationItemContent
  ) {
    setNotifications(notifications => [
      ...notifications,
      { id: crypto.randomUUID(), content, severity },
    ]);
  }

  const providerValue = useMemo(() => {
    return {
      notifications,
      removeNotification,
      addNotification,
    };
  }, [notifications]);

  return (
    <ToastNotificationContext.Provider value={providerValue}>
      {children}
    </ToastNotificationContext.Provider>
  );
};

export function useToastNotifications() {
  const context = useContext(ToastNotificationContext);

  if (!context) {
    throw new Error(
      'useToastNotifications must be used within a ToastNotificationProvider'
    );
  }

  return context;
}

export type AddNotification = (
  severity: ToastNotificationSeverity,
  content: ToastNotificationItemContent
) => void;
