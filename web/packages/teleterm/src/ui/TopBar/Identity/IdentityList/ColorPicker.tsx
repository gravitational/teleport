/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { useState } from 'react';
import styled from 'styled-components';

import { Box, Flex } from 'design';
import { Pencil } from 'design/Icon';
import { useRefClickOutside } from 'shared/hooks/useRefClickOutside';

import {
  ProfileColor,
  profileColorMapping,
  profileColors,
} from 'teleterm/ui/services/workspacesService';

import { UserIcon } from '../IdentitySelector/UserIcon';

export function ColorPicker(props: {
  letter: string;
  color: ProfileColor;
  setColor(color: ProfileColor): void;
}) {
  const [open, setOpen] = useState(false);
  const [hoveredColor, setHoveredColor] = useState<ProfileColor | undefined>();
  const ref = useRefClickOutside<HTMLDivElement>({ open, setOpen });

  const $userIcon = (
    <UserIcon
      color={hoveredColor || props.color}
      onClick={() => setOpen(o => !o)}
      css={`
        cursor: pointer;
        height: 34px;
        width: 34px;

        &:hover {
          opacity: 0.9;
        }
      `}
      letter={props.letter}
    />
  );

  return (
    <Box
      css={`
        position: relative;
      `}
    >
      {$userIcon}
      <AbsolutePencilIcon />
      {open && (
        <Flex
          ref={ref}
          alignItems="center"
          css={`
            position: absolute;
            top: -4px;
            left: -4px;
            border-radius: 20px;
            box-shadow: rgba(0, 0, 0, 0.2) 0 1px 4px;
            z-index: 1;
          `}
          backgroundColor="levels.popout"
          flexDirection="row"
          p={1}
        >
          {$userIcon}
          <Flex alignItems="center" flexDirection="column" gap={2} px={2}>
            {profileColors.options.map(profileColor => (
              <Circle
                tabIndex={0}
                key={profileColor}
                color={profileColorMapping[profileColor]}
                onMouseEnter={() => setHoveredColor(profileColor)}
                onMouseLeave={() => setHoveredColor(undefined)}
                onClick={() => {
                  props.setColor(profileColor);
                  setOpen(false);
                }}
              />
            ))}
          </Flex>
        </Flex>
      )}
    </Box>
  );
}

const Circle = styled.button<{ color?: string }>`
  border-radius: 50%;
  background: ${props => props.color};
  height: 16px;
  width: 16px;
  border: none;
  cursor: pointer;
  box-shadow: rgba(0, 0, 0, 0.15) 0 1px 3px;

  &:hover {
    opacity: 0.9;
  }
`;

const AbsolutePencilIcon = styled(Pencil).attrs({ size: 11 })`
  position: absolute;
  bottom: -1px;
  left: 21px;
  border-radius: 50%;
  color: black;
  background: rgb(240, 240, 240);
  box-shadow: rgba(0, 0, 0, 0.15) 0 1px 3px;
  height: 16px;
  width: 16px;
`;
