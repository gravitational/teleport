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

import { useState } from 'react';

import type {
  NotificationItem,
  NotificationSeverity,
} from '@gravitational/shared/components/Notification';
import { ButtonPrimary, Flex } from 'design';

import { unique } from 'teleterm/ui/utils/uid';

import { Notifications } from '.';

export default {
  title: 'Teleterm/components/Notifications',
};

function useNotifications() {
  const [items, setItems] = useState<NotificationItem[]>([]);

  function removeItem(id: string) {
    setItems(prevItems => prevItems.filter(item => item.id !== id));
  }

  return { items, setItems, removeItem };
}

export const TitleAndDescriptionContent = () => {
  const { setItems, removeItem, items } = useNotifications();

  function notify(severity: NotificationSeverity) {
    setItems(prevItems => [
      ...prevItems,
      {
        id: unique(),
        severity,
        content: {
          title: 'Notification title',
          description:
            "Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s.",
        },
      },
    ]);
  }

  return (
    <Flex>
      <ButtonPrimary onClick={() => notify('info')} mr={1}>
        Info
      </ButtonPrimary>
      <ButtonPrimary onClick={() => notify('warn')} mr={1}>
        Warning
      </ButtonPrimary>
      <ButtonPrimary onClick={() => notify('error')} mr={1}>
        Error
      </ButtonPrimary>
      <Notifications items={items} onRemoveItem={removeItem} />
    </Flex>
  );
};

export const StringContent = () => {
  const { setItems, removeItem, items } = useNotifications();

  function notify(severity: NotificationSeverity) {
    setItems(prevItems => [
      ...prevItems,
      {
        id: unique(),
        severity,
        content:
          "Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s.",
      },
    ]);
  }

  return (
    <Flex>
      <ButtonPrimary onClick={() => notify('info')} mr={1}>
        Info
      </ButtonPrimary>
      <ButtonPrimary onClick={() => notify('warn')} mr={1}>
        Warning
      </ButtonPrimary>
      <ButtonPrimary onClick={() => notify('error')} mr={1}>
        Error
      </ButtonPrimary>
      <Notifications items={items} onRemoveItem={removeItem} />
    </Flex>
  );
};
