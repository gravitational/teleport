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

export function UserDisplayName({
  username,
  primaryText,
  secondaryText,
  layout = 'tooltip',
  className,
}: {
  username: string;
  primaryText?: string;
  secondaryText?: string;
  layout?: UserDisplayNameLayout;
  className?: string;
}) {
  const displayPrimary = normalizeText(primaryText);
  const displaySecondary = normalizeText(secondaryText);
  const primary = displayPrimary || username;
  const tooltipLabel = getTooltipAriaLabel(primary, displaySecondary, username);

  const primaryValue = (ariaLabel?: string) => (
    <PrimaryValue title={primary} aria-label={ariaLabel}>
      {primary}
    </PrimaryValue>
  );

  const secondaryValue = displaySecondary && (
    <SecondaryValue title={displaySecondary}>{displaySecondary}</SecondaryValue>
  );

  const separatedSecondaryValue = displaySecondary && (
    <SeparatedSecondaryValue title={displaySecondary}>
      {displaySecondary}
    </SeparatedSecondaryValue>
  );

  const supportingValues = displayPrimary ? (
    <>
      <UsernameValue title={username}>{username}</UsernameValue>
      {separatedSecondaryValue}
    </>
  ) : (
    secondaryValue
  );

  switch (layout) {
    case 'inline':
      return (
        <Root className={className}>
          <DisplayLine>
            {primaryValue()}
            {displayPrimary ? (
              <InlineSupportingValues>
                {supportingValues}
              </InlineSupportingValues>
            ) : (
              supportingValues
            )}
          </DisplayLine>
        </Root>
      );

    case 'stacked':
      return (
        <Root className={className}>
          <DisplayLine>{primaryValue()}</DisplayLine>
          {displayPrimary ? (
            <SupportingLine>{supportingValues}</SupportingLine>
          ) : (
            supportingValues
          )}
        </Root>
      );

    case 'tooltip':
      return (
        <Root className={className}>
          {displayPrimary ? (
            <DisplayLine>
              <HoverTooltip tipContent={username}>
                {primaryValue(tooltipLabel)}
              </HoverTooltip>
            </DisplayLine>
          ) : (
            <DisplayLine>{primaryValue()}</DisplayLine>
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

const SupportingLine = styled.span`
  ${containedContent}
  display: inline-flex;
  align-items: baseline;
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

const SeparatedSecondaryValue = styled(SecondaryValue)`
  &::before {
    content: '•';
    margin: 0 ${props => props.theme.space[1]}px;
  }
`;

// Decorative delimiters stay out of textContent
const InlineSupportingValues = styled(Text).attrs({
  as: 'span',
  color: 'text.muted',
  typography: 'body3',
})`
  ${containedContent}
  display: inline-flex;
  align-items: baseline;

  &::before {
    content: '(';
  }

  &::after {
    content: ')';
  }
`;
