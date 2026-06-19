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

import { type ComponentProps, memo, type ReactElement } from 'react';
import styled from 'styled-components';

import { Flex, Image } from 'design';
import type { IconProps } from 'design/Icon/Icon';

import { iconNames, resourceIconSpecs } from './resourceIconSpecs';
import type { IconSpec, ResourceIconName } from './resourceIconSpecs';
import { ThemedImage } from './ThemedImage';

interface ResourceIconProps extends ComponentProps<typeof Image> {
  /**
   * Determines which icon will be displayed. See `resourceIconSpecs` for the* list of
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
export const ResourceIcon = memo(function ResourceIcon({
  name,
  size,
  width,
  height,
  ...rest
}: ResourceIconProps) {
  const spec: IconSpec | undefined = resourceIconSpecs[name];
  if (!spec) {
    return null;
  }

  // When a standard size is used, the icon renders at the "inner" size centered within an
  // "outer" container, matching the historical ResourceIcon padding.
  const innerWidth = size != null ? sizeToInnerPx(size) : (width as Dimension);
  const innerHeight =
    size != null ? sizeToInnerPx(size) : (height as Dimension);

  const common = {
    'data-testid': `res-icon-${name}`,
    width: innerWidth,
    height: innerHeight,
    ...rest,
  };

  let element: ReactElement;
  switch (spec.kind) {
    case 'mono': {
      const Component = spec.Icon;
      element = <Component {...common} />;
      break;
    }

    case 'themed':
      element = <ThemedImage {...common} dark={spec.dark} light={spec.light} />;
      break;

    case 'static':
      element = <Image {...common} src={spec.src} />;
      break;
  }

  if (size != null) {
    return <Container $size={size}>{element}</Container>;
  }

  return element;
});

type Dimension = number | string | undefined;

const Container = styled(Flex)<{ $size: IconProps['size'] }>`
  width: ${props => sizeToOuterPx(props.$size)};
  height: ${props => sizeToOuterPx(props.$size)};
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
function sizeToInnerPx(size: IconProps['size']) {
  if (typeof size === 'number') {
    return `${Math.floor(size * 0.8)}px`;
  }
  if (size === 'small') return '14px';
  if (size === 'medium') return '16px';
  if (size === 'large') return '20px';
  if (size === 'extra-large') return '24px';
  return '24px';
}

function sizeToOuterPx(size: IconProps['size']) {
  if (typeof size === 'number') {
    return `${size}px`;
  }
  if (size === 'small') return '16px';
  if (size === 'medium') return '20px';
  if (size === 'large') return '24px';
  if (size === 'extra-large') return '32px';
  return '32px';
}

export { resourceIconSpecs, iconNames };
export type { ResourceIconName };
