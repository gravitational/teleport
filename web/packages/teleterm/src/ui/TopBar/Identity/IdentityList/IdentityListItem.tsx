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

import styled from 'styled-components';

import { ButtonText, Flex, P3, Text } from 'design';
import { type IconComponentType, Logout, Trash } from 'design/Icon';
import { Cluster } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';

import { useKeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { ListItem } from 'teleterm/ui/components/ListItem';
import { ProfileStatusError } from 'teleterm/ui/components/ProfileStatusError';
import { WorkspaceColor } from 'teleterm/ui/services/workspacesService';
import { RootClusterUri, routing } from 'teleterm/ui/uri';

import { UserIcon } from '../IdentitySelector/UserIcon';

export function IdentityListItem(props: {
  index: number;
  uri: RootClusterUri;
  color: WorkspaceColor;
  cluster: Cluster | undefined;
  onSelect(): void;
  onLogout(): void;
  onForget(): void;
}) {
  const { isActive } = useKeyboardArrowsNavigation({
    index: props.index,
    onRun: props.onSelect,
  });
  const profileName = routing.parseClusterName(props.uri);
  const subtitle = getSubtitle(props.cluster);

  return (
    <StyledListItem
      onClick={props.onSelect}
      onKeyDown={e =>
        (e.key === 'Enter' || e.key === 'Space') && props.onSelect()
      }
      isActive={isActive}
      title={`Switch to ${profileName}`}
    >
      <Flex width="100%" justifyContent="space-between">
        <WithIconItem
          letter={getProfileNameLetter(props.uri)}
          color={props.color}
          title={profileName}
          subtitle={subtitle}
        />
        <Flex alignItems="center" gap={2}>
          {props.cluster?.connected ? (
            <SideButton
              Icon={Logout}
              title={`Log out from ${profileName}`}
              onClick={props.onLogout}
            />
          ) : (
            <SideButton
              Icon={Trash}
              title={`Forget ${profileName}`}
              onClick={props.onForget}
            />
          )}
        </Flex>
      </Flex>
      {props.cluster?.profileStatusError && (
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

function SideButton(props: {
  title: string;
  Icon: IconComponentType;
  onClick: () => void;
}) {
  return (
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
      title={props.title}
      onClick={e => {
        e.stopPropagation();
        props.onClick();
      }}
    >
      <props.Icon size="medium" />
    </ButtonText>
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
          line-height: 1.3;
        `}
      >
        {props.title}
      </Text>

      {props.subtitle && (
        <P3
          color="text.slightlyMuted"
          css={`
            line-height: 1.3;
          `}
        >
          {props.subtitle}
        </P3>
      )}
    </Flex>
  );
}

export function getProfileNameLetter(uri: RootClusterUri): string {
  return routing.parseClusterName(uri).at(0);
}

/**
 * Maps cluster/profile state to the subtitle shown in the identity list.
 *
 * | How this state happened                                                                             | Internal state                                           | Subtitle                   |
 * | ----------------------------------------------------------------------------------------------------| -------------------------------------------------------- | -------------------------- |
 * | `tsh logout` removed the tsh profile, but Connect still remembers the workspace.                    | `cluster` is undefined.                                  | In history                 |
 * | `tsh logout --proxy=... --user=...` or Connect logout removed the credentials but kept the profile. | `cluster` exists, but `loggedInUser.name` is empty.      | Not logged in              |
 * | The user's credentials expired.                                                                     | `cluster` has `loggedInUser.name`, but is not connected. | `<user> · Session expired` |
 * | The user is currently logged in.                                                                    | `cluster` has `loggedInUser.name` and is connected.      | `<user>`                   |
 */
function getSubtitle(cluster: Cluster | undefined): string {
  if (!cluster) {
    return 'In history';
  }

  if (!cluster.loggedInUser?.name) {
    return 'Not logged in';
  }

  if (!cluster.connected) {
    return `${cluster.loggedInUser.name} · Session expired`;
  }

  return cluster.loggedInUser.name;
}
