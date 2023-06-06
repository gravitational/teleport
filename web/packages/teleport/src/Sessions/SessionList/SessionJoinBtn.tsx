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

import { ButtonBorder, Text, Box, Menu, MenuItem } from 'design';
import { CarrotDown, Warning } from 'design/Icon';

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
  const [anchorEl, setAnchorEl] = useState<HTMLElement>(null);

  function closeMenu() {
    setAnchorEl(null);
  }

  return (
    <JoinMenu anchorEl={anchorEl} setAnchorEl={setAnchorEl}>
      {showCTA && (
        <Box mx="12px" my="3">
          <ButtonLockedFeature
            noIcon
            height="40px"
            event={CtaEvent.CTA_ACTIVE_SESSIONS}
          >
            Join Active Sessions with Teleport Enterprise
          </ButtonLockedFeature>
        </Box>
      )}
      <JoinMenuItem
        title="As an Observer"
        description={modeDescription.observer}
        url={cfg.getSshSessionRoute({ sid, clusterId }, 'observer')}
        hasAccess={participantModes.includes('observer')}
        participantMode="observer"
        key="observer"
        showCTA={showCTA}
        closeMenu={closeMenu}
      />
      <JoinMenuItem
        title="As a Moderator"
        description={modeDescription.moderator}
        url={cfg.getSshSessionRoute({ sid, clusterId }, 'moderator')}
        hasAccess={participantModes.includes('moderator')}
        participantMode="moderator"
        key="moderator"
        showCTA={showCTA}
        closeMenu={closeMenu}
      />
      <JoinMenuItem
        title="As a Peer"
        description={modeDescription.peer}
        url={cfg.getSshSessionRoute({ sid, clusterId }, 'peer')}
        hasAccess={participantModes.includes('peer')}
        participantMode="peer"
        key="peer"
        showCTA={showCTA}
        closeMenu={closeMenu}
      />
    </JoinMenu>
  );
};

function JoinMenu({
  children,
  anchorEl,
  setAnchorEl,
}: {
  children: React.ReactNode;
  anchorEl: HTMLElement;
  setAnchorEl: React.Dispatch<React.SetStateAction<HTMLElement>>;
}) {
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
      <Menu
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'right',
        }}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'right',
        }}
        anchorEl={anchorEl}
        open={Boolean(anchorEl)}
        onClose={handleClose}
      >
        {children}
      </Menu>
    </Box>
  );
}

function JoinMenuItem({
  title,
  description,
  hasAccess,
  participantMode,
  url,
  showCTA,
  closeMenu,
}: {
  title: string;
  description: string;
  hasAccess: boolean;
  participantMode: ParticipantMode;
  url: string;
  showCTA: boolean;
  closeMenu: () => void;
}) {
  if (hasAccess && !showCTA) {
    return (
      <MenuItem
        as="a"
        href={url}
        target="_blank"
        onClick={closeMenu}
        css={`
          text-decoration: none;
          padding: 8px 12px;
          color: ${({ theme }) => theme.colors.text.main};
          user-select: none;
          border-bottom: 1px solid
            ${({ theme }) => theme.colors.spotBackground[0]};
        `}
      >
        <Box height="fit-content" width="264px">
          <Text typography="h6">{title}</Text>
          <Text color="text.slightlyMuted">{description}</Text>
        </Box>
      </MenuItem>
    );
  }
  return (
    <MenuItem
      css={`
        text-decoration: none;
        padding: 8px 12px;
        color: ${({ theme }) => theme.colors.text.disabled};
        user-select: none;
        cursor: auto;
        border-bottom: 1px solid
          ${({ theme }) => theme.colors.spotBackground[0]};
        &:hover {
          background-color: ${({ theme }) => theme.colors.levels.elevated};
          color: ${({ theme }) => theme.colors.text.disabled};
        }
      `}
    >
      <Box height="fit-content" width="264px">
        <Text typography="h6">{title}</Text>
        <Text>{description}</Text>
        {!showCTA && (
          <Box color="text.main" px={1} mt={1}>
            <Text fontSize="10px" color="text.slightlyMuted">
              <Warning color="error.main" mr={2} />
              {modeWarningText[participantMode]}
            </Text>
          </Box>
        )}
      </Box>
    </MenuItem>
  );
}

const modeDescription = {
  observer: 'Can view output but cannot send input.',
  moderator: 'Can view output & terminate the session.',
  peer: 'Can view output & send input.',
};

const modeWarningText = {
  observer: 'You do not have permission to join as an observer.',
  moderator: 'You do not have permission to join as a moderator.',
  peer: 'You do not have permission to join as a peer.',
};
