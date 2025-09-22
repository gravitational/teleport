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

import { useRef } from 'react';

import { Flex } from 'design';

import { AccessRequestCheckout } from 'teleterm/ui/AccessRequestCheckout';
import { NotificationsHost } from 'teleterm/ui/components/Notifcations';
import { StatusBar } from 'teleterm/ui/StatusBar';
import { TabHostContainer } from 'teleterm/ui/TabHost';
import { TopBar } from 'teleterm/ui/TopBar';

export function LayoutManager() {
  const topBarConnectMyComputerRef = useRef<HTMLDivElement>(null);
  const topBarAccessRequestRef = useRef<HTMLDivElement>(null);

  return (
    <Flex flex="1" flexDirection="column" minHeight={0}>
      <TopBar
        connectMyComputerRef={topBarConnectMyComputerRef}
        accessRequestRef={topBarAccessRequestRef}
      />
      <Flex
        flex="1"
        minHeight={0}
        css={`
          position: relative;
        `}
      >
        <TabHostContainer
          topBarConnectMyComputerRef={topBarConnectMyComputerRef}
          topBarAccessRequestRef={topBarAccessRequestRef}
        />
        <NotificationsHost />
      </Flex>
      <AccessRequestCheckout />
      <StatusBar
        onAssumedRolesClick={() => {
          // This is a little hacky, but has one advantage:
          // we don't need to expose a way to open/close the popover.
          // This would require extra effort, since each workspace has its own
          // AccessRequestsContext, which is located lower in the component tree
          // (so we would have to inject a component through the portal).
          const menu = topBarAccessRequestRef.current?.querySelector(
            '#access-requests-menu'
          );
          if (menu instanceof HTMLButtonElement) {
            menu.click();
          }
        }}
      />
    </Flex>
  );
}
