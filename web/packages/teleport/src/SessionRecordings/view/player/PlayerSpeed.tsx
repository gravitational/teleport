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

import { useMemo, useRef, useState } from 'react';
import styled from 'styled-components';

import Flex from 'design/Flex';
import { Check } from 'design/Icon';
import Menu, { MenuItem } from 'design/Menu';
import { HoverTooltip } from 'design/Tooltip';

interface PlayerSpeedProps {
  speed: number;
  portalRoot: HTMLElement;
  onSpeedChange: (speed: number) => void;
}

const SpeedButton = styled.button`
  background: transparent;
  border: none;
  color: ${p => p.theme.colors.text.main};
  cursor: pointer;
  height: 32px;
  font-size: ${p => p.theme.fontSizes[2]}px;
  padding: ${p => p.theme.space[1]}px
    ${p => p.theme.space[2] + p.theme.space[1]}px;
  line-height: 1;
  border-radius: ${p => p.theme.radii[3]}px;

  &:hover {
    background: ${p => p.theme.colors.spotBackground[1]};
  }
`;

const StyledMenu = styled(Menu)`
  width: 150px;
`;

const StyledMenuItem = styled(MenuItem)`
  font-size: ${p => p.theme.fontSizes[2]}px;
  display: flex;
  align-items: center;
`;

const AVAILABLE_SPEEDS = [0.25, 0.5, 1, 1.5, 2, 3, 4];

export function PlayerSpeed({
  onSpeedChange,
  portalRoot,
  speed,
}: PlayerSpeedProps) {
  const [isOpen, setIsOpen] = useState(false);

  const ref = useRef<HTMLButtonElement>(null);

  const items = useMemo(() => {
    return AVAILABLE_SPEEDS.map(s => (
      <StyledMenuItem
        key={s}
        onClick={() => {
          onSpeedChange(s);
        }}
      >
        <Flex height="24px" width="24px">
          {s === speed && <Check size="small" />}
        </Flex>
        {s}x
      </StyledMenuItem>
    ));
  }, [onSpeedChange, speed]);

  return (
    <>
      <HoverTooltip tipContent="Playback Speed" portalRoot={portalRoot}>
        <SpeedButton
          onClick={() => {
            setIsOpen(true);
          }}
          ref={ref}
        >
          {speed}x
        </SpeedButton>
      </HoverTooltip>

      <StyledMenu
        anchorEl={ref.current}
        open={isOpen}
        onClose={() => setIsOpen(false)}
        // hack to properly position the menu
        getContentAnchorEl={null}
        anchorOrigin={{
          vertical: 'top',
          horizontal: 'right',
        }}
        transformOrigin={{
          vertical: 'bottom',
          horizontal: 'right',
        }}
        menuListProps={{ onClick: () => setIsOpen(false) }}
      >
        {items}
      </StyledMenu>
    </>
  );
}
