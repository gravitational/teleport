/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { ToastNotification } from './ToastNotification';
import { useToastNotifications } from './ToastNotificationContext';

/**
 * Renders a list of notification toasts in the top right corner of the screen.
 * Toasts are sticky when scrolling is enabled on the screen.
 *
 * Info like notifications (success, info, neutral) are automatically removed
 * from the notification list after a set time.
 *
 * Requires ToastNotificationProvider.
 */
export const ToastNotifications = () => {
  const { remove: removeNotification, notifications } = useToastNotifications();

  return (
    <TopRightStickyContainer>
      <TopRightFlexedContainer>
        {notifications.map(item => (
          <ToastNotification
            key={item.id}
            item={item}
            onRemove={() => removeNotification(item.id)}
          />
        ))}
      </TopRightFlexedContainer>
    </TopRightStickyContainer>
  );
};

const TopRightStickyContainer = styled.div`
  position: sticky;
  top: 0;
  right: 0;
  z-index: 1;
`;

const TopRightFlexedContainer = styled.div`
  position: absolute;
  top: ${p => p.theme.space[2]}px;
  right: ${p => p.theme.space[5]}px;
  display: flex;
  flex-direction: column;
  gap: ${p => p.theme.space[3]}px;
`;
