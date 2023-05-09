/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { useState } from 'react';
import styled from 'styled-components';

import { ButtonBorder, Text, Box, Menu, MenuItem, Flex } from 'design';
import { CarrotDown } from 'design/Icon';

import cfg from 'teleport/config';
import { ParticipantMode } from 'teleport/services/session';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import { CtaEvent } from 'teleport/services/userEvent';

export const SessionJoinBtn = ({
  sid,
  clusterId,
  participantModes,
  showCTA,
}: {
  sid: string;
  clusterId: string;
  participantModes: ParticipantMode[];
  showCTA: boolean;
}) => {
  // Sorts the list of participantModes so that they are consistently shown in the order of "observer" -> "moderator" -> "peer"
  const modes = {
    observer: 1,
    moderator: 2,
    peer: 3,
  };
  const sortedParticipantModes = participantModes.sort(
    (a, b) => modes[a] - modes[b]
  );

  if (showCTA) {
    return <LockedFeatureJoinMenu modes={sortedParticipantModes} />;
  }

  return (
    <JoinMenu>
      {sortedParticipantModes.map(participantMode => (
        <MenuItem
          key={participantMode}
          as="a"
          href={cfg.getSshSessionRoute({ sid, clusterId }, participantMode)}
          target="_blank"
          style={{ textTransform: 'capitalize' }}
        >
          {participantMode}
        </MenuItem>
      ))}
    </JoinMenu>
  );
};

function JoinMenu({ children }: { children: React.ReactNode }) {
  const [anchorEl, setAnchorEl] = useState<HTMLElement>(null);

  const handleClickListItem = (event: React.MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget);
  };

  const handleClose = () => {
    setAnchorEl(null);
  };

  return (
    <Box textAlign="center" width="80px">
      <ButtonBorder size="small" onClick={handleClickListItem}>
        Join
        <CarrotDown ml={1} fontSize={2} color="text.slightlyMuted" />
      </ButtonBorder>
      <InternalJoinMenu anchorEl={anchorEl} handleClose={handleClose}>
        {children}
      </InternalJoinMenu>
    </Box>
  );
}

type LockedFeatureJoinMenu = {
  modes: ParticipantMode[];
};
function LockedFeatureJoinMenu({ modes }: LockedFeatureJoinMenu) {
  const [anchorEl, setAnchorEl] = useState<HTMLElement>(null);

  const handleClickListItem = (event: React.MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget);
  };

  const handleClose = () => {
    setAnchorEl(null);
  };

  return (
    <Box textAlign="center" width="80px">
      <ButtonBorder size="small" onClick={handleClickListItem}>
        Join
        <CarrotDown ml={1} fontSize={2} color="text.slightlyMuted" />
      </ButtonBorder>
      <LockedFeatureInternalJoinMenu
        anchorEl={anchorEl}
        handleClose={handleClose}
        modes={modes}
      />
    </Box>
  );
}

type InternalJoinMenuProps = {
  anchorEl: HTMLElement;
  handleClose: () => void;
  children: React.ReactNode;
};
function InternalJoinMenu({
  anchorEl,
  handleClose,
  children,
}: InternalJoinMenuProps) {
  return (
    <Menu anchorEl={anchorEl} open={Boolean(anchorEl)} onClose={handleClose}>
      <Text px="2" fontSize="11px" color="grey.400" bg="subtle">
        Join as...
      </Text>
      {children}
    </Menu>
  );
}

type LockedFeatureInternalJoinMenuProps = {
  anchorEl: HTMLElement;
  handleClose: () => void;
  modes: ParticipantMode[];
};
function LockedFeatureInternalJoinMenu({
  anchorEl,
  handleClose,
  modes,
}: LockedFeatureInternalJoinMenuProps) {
  return (
    <Menu
      anchorEl={anchorEl}
      open={Boolean(anchorEl)}
      onClose={handleClose}
      menuListCss={() => {
        return { backgroundColor: '#222c59' };
      }}
    >
      <div></div>
      <LockedJoinMenuContainer>
        <ButtonLockedFeature
          noIcon
          height="40px"
          event={CtaEvent.CTA_ACTIVE_SESSIONS}
        >
          Join Active Sessions with Teleport Enterprise
        </ButtonLockedFeature>
        <Box ml="3">
          {modes.includes('observer') ? (
            <LockedJoinItem
              name={'As an Observer'}
              info={'Watch: cannot control any part of the session'}
            />
          ) : null}

          {modes.includes('moderator') ? (
            <LockedJoinItem
              name={'As a Moderator'}
              info={'Review: can view output & terminate the session'}
            />
          ) : null}

          {modes.includes('peer') ? (
            <LockedJoinItem
              name={'As a Peer'}
              info={'Collaborate: can view output and send input'}
            />
          ) : null}
        </Box>
      </LockedJoinMenuContainer>
    </Menu>
  );
}

const LockedJoinMenuContainer = styled(Flex)`
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  padding: 16px 12px;
  gap: 12px;
  background-color: #222c59;
`;

type LockedJoinItemProps = {
  name: string;
  info: string;
};
function LockedJoinItem({ name, info }: LockedJoinItemProps) {
  return (
    <Box mb="3">
      <Text fontSize="16px">{name}</Text>
      <Text fontSize="14px">{info}</Text>
    </Box>
  );
}
