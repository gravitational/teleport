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
import styled, { useTheme } from 'styled-components';

import { Flex, Image } from 'design';
import { IconProps } from 'design/Icon/Icon';

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

  /**
   * Use a standard size. Otherwise, use `width` and `height` props.
   */
  size?: IconProps['size'];
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
  if (props.size) {
    const width = sizetoInnerPx(props.size);
    const height = sizetoInnerPx(props.size);
    return (
      <Container $size={props.size}>
        <Image
          src={icon}
          data-testid={`res-icon-${name}`}
          {...props}
          width={width}
          height={height}
        />
      </Container>
    );
  }

  return <Image src={icon} data-testid={`res-icon-${name}`} {...props} />;
};

const Container = styled(Flex)<{ $size: IconProps['size'] }>`
  width: ${props => sizetoOuterPx(props.$size)};
  height: ${props => sizetoOuterPx(props.$size)};
  align-items: center;
  justify-content: center;
`;

/**
 * Convert a standard size to a pixel width/height. This is different to the
 * conversion done for Icons as they include in-asset padding.
 *
 * @param size the standard size to convert.
 * @returns the pixel size
 */
function sizetoInnerPx(size: IconProps['size']) {
  if (size === 'small') return '14px';
  if (size === 'medium') return '16px';
  if (size === 'large') return '20px';
  if (size === 'extra-large') return '24px';
  return '24px';
}

function sizetoOuterPx(size: IconProps['size']) {
  if (size === 'small') return '16px';
  if (size === 'medium') return '20px';
  if (size === 'large') return '24px';
  if (size === 'extra-large') return '32px';
  return '32px';
}

export { type ResourceIconName, resourceIconSpecs, iconNames };
