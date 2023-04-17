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
    <Menu
      anchorOrigin={{
        vertical: 'bottom',
        horizontal: 'center',
      }}
      transformOrigin={{
        vertical: 'top',
        horizontal: 'center',
      }}
      anchorEl={anchorEl}
      open={Boolean(anchorEl)}
      onClose={handleClose}
    >
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
      anchorOrigin={{
        vertical: 'bottom',
        horizontal: 'center',
      }}
      transformOrigin={{
        vertical: 'top',
        horizontal: 'center',
      }}
      anchorEl={anchorEl}
      open={Boolean(anchorEl)}
      onClose={handleClose}
      style={{ backgroundColor: theme.colors.levels.surface }}
    >
      <LockedJoinMenuContainer>
        <ButtonLockedFeature>
          Join Active Sessions with Teleport Enterprise
        </ButtonLockedFeature>
        <Box style={{ color: theme.colors.text.secondary }} ml="3">
          <Box mb="3">
            <Text fontSize="16px">As an Observer</Text>
            <Text fontSize="14px">
              Watch: cannot control any part of the session
            </Text>
          </Box>
          <Box mb="3">
            <Text fontSize="16px">As a Moderator</Text>
            <Text fontSize="14px">
              Review: can view output & terminate the session
            </Text>
          </Box>
          <Box>
            <Text fontSize="16px">As a Peer</Text>
            <Text fontSize="14px">
              Collaborate: can view output and send input
            </Text>
          </Box>
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
