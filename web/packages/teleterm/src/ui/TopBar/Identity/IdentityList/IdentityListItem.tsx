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
import { useCallback } from 'react';
import styled from 'styled-components';

import { ButtonText, Flex, P3, Text } from 'design';
import { Logout } from 'design/Icon';
import { Cluster } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';

import { useKeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { ListItem } from 'teleterm/ui/components/ListItem';
import { ProfileStatusError } from 'teleterm/ui/components/ProfileStatusError';
import { useStoreSelector } from 'teleterm/ui/hooks/useStoreSelector';
import { WorkspaceColor } from 'teleterm/ui/services/workspacesService';

import { UserIcon } from '../IdentitySelector/UserIcon';

export function IdentityListItem(props: {
  index: number;
  cluster: Cluster;
  onSelect(): void;
  /** If defined, the logout button is rendered. */
  onLogout?(): void;
}) {
  const { isActive } = useKeyboardArrowsNavigation({
    index: props.index,
    onRun: props.onSelect,
  });
  const workspaceColor = useStoreSelector(
    'workspacesService',
    useCallback(
      state => state.workspaces[props.cluster.uri]?.color,
      [props.cluster.uri]
    )
  );

  return (
    <StyledListItem
      onClick={props.onSelect}
      onKeyDown={e =>
        (e.key === 'Enter' || e.key === 'Space') && props.onSelect()
      }
      isActive={isActive}
      title={`Switch to ${props.cluster.name}`}
    >
      <Flex width="100%" justifyContent="space-between">
        <WithIconItem
          letter={getClusterLetter(props.cluster)}
          color={workspaceColor}
          title={props.cluster.name}
          subtitle={props.cluster.loggedInUser?.name}
        />
        {props.onLogout && (
          <ButtonText
            intent="danger"
            size="small"
            className="logout"
            css={`
              visibility: hidden;
              transition: none;
            `}
            p={1}
            ml={4}
            title={`Log out from ${props.cluster.name}`}
            onClick={e => {
              e.stopPropagation();
              props.onLogout();
            }}
          >
            <Logout size="small" />
          </ButtonText>
        )}
      </Flex>
      {props.cluster.profileStatusError && (
        <ProfileStatusError
          error={props.cluster.profileStatusError}
          // Align the error with the user icon.
          css={`
            margin-left: ${props => props.theme.space[2]}px;
            gap: 10px;
          `}
        />
      )}
    </StyledListItem>
  );
}

export function AddClusterItem(props: { index: number; onClick(): void }) {
  const { isActive } = useKeyboardArrowsNavigation({
    index: props.index,
    onRun: props.onClick,
  });

  return (
    <StyledListItem isActive={isActive} onClick={props.onClick}>
      <WithIconItem letter="+" title="Add Cluster…" />
    </StyledListItem>
  );
}

const StyledListItem = styled(ListItem)`
  padding: ${props => props.theme.space[2]}px ${props => props.theme.space[3]}px;
  flex-direction: column;
  align-items: start;
  gap: ${props => props.theme.space[1]}px;
  border-radius: 0;
  height: 100%;
  &:hover .logout {
    visibility: visible;
  }
`;

function WithIconItem(props: {
  letter: string;
  title: string;
  subtitle?: string;
  color?: WorkspaceColor;
}) {
  return (
    <Flex gap={2} alignItems="center">
      <UserIcon letter={props.letter} color={props.color} />
      <TitleAndSubtitle subtitle={props.subtitle} title={props.title} />
    </Flex>
  );
}

export function TitleAndSubtitle(props: { title: string; subtitle?: string }) {
  return (
    <Flex flexDirection="column">
      <Text
        typography="body2"
        fontWeight="400"
        css={`
          line-height: 1.25;
        `}
      >
        {props.title}
      </Text>

      {props.subtitle && (
        <P3
          color="text.slightlyMuted"
          css={`
            line-height: 1.25;
          `}
        >
          {props.subtitle}
        </P3>
      )}
    </Flex>
  );
}

export function getClusterLetter(cluster: Cluster): string {
  return cluster.name.at(0);
}
