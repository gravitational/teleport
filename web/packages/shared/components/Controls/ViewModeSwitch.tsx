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

import React from 'react';
import styled from 'styled-components';
import { Rows, SquaresFour } from 'design/Icon';

import { ViewMode } from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';

import { HoverTooltip } from 'shared/components/ToolTip';

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
          css={`
            border-right: 1px solid
              ${props => props.theme.colors.spotBackground[2]};
            border-top-left-radius: 4px;
            border-bottom-left-radius: 4px;
          `}
          role="radio"
          aria-label="Card View"
          aria-checked={currentViewMode === ViewMode.CARD}
        >
          <SquaresFour size="small" color="text.main" />
        </ViewModeSwitchButton>
      </HoverTooltip>
      <HoverTooltip tipContent="List View">
        <ViewModeSwitchButton
          className={currentViewMode === ViewMode.LIST ? 'selected' : ''}
          onClick={() => setCurrentViewMode(ViewMode.LIST)}
          css={`
            border-top-right-radius: 4px;
            border-bottom-right-radius: 4px;
          `}
          role="radio"
          aria-label="List View"
          aria-checked={currentViewMode === ViewMode.LIST}
        >
          <Rows size="small" color="text.main" />
        </ViewModeSwitchButton>
      </HoverTooltip>
    </ViewModeSwitchContainer>
  );
};

const ViewModeSwitchContainer = styled.div`
  height: 22px;
  width: 48px;
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

const ViewModeSwitchButton = styled.button`
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

  &:focus-visible {
    outline: ${p => p.theme.borders[1]}
      ${p => p.theme.colors.text.slightlyMuted};
  }

  &:focus-visible,
  &:hover {
    background-color: ${p => p.theme.colors.spotBackground[0]};
  }
`;
