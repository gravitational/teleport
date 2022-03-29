import React from 'react';
import { NotificationItem } from './types';
import { Notification } from './Notification';
import styled from 'styled-components';

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
`;
