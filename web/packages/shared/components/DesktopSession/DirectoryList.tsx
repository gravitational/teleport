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

import { Flex, Stack, Text } from 'design';
import { ButtonPrimary, ButtonSecondary } from 'design/Button/Button';
import { Eject, FolderPlus, Plus } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';
import { MenuIcon } from 'shared/components/MenuAction';

interface SharedDirectoriesProps {
  sharedDirectories: DirectoryItem[];
  onRemoveSharedDirectory: (id: number) => void;
  onAddSharedDirectory: () => void;
  canRemoveSharedDirectory: boolean;
  canSharedDirectories: boolean;
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
}: SharedDirectoriesProps) {
  return (
    <MenuIcon
      Icon={(props) => <FolderPlus {...props} size="large" />}
      buttonIconProps={{
        disabled: !canSharedDirectories,
        // square highlight instead of default circle
        css: 'border-radius: 0',
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
      tooltip={
        !canSharedDirectories
          ? 'Directory sharing is not enabled for this session'
          : 'Share local directories with the Desktop'
      }
    >
      <Container>
        <Stack gap={1} fullWidth>
          {/* Header row */}
          {sharedDirectoryHeader(sharedDirectories.length)}

          {/* Directory list */}
          <Stack gap={1} fullWidth>
            {sharedDirectories.map(dir => (
              <Flex justifyContent="space-between" alignItems="center">
                <Text>{dir.name}</Text>
                <HoverTooltip
                  placement="bottom"
                  tipContent={
                    canRemoveSharedDirectory
                      ? 'Remove Shared Directory'
                      : 'This version of Windows Desktop Server does not support removal of shared directories'
                  }
                >
                  <ButtonSecondary
                    size="small"
                    p={1}
                    minWidth={0}
                    height="auto"
                    title={'Unshare Directory'}
                    onClick={() => onRemoveSharedDirectory(dir.id)}
                    disabled={!canRemoveSharedDirectory}
                  >
                    <Eject size="small" disabled={!canRemoveSharedDirectory} />
                  </ButtonSecondary>
                </HoverTooltip>
              </Flex>
            ))}

            {/* Share Button */}
            {addDirectoryButton(sharedDirectories.length, onAddSharedDirectory)}
          </Stack>
        </Stack>
      </Container>
    </MenuIcon>
  );
}

function addDirectoryButton(directoryCount: number, onClick: () => void) {
  return (
    <Flex justifyContent="space-between" alignItems="center">
      <PurpleText $purple={directoryCount > 0}>
        <Text>Connect a shared directory</Text>
      </PurpleText>
      <ButtonPrimary
        size="small"
        p={1}
        minWidth={0}
        height="auto"
        onClick={onClick}
        compact={true}
        $inputAlignment={false}
      >
        <Plus size="small" />
      </ButtonPrimary>
    </Flex>
  );
}

function sharedDirectoryHeader(directoryCount: number) {
  if (directoryCount == 0) {
    return;
  }

  const headerText =
    directoryCount == 1 ? 'Shared Directory' : 'Shared Directories';
  return (
    <div>
      <Flex justifyContent="space-between" alignItems="center">
        <Text typography="h4">
          {directoryCount > 0 ? `${directoryCount} ` + headerText : headerText}
        </Text>
      </Flex>
      <hr></hr>
    </div>
  );
}

const Container = styled.div`
  background: ${props => props.theme.colors.levels.elevated};
  padding: ${props => props.theme.space[4]}px;
  width: 370px;
`;

const PurpleText = styled.div<{ $purple: boolean }>`
  color: ${p => (p.$purple ? p.theme.colors.brand : 'inherit')};
`;
