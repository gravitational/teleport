/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import styled from 'styled-components';

import { Rows, SquaresFour } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';
import { ViewMode } from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';

export const ViewModeSwitch = ({
  currentViewMode,
  setCurrentViewMode,
}: {
  currentViewMode: ViewMode;
  setCurrentViewMode: (viewMode: ViewMode) => void;
}) => {
  return (
    <ViewModeSwitchContainer
      aria-label="View Mode Switch"
      aria-orientation="horizontal"
      role="radiogroup"
    >
      <HoverTooltip tipContent="Card View">
        <ViewModeSwitchButton
          className={currentViewMode === ViewMode.CARD ? 'selected' : ''}
          onClick={() => setCurrentViewMode(ViewMode.CARD)}
          role="radio"
          aria-label="Card View"
          aria-checked={currentViewMode === ViewMode.CARD}
          first
        >
          <SquaresFour size="small" color="text.main" />
        </ViewModeSwitchButton>
      </HoverTooltip>
      <HoverTooltip tipContent="List View">
        <ViewModeSwitchButton
          className={currentViewMode === ViewMode.LIST ? 'selected' : ''}
          onClick={() => setCurrentViewMode(ViewMode.LIST)}
          role="radio"
          aria-label="List View"
          aria-checked={currentViewMode === ViewMode.LIST}
          last
        >
          <Rows size="small" color="text.main" />
        </ViewModeSwitchButton>
      </HoverTooltip>
    </ViewModeSwitchContainer>
  );
};

const ViewModeSwitchContainer = styled.div`
  height: 22px;
  border: ${p => p.theme.borders[1]} ${p => p.theme.colors.spotBackground[2]};
  border-radius: ${p => p.theme.radii[2]}px;
  display: flex;

  .selected {
    background-color: ${p => p.theme.colors.spotBackground[1]};

    &:focus-visible,
    &:hover {
      background-color: ${p => p.theme.colors.spotBackground[1]};
    }
  }
`;

const ViewModeSwitchButton = styled.button<{ first?: boolean; last?: boolean }>`
  height: 100%;
  width: 100%;
  overflow: hidden;
  border: none;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  background-color: transparent;
  outline: none;
  transition: outline-width 150ms ease;

  ${p =>
    p.first &&
    `
    border-top-left-radius: ${p.theme.radii[2]}px;
    border-bottom-left-radius: ${p.theme.radii[2]}px;
    border-right: ${p.theme.borders[1]} ${p.theme.colors.spotBackground[2]};
  `}
  ${p =>
    p.last &&
    `
    border-top-right-radius: ${p.theme.radii[2]}px;
    border-bottom-right-radius: ${p.theme.radii[2]}px;
  `}

  &:focus-visible {
    outline: ${p => p.theme.borders[1]}
      ${p => p.theme.colors.text.slightlyMuted};
  }

  &:focus-visible,
  &:hover {
    background-color: ${p => p.theme.colors.spotBackground[0]};
  }
`;
