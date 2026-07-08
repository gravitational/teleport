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

import { Flex, Stack, P2, H3 } from 'design';
import { ButtonPrimary, ButtonProps } from 'design/Button/Button';
import { FolderPlus, Plus } from 'design/Icon';
import { MenuIcon } from 'shared/components/MenuAction';

interface SharedDirectoriesProps {
  sharedDirectories: DirectoryItem[];
  onAddSharedDirectory: () => void;
  canShareDirectories: boolean;
  maxSharedDirectories: number;
  directorySharingMessage: string;
}

export interface DirectoryItem {
  name: string;
  id: number;
}

export function SharedDirectoryList({
  sharedDirectories,
  onAddSharedDirectory,
  canShareDirectories,
  maxSharedDirectories,
  directorySharingMessage,
}: SharedDirectoriesProps) {
  return (
    <MenuIcon
      Icon={props => <FolderPlus {...props} size="large" />}
      buttonIconProps={{
        disabled: !canShareDirectories,
        // square highlight instead of default circle
        css: 'border-radius: 0',
        'aria-label': directorySharingMessage,
      }}
      // Right align the menu with the icon
      menuProps={{
        anchorOrigin: {
          vertical: 'bottom',
          horizontal: 'right',
        },
        transformOrigin: {
          vertical: 'top',
          horizontal: 'right',
        },
      }}
      tooltip={directorySharingMessage}
    >
      <Container data-testid="shared-directory-menu">
        {/* Without e.stopPropagation here, clicking anywhere within the menu
            container closes it. This handler prevents the menu from closing
            until the user clicks outside of the container. */}
        <Stack gap={3} fullWidth onClick={e => e.stopPropagation()}>
          {/* Header/Share Button */}
          <Flex justifyContent="space-between" alignItems="center">
            <DropdownHeader directoryCount={sharedDirectories.length} />
            <ShareButton
              directoryCount={sharedDirectories.length}
              maxSharedDirectories={maxSharedDirectories}
              onClick={onAddSharedDirectory}
            />
          </Flex>

          {!!sharedDirectories.length && (
            <DirectoryEntries aria-label="Shared directories">
              {sharedDirectories.map(dir => (
                <DirectoryEntry name={dir.name} id={dir.id} key={dir.id} />
              ))}
            </DirectoryEntries>
          )}
        </Stack>
      </Container>
    </MenuIcon>
  );
}

function DirectoryEntry(props: { name: string; id: number }) {
  return (
    <DirectoryEntriesItem>
      <P2>{props.name}</P2>
    </DirectoryEntriesItem>
  );
}

function ShareButton(
  props: {
    directoryCount: number;
    maxSharedDirectories: number;
  } & ButtonProps<typeof ButtonPrimary>
) {
  const maxDirectoriesReached =
    props.directoryCount >= props.maxSharedDirectories;

  const shareButtonTitle = maxDirectoriesReached
    ? `Cannot share more than ${props.maxSharedDirectories} directories at once`
    : 'Share a directory';

  return (
    <ButtonPrimary
      title={shareButtonTitle}
      size="small"
      compact={true}
      $inputAlignment={false}
      disabled={maxDirectoriesReached}
      {...props}
    >
      <Plus size="small" />
    </ButtonPrimary>
  );
}

function DropdownHeader(props: { directoryCount: number }) {
  const headerText = (() => {
    switch (props.directoryCount) {
      case 0:
        return 'Share a directory';
      case 1:
        return '1 shared directory';
      default:
        return `${props.directoryCount} shared directories`;
    }
  })();
  return <H3> {headerText} </H3>;
}

const Container = styled.div`
  background: ${props => props.theme.colors.levels.elevated};
  padding: ${props => props.theme.space[3]}px;
  width: 370px;
`;

const DirectoryEntries = styled.ul`
  display: flex;
  flex-direction: column;
  gap: ${props => props.theme.space[3]}px;
  list-style: none;
  margin: 0;
  padding: 0;
`;

const DirectoryEntriesItem = styled.li`
  display: flex;
  align-items: center;
  justify-content: space-between;
`;
