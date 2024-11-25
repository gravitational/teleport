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

import { Box, ButtonBorder, Flex, H3, Menu, MenuItem, Text } from 'design';
import { ChevronDown, Warning } from 'design/Icon';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import cfg from 'teleport/config';
import { ParticipantMode } from 'teleport/services/session';
import { CtaEvent } from 'teleport/services/userEvent';

export const SessionJoinBtn = ({
  sid,
  clusterId,
  participantModes,
  showCTA,
  showModeratedCTA,
}: {
  sid: string;
  clusterId: string;
  participantModes: ParticipantMode[];
  showCTA: boolean;
  showModeratedCTA: boolean;
}) => {
  const [anchorEl, setAnchorEl] = useState<HTMLElement>(null);

  function closeMenu() {
    setAnchorEl(null);
  }

  return (
    <JoinMenu anchorEl={anchorEl} setAnchorEl={setAnchorEl}>
      {showCTA && (
        <Box mx="12px" my={3}>
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
        showCTA={showCTA || showModeratedCTA}
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
      {showModeratedCTA && (
        <ButtonLockedFeature
          noIcon
          height="40px"
          event={CtaEvent.CTA_ACTIVE_SESSIONS}
          m={3}
          width="90%"
        >
          Join as a moderator with Teleport Enterprise
        </ButtonLockedFeature>
      )}
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
        <ChevronDown ml={1} size="small" color="text.slightlyMuted" />
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
          <H3>{title}</H3>
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
        <H3>{title}</H3>
        <Text>{description}</Text>
        {!showCTA && (
          <Box color="text.main" px={1} mt={1}>
            <Flex>
              <Warning color="error.main" mr={2} size="small" />
              <Text typography="body4" color="text.slightlyMuted">
                {modeWarningText[participantMode]}
              </Text>
            </Flex>
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
