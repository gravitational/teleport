/*
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

import {
  style,
  px,
  compose,
  BorderRadiusProps as StyledSystemBorderRadiusProps,
  ResponsiveValue,
  TLengthStyledSystem,
} from 'styled-system';
import { Property } from 'csstype';

export interface BorderRadiusProps<TLength = TLengthStyledSystem>
  extends StyledSystemBorderRadiusProps<TLength> {
  borderTopLeftRadius?: ResponsiveValue<Property.BorderTopRightRadius<TLength>>;
  borderTopRightRadius?: ResponsiveValue<
    Property.BorderTopRightRadius<TLength>
  >;
  borderBottomLeftRadius?: ResponsiveValue<
    Property.BorderBottomLeftRadius<TLength>
  >;
  borderBottomRightRadius?: ResponsiveValue<
    Property.BorderBottomRightRadius<TLength>
  >;
}

export const borderTopLeftRadius = style({
  prop: 'borderTopLeftRadius',
  key: 'radii',
  transformValue: px,
});

export const borderTopRightRadius = style({
  prop: 'borderTopRightRadius',
  key: 'radii',
  transformValue: px,
});

export const borderRadiusBottomRight = style({
  prop: 'borderBottomRightRadius',
  key: 'radii',
  transformValue: px,
});

export const borderBottomLeftRadius = style({
  prop: 'borderBottomLeftRadius',
  key: 'radii',
  transformValue: px,
});

export const borderRadius = style({
  prop: 'borderRadius',
  key: 'radii',
  transformValue: px,
});

const combined = compose(
  borderRadius,
  borderTopLeftRadius,
  borderTopRightRadius,
  borderRadiusBottomRight,
  borderBottomLeftRadius
);

export default combined;
