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

import {
  GridFour,
  GridNine,
  Rows,
  RowsComfortable,
  RowsDense,
  SquaresFour,
} from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';
import {
  ViewModeSwitchButton,
  ViewModeSwitchContainer,
} from 'shared/components/Controls/ViewModeSwitch';

export enum ViewMode {
  Card,
  List,
}

export enum Density {
  Comfortable,
  Compact,
}

interface ViewSwitcherProps {
  viewMode: ViewMode;
  setViewMode: (viewMode: ViewMode) => void;
  density: Density;
  setDensity: (density: Density) => void;
}

export function ViewSwitcher({
  viewMode,
  setViewMode,
  density,
  setDensity,
}: ViewSwitcherProps) {
  return (
    <>
      <ViewModeSwitchContainer
        aria-label="View Mode Switch"
        aria-orientation="horizontal"
        role="radiogroup"
      >
        <HoverTooltip tipContent="Card View">
          <ViewModeSwitchButton
            className={viewMode === ViewMode.Card ? 'selected' : ''}
            onClick={() => setViewMode(ViewMode.Card)}
            role="radio"
            aria-label="Card View"
            aria-checked={viewMode === ViewMode.Card}
            first
          >
            <SquaresFour size="small" color="text.main" />
          </ViewModeSwitchButton>
        </HoverTooltip>
        <HoverTooltip tipContent="List View">
          <ViewModeSwitchButton
            className={viewMode === ViewMode.List ? 'selected' : ''}
            onClick={() => setViewMode(ViewMode.List)}
            role="radio"
            aria-label="List View"
            aria-checked={viewMode === ViewMode.List}
            last
          >
            <Rows size="small" color="text.main" />
          </ViewModeSwitchButton>
        </HoverTooltip>
      </ViewModeSwitchContainer>

      <ViewModeSwitchContainer
        aria-label="Density Switch"
        aria-orientation="horizontal"
        role="radiogroup"
      >
        <HoverTooltip tipContent="Compact View">
          <ViewModeSwitchButton
            className={density === Density.Compact ? 'selected' : ''}
            onClick={() => setDensity(Density.Compact)}
            role="radio"
            aria-label="Compact View"
            aria-checked={density === Density.Compact}
            first
          >
            {viewMode === ViewMode.Card ? (
              <GridNine size="small" color="text.main" />
            ) : (
              <RowsDense size="small" color="text.main" />
            )}
          </ViewModeSwitchButton>
        </HoverTooltip>
        <HoverTooltip tipContent="Comfortable View">
          <ViewModeSwitchButton
            className={density === Density.Comfortable ? 'selected' : ''}
            onClick={() => setDensity(Density.Comfortable)}
            role="radio"
            aria-label="Comfortable View"
            aria-checked={density === Density.Comfortable}
            last
          >
            {viewMode === ViewMode.Card ? (
              <GridFour size="small" color="text.main" />
            ) : (
              <RowsComfortable size="small" color="text.main" />
            )}
          </ViewModeSwitchButton>
        </HoverTooltip>
      </ViewModeSwitchContainer>
    </>
  );
}
