/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import styled, { css } from 'styled-components';

import { Text } from 'design';
import { HoverTooltip } from 'design/Tooltip';

export type UserDisplayNameLayout = 'inline' | 'stacked' | 'tooltip';

export interface UserDisplayNameProps {
  username: string;
  primaryText?: string;
  secondaryText?: string;
  layout?: UserDisplayNameLayout;
  className?: string;
}

export function UserDisplayName({
  username,
  primaryText,
  secondaryText,
  layout = 'tooltip',
  className,
}: UserDisplayNameProps) {
  const displayPrimary = normalizeText(primaryText);
  const displaySecondary = normalizeText(secondaryText);
  const primary = displayPrimary || username;

  const primaryValue = <PrimaryValue title={primary}>{primary}</PrimaryValue>;

  const secondaryValue = displaySecondary && (
    <SecondaryValue title={displaySecondary}>{displaySecondary}</SecondaryValue>
  );

  const usernameValue =
    displayPrimary &&
    (layout === 'inline' ? (
      <InlineUsernameValue title={username}>{username}</InlineUsernameValue>
    ) : (
      <UsernameValue title={username}>{username}</UsernameValue>
    ));

  switch (layout) {
    case 'inline':
      return (
        <Root className={className}>
          <DisplayLine>
            {primaryValue}
            {secondaryValue}
            {usernameValue}
          </DisplayLine>
        </Root>
      );

    case 'stacked':
      return (
        <Root className={className}>
          <DisplayLine>{primaryValue}</DisplayLine>
          {secondaryValue}
          {usernameValue}
        </Root>
      );

    case 'tooltip':
      return (
        <Root className={className}>
          {displayPrimary ? (
            <HoverTooltip tipContent={username}>
              <DisplayLine
                aria-label={getTooltipAriaLabel(
                  primary,
                  displaySecondary,
                  username
                )}
              >
                {primaryValue}
              </DisplayLine>
            </HoverTooltip>
          ) : (
            <DisplayLine>{primaryValue}</DisplayLine>
          )}
          {secondaryValue}
        </Root>
      );

    default:
      layout satisfies never;
      return null;
  }
}

function normalizeText(text?: string) {
  const trimmedText = text?.trim();
  return trimmedText || undefined;
}

function getTooltipAriaLabel(
  primary: string,
  secondary: string | undefined,
  username: string
) {
  return [primary, secondary, `username ${username}`]
    .filter(Boolean)
    .join(', ');
}

// `min-width: 0` lets a flex item shrink below its content size, which is what
// allows the child text to actually trigger ellipsis instead of forcing the
// container to grow.
// `max-width: 100%` then keeps the element from spilling past its parent.
// Without either, long names will push the layout sideways instead of getting truncated.
const containedContent = css`
  min-width: 0;
  max-width: 100%;
`;

const singleLineText = styled(Text).attrs({
  as: 'span',
})`
  ${containedContent}
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
`;

const Root = styled.span`
  ${containedContent}
  display: inline-flex;
  flex-direction: column;
`;

const DisplayLine = styled.span`
  ${containedContent}
  display: inline-flex;
  align-items: baseline;
  gap: ${props => props.theme.space[1]}px;
`;

const PrimaryValue = styled(singleLineText).attrs({
  typography: 'body2',
})``;

const UsernameValue = styled(singleLineText).attrs({
  color: 'text.muted',
  typography: 'body3',
})``;

const SecondaryValue = styled(singleLineText).attrs({
  color: 'text.muted',
  typography: 'body3',
})``;

// The parentheses are decorative wrappers around the inline username — using
// `::before/::after` keeps them out of the React text content so they don't
// appear in `textContent`, snapshots, or the accessibility tree, and lets us
// style them independently from the value itself.
const InlineUsernameValue = styled(UsernameValue)`
  &::before {
    content: '(';
  }

  &::after {
    content: ')';
  }
`;
