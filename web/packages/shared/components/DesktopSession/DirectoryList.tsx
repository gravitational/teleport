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
import { Eject, FolderShared, Plus } from 'design/Icon';
import { MenuIcon } from 'shared/components/MenuAction';

interface SharedDirectoriesProps {
  sharedDirectories: DirectoryItem[];
  onRemoveSharedDirectory: (id: number) => void;
  onAddSharedDirectory: () => void;
  canRemoveSharedDirectory: boolean;
  canSharedDirectories: boolean;
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
      Icon={FolderShared}
      buttonIconProps={{ disabled: !canSharedDirectories }}
      tooltip={
        !canSharedDirectories
          ? 'Directory sharing is not enabled for this session'
          : undefined
      }
    >
      <Container>
        <Stack gap={1} fullWidth>
          {/* Header row */}
          <Flex justifyContent="space-between" alignItems="center">
            <Text typography="h4">Shared Directories</Text>
            <ButtonPrimary
              size="small"
              p={1}
              minWidth={0}
              height="auto"
              onClick={onAddSharedDirectory}
              compact={true}
              $inputAlignment={false}
            >
              <Plus size="small" />
            </ButtonPrimary>
          </Flex>

          {/* Directory list */}
          <Stack gap={1} fullWidth>
            {sharedDirectories.map(dir => (
              <Flex
                key={dir.id}
                justifyContent="space-between"
                alignItems="center"
              >
                <Text>{dir.name}</Text>
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
              </Flex>
            ))}
          </Stack>
        </Stack>
      </Container>
    </MenuIcon>
  );
}

export interface DirectoryItem  {
  name: string;
  id: number;
};

const Container = styled.div`
  background: ${props => props.theme.colors.levels.elevated};
  padding: ${props => props.theme.space[4]}px;
  width: 370px;
`;
