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

const kind = ({
  kind,
  theme,
  withHoverState = false,
}: {
  kind?: LabelKind;
  theme: Theme;
  withHoverState?: boolean;
}) => {
  if (kind === 'secondary') {
    return {
      backgroundColor: theme.colors.spotBackground[0],
      color: theme.colors.text.main,
      fontWeight: theme.fontWeights.regular,
      ...(withHoverState && {
        '&:hover': {
          text: theme.colors.text.primaryInverse,
          background: theme.colors.interactive.tonal.neutral[1],
        },
      }),
    };
  }

  if (kind === 'warning') {
    return {
      backgroundColor: theme.colors.warning.main,
      color: theme.colors.text.primaryInverse,
      ...(withHoverState && {
        '&:hover': {
          color: theme.colors.text.primaryInverse,
          backgroundColor: theme.colors.interactive.solid['alert'].hover,
        },
      }),
    };
  }

  if (kind === 'danger') {
    return {
      backgroundColor: theme.colors.error.main,
      color: theme.colors.text.primaryInverse,
      ...(withHoverState && {
        '&:hover': {
          color: theme.colors.text.primaryInverse,
          backgroundColor: theme.colors.interactive.solid['danger'].hover,
        },
      }),
    };
  }

  if (kind === 'success') {
    return {
      backgroundColor: theme.colors.success.main,
      color: theme.colors.text.primaryInverse,
      ...(withHoverState && {
        '&:hover': {
          color: theme.colors.text.primaryInverse,
          backgroundColor: theme.colors.interactive.solid['success'].hover,
        },
      }),
    };
  }

  if (kind === 'outline-primary') {
    return {
      color: theme.colors.brand,
      backgroundColor: 'transparent',
      borderColor: theme.colors.brand,
      borderWidth: 1,
      borderStyle: 'solid',
      fontWeight: theme.fontWeights.regular,
      ...(withHoverState && {
        '&:hover': {
          color: theme.colors.text.primaryInverse,
          backgroundColor: theme.colors.interactive.solid['primary'].hover,
        },
      }),
    };
  }

  if (kind === 'outline-secondary') {
    return {
      color: theme.colors.text.slightlyMuted,
      backgroundColor: theme.colors.interactive.tonal.neutral[0],
      borderColor: theme.colors.text.slightlyMuted,
      borderWidth: 1,
      borderStyle: 'solid',
      fontWeight: theme.fontWeights.regular,
      ...(withHoverState && {
        '&:hover': {
          text: theme.colors.text.primaryInverse,
          background: theme.colors.interactive.tonal.neutral[1],
        },
      }),
    };
  }

  if (kind === 'outline-success') {
    return {
      color: theme.colors.interactive.solid.success.hover,
      backgroundColor: theme.colors.interactive.tonal.success[0],
      borderColor: theme.colors.interactive.solid.success.hover,
      borderWidth: 1,
      borderStyle: 'solid',
      fontWeight: theme.fontWeights.regular,
      ...(withHoverState && {
        '&:hover': {
          color: theme.colors.text.primaryInverse,
          backgroundColor: theme.colors.interactive.solid['success'].hover,
        },
      }),
    };
  }

  if (kind === 'outline-warning') {
    return {
      color: theme.colors.dataVisualisation.primary.sunflower,
      backgroundColor: theme.colors.interactive.tonal.alert[0],
      borderColor: theme.colors.dataVisualisation.primary.sunflower,
      borderWidth: 1,
      borderStyle: 'solid',
      fontWeight: theme.fontWeights.regular,
      ...(withHoverState && {
        '&:hover': {
          color: theme.colors.text.primaryInverse,
          backgroundColor: theme.colors.interactive.solid['alert'].hover,
        },
      }),
    };
  }

  if (kind === 'outline-danger') {
    return {
      color: theme.colors.dataVisualisation.tertiary.abbey,
      backgroundColor: theme.colors.interactive.tonal.danger[0],
      borderColor: theme.colors.interactive.solid.danger.default,
      borderWidth: 1,
      borderStyle: 'solid',
      fontWeight: theme.fontWeights.regular,
      ...(withHoverState && {
        '&:hover': {
          color: theme.colors.text.primaryInverse,
          backgroundColor: theme.colors.interactive.solid['danger'].hover,
        },
      }),
    };
  }

  // default is primary
  return {
    backgroundColor: theme.colors.brand,
    color: theme.colors.text.primaryInverse,
    ...(withHoverState && {
      '&:hover': {
        color: theme.colors.text.primaryInverse,
        backgroundColor: theme.colors.interactive.solid['primary'].hover,
      },
    }),
  };
};

/**
 * @deprecated Use `Status` from `design/Status` for semantic states
 * (success, warning, danger) or `Tag` from `design/Tag` for neutral metadata.
 */
export type LabelKind =
  | 'primary'
  | 'secondary'
  | 'warning'
  | 'danger'
  | 'success'
  | 'outline-secondary'
  | 'outline-warning'
  | 'outline-danger'
  | 'outline-primary'
  | 'outline-success';

/**
 * @deprecated Use `StatusProps` from `design/Status` for semantic states
 * (success, warning, danger) or `TagProps` from `design/Tag` for neutral metadata.
 */
export type LabelProps = {
  kind?: LabelKind;
  withHoverState?: boolean;
  children?: React.ReactNode;
} & SpaceProps &
  BorderProps;

/**
 * @deprecated Use `Status` from `design/Status` for semantic states
 * (success, warning, danger) or `Tag` from `design/Tag` for neutral metadata.
 */
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

/** @deprecated Use `<Status kind="primary" variant="filled">` from `design/Status`. */
export const Primary = (props: LabelPropsWithoutKind) => (
  <Label kind="primary" {...props} />
);
/** @deprecated Use `<Tag>` from `design/Tag`. */
export const Secondary = (props: LabelPropsWithoutKind) => (
  <Label kind="secondary" {...props} />
);
/** @deprecated Use `<Status kind="warning" variant="filled">` from `design/Status`. */
export const Warning = (props: LabelPropsWithoutKind) => (
  <Label kind="warning" {...props} />
);
/** @deprecated Use `<Status kind="danger" variant="filled">` from `design/Status`. */
export const Danger = (props: LabelPropsWithoutKind) => (
  <Label kind="danger" {...props} />
);
/** @deprecated Use `<Tag variant="outline">` from `design/Tag`. */
export const SecondaryOutlined = (props: LabelPropsWithoutKind) => (
  <Label kind="outline-secondary" {...props} />
);
/** @deprecated Use `<Status kind="success">` from `design/Status`. */
export const SuccessOutlined = (props: LabelPropsWithoutKind) => (
  <Label kind="outline-success" {...props} />
);
/** @deprecated Use `<Status kind="warning">` from `design/Status`. */
export const WarningOutlined = (props: LabelPropsWithoutKind) => (
  <Label kind="outline-warning" {...props} />
);
/** @deprecated Use `<Status kind="danger">` from `design/Status`. */
export const DangerOutlined = (props: LabelPropsWithoutKind) => (
  <Label kind="outline-danger" {...props} />
);
