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
    getColor: theme => theme.colors.warning.main,
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
