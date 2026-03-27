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

import { Flex, H2, Text, Toggle, Stack } from 'design';
import { ButtonWarningBorder , ButtonBorder, ButtonPrimary, ButtonSecondary} from 'design/Button/Button';
import { FolderShared, Eject, Plus } from 'design/Icon';
import { MenuIcon } from 'shared/components/MenuAction';

interface SharedDirectoriesProps {
    sharedDirectories: DirectoryItem[],
    onRemoveSharedDirectory: (number) => void;
    onAddSharedDirectory: () => void;
}

export function SharedDirectoryList({
  sharedDirectories,
  onRemoveSharedDirectory,
  onAddSharedDirectory,
}: SharedDirectoriesProps) {     
  return (
    <MenuIcon Icon={FolderShared}>
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
            <Flex key={dir.DirectoryId} justifyContent="space-between" alignItems="center">              
              <Text>{dir.Name}</Text>
              <ButtonSecondary
                size="small"
                p={1}
                minWidth={0}
                height="auto"
                title={'Unshare Directory'}                
                onClick={() => onRemoveSharedDirectory(dir.DirectoryId)}
              >
            <Eject size="small" />
          </ButtonSecondary>          
            </Flex>
          ))}
        </Stack>
      </Stack>

      </Container>
    </MenuIcon>
  );
}

export type DirectoryItem = {
    DirectoryId: number,
    Name: string
};

const Container = styled.div`
  background: ${props => props.theme.colors.levels.elevated};
  padding: ${props => props.theme.space[4]}px;
  width: 370px;
`;