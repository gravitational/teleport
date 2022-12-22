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
