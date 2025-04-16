/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import styled from 'styled-components';

import {
  Notification,
  type NotificationItem,
} from 'shared/components/Notification';

interface NotificationsProps {
  items: NotificationItem[];

  onRemoveItem(id: string): void;
}

export function Notifications(props: NotificationsProps) {
  return (
    <Container>
      {props.items.map(item => (
        <Notification
          mb={3}
          key={item.id}
          item={item}
          onRemove={() => props.onRemoveItem(item.id)}
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
