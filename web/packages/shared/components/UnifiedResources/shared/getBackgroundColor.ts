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

import { Theme } from 'design/theme/themes/types';

export interface BackgroundColorProps {
  requiresRequest?: boolean;
  selected?: boolean;
  pinned?: boolean;
  theme: Theme;
}

export const getBackgroundColor = (props: BackgroundColorProps) => {
  if (props.requiresRequest && props.pinned) {
    return props.theme.colors.interactive.tonal.primary[0];
  }
  if (props.requiresRequest) {
    return props.theme.colors.spotBackground[0];
  }
  if (props.selected) {
    return props.theme.colors.interactive.tonal.primary[2];
  }
  if (props.pinned) {
    return props.theme.colors.interactive.tonal.primary[1];
  }
  return 'transparent';
};
