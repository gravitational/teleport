/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import styled from 'styled-components';

import { AlignSelfProps, HeightProps, SpaceProps, WidthProps } from 'styled-system';

import { ExecutionProps } from 'styled-components/dist/types';

import { alignSelf, gap, height, space, width } from 'design/system';
import { GapProps } from 'design/system/gap';

interface SizeProps {
  size?: 'small' | 'medium' | 'large';
}

const size = (props: SizeProps) => {
  switch (props.size) {
    case 'small':
      return {
        fontSize: '10px',
        minHeight: '24px',
        padding: '0px 16px',
      };
    case 'large':
      return {
        minHeight: '40px',
        fontSize: '12px',
        padding: '0px 40px',
      };
    default:
      // medium
      return {
        minHeight: '32px',
        fontSize: `12px`,
        padding: '0px 24px',
      };
  }
};

const themedStyles = (props: ExecutionProps & ButtonProps) => {
  const { colors } = props.theme;
  const { kind } = props;

  let disabledStyle = {
    background: kind === 'text' ? 'none' : colors.buttons.bgDisabled,
    color: colors.buttons.textDisabled,
    cursor: 'auto',
    pointerEvents: null,
  };

  let style:
    | {
        '&:disabled': typeof disabledStyle;
      }
    | {
        '&[disabled]': typeof disabledStyle;
      } = {
    '&:disabled': disabledStyle,
  };

  // Using the pseudo class `:disabled` to style disabled state
  // doesn't work for non form elements (e.g. anchor). So
  // we target by attribute with square brackets. Only true
  // when we change the underlying type for this component (button)
  // using the `as` prop (eg: a, NavLink, Link).
  if (props.as && props.disabled) {
    disabledStyle.pointerEvents = 'none';
    style = { '&[disabled]': disabledStyle };
  }

  return {
    ...kinds(props),
    ...style,
    ...size(props),
    ...space(props),
    ...width(props),
    ...block(props),
    ...height(props),
    ...textTransform(props),
    ...alignSelf(props),
    // Since a Button has `display: inline-flex`, we want to be able to set gap within it in case we
    // need to use an icon.
    ...gap(props),
  };
};

export const kinds = (props: ExecutionProps & ButtonProps) => {
  const { kind, theme } = props;
  switch (kind) {
    case 'secondary':
      return {
        color: theme.colors.buttons.text,
        background: theme.colors.buttons.secondary.default,
        '&:hover, &:focus': {
          background: theme.colors.buttons.secondary.hover,
        },
        '&:active': {
          background: theme.colors.buttons.secondary.active,
        },
      };
    case 'border':
      return {
        color: theme.colors.buttons.text,
        background: theme.colors.buttons.border.default,
        border: '1px solid ' + theme.colors.buttons.border.border,
        '&:hover, &:focus': {
          background: theme.colors.buttons.border.hover,
        },
        '&:active': {
          background: theme.colors.buttons.border.active,
        },
      };
    case 'warning':
      return {
        color: theme.colors.buttons.warning.text,
        background: theme.colors.buttons.warning.default,
        '&:hover, &:focus': {
          background: theme.colors.buttons.warning.hover,
        },
        '&:active': {
          background: theme.colors.buttons.warning.active,
        },
      };
    case 'text':
      return {
        color: theme.colors.buttons.text,
        background: 'none',
        'text-transform': 'none',
        '&:hover, &:focus': {
          background: 'none',
          'text-decoration': 'underline',
        },
      };
    case 'primary':
    default:
      return {
        color: theme.colors.buttons.primary.text,
        background: theme.colors.buttons.primary.default,
        '&:hover, &:focus': {
          background: theme.colors.buttons.primary.hover,
        },
        '&:active': {
          background: theme.colors.buttons.primary.active,
        },
      };
  }
};

export interface BlockProps {
  block?: boolean;
}

const block = (props: BlockProps) =>
  props.block
    ? {
        width: '100%',
      }
    : null;

export interface TextTransformProps {
  textTransform?: string;
}

const textTransform = (props: TextTransformProps) =>
  props.textTransform ? { textTransform: props.textTransform } : null;

interface ButtonBaseProps {
  kind?: 'primary' | 'secondary' | 'border' | 'warning' | 'text';
  disabled?: boolean;
}

export type ButtonProps = ButtonBaseProps &
  SizeProps &
  SpaceProps &
  WidthProps &
  BlockProps &
  HeightProps &
  TextTransformProps &
  AlignSelfProps &
  GapProps;

const Button = styled.button<ButtonProps>`
  line-height: 1.5;
  margin: 0;
  display: inline-flex;
  justify-content: center;
  align-items: center;
  box-sizing: border-box;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-family: inherit;
  font-weight: 600;
  outline: none;
  position: relative;
  text-align: center;
  text-decoration: none;
  text-transform: uppercase;
  transition: all 0.3s;
  -webkit-font-smoothing: antialiased;

  ${themedStyles}
`;

Button.defaultProps = {
  size: 'medium',
  kind: 'primary',
};

Button.displayName = 'Button';

export default Button;

export const ButtonPrimary = styled(Button).attrs({ kind: 'primary' })<
  Omit<ButtonProps, 'kind'>
>``;
export const ButtonSecondary = styled(Button).attrs({ kind: 'secondary' })<
  Omit<ButtonProps, 'kind'>
>``;
export const ButtonBorder = styled(Button).attrs({ kind: 'border' })<
  Omit<ButtonProps, 'kind'>
>``;
export const ButtonWarning = styled(Button).attrs({ kind: 'warning' })<
  Omit<ButtonProps, 'kind'>
>``;
export const ButtonText = styled(Button).attrs({ kind: 'text' })<
  Omit<ButtonProps, 'kind'>
>``;
