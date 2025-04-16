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

import { PropsWithChildren, ReactNode } from 'react';
import styled from 'styled-components';

import Flex from 'design/Flex';
import { space, SpaceProps } from 'design/system';
import Text from 'design/Text';
import { IconTooltip } from 'design/Tooltip';

interface LabelInputProps extends SpaceProps {
  hasError?: boolean;
}

export const LabelInput = styled.label<LabelInputProps>`
  color: ${props =>
    props.hasError
      ? props.theme.colors.error.main
      : props.theme.colors.text.main};
  display: block;
  width: 100%;
  margin-bottom: ${props => props.theme.space[1]}px;
  ${props => props.theme.typography.body3}
  ${space}
`;

/**
 * Renders the label body, optionally decorated with an asterisk that marks it
 * as a label for a required field, as well as an optional info icon with
 * tooltip. Can be used inside {@link LabelInput} or a different label-like
 * element, such as `<legend>`.
 */
export function LabelContent({
  required,
  tooltipContent,
  tooltipSticky,
  children,
  ...otherProps
}: PropsWithChildren<
  SpaceProps & {
    /** Adds a red asterisk after the field name. */
    required?: boolean;
    tooltipContent?: ReactNode;
    tooltipSticky?: boolean;
  }
>) {
  return (
    <Flex flexDirection="row" gap={2} alignItems="center" {...otherProps}>
      <Text typography="body3">
        {children}
        {required && (
          <RequiredIndicator aria-label="(required)"> *</RequiredIndicator>
        )}
      </Text>
      {tooltipContent && (
        <IconTooltip sticky={tooltipSticky}>{tooltipContent}</IconTooltip>
      )}
    </Flex>
  );
}

const RequiredIndicator = styled.span`
  color: ${props => props.theme.colors.interactive.solid.danger.default};
`;
