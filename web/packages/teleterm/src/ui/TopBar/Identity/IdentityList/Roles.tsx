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

import styled from 'styled-components';

import Flex, { Stack } from 'design/Flex';
import { ChevronDown } from 'design/Icon';
import { P3 } from 'design/Text';

export function Roles(props: {
  roles: string[];
  expanded: boolean;
  setExpanded(expanded: boolean): void;
}) {
  return (
    <Details
      open={props.expanded}
      onToggle={event => props.setExpanded(event.currentTarget.open)}
    >
      <Summary>
        <Flex gap={1}>
          <Chevron size="small" />
          Roles ({props.roles.length})
        </Flex>
      </Summary>
      <Expandable>
        {props.roles.length ? (
          <List>
            {props.roles.map(role => (
              <li key={role}>{role}</li>
            ))}
          </List>
        ) : (
          <P3 ml={1}>No roles.</P3>
        )}
      </Expandable>
    </Details>
  );
}

const Details = styled.details`
  ${props => props.theme.typography.body3};
  color: ${props => props.theme.colors.text.slightlyMuted};
  width: 100%;

  &::details-content {
    transition:
      height 0.2s ease,
      content-visibility 0.2s ease allow-discrete;
    height: 0;
    overflow: clip;
  }

  interpolate-size: allow-keywords;

  &[open]::details-content {
    height: auto;
  }
`;

const Chevron = styled(ChevronDown)`
  transition: transform 0.2s ease;

  ${Details}[open] & {
    transform: rotate(180deg);
  }
`;

const Summary = styled.summary`
  cursor: pointer;
  user-select: none;
  width: 100%;
  list-style: none;
  border-radius: ${props => props.theme.space[1]}px;

  &:hover {
    color: ${props => props.theme.colors.text.main};
  }

  &:focus-visible {
    outline: 2px solid ${props => props.theme.colors.text.slightlyMuted};
  }
`;

const Expandable = styled(Stack).attrs({ pl: 3 })`
  position: relative;

  &::before {
    content: '';
    position: absolute;
    left: ${props => props.theme.space[2]}px;
    top: ${props => props.theme.space[1]}px;
    bottom: ${props => props.theme.space[1]}px;
    width: 2px;
    background: ${({ theme }) => theme.colors.interactive.tonal.neutral[0]};
  }
`;

const List = styled.ul`
  box-sizing: border-box;
  margin: 0;
  padding: 0 0 0 ${props => props.theme.space[3]}px;
  min-height: 0;
  width: 100%;
  overflow-y: auto;
  max-height: 30vh;
`;
