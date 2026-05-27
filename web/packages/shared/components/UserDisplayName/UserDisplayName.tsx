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

export type UserDisplayNameProps = {
  username: string;
  primaryText?: string;
  secondaryText?: string;
  layout?: UserDisplayNameLayout;
  className?: string;
};

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

  switch (layout) {
    case 'inline':
      return (
        <Root className={className}>
          <DisplayLine>
            <PrimaryValue title={primary}>{primary}</PrimaryValue>
            {displaySecondary && (
              <InlineSecondaryValue title={displaySecondary}>
                {displaySecondary}
              </InlineSecondaryValue>
            )}
            {displayPrimary && (
              <InlineUsernameValue title={username}>
                {username}
              </InlineUsernameValue>
            )}
          </DisplayLine>
        </Root>
      );

    case 'stacked':
      return (
        <Root className={className}>
          <DisplayLine>
            <PrimaryValue title={primary}>{primary}</PrimaryValue>
          </DisplayLine>
          {displaySecondary && (
            <SecondaryValue title={displaySecondary}>
              {displaySecondary}
            </SecondaryValue>
          )}
          {displayPrimary && (
            <UsernameValue title={username}>{username}</UsernameValue>
          )}
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
                <PrimaryValue title={primary}>{primary}</PrimaryValue>
              </DisplayLine>
            </HoverTooltip>
          ) : (
            <DisplayLine>
              <PrimaryValue title={primary}>{primary}</PrimaryValue>
            </DisplayLine>
          )}
          {displaySecondary && (
            <SecondaryValue title={displaySecondary}>
              {displaySecondary}
            </SecondaryValue>
          )}
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

const containedContent = css`
  min-width: 0;
  max-width: 100%;
`;

const singleLineText = styled(Text).attrs({
  as: 'span',
})`
  ${containedContent}
  white-space: nowrap;
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
  overflow: hidden;
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

const InlineSecondaryValue = styled(SecondaryValue)`
  &::before {
    content: '<';
  }

  &::after {
    content: '>';
  }
`;

const InlineUsernameValue = styled(UsernameValue)`
  &::before {
    content: '(';
  }

  &::after {
    content: ')';
  }
`;
