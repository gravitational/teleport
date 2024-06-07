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
import { bool, string } from 'prop-types';

import { space, width, height, alignSelf, gap } from 'design/system';

const Button = ({ children, setRef = undefined, ...props }) => {
  return (
    <StyledButton {...props} ref={setRef}>
      {children}
    </StyledButton>
  );
};

const size = props => {
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

const themedStyles = props => {
  const { colors } = props.theme;
  const { kind } = props;

  let disabledStyle = {
    background: kind === 'text' ? 'none' : colors.buttons.bgDisabled,
    color: colors.buttons.textDisabled,
    cursor: 'auto',
  };

  let style = {
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

export const kinds = props => {
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
    case 'warning-border':
      return {
        color: theme.colors.buttons.warning.default,
        background: theme.colors.buttons.border.default,
        border: '1px solid ' + theme.colors.buttons.warning.default,
        '&:hover, &:focus': {
          background: theme.colors.buttons.warning.hover,
          color: theme.colors.buttons.warning.text,
        },
        '&:active': {
          background: theme.colors.buttons.warning.active,
          color: theme.colors.buttons.warning.text,
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

const block = props =>
  props.block
    ? {
        width: '100%',
      }
    : null;

const textTransform = props =>
  props.textTransform ? { textTransform: props.textTransform } : null;

const StyledButton = styled.button`
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

Button.propTypes = {
  /**
   * block specifies if an element's display is set to block or not.
   * Set to true to set display to block.
   */
  block: bool,

  /**
   * kind specifies the styling a button takes.
   * Select from primary (default), secondary, warning.
   */
  kind: string,

  /**
   * size specifies the size of button.
   * Select from small, medium (default), large
   */
  size: string,

  /**
   * textTransform specifies the case transform of the button text.
   * default is UPPERCASE
   *
   * TODO (avatus): eventually, we will move away from every button being
   * uppercase and this probably won't be needed anymore. This is a temporary
   * fix before we audit the whole site and migrate
   */
  textTransform: string,

  /**
   * styled-system
   */
  ...space.propTypes,
  ...height.propTypes,
  ...alignSelf.propTypes,
};

Button.defaultProps = {
  size: 'medium',
  kind: 'primary',
};

Button.displayName = 'Button';

export default Button;
export const ButtonPrimary = props => <Button kind="primary" {...props} />;
export const ButtonSecondary = props => <Button kind="secondary" {...props} />;
export const ButtonBorder = props => <Button kind="border" {...props} />;
export const ButtonWarning = props => <Button kind="warning" {...props} />;
export const ButtonWarningBorder = props => (
  <Button kind="warning-border" {...props} />
);
export const ButtonText = props => <Button kind="text" {...props} />;
