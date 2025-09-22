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

import React from 'react';
import styled from 'styled-components';

import { border, BorderProps, space, SpaceProps } from '../system';
import { Theme } from '../theme';

const kind = ({ kind, theme }: { kind?: LabelKind; theme: Theme }) => {
  if (kind === 'secondary') {
    return {
      backgroundColor: theme.colors.spotBackground[0],
      color: theme.colors.text.main,
      fontWeight: theme.fontWeights.regular,
    };
  }

  if (kind === 'warning') {
    return {
      backgroundColor: theme.colors.warning.main,
      color: theme.colors.text.primaryInverse,
    };
  }

  if (kind === 'danger') {
    return {
      backgroundColor: theme.colors.error.main,
      color: theme.colors.text.primaryInverse,
    };
  }

  if (kind === 'success') {
    return {
      backgroundColor: theme.colors.success.main,
      color: theme.colors.text.primaryInverse,
    };
  }

  if (kind === 'outline-secondary') {
    return {
      color: theme.colors.text.main,
      backgroundColor: 'transparent',
      borderColor: theme.colors.interactive.tonal.neutral[0],
      borderWidth: 1,
      borderStyle: 'solid',
      fontWeight: theme.fontWeights.regular,
    };
  }

  if (kind === 'outline-warning') {
    return {
      color: theme.colors.dataVisualisation.primary.sunflower,
      backgroundColor: theme.colors.interactive.tonal.alert[0],
      borderColor: theme.colors.interactive.tonal.alert[2],
      borderWidth: 1,
      borderStyle: 'solid',
      fontWeight: theme.fontWeights.regular,
    };
  }

  if (kind === 'outline-danger') {
    return {
      color: theme.colors.interactive.solid.danger.default,
      backgroundColor: theme.colors.interactive.tonal.danger[0],
      borderColor: theme.colors.interactive.tonal.danger[2],
      borderWidth: 1,
      borderStyle: 'solid',
      fontWeight: theme.fontWeights.regular,
    };
  }

  // default is primary
  return {
    backgroundColor: theme.colors.brand,
    color: theme.colors.text.primaryInverse,
  };
};

export type LabelKind =
  | 'primary'
  | 'secondary'
  | 'warning'
  | 'danger'
  | 'success'
  | 'outline-secondary'
  | 'outline-warning'
  | 'outline-danger';

type LabelProps = {
  kind?: LabelKind;
  children?: React.ReactNode;
} & SpaceProps &
  BorderProps;

const Label = styled.div<LabelProps>`
  box-sizing: border-box;
  border-radius: 999px;
  display: inline-block;
  font-size: 10px;
  font-weight: 500;
  padding: 0 8px;
  margin: 1px 0;
  vertical-align: middle;
  overflow: hidden;

  ${kind}
  ${space}
  ${border}
`;

export default Label;

type LabelPropsWithoutKind = Omit<LabelProps, 'kind'>;

export const Primary = (props: LabelPropsWithoutKind) => (
  <Label kind="primary" {...props} />
);
export const Secondary = (props: LabelPropsWithoutKind) => (
  <Label kind="secondary" {...props} />
);
export const Warning = (props: LabelPropsWithoutKind) => (
  <Label kind="warning" {...props} />
);
export const Danger = (props: LabelPropsWithoutKind) => (
  <Label kind="danger" {...props} />
);
export const SecondaryOutlined = (props: LabelPropsWithoutKind) => (
  <Label kind="outline-secondary" {...props} />
);
export const WarningOutlined = (props: LabelPropsWithoutKind) => (
  <Label kind="outline-warning" {...props} />
);
export const DangerOutlined = (props: LabelPropsWithoutKind) => (
  <Label kind="outline-danger" {...props} />
);
