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

export type RadioButtonSize = 'large' | 'small';

interface RadioButtonProps {
  size?: RadioButtonSize;

  // Input properties
  autoFocus?: boolean;
  checked?: boolean;
  defaultChecked?: boolean;
  disabled?: boolean;
  id?: string;
  name?: string;
  readonly?: boolean;
  role?: string;
  value?: string;

  // Container properties
  className?: string;
  style?: React.CSSProperties;

  onChange?: (e: React.ChangeEvent<HTMLInputElement>) => void;
}

export const RadioButton = forwardRef<HTMLInputElement, RadioButtonProps>(
  (props, ref) => {
    // Implementation note: the structure of radio buttons is actually a copy of
    // the Checkbox component; however, because of subtle differences, I decided
    // not to extract any common parts, as they would only add unnecessary
    // complexity.
    const { style, className, size, ...inputProps } = props;
    return (
      // The outer wrapper and inner wrapper are separate to allow using
      // positioning CSS attributes on the radio button while still maintaining
      // its internal integrity that requires the internal wrapper to be
      // positioned.
      <OuterWrapper style={style} className={className}>
        <InnerWrapper>
          {/* The radio button is rendered as two items placed on top of each
            other: the actual radio button, which is a native input control, and
            a state indicator. Note that we avoid the usual "label with content"
            trick, because we want to be able to use this component both with
            and without surrounding labels. Instead, we use absolute positioning
            and an actually rendered input with a custom appearance. */}
          <RadioButtonInternal ref={ref} rbSize={size} {...inputProps} />
          <Indicator rbSize={size} />
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

const Indicator = styled.span<{ rbSize?: RadioButtonSize }>`
  border-radius: 50%;
  position: absolute;
  ${indicatorSize}
  left: 0;
  right: 0;
  top: 0;
  bottom: 0;
  margin: auto;
  pointer-events: none;
  opacity: 0;

  transition: all 150ms;

  input:checked + & {
    opacity: 1;
  }

  input:enabled + & {
    background: ${props =>
      props.theme.colors.interactive.solid.primary.default};
  }

  input:enabled:hover + &,
  .teleport-radio-button__force-hover input + & {
    background-color: ${props =>
      props.theme.colors.interactive.solid.primary.hover};
  }

  input:enabled:focus-visible + &,
  .teleport-radio-button__force-focus-visible input + & {
    background-color: ${props =>
      props.theme.colors.interactive.solid.primary.default};
  }

  input:enabled:active + &,
  .teleport-radio-button__force-active input + & {
    background-color: ${props =>
      props.theme.colors.interactive.solid.primary.active};
  }

  input:disabled + & {
    background-color: ${props => props.theme.colors.text.disabled};
  }
`;

function indicatorSize(props: { rbSize?: RadioButtonSize }) {
  const { rbSize = 'large' } = props;
  let s = '';
  switch (rbSize) {
    case 'large':
      s = '10px';
      break;
    case 'small':
      s = '8px';
      break;
    default:
      rbSize satisfies never;
  }
  return { width: s, height: s };
}

export const RadioButtonInternal = styled.input.attrs({ type: 'radio' })<{
  rbSize?: RadioButtonSize;
}>`
  appearance: none;
  border-style: solid;
  border-color: ${props => props.theme.colors.text.muted};
  border-radius: 50%;
  background-color: transparent;
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
      border-color: ${props =>
        props.theme.colors.interactive.solid.primary.default};
    }

    &:hover,
    .teleport-radio-button__force-hover & {
      background-color: ${props =>
        props.theme.colors.interactive.tonal.neutral[0]};
      border-color: ${props => props.theme.colors.text.slightlyMuted};
      box-shadow:
        0px 2px 1px -1px rgba(0, 0, 0, 0.2),
        0px 1px 1px 0px rgba(0, 0, 0, 0.14),
        0px 1px 3px 0px rgba(0, 0, 0, 0.12);

      &:checked {
        background-color: transparent;
        border-color: ${props =>
          props.theme.colors.interactive.solid.primary.hover};
      }
    }

    &:focus-visible,
    .teleport-radio-button__force-focus-visible & {
      background-color: ${props =>
        props.theme.colors.interactive.tonal.neutral[0]};
      border-color: ${props =>
        props.theme.colors.interactive.solid.primary.default};
      outline: 3px solid
        ${props => props.theme.colors.interactive.solid.primary.default};
      outline-offset: -1px;

      &:checked {
        border-color: ${props =>
          props.theme.colors.interactive.solid.primary.default};
        background-color: transparent;
      }
    }

    &:active,
    .teleport-radio-button__force-active & {
      background-color: ${props =>
        props.theme.colors.interactive.tonal.neutral[1]};
      border-color: ${props => props.theme.colors.text.slightlyMuted};

      &:checked {
        border-color: ${props =>
          props.theme.colors.interactive.solid.primary.active};
        background-color: transparent;
      }
    }
  }

  &:disabled {
    border-color: ${props => props.theme.colors.text.disabled};
  }

  ${size}
`;

/**
 * Returns dimensions of a radio button with a given `size` property. Since its name
 * conflicts with the native `size` attribute with a different type and
 * semantics, we use `rbSize` here.
 */
function size(props: { rbSize?: RadioButtonSize }) {
  const { rbSize = 'large' } = props;
  let s = '';
  let borderWidth = '';
  switch (rbSize) {
    case 'large':
      s = '18px';
      borderWidth = '1.5px';
      break;
    case 'small':
      s = '14px';
      borderWidth = '1px';
      break;
    default:
      rbSize satisfies never;
  }
  return { width: s, height: s, borderWidth };
}
