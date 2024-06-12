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
import { formatDistanceToNow } from 'date-fns';
import styled from 'styled-components';
import { Text } from 'design';

import { Notification as NotificationIcon, UserList } from 'design/Icon';
import { useRefClickOutside } from 'shared/hooks/useRefClickOutside';
import { useStore } from 'shared/libs/stores';
import { assertUnreachable } from 'shared/utils/assertUnreachable';
import { HoverTooltip } from 'shared/components/ToolTip';

import {
  Dropdown,
  DropdownItem,
  DropdownItemButton,
  DropdownItemIcon,
  STARTING_TRANSITION_DELAY,
  INCREMENT_TRANSITION_DELAY,
  DropdownItemLink,
} from 'teleport/components/Dropdown';
import useTeleport from 'teleport/useTeleport';
import {
  Notification,
  NotificationKind,
} from 'teleport/stores/storeNotifications';

import { ButtonIconContainer } from '../Shared';

export function Notifications({ iconSize = 24 }: { iconSize?: number }) {
  const ctx = useTeleport();
  useStore(ctx.storeNotifications);

  const notices = ctx.storeNotifications.getNotifications();

  const [open, setOpen] = useState(false);

  const ref = useRefClickOutside<HTMLDivElement>({ open, setOpen });

  let transitionDelay = STARTING_TRANSITION_DELAY;
  const items = notices.map(notice => {
    const currentTransitionDelay = transitionDelay;
    transitionDelay += INCREMENT_TRANSITION_DELAY;

    return (
      <DropdownItem
        open={open}
        $transitionDelay={currentTransitionDelay}
        key={notice.id}
        data-testid="note-item"
      >
        <NotificationItem notice={notice} close={() => setOpen(false)} />
      </DropdownItem>
    );
  });

  return (
    <HoverTooltip
      anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      transformOrigin={{ vertical: 'top', horizontal: 'center' }}
      tipContent="Notifications"
      css={`
        height: 100%;
      `}
    >
      <NotificationButtonContainer ref={ref} data-testid="tb-note">
        <ButtonIconContainer
          onClick={() => setOpen(!open)}
          data-testid="tb-note-button"
        >
          {items.length > 0 && <AttentionDot data-testid="tb-note-attention" />}
          <NotificationIcon
            color={open ? 'text.main' : 'text.muted'}
            size={iconSize}
          />
        </ButtonIconContainer>

        <Dropdown
          open={open}
          style={{
            width: '300px',
            maxHeight: '80vh',
            overflowY: 'auto',
            overflowX: 'hidden',
          }}
          data-testid="tb-note-dropdown"
        >
          {items.length ? (
            items
          ) : (
            <Text textAlign="center" p={2}>
              No notifications
            </Text>
          )}
        </Dropdown>
      </NotificationButtonContainer>
    </HoverTooltip>
  );
}

function NotificationItem({
  notice,
  close,
}: {
  notice: Notification;
  close(): void;
}) {
  const today = new Date();
  const numDays = formatDistanceToNow(notice.date);

  let dueText;
  if (notice.date <= today) {
    dueText = `was overdue for a review ${numDays} ago`;
  } else {
    dueText = `needs your review within ${numDays}`;
  }
  switch (notice.item.kind) {
    case NotificationKind.AccessList:
      return (
        <NotificationLink to={notice.item.route} onClick={close}>
          <NotificationItemButton>
            <DropdownItemIcon>
              <UserList mt="1px" />
            </DropdownItemIcon>
            <Text>
              Access list <b>{notice.item.resourceName}</b> {dueText}.
            </Text>
          </NotificationItemButton>
        </NotificationLink>
      );
    default:
      assertUnreachable(notice.item.kind);
  }
}

const NotificationButtonContainer = styled.div`
  position: relative;
  height: 100%;
`;

const AttentionDot = styled.div`
  position: absolute;
  width: 7px;
  height: 7px;
  border-radius: 100px;
  background-color: ${p => p.theme.colors.buttons.warning.default};
  top: 10px;
  right: 15px;
  @media screen and (min-width: ${p => p.theme.breakpoints.large}px) {
    top: 20px;
    right: 25px;
  }
`;

const NotificationItemButton = styled(DropdownItemButton)`
  align-items: flex-start;
  line-height: 20px;
`;

const NotificationLink = styled(DropdownItemLink)`
  padding: 0;
  z-index: 999;
`;
