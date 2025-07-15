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

import { createContext, useCallback, useContext, useState } from 'react';
import styled from 'styled-components';

import {
  Notification,
  NotificationItem,
  NotificationItemContent,
  NotificationSeverity,
} from 'shared/components/Notification';

interface NotificationContextType {
  addNotification: (
    severity: NotificationSeverity,
    content: NotificationItemContent
  ) => void;
}

// A context to be used by components that only need to add notifications and should
// not re-render when notification state changes.
const AddNotificationContext = createContext<
  NotificationContextType | undefined
>(undefined);

// Internal context for rendering notification component.
interface NotificationContext extends NotificationContextType {
  removeNotification: (id: string) => void;
  notifications: NotificationItem[];
}
const NotificationContext = createContext<NotificationContext | undefined>(
  undefined
);

export const useNotification = () => {
  const context = useContext(AddNotificationContext);
  if (!context) {
    throw new Error(
      'useNotification must be used within a NotificationProvider'
    );
  }
  return context;
};

interface NotificationProviderProps {
  children: React.ReactNode;
}

export const NotificationProvider = ({
  children,
}: NotificationProviderProps) => {
  const [notifications, setNotifications] = useState<NotificationItem[]>([]);

  const addNotification = useCallback(
    (severity: NotificationSeverity, content: NotificationItemContent) => {
      setNotifications(n => [
        ...n,
        {
          id: crypto.randomUUID(),
          severity,
          content,
        },
      ]);
    },
    []
  );

  const removeNotification = useCallback((id: string) => {
    setNotifications(n => n.filter(item => item.id !== id));
  }, []);

  return (
    <NotificationContext.Provider
      value={{ notifications, removeNotification, addNotification }}
    >
      <AddNotificationContext.Provider value={{ addNotification }}>
        {children}
      </AddNotificationContext.Provider>
    </NotificationContext.Provider>
  );
};

export const NotificationOutlet = () => {
  const context = useContext(NotificationContext);
  if (!context) {
    throw new Error(
      'NotificationOutlet must be used within a NotificationProvider'
    );
  }
  const { notifications, removeNotification } = context;

  if (!notifications.length) {
    return null;
  }

  return (
    <NotificationContainer>
      {notifications.map(item => (
        <Notification
          mb={3}
          key={item.id}
          item={item}
          onRemove={() => removeNotification(item.id)}
        />
      ))}
    </NotificationContainer>
  );
};

const NotificationContainer = styled.div`
  position: absolute;
  top: ${props => props.theme.space[2]}px;
  right: ${props => props.theme.space[5]}px;
  z-index: 1000;
`;
