import React from 'react';
import styled, { useTheme } from 'styled-components';

import Portal from 'design/Modal/Portal';
import {
  Notification,
  NotificationItem as InnerNotificationItem,
} from 'shared/components/Notification';

export type NotificationItem = InnerNotificationItem & {
  dismissAfterMs?: number;
};

type NotificationsProps = {
  items: NotificationItem[];
  dismiss: (id: string) => void;
  prefix?: string;
};

/**
 * A simple wrapper around NotificationItem that doesn't require a unique ID,
 * since it is automatically generated.
 */
export type NotificationEntry = Omit<NotificationItem, 'id'>;

/**
 * A container element for the notifications widget that is anchored to the
 * bottom right of the page. It must be rendered on the document's root, usually
 * via a Portal.
 */
const Container = styled.div`
  position: absolute;
  bottom: 12px;
  right: 12px;
  z-index: 10000;
`;

/**
 * An empty element that renders nothing in place of an icon.
 */
const Empty = () => {
  // eslint-disable-next-line react/jsx-no-useless-fragment
  return <></>;
};

type dismissProps = {
  isAutoRemovable: boolean;
  autoRemoveDurationMs?: number;
};

function dismissAfterProps(item: NotificationItem): dismissProps {
  if (item.dismissAfterMs) {
    return {
      isAutoRemovable: true,
      autoRemoveDurationMs: item.dismissAfterMs,
    };
  }

  return {
    isAutoRemovable: false,
  };
}

/**
 * Creates a notifications widget. This widget can be placed anywhere and will
 * use a Portal element to render to the document root so notifications can be
 * properly anchored to the bottom right of the page.
 */
export function Notifications({
  items,
  dismiss,
  prefix = 'notification-',
}: NotificationsProps) {
  const theme = useTheme();

  // dummy getColor since we use a dummy Empty icon
  const getColor = () => '#000000';

  function getStyle(item: NotificationItem) {
    let background = theme.colors.elevated;
    let color = theme.colors.text.main;

    switch (item.severity) {
      case 'info':
        background = theme.colors.success.main;
        color = theme.colors.levels.sunken;
        break;
      case 'warn':
        background = theme.colors.warning.main;
        color = theme.colors.levels.sunken;
        break;
      case 'error':
        background = theme.colors.error.main;
        color = theme.colors.levels.sunken;
        break;
    }

    return {
      background,
      color,
      marginTop: theme.space[3],
    };
  }

  return (
    // Note: Empty <Portal> will refer to the document root.
    <Portal>
      <Container>
        {items.map(item => (
          <Notification
            css={getStyle(item)}
            key={`${prefix}${item.id}`}
            item={item}
            onRemove={() => dismiss(item.id)}
            Icon={Empty}
            getColor={getColor}
            {...dismissAfterProps(item)}
          />
        ))}
      </Container>
    </Portal>
  );
}
