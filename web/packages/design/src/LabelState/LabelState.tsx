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

import styled from 'styled-components';

import {
  color,
  ColorProps,
  fontSize,
  FontSizeProps,
  space,
  SpaceProps,
  width,
  WidthProps,
} from 'design/system';
import { Theme } from 'design/theme/themes/types';
import { fade } from 'design/theme/utils/colorManipulator';

export type LabelKind =
  | 'primary'
  | 'secondary'
  | 'warning'
  | 'danger'
  | 'success';

interface KindsProps {
  kind?: LabelKind;
  shadow?: boolean;
}

interface ThemedKindsProps extends KindsProps {
  theme: Theme;
}

const kinds = ({ theme, kind, shadow }: ThemedKindsProps) => {
  // default is primary
  const styles: { background: string; color: string; boxShadow?: string } = {
    background: theme.colors.brand,
    color: theme.colors.text.primaryInverse,
  };

  if (kind === 'secondary') {
    styles.background = theme.colors.spotBackground[0];
    styles.color = theme.colors.text.main;
  }

  if (kind === 'warning') {
    styles.background = theme.colors.warning.main;
    styles.color = theme.colors.text.primaryInverse;
  }

  if (kind === 'danger') {
    styles.background = theme.colors.error.main;
    styles.color = theme.colors.text.primaryInverse;
  }

  if (kind === 'success') {
    styles.background = theme.colors.success.main;
    styles.color = theme.colors.text.primaryInverse;
  }

  if (shadow) {
    styles.boxShadow = `
    0 0 8px ${fade(styles.background, 0.24)},
    0 4px 16px ${fade(styles.background, 0.56)}
    `;
  }

  return styles;
};

interface LabelStateProps
  extends SpaceProps,
    KindsProps,
    WidthProps,
    ColorProps,
    FontSizeProps {}

const LabelState = styled.span<LabelStateProps>`
  box-sizing: border-box;
  border-radius: 100px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 16px;
  line-height: 1.4;
  padding: 0 8px;
  font-size: ${props => props.theme.fontSizes[0]}px;
  font-weight: 500;
  text-transform: uppercase;
  ${space}
  ${kinds}
  ${width}
  ${color}
  ${fontSize}
`;

export default LabelState;

type LabelStatePropsWithoutKind = Omit<LabelStateProps, 'kind'>;

export const StateDanger = (props: LabelStatePropsWithoutKind) => (
  <LabelState kind="danger" {...props} />
);
export const StateInfo = (props: LabelStatePropsWithoutKind) => (
  <LabelState kind="secondary" {...props} />
);
export const StateWarning = (props: LabelStatePropsWithoutKind) => (
  <LabelState kind="warning" {...props} />
);
export const StateSuccess = (props: LabelStatePropsWithoutKind) => (
  <LabelState kind="success" {...props} />
);
