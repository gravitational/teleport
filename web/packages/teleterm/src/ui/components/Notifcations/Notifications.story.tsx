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

import React, { useState } from 'react';
import { ButtonPrimary, Flex } from 'design';

import { unique } from 'teleterm/ui/utils/uid';

import { Notifications } from '.';

import type { NotificationItem } from '@gravitational/shared/components/Notification';

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

  function notify(severity: NotificationItem['severity']) {
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

  function notify(severity: NotificationItem['severity']) {
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
