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

import React, { useEffect, useRef, useState } from 'react';
import styled, { keyframes } from 'styled-components';

import { Box, ButtonPrimary, Text } from 'design';

import { DesktopItem } from 'teleport/Discover/Desktop/DiscoverDesktops/DesktopItem';
import { useDiscover } from 'teleport/Discover/useDiscover';
import {
  Header,
  Mark,
  ResourceKind,
  useShowHint,
} from 'teleport/Discover/Shared';
import { ProxyDesktopServiceDiagram } from 'teleport/Discover/Desktop/DiscoverDesktops/ProxyDesktopServiceDiagram';
import { usePoll } from 'teleport/Discover/Shared/usePoll';
import { useTeleport } from 'teleport';
import useStickyClusterId from 'teleport/useStickyClusterId';
import { usePingTeleport } from 'teleport/Discover/Shared/PingTeleportContext';
import cfg from 'teleport/config';
import { NavLink } from 'teleport/components/Router';
import { DiscoverEventStatus } from 'teleport/services/userEvent';

import { HintBox } from 'teleport/Discover/Shared/HintBox';

import { useJoinTokenSuspender } from 'teleport/Discover/Shared/useJoinTokenSuspender';

import type { WindowsDesktopService } from 'teleport/services/desktops';

const Desktops = styled.div`
  margin-top: 120px;
  margin-left: -40px;
  display: flex;
`;

const FoundDesktops = styled.div`
  position: relative;
  margin-left: 125px;
  margin-top: -43px;
`;

const MAX_COUNT = 14;
const POLL_INTERVAL = 1000 * 3; // 3 seconds

const fadeIn = keyframes`
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
`;

const ContentBox = styled.div`
  box-sizing: border-box;
  color: rgba(0, 0, 0, 0.8);
  border-radius: 10px;
  box-shadow: 0 10px 15px rgba(0, 0, 0, 0.5);
  background: white;
  position: relative;
  animation: ${fadeIn} 0.9s ease-in 1s forwards;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: 10px 10px 10px 15px;
  opacity: 0;
  width: 240px;
`;

const ViewLink = styled(NavLink)`
  background: ${props => props.theme.colors.buttons.link.default};
  color: ${props => props.theme.colors.text.main};
  border-radius: 5px;
  margin-top: 10px;
  text-decoration: none;
  padding: 3px 10px;
  text-align: center;
  cursor: pointer;

  &:hover {
    background: ${props => props.theme.colors.buttons.link.hover};
  }
`;

export function DiscoverDesktops() {
  const ctx = useTeleport();
  const { emitEvent, nextStep } = useDiscover();

  const { joinToken } = useJoinTokenSuspender([ResourceKind.Desktop]);
  const { result: desktopService, active } =
    usePingTeleport<WindowsDesktopService>(joinToken);

  const showHint = useShowHint(active);

  const [enabled, setEnabled] = useState(true);
  const { clusterId } = useStickyClusterId();

  const result = usePoll(
    signal =>
      ctx.desktopService.fetchDesktops(clusterId, { limit: MAX_COUNT }, signal),
    enabled,
    POLL_INTERVAL
  );

  const desktopServiceRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (enabled && result && result.agents.length === MAX_COUNT) {
      setEnabled(false);
    }
  }, [enabled, result]);

  const ref = useRef<HTMLDivElement>();

  const desktops = [];

  if (result && result.agents) {
    const foundDesktops = result.agents.filter(
      desktop => desktop.host_id === desktopService.name
    );

    if (foundDesktops.length) {
      for (const desktop of foundDesktops.values()) {
        const os =
          desktop.labels.find(label => label.name === 'teleport.dev/os')
            ?.value || 'unknown os';
        const osVersion =
          desktop.labels.find(label => label.name === 'teleport.dev/os_version')
            ?.value || 'unknown version';

        desktops.push({
          os,
          osVersion,
          computerName: desktop.name,
          address: desktop.addr,
        });
      }
    }
  }

  function handleNextStep() {
    emitEvent(
      { stepStatus: DiscoverEventStatus.Success },
      { autoDiscoverResourcesCount: desktops.length }
    );
    nextStep();
  }

  const items = desktops
    .slice(0, 3)
    .map((desktop, index) => (
      <DesktopItem
        key={index}
        index={index}
        os={desktop.os}
        osVersion={desktop.osVersion}
        computerName={desktop.computerName}
        address={desktop.address}
        desktopServiceElement={desktopServiceRef.current}
        containerElement={ref.current}
      />
    ));

  // We show 3 desktops maximum, and fetch 14 in order to be able to
  // display the message "We've found 10+ more desktops".
  //
  // This gives the user a better idea that a lot of desktops have been
  // discovered, without us having to fetch all of them to get an actual count.
  const extraDesktopCount = desktops.length - 3;

  let viewMore;
  if (extraDesktopCount > 0) {
    let amount = '1';
    let word = 'Desktops';
    if (extraDesktopCount === 1) {
      word = 'Desktop';
    } else {
      if (extraDesktopCount > 11) {
        amount = '10+';
      } else {
        amount = `${extraDesktopCount}`;
      }
    }

    viewMore = (
      <ContentBox key="view-more">
        We've found {amount} more {word}.{' '}
        <ViewLink to={cfg.getDesktopsRoute(clusterId)}>
          View them all here
        </ViewLink>
      </ContentBox>
    );
  }

  let content;
  if (desktops.length > 0) {
    content = (
      <>
        {items}
        {viewMore}
      </>
    );
  }

  let hint;
  if (showHint && desktops.length === 0) {
    hint = (
      <Box mt={5}>
        <HintBox header="We're still trying to discover Desktops">
          <Text mb={3}>
            There are a couple of possible reasons for why we haven't been able
            to detect your server.
          </Text>

          <Text mb={1}>
            - There aren't any desktops connected to Active Directory
          </Text>

          <Text mb={3}>
            - A Desktop could have had issues joining the Teleport Desktop
            Service. Check the logs for errors by running{' '}
            <Mark>journalctl -fu teleport</Mark> on your Desktop Service.
          </Text>

          <Text>
            We'll continue to try and discover Desktops whilst you diagnose the
            issue, but you can safely leave this page.
          </Text>
        </HintBox>
      </Box>
    );
  }

  return (
    <Box>
      <Header>Discover Desktops</Header>
      <Text>
        We're discovering Desktops that are already connected to your Active
        Directory.
      </Text>

      <Desktops ref={ref}>
        <ProxyDesktopServiceDiagram
          result={desktopService}
          desktopServiceRef={desktopServiceRef}
        />

        <FoundDesktops>{content}</FoundDesktops>
      </Desktops>

      {hint}

      <Box mt={5}>
        <ButtonPrimary width="165px" mr={3} onClick={handleNextStep}>
          Finish
        </ButtonPrimary>
      </Box>
    </Box>
  );
}
