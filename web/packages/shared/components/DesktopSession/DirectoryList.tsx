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

import { Flex, Stack, P1, P2, H3 } from 'design';
import {
  ButtonFill,
  ButtonIntent,
  ButtonPrimary,
  ButtonProps,
  ButtonSecondary,
} from 'design/Button/Button';
import { Eject, FolderPlus, Plus } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';
import { MenuIcon } from 'shared/components/MenuAction';

interface SharedDirectoriesProps {
  sharedDirectories: DirectoryItem[];
  onRemoveSharedDirectory: (id: number) => void;
  onAddSharedDirectory: () => void;
  canRemoveSharedDirectory: boolean;
  canSharedDirectories: boolean;
  maxSharedDirectories: number;
}

export interface DirectoryItem {
  name: string;
  id: number;
}

export function SharedDirectoryList({
  sharedDirectories,
  onRemoveSharedDirectory,
  onAddSharedDirectory,
  canRemoveSharedDirectory,
  canSharedDirectories,
  maxSharedDirectories,
}: SharedDirectoriesProps) {
  const label = !canSharedDirectories
    ? 'Directory sharing is not enabled for this session'
    : 'Share local directories with the desktop';

  return (
    <MenuIcon
      Icon={props => <FolderPlus {...props} size="large" />}
      buttonIconProps={{
        disabled: !canSharedDirectories,
        // square highlight instead of default circle
        css: 'border-radius: 0',
        'aria-label': label,
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
      tooltip={label}
    >
      <Container data-testid="shared-directory-menu">
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
                <DirectoryEntry
                  name={dir.name}
                  id={dir.id}
                  isRemoveSupported={canRemoveSharedDirectory}
                  onRemove={onRemoveSharedDirectory}
                  key={dir.id}
                />
              ))}
            </DirectoryEntries>
          )}

          {/* If not supported, explain to the user that removal is not supported for the
              connected WDS version, but may be supported on new versions. */}
          <RemovalSupportInformation
            isRemoveSupported={canRemoveSharedDirectory}
            directoryCount={sharedDirectories.length}
          />
        </Stack>
      </Container>
    </MenuIcon>
  );
}

function DirectoryEntry(props: {
  name: string;
  id: number;
  isRemoveSupported: boolean;
  onRemove: (id: number) => void;
}) {
  let buttonProps: {
    disabled: boolean;
    intent: ButtonIntent;
    fill: ButtonFill;
  } = {
    disabled: false,
    intent: 'primary',
    fill: 'minimal',
  };
  let hoverText = 'Disconnect 1 shared directory';
  if (!props.isRemoveSupported) {
    hoverText = `
      Disconnecting shared directories is not supported by this version of
      Windows Desktop Service. To enable this feature, contact your Teleport
      administrator about upgrading the Windows Desktop Service instance(s) in
      your Teleport cluster.
      `;
  }

  if (!props.isRemoveSupported) {
    buttonProps = {
      disabled: true,
      intent: 'neutral',
      fill: 'filled',
    };
  }

  return (
    <DirectoryEntriesItem>
      <P2>{props.name}</P2>
      <HoverTooltip placement="bottom" tipContent={hoverText}>
        <Flex flexShrink={0}>
          <ButtonSecondary
            aria-label={`Disconnect shared directory ${props.name}`}
            size="small"
            compact={true}
            onClick={() => props.onRemove(props.id)}
            {...buttonProps}
          >
            <Eject size="small" />
          </ButtonSecondary>
        </Flex>
      </HoverTooltip>
    </DirectoryEntriesItem>
  );
}

function RemovalSupportInformation(props: {
  isRemoveSupported: boolean;
  directoryCount: number;
}) {
  if (props.isRemoveSupported) {
    return;
  }

  const copyText =
    props.directoryCount > 1
      ? 'Disconnect these shared directories by restarting your session.'
      : 'Disconnect this shared directory by restarting your session.';

  return <P1 color="text.muted">{copyText}</P1>;
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
