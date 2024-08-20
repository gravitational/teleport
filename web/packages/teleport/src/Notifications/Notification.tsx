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

import React, { useState } from 'react';
import styled from 'styled-components';
import { formatDistanceToNowStrict } from 'date-fns';

import * as Icons from 'design/Icon';

import Text, { P3 } from 'design/Text';
import { ButtonSecondary } from 'design/Button';

import { MenuIcon, MenuItem } from 'shared/components/MenuAction';
import { IGNORE_CLICK_CLASSNAME } from 'shared/hooks/useRefClickOutside/useRefClickOutside';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { Theme } from 'design/theme/themes/types';

import { useAsync } from 'shared/hooks/useAsync';

import {
  Notification as NotificationType,
  NotificationState,
} from 'teleport/services/notifications';
import history from 'teleport/services/history';

import useStickyClusterId from 'teleport/useStickyClusterId';

import { useTeleport } from '..';

import { NotificationContent } from './notificationContentFactory';
import { View } from './Notifications';

export function Notification({
  notification,
  view = 'All',
  closeNotificationsList,
  removeNotification,
  markNotificationAsClicked,
}: {
  notification: NotificationType;
  view?: View;
  closeNotificationsList: () => void;
  removeNotification: (notificationId: string) => void;
  markNotificationAsClicked: (notificationId: string) => void;
}) {
  const ctx = useTeleport();
  const { clusterId } = useStickyClusterId();

  const content = ctx.notificationContentFactory(notification);

  const [markAsClickedAttempt, markAsClicked] = useAsync(() =>
    ctx.notificationService
      .upsertNotificationState(clusterId, {
        notificationId: notification.id,
        notificationState: NotificationState.CLICKED,
      })
      .then(res => {
        markNotificationAsClicked(notification.id);
        return res;
      })
  );

  const [hideNotificationAttempt, hideNotification] = useAsync(() => {
    return ctx.notificationService
      .upsertNotificationState(clusterId, {
        notificationId: notification.id,
        notificationState: NotificationState.DISMISSED,
      })
      .then(() => {
        removeNotification(notification.id);
      });
  });

  function onMarkAsClicked() {
    if (notification.localNotification) {
      ctx.storeNotifications.markNotificationAsClicked(notification.id);
      markNotificationAsClicked(notification.id);
      return;
    }
    markAsClicked();
  }

  // Whether to show the text content dialog. This is only ever used for user-created notifications which only contain informational text
  // and don't redirect to any page.
  const [showTextContentDialog, setShowTextContentDialog] = useState(false);

  // If the notification is unsupported or hidden, or if the view is "Unread" and the notification has been read,
  // it should not be shown.
  if (!content || (view === 'Unread' && notification.clicked)) {
    // If this is a text content notification, the dialog should still be renderable. This is to prevent the text content dialog immediately disappearing
    // when trying to open an unread text notification, since clicking on the notification instantly marks it as read.
    if (content.kind == 'text') {
      return (
        <Dialog open={showTextContentDialog} className={IGNORE_CLICK_CLASSNAME}>
          <DialogHeader>
            <DialogTitle>{content.title}</DialogTitle>
          </DialogHeader>
          <DialogContent>{content.textContent}</DialogContent>
          <DialogFooter>
            <ButtonSecondary
              onClick={() => setShowTextContentDialog(false)}
              size="small"
              className={IGNORE_CLICK_CLASSNAME}
            >
              Close
            </ButtonSecondary>
          </DialogFooter>
        </Dialog>
      );
    }
    return null;
  }

  let AccentIcon;
  switch (content.type) {
    case 'success':
    case 'success-alt':
      AccentIcon = Icons.Check;
      break;
    case 'informational':
      AccentIcon = Icons.Info;
      break;
    case 'warning':
      AccentIcon = Icons.Warning;
      break;
    case 'failure':
      AccentIcon = Icons.Cross;
      break;
  }

  const formattedDate = formatDate(notification.createdDate);

  function onNotificationClick(e: React.MouseEvent<HTMLElement>) {
    // Prevents this from being triggered when the user is just clicking away from
    // an open "mark as read/hide this notification" menu popover.
    if (e.currentTarget.contains(e.target as HTMLElement)) {
      onClick();
    }
  }

  function onClick() {
    if (content.kind === 'text') {
      setShowTextContentDialog(true);
      onMarkAsClicked();
      return;
    }
    onMarkAsClicked();
    closeNotificationsList();
    history.push(content.redirectRoute);
  }

  const isClicked =
    notification.clicked || markAsClickedAttempt.status === 'processing';

  return (
    <>
      <Container
        data-testid="notification-item"
        clicked={isClicked}
        onClick={onNotificationClick}
        className="notification"
        tabIndex={0}
        onKeyDown={e => {
          if (e.key === 'Enter') {
            onClick();
          }
        }}
      >
        <GraphicContainer>
          <MainIconContainer type={content.type}>
            <content.icon size={18} />
          </MainIconContainer>
          <AccentIconContainer type={content.type}>
            <AccentIcon size={10} />
          </AccentIconContainer>
        </GraphicContainer>
        <ContentContainer>
          <ContentBody>
            <Text>{content.title}</Text>
            {content.kind === 'redirect' && content.QuickAction && (
              <content.QuickAction markAsClicked={onMarkAsClicked} />
            )}
            {hideNotificationAttempt.status === 'error' && (
              <Text typography="body4" color="error.main">
                Failed to hide notification:{' '}
                {hideNotificationAttempt.statusText}
              </Text>
            )}
            {markAsClickedAttempt.status === 'error' && (
              <P3 color="error.main">
                Failed to mark notification as read:{' '}
                {markAsClickedAttempt.statusText}
              </P3>
            )}
          </ContentBody>
          <SideContent>
            {!content?.hideDate && (
              <Text typography="body4">{formattedDate}</Text>
            )}
            {!notification.localNotification && (
              <MenuIcon
                menuProps={{
                  anchorOrigin: { vertical: 'bottom', horizontal: 'right' },
                  transformOrigin: { vertical: 'top', horizontal: 'right' },
                  backdropProps: { className: IGNORE_CLICK_CLASSNAME },
                }}
                buttonIconProps={{ style: { borderRadius: '4px' } }}
              >
                {!isClicked && (
                  <MenuItem
                    onClick={onMarkAsClicked}
                    className={IGNORE_CLICK_CLASSNAME}
                  >
                    Mark as read
                  </MenuItem>
                )}
                <MenuItem
                  onClick={hideNotification}
                  className={IGNORE_CLICK_CLASSNAME}
                >
                  Hide this notification
                </MenuItem>
              </MenuIcon>
            )}
          </SideContent>
        </ContentContainer>
      </Container>
      {content.kind === 'text' && (
        <Dialog open={showTextContentDialog} className={IGNORE_CLICK_CLASSNAME}>
          <DialogHeader>
            <DialogTitle>{content.title}</DialogTitle>
          </DialogHeader>
          <DialogContent>{content.textContent}</DialogContent>
          <DialogFooter>
            <ButtonSecondary
              onClick={() => setShowTextContentDialog(false)}
              size="small"
              className={IGNORE_CLICK_CLASSNAME}
            >
              Close
            </ButtonSecondary>
          </DialogFooter>
        </Dialog>
      )}
    </>
  );
}

// formatDate returns how long ago the provided date is in a readable and concise format, ie. "2h ago"
function formatDate(date: Date) {
  let distance = formatDistanceToNowStrict(date);

  distance = distance
    .replace(/seconds?/g, 's')
    .replace(/minutes?/g, 'm')
    .replace(/hours?/g, 'h')
    .replace(/days?/g, 'd')
    .replace(/months?/g, 'mo')
    .replace(/years?/g, 'y')
    .replace(' ', '');

  return `${distance} ago`;
}

const Container = styled.div<{ clicked?: boolean }>`
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: ${props => props.theme.space[3]}px;
  width: 100%;
  padding: ${props => props.theme.space[3]}px;
  border-radius: ${props => props.theme.radii[3]}px;
  cursor: pointer;

  ${props => getInteractiveStateStyles(props.theme, props.clicked)}
`;

function getInteractiveStateStyles(theme: Theme, clicked: boolean): string {
  if (clicked) {
    return `
        background: transparent;
        &:hover {
          background: ${theme.colors.interactive.tonal.neutral[0].background};
        }
        &:active {
          outline: none;
          background: ${theme.colors.interactive.tonal.neutral[1].background};
        }
        &:focus {
          outline: ${theme.borders[2]} ${theme.colors.text.slightlyMuted};
        }
        `;
  }

  return `
    background: ${theme.colors.interactive.tonal.primary[0].background};
    &:hover {
      background: ${theme.colors.interactive.tonal.primary[1].background};
    }
    &:active {
      outline: none;
      background: ${theme.colors.interactive.tonal.primary[2].background};
    }
    &:focus {
      outline: ${theme.borders[2]} ${theme.colors.interactive.solid.primary.default.background};
    }
    `;
}

const ContentContainer = styled.div`
  display: flex;
  justify-content: space-between;
  gap: ${props => props.theme.space[2]}px;
  width: 100%;
`;

const ContentBody = styled.div`
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: flex-start;
  gap: ${props => props.theme.space[2]}px;

  button {
    text-transform: none;
  }
`;

const SideContent = styled.div`
  display: flex;
  flex-direction: column;
  justify-content: flex-start;
  gap: ${props => props.theme.space[2]}px;
  align-items: center;
  white-space: nowrap;
`;

const GraphicContainer = styled.div`
  height: 48px;
  width: 48px;
  position: relative;
  flex-shrink: 0;
`;

function getIconColors(
  theme: Theme,
  type: NotificationContent['type']
): {
  primary: string;
  secondary: string;
} {
  switch (type) {
    case 'success':
      return {
        primary: theme.colors.interactive.solid.success.active.background,
        secondary: theme.colors.interactive.tonal.success[0].background,
      };
    case 'success-alt':
      return {
        primary: theme.colors.interactive.solid.accent.active.background,
        secondary: theme.colors.interactive.tonal.informational[0].background,
      };
    case 'informational':
      return {
        primary: theme.colors.brand,
        secondary: theme.colors.interactive.tonal.primary[0].background,
      };
    case `warning`:
      return {
        primary: theme.colors.interactive.solid.alert.active.background,
        secondary: theme.colors.interactive.tonal.alert[0].background,
      };
    case 'failure':
      return {
        primary: theme.colors.error.main,
        secondary: theme.colors.interactive.tonal.danger[0].background,
      };
  }
}

const MainIconContainer = styled.div<{ type: NotificationContent['type'] }>`
  display: flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border-radius: ${props => props.theme.radii[3]}px;
  position: absolute;
  z-index: 1;
  top: 0;
  left: 0;

  border: ${props => props.theme.borders[1]};
  border-color: ${props => getIconColors(props.theme, props.type).primary};

  background-color: ${props =>
    getIconColors(props.theme, props.type).secondary};
`;

const AccentIconContainer = styled.div<{ type: NotificationContent['type'] }>`
  height: 18px;
  width: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: ${props => props.theme.radii[2]}px;
  position: absolute;
  z-index: 2;
  bottom: 0;
  right: 0;
  color: ${props => props.theme.colors.text.primaryInverse};

  background-color: ${props => getIconColors(props.theme, props.type).primary};
`;
