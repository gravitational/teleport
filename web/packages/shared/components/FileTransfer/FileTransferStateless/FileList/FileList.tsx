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

import { TransferredFile } from '../types';
import { FileListItem } from './FileListItem';

interface FileListProps {
  files: TransferredFile[];

  onCancel(id: string): void;
}

export function FileList(props: FileListProps) {
  if (!props.files.length) {
    return null;
  }

  return (
    <Ul>
      {props.files.map(file => (
        <FileListItem key={file.id} file={file} onCancel={props.onCancel} />
      ))}
    </Ul>
  );
}

const Ul = styled.ul`
  padding-left: 0;
  overflow: auto;
  max-height: 300px;
  margin-top: 0;
  margin-bottom: 0;
  // scrollbars
  padding-right: 16px;
  margin-right: -16px;
`;
