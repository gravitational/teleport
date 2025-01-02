import { createPortal } from 'react-dom';
import styled from 'styled-components';

import {
  NotificationItem as InnerNotificationItem,
  Notification,
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
 * via a portal.
 */
const Container = styled.div`
  position: absolute;
  bottom: 12px;
  right: 12px;
  z-index: 10000;
`;

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
 * use a portal to render to the document root so notifications can be
 * properly anchored to the bottom right of the page.
 */
export function Notifications({
  items,
  dismiss,
  prefix = 'notification-',
}: NotificationsProps) {
  return createPortal(
    <Container>
      {items.map(item => (
        <Notification
          key={`${prefix}${item.id}`}
          item={item}
          onRemove={() => dismiss(item.id)}
          {...dismissAfterProps(item)}
        />
      ))}
    </Container>,
    document.body
  );
}
