/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
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
