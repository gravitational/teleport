import React from 'react';

import styled from 'styled-components';

import { NotificationItem } from './types';
import { Notification } from './Notification';

interface NotificationsProps {
  items: NotificationItem[];

  onRemoveItem(id: string): void;
}

export function Notifications(props: NotificationsProps) {
  return (
    <Container>
      {props.items.map(item => (
        <Notification
          key={item.id}
          item={item}
          onRemove={() => props.onRemoveItem(item.id)}
        />
      ))}
    </Container>
  );
}

const Container = styled.div`
  position: fixed;
  bottom: 12px;
  right: 12px;
  z-index: 10;
`;
