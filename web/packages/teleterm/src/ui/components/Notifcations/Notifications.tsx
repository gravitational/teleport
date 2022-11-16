import React from 'react';
import styled from 'styled-components';
import { Notification } from 'shared/components/Notification';
import { Info, Warning } from 'design/Icon';

import type { NotificationItem } from 'shared/components/Notification';

interface NotificationsProps {
  items: NotificationItem[];

  onRemoveItem(id: string): void;
}

const notificationConfig: Record<
  NotificationItem['severity'],
  { Icon: React.ElementType; getColor(theme): string; isAutoRemovable: boolean }
> = {
  error: {
    Icon: Warning,
    getColor: theme => theme.colors.danger,
    isAutoRemovable: false,
  },
  warn: {
    Icon: Warning,
    getColor: theme => theme.colors.warning,
    isAutoRemovable: true,
  },
  info: {
    Icon: Info,
    getColor: theme => theme.colors.info,
    isAutoRemovable: true,
  },
};

export function Notifications(props: NotificationsProps) {
  return (
    <Container>
      {props.items.map(item => (
        <Notification
          style={{ marginBottom: '12px' }}
          key={item.id}
          item={item}
          onRemove={() => props.onRemoveItem(item.id)}
          {...notificationConfig[item.severity]}
        />
      ))}
    </Container>
  );
}

const Container = styled.div`
  position: absolute;
  bottom: 0;
  right: 12px;
  z-index: 10;
`;
