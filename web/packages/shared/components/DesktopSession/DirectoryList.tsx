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
import { useTheme } from 'styled-components';

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
      Icon={props => <FolderPlus {...props} size="large" />}
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
        <Stack gap={2} fullWidth>
          {/* Header/Share Button */}
          {addDirectoryButton(sharedDirectories.length, onAddSharedDirectory)}

          {/* Directory list */}
          {sharedDirectories.map(dir => (
            <Flex justifyContent="space-between" alignItems="center">
                <Text  fontFamily={useTheme().fonts.mono} fontSize={3}>{dir.name}</Text>
              <HoverTooltip
                placement="bottom"
                tipContent={
                  canRemoveSharedDirectory
                    ? 'Remove Shared Directory'
                    : 'This version of Windows Desktop Server does not support removal of shared directories'
                }
              >
                <Flex flexShrink={0}>
                  <ButtonSecondary
                    size="small"
                    compact={true}
                    onClick={() => onRemoveSharedDirectory(dir.id)}
                    disabled={!canRemoveSharedDirectory}
                  >
                    <Eject size="small" disabled={!canRemoveSharedDirectory} />
                  </ButtonSecondary>
                </Flex>
              </HoverTooltip>
            </Flex>
          ))}          
        </Stack>
      </Container>
    </MenuIcon>
  );
}

function addDirectoryButton(directoryCount: number, onClick: () => void) {
  return (
    <div>
    <Flex justifyContent="space-between" alignItems="center">
      {sharedDirectoryHeader(directoryCount)}
      <ButtonPrimary
        size="small"
        onClick={onClick}
        compact={true}
        $inputAlignment={false}
      >
        <Plus size="small" />
      </ButtonPrimary>
    </Flex>
    {directoryCount > 0 ? <hr></hr> : null}
    </div>
  );
}

function sharedDirectoryHeader(directoryCount: number) {
  if (directoryCount == 0) {
    return (
          <Text fontSize={3}>Connect a shared directory</Text>
      )
  }
  const headerText =
    directoryCount == 1 ? 'shared directory' : 'shared directories';
  return (
    <Text typography="h2">
      {directoryCount > 0 ? `${directoryCount} ` + headerText : headerText}
    </Text>
  );
}

const Container = styled.div`
  background: ${props => props.theme.colors.levels.elevated};
  padding: ${props => props.theme.space[3]}px;
  width: 370px;
`;

const SpaceText = styled.div`
  space: 10;
`;
