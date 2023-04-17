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

import theme from 'design/theme';

import cfg from 'teleport/config';
import { ParticipantMode } from 'teleport/services/session';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';

export const SessionJoinBtn = ({
  sid,
  clusterId,
  participantModes,
}: {
  sid: string;
  clusterId: string;
  participantModes: ParticipantMode[];
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

  const handleClickListItem = event => {
    setAnchorEl(event.currentTarget);
  };

  const handleClose = () => {
    setAnchorEl(null);
  };

  return (
    <Box textAlign="center" width="80px">
      <ButtonBorder size="small" onClick={handleClickListItem}>
        Join
        <CarrotDown ml={1} fontSize={2} color="text.secondary" />
      </ButtonBorder>
      {cfg.isTeams ? (
        <LockedFeatureInternalJoinMenu
          anchorEl={anchorEl}
          handleClose={handleClose}
        />
      ) : (
        <InternalJoinMenu anchorEl={anchorEl} handleClose={handleClose}>
          {children}
        </InternalJoinMenu>
      )}
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

type LockedFeatureInternalJoinMenu = InternalJoinMenuProps;
function LockedFeatureInternalJoinMenu({ anchorEl, handleClose }) {
  return (
    <Menu
      anchorEl={anchorEl}
      open={Boolean(anchorEl)}
      onClose={handleClose}
      menuListCss={() => `
        background-color: ${theme.colors.levels.surface};
  `}
    >
      <div></div> {/* this div makes the menu properly positioned */}
      <LockedJoinMenuContainer>
        <ButtonLockedFeature>
          Join Active Sessions with Teleport Enterprise
        </ButtonLockedFeature>
        <Box style={{ color: theme.colors.text.secondary }} ml="3">
          <LockedJoinItem
            name={'As an Observer'}
            info={'Watch: cannot control any part of the session'}
          />
          <LockedJoinItem
            name={'As a Moderator'}
            info={'Review: can view output & terminate the session'}
          />
          <LockedJoinItem
            name={'As a Peer'}
            info={'Collaborate: can view output and send input'}
          />
        </Box>
      </LockedJoinMenuContainer>
    </Menu>
  );
}

const LockedJoinMenuContainer = styled(Flex)(
  () => `
    background-color: ${theme.colors.levels.surface};
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    padding: 16px 12px;
    gap: 12px;
  `
);

type LockedJoinItemProps = {
  name: string;
  info: string;
};
function LockedJoinItem({ name, info }: LockedJoinItemProps) {
  return (
    <StyledJoinItem
      mb="3"
      style={{
        '&:hover': {
          background: 'red',
        },
      }}
    >
      <Text fontSize="16px">{name}</Text>
      <Text fontSize="14px">{info}</Text>
    </StyledJoinItem>
  );
}

const StyledJoinItem = styled(Box)(
  () => `
  &:hover {
    color: white;
  }
  `
);
