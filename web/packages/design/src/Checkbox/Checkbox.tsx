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

import React, { forwardRef } from 'react';
import styled from 'styled-components';

import * as Icon from 'design/Icon';

export type CheckboxSize = 'large' | 'small';

interface CheckboxInputProps {
  size?: CheckboxSize;

  // Input properties
  autoFocus?: boolean;
  checked?: boolean;
  defaultChecked?: boolean;
  disabled?: boolean;
  id?: string;
  name?: string;
  readOnly?: boolean;
  role?: string;
  type?: 'checkbox' | 'radio';
  value?: string;

  // TODO(bl-nero): Support the "indeterminate" property.

  // Container properties
  className?: string;
  style?: React.CSSProperties;

  onChange?: (e: React.ChangeEvent<HTMLInputElement>) => void;
}

export const CheckboxInput = forwardRef<HTMLInputElement, CheckboxInputProps>(
  (props, ref) => {
    const { style, className, size, ...inputProps } = props;
    return (
      // The outer wrapper and inner wrapper are separate to allow using
      // positioning CSS attributes on the checkbox while still maintaining its
      // internal integrity that requires the internal wrapper to be positioned.
      <OuterWrapper style={style} className={className}>
        <InnerWrapper>
          {/* The checkbox is rendered as two items placed on top of each other:
            the actual checkbox, which is a native input control, and an SVG
            checkmark. Note that we avoid the usual "label with content" trick,
            because we want to be able to use this component both with and
            without surrounding labels. Instead, we use absolute positioning and
            an actually rendered input with a custom appearance. */}
          <CheckboxInternal ref={ref} cbSize={size} {...inputProps} />
          <Checkmark />
        </InnerWrapper>
      </OuterWrapper>
    );
  }
);

const OuterWrapper = styled.span`
  display: inline-block;
  line-height: 0;
  margin: 3px;
`;

const InnerWrapper = styled.span`
  display: inline-block;
  position: relative;
`;

const Checkmark = styled(Icon.CheckThick)`
  position: absolute;
  left: 1px;
  top: 1px;
  right: 1px;
  bottom: 1px;
  pointer-events: none;
  color: ${props => props.theme.colors.text.primaryInverse};
  opacity: 0;

  transition: all 150ms;

  input:checked + & {
    opacity: 1;
  }

  input:disabled + & {
    color: ${props => props.theme.colors.text.main};
  }
`;

const CheckboxInternal = styled.input.attrs(props => ({
  // TODO(bl-nero): Make radio buttons a separate control.
  type: props.type || 'checkbox',
}))<{ cbSize?: CheckboxSize }>`
  // reset the appearance so we can style the background
  -webkit-appearance: none;
  -moz-appearance: none;
  appearance: none;
  border: 1.5px solid ${props => props.theme.colors.text.muted};
  border-radius: ${props => props.theme.radii[2]}px;
  background: transparent;
  cursor: ${props => (props.disabled ? 'not-allowed' : 'pointer')};

  position: relative;
  margin: 0;

  // Give it some animation, but don't animate focus-related properties.
  transition:
    border-color 150ms,
    background-color 150ms,
    box-shadow 150ms;

  // State-specific styles. Note: the "force" classes are required for
  // Storybook, where we want to show all the states, even though we can't
  // enforce them.
  &:enabled {
    &:checked {
      background-color: ${props => props.theme.colors.buttons.primary.default};
      border-color: transparent;
    }

    &:hover,
    .teleport-checkbox__force-hover & {
      background-color: ${props =>
        props.theme.colors.interactive.tonal.neutral[0]};
      border-color: ${props => props.theme.colors.text.slightlyMuted};

      &:checked {
        background-color: ${props => props.theme.colors.buttons.primary.hover};
        border-color: transparent;
        box-shadow:
          0px 2px 1px -1px rgba(0, 0, 0, 0.2),
          0px 1px 1px 0px rgba(0, 0, 0, 0.14),
          0px 1px 3px 0px rgba(0, 0, 0, 0.12);
      }
    }

    &:focus-visible,
    .teleport-checkbox__force-focus-visible & {
      background-color: ${props =>
        props.theme.colors.interactive.tonal.neutral[0]};
      border-color: ${props => props.theme.colors.buttons.primary.default};
      outline: none;
      border-width: 2px;

      &:checked {
        background-color: ${props =>
          props.theme.colors.buttons.primary.default};
        border-color: transparent;
        outline: 2px solid
          ${props => props.theme.colors.buttons.primary.default};
        outline-offset: 1px;
      }
    }

    &:active,
    .teleport-checkbox__force-active & {
      background-color: ${props =>
        props.theme.colors.interactive.tonal.neutral[1]};
      border-color: ${props => props.theme.colors.text.slightlyMuted};

      &:checked {
        background-color: ${props => props.theme.colors.buttons.primary.active};
        border-color: transparent;
      }
    }
  }

  &:disabled {
    background-color: ${props =>
      props.theme.colors.interactive.tonal.neutral[0]};
    border-color: transparent;
  }

  ${size}
`;

/**
 * Returns dimensions of a checkbox with a given `size` property. Since its name
 * conflicts with the native `size` attribute with a different type and
 * semantics, we use `cbSize` here.
 */
function size(props: { cbSize?: CheckboxSize }) {
  const { cbSize = 'large' } = props;
  let s = '';
  switch (cbSize) {
    case 'large':
      s = '18px';
      break;
    case 'small':
      s = '14px';
      break;
    default:
      cbSize satisfies never;
  }
  return { width: s, height: s };
}
