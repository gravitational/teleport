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

import React, { useRef } from 'react';
import { Flex } from 'design';

import { AccessRequestCheckout } from 'teleterm/ui/AccessRequestCheckout';
import { TabHostContainer } from 'teleterm/ui/TabHost';
import { TopBar } from 'teleterm/ui/TopBar';
import { StatusBar } from 'teleterm/ui/StatusBar';
import { NotificationsHost } from 'teleterm/ui/components/Notifcations';

export function LayoutManager() {
  const topBarContainerRef = useRef<HTMLDivElement>();

  return (
    <Flex flex="1" flexDirection="column" minHeight={0}>
      <TopBar topBarContainerRef={topBarContainerRef} />
      <Flex
        flex="1"
        minHeight={0}
        css={`
          position: relative;
        `}
      >
        <TabHostContainer topBarContainerRef={topBarContainerRef} />
        <NotificationsHost />
      </Flex>
      <AccessRequestCheckout />
      <StatusBar />
    </Flex>
  );
}
