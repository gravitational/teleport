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

import { Flex, H2, Text, Toggle } from 'design';
import { FolderPlus, Trash } from 'design/Icon';
import { MenuIcon } from 'shared/components/MenuAction';

interface SharedDirectoriesProps {
    sharedDirectories: DirectoryItem[],
    onRemoveSharedDirectory: (number) => void;
}

export function SharedDirectoryList({
  sharedDirectories,
  onRemoveSharedDirectory,
}: SharedDirectoriesProps) {

    console.log("shared directories" + sharedDirectories)
    //const sharedDirectoryList = sharedDirectories.map(directory => (
    //     <Toggle
    //        isToggled={true}
    //        onToggle={() => {onRemoveSharedDirectory(directory.DirectoryId)}}
    //    ></Toggle>
    //))
    const sharedDirectoryList = sharedDirectories.map(directory => (
    <Flex key={directory.DirectoryId} alignItems="center" gap={2}>
        <span>{directory.Name}</span>
        <button onClick={() => onRemoveSharedDirectory(directory.DirectoryId)}>
            <Trash />
        </button>
    </Flex>
))

  return (
    <MenuIcon Icon={FolderPlus} tooltip="New Shared Directory">
      <Container>
        <Flex
          gap={2}
          flexDirection="column"
          onClick={e => {
            // Stop the menu from closing when clicking inside the settings container.
            e.stopPropagation();
          }}
        >
        
        <H2 mb={2}>Shared Directories</H2>

        <>
        {sharedDirectoryList}
        </>

        </Flex>
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