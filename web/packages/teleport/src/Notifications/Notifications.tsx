/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { isBefore } from 'date-fns';
import { useCallback, useEffect, useState } from 'react';
import styled from 'styled-components';

import { Alert, Box, Flex, Indicator, Text } from 'design';
import { BellRinging, Notification as NotificationIcon } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';
import {
  useInfiniteScroll,
  useKeyBasedPagination,
} from 'shared/hooks/useInfiniteScroll';
import { useRefClickOutside } from 'shared/hooks/useRefClickOutside';
import { IGNORE_CLICK_CLASSNAME } from 'shared/hooks/useRefClickOutside/useRefClickOutside';
import Logger from 'shared/libs/logger';

import { useTeleport } from 'teleport';
import { Dropdown } from 'teleport/components/Dropdown';
import { ButtonIconContainer } from 'teleport/TopBar/Shared';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { Notification } from './Notification';

const PAGE_SIZE = 15;

const logger = Logger.create('Notifications');

const NOTIFICATION_DROPDOWN_ID = 'tb-notifications-dropdown';

export function Notifications({ iconSize = 24 }: { iconSize?: number }) {
  const ctx = useTeleport();
  const { clusterId } = useStickyClusterId();
  const [userLastSeenNotification, setUserLastSeenNotification] =
    useState<Date>();

  const {
    resources: notifications,
    fetch,
    attempt,
    updateFetchedResources,
  } = useKeyBasedPagination({
    fetchMoreSize: PAGE_SIZE,
    initialFetchSize: PAGE_SIZE,
    fetchFunc: useCallback(
      async paginationParams => {
        const response = await ctx.notificationService.fetchNotifications({
          clusterId,
          startKey: paginationParams.startKey,
          limit: paginationParams.limit,
        });

        setUserLastSeenNotification(response.userLastSeenNotification);

        return {
          agents: response.notifications,
          startKey: response.nextKey,
        };
      },
      [clusterId, ctx.notificationService]
    ),
  });

  // Fetch first page on first render.
  useEffect(() => {
    fetch();
  }, []);

  const { setTrigger } = useInfiniteScroll({
    fetch,
  });

  const [view, setView] = useState<View>('All');
  const [open, setOpen] = useState(false);

  const ref = useRefClickOutside<HTMLDivElement>({ open, setOpen });

  function onIconClick() {
    if (!open) {
      setOpen(true);

      if (notifications.length) {
        const latestNotificationTime = notifications[0].createdDate;
        // If the current userLastSeenNotification is already set to the most recent notification's time, don't do anything.
        if (userLastSeenNotification === latestNotificationTime) {
          return;
        }

        const previousLastSeenTime = userLastSeenNotification;

        // Update the visual state right away for a snappier UX.
        setUserLastSeenNotification(latestNotificationTime);

        ctx.notificationService
          .upsertLastSeenNotificationTime(clusterId, {
            time: latestNotificationTime,
          })
          .then(res => setUserLastSeenNotification(res.time))
          .catch(err => {
            setUserLastSeenNotification(previousLastSeenTime);
            logger.error(`Notification last seen time update failed.`, err);
          });
      }
    } else {
      setOpen(false);
    }
  }

  const unseenNotifsCount = notifications.filter(notif =>
    isBefore(userLastSeenNotification, notif.createdDate)
  ).length;

  function removeNotification(notificationId: string) {
    const notificationsCopy = [...notifications];
    const index = notificationsCopy.findIndex(
      notif => notif.id == notificationId
    );
    notificationsCopy.splice(index, 1);

    updateFetchedResources(notificationsCopy);
  }

  function markNotificationAsClicked(notificationId: string) {
    const newNotifications = notifications.map(notification => {
      return notification.id === notificationId
        ? { ...notification, clicked: true }
        : notification;
    });

    updateFetchedResources(newNotifications);
  }

  return (
    <NotificationButtonContainer
      ref={ref}
      data-testid="tb-notifications"
      className={IGNORE_CLICK_CLASSNAME}
    >
      <HoverTooltip
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
        transformOrigin={{ vertical: 'top', horizontal: 'center' }}
        tipContent="Notifications"
        css={`
          height: 100%;
        `}
      >
        <ButtonIconContainer
          onClick={onIconClick}
          onKeyUp={e => (e.key === 'Enter' || e.key === ' ') && onIconClick()}
          data-testid="tb-notifications-button"
          open={open}
          role="button"
          tabIndex={0}
          aria-label="Notifications"
          aria-haspopup="menu"
          aria-controls={NOTIFICATION_DROPDOWN_ID}
          aria-expanded={open}
        >
          {unseenNotifsCount > 0 && (
            <UnseenBadge data-testid="tb-notifications-badge">
              {unseenNotifsCount >= 9 ? '9+' : unseenNotifsCount}
            </UnseenBadge>
          )}
          <NotificationIcon
            color={open ? 'text.main' : 'text.muted'}
            size={iconSize}
          />
        </ButtonIconContainer>
      </HoverTooltip>

      <NotificationsDropdown
        open={open}
        id={NOTIFICATION_DROPDOWN_ID}
        data-testid={NOTIFICATION_DROPDOWN_ID}
        role="menu"
      >
        <Header view={view} setView={setView} />
        {attempt.status === 'failed' && (
          <Box px={3}>
            <Alert>Could not load notifications: {attempt.statusText}</Alert>
          </Box>
        )}
        {attempt.status === 'success' && notifications.length === 0 && (
          <EmptyState />
        )}
        <NotificationsList>
          <>
            {!!notifications.length &&
              notifications.map(notif => (
                <Notification
                  notification={notif}
                  key={notif.id}
                  view={view}
                  closeNotificationsList={() => setOpen(false)}
                  markNotificationAsClicked={markNotificationAsClicked}
                  removeNotification={removeNotification}
                />
              ))}
            {open && <div ref={setTrigger} />}
            {attempt.status === 'processing' && (
              <Flex
                width="100%"
                justifyContent="center"
                alignItems="center"
                mt={2}
              >
                <Indicator />
              </Flex>
            )}
          </>
        </NotificationsList>
      </NotificationsDropdown>
    </NotificationButtonContainer>
  );
}

function Header({
  view,
  setView,
}: {
  view: View;
  setView: (view: View) => void;
}) {
  return (
    <Box
      css={`
        padding: 0px ${p => p.theme.space[3]}px;
        width: 100%;
      `}
    >
      <Flex
        css={`
          flex-direction: column;
          box-sizing: border-box;
          gap: 12px;
          border-bottom: 1px solid
            ${p => p.theme.colors.interactive.tonal.neutral[2]};
          padding-bottom: ${p => p.theme.space[3]}px;
          margin-bottom: ${p => p.theme.space[3]}px;
        `}
      >
        <Text typography="dropdownTitle">Notifications</Text>
        <Flex gap={2}>
          <ViewButton selected={view === 'All'} onClick={() => setView('All')}>
            All
          </ViewButton>
          <ViewButton
            selected={view === 'Unread'}
            onClick={() => setView('Unread')}
          >
            Unread
          </ViewButton>
        </Flex>
      </Flex>
    </Box>
  );
}

function EmptyState() {
  return (
    <Flex
      flexDirection="column"
      alignItems="center"
      justifyContent="center"
      width="100%"
      height="100%"
      mt={4}
      mb={4}
    >
      <Flex
        css={`
          align-items: center;
          justify-content: center;
          height: 88px;
          width: 88px;
          background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
          border-radius: ${p => p.theme.radii[7]}px;
          border: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[1]};
        `}
      >
        <BellRinging size={40} />
      </Flex>
      <Text mt={4} typography="h2" textAlign="center">
        You currently have no notifications.
      </Text>
    </Flex>
  );
}

const NotificationsDropdown = styled(Dropdown)`
  width: 450px;
  padding: 0px;
  padding-top: ${p => p.theme.space[3]}px;
  align-items: center;
  height: 80vh;

  right: -40px;
  @media screen and (min-width: ${p => p.theme.breakpoints.small}px) {
    right: -52px;
  }
  @media screen and (min-width: ${p => p.theme.breakpoints.large}px) {
    right: -140px;
  }
`;

const ViewButton = styled.div<{ selected: boolean }>`
  cursor: pointer;
  align-items: center;
  // TODO(rudream): Clean up radii order in sharedStyles.
  border-radius: 36px;
  display: flex;
  width: fit-content;
  padding: ${p => p.theme.space[1]}px ${p => p.theme.space[3]}px;
  justify-content: space-around;
  font-size: 14px;
  font-weight: 300;
  color: ${props =>
    props.selected
      ? props.theme.colors.text.primaryInverse
      : props.theme.colors.text.muted};
  background-color: ${props =>
    props.selected ? props.theme.colors.brand : 'transparent'};

  .selected {
    color: ${props => props.theme.colors.text.primaryInverse};
    background-color: ${props => props.theme.colors.brand};
    transition: color 0.2s ease-in 0s;
  }
`;

export type View = 'All' | 'Unread';

const NotificationsList = styled.div`
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: ${p => p.theme.space[2]}px;
  width: 100%;
  max-height: 100%;
  overflow-y: auto;
  padding: ${p => p.theme.space[3]}px;
  padding-top: 2px;
  // Subtract the width of the scrollbar from the right padding.
  padding-right: ${p => `${p.theme.space[3] - 8}px`};

  ::-webkit-scrollbar-thumb {
    background-color: ${p => p.theme.colors.interactive.tonal.neutral[2]};
    border-radius: ${p => p.theme.radii[2]}px;
    // Trick to make the scrollbar thumb 2px narrower than the track.
    border: 2px solid transparent;
    background-clip: padding-box;
  }

  ::-webkit-scrollbar {
    width: 8px;
    border-radius: ${p => p.theme.radii[2]}px;
    border-radius: ${p => p.theme.radii[2]}px;
    background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
  }

  .notification {
    width: ${p => `${450 - p.theme.space[3] * 2}px`};
  }
`;

const NotificationButtonContainer = styled.div`
  position: relative;
  height: 100%;
`;

const UnseenBadge = styled.div`
  position: absolute;
  width: 16px;
  height: 16px;
  font-size: 10px;
  border-radius: 100%;
  color: ${p => p.theme.colors.text.primaryInverse};
  background-color: ${p => p.theme.colors.buttons.warning.default};
  margin-top: -21px;
  margin-right: -13px;
  display: flex;
  align-items: center;
  justify-content: center;
`;
