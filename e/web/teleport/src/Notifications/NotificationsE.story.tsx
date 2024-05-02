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

import React from 'react';

import Flex from 'design/Flex';

import { Notification } from 'teleport/Notifications/Notification';
import { ContextProvider } from 'teleport/index';
import { notifications } from 'teleport/Notifications/fixtures';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

export default {
  title: 'TeleportE/Notifications',
};

export const NotificationsTypesE = () => {
  const ctx = createTeleportContextE();

  return (
    <ContextProvider ctx={ctx}>
      <Flex
        p={4}
        gap={4}
        css={`
          background: ${props => props.theme.colors.levels.surface};
          width: fit-content;
          height: fit-content;
          flex-direction: column;
        `}
      >
        {notifications.map(notification => {
          return (
            <Notification notification={notification} key={notification.id} />
          );
        })}
      </Flex>
    </ContextProvider>
  );
};
