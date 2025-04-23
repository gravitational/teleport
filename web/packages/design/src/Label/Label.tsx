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

import { space, SpaceProps } from '../system';
import { Theme } from '../theme';

const kind = ({ kind, theme }: { kind?: LabelKind; theme: Theme }) => {
  if (kind === 'secondary') {
    return {
      backgroundColor: theme.colors.spotBackground[0],
      color: theme.colors.text.main,
      fontWeight: 400,
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
  | 'success';

interface LabelProps extends SpaceProps {
  kind?: LabelKind;
  children?: React.ReactNode;
}

const Label = styled.div<LabelProps>`
  box-sizing: border-box;
  border-radius: 10px;
  display: inline-block;
  font-size: 10px;
  font-weight: 500;
  padding: 0 8px;
  margin: 1px 0;
  vertical-align: middle;

  ${kind}
  ${space}
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
