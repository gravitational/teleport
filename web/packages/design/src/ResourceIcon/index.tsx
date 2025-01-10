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

import { ComponentProps } from 'react';
import { useTheme } from 'styled-components';

import { Image } from 'design';

import {
  iconNames,
  ResourceIconName,
  resourceIconSpecs,
} from './resourceIconSpecs';

interface ResourceIconProps extends ComponentProps<typeof Image> {
  /**
   * Determines which icon will be displayed. See `iconSpecs` for the list of
   * available names.
   */
  name: ResourceIconName;
}

/**
 * Displays a resource icon of a given name for current theme. The icon
 * component exposes props of the underlying `Image`.
 */
export const ResourceIcon = ({ name, ...props }: ResourceIconProps) => {
  const theme = useTheme();
  const icon = resourceIconSpecs[name]?.[theme.type];
  if (!icon) {
    return null;
  }
  return <Image src={icon} data-testid={`res-icon-${name}`} {...props} />;
};

export { type ResourceIconName, resourceIconSpecs, iconNames };
