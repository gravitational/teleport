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
  shouldDisplayWarning: boolean;
  showingStatusInfo: boolean;
}

export const getBackgroundColor = (props: BackgroundColorProps) => {
  if (props.shouldDisplayWarning) {
    return 'transparent';
  }
  if (props.selected) {
    return props.theme.colors.interactive.tonal.primary[2];
  }
  if (props.pinned) {
    return props.theme.colors.interactive.tonal.primary[1];
  }
  return 'transparent';
};

export const getStatusBackgroundColor = (props: {
  showingStatusInfo: boolean;
  theme: Theme;
  action: '' | 'hover';
  viewType: 'card' | 'list';
}) => {
  switch (props.action) {
    case 'hover':
      return props.theme.colors.interactive.tonal.alert[1];
    case '':
      if (props.showingStatusInfo) {
        return props.theme.colors.interactive.tonal.alert[2];
      }

      switch (props.viewType) {
        case 'card':
          return 'transparent';
        case 'list':
          return props.theme.colors.interactive.tonal.alert[0];
        default:
          props.viewType satisfies never;
          return;
      }
    default:
      props.action satisfies never;
  }
};
