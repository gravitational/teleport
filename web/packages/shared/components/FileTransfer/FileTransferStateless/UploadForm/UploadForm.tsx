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

import React, { useRef, useState } from 'react';
import styled from 'styled-components';

import { Text } from 'design';
import { NoteAdded } from 'design/Icon';

import { Form, PathInput } from '../CommonElements';

interface UploadFormProps {
  onAddUpload(destinationPath: string, file: File): void;
}

export function UploadForm(props: UploadFormProps) {
  const dropzoneRef = useRef<HTMLButtonElement>();
  const fileSelectorRef = useRef<HTMLInputElement>();
  const [destinationPath, setDestinationPath] = useState('~/');

  function onFileSelected(e: React.ChangeEvent<HTMLInputElement>): void {
    upload(Array.from(e.target.files));
  }

  function upload(files: File[]): void {
    files.forEach(file => {
      props.onAddUpload(destinationPath, file);
    });
  }

  function openFilePicker(): void {
    // reset all selected files
    fileSelectorRef.current.value = '';
    fileSelectorRef.current.click();
  }

  function handleDrop(e: React.DragEvent<HTMLButtonElement>): void {
    removeDropzoneStyle(e);

    const { files } = e.dataTransfer;
    e.preventDefault();
    e.stopPropagation();
    upload(Array.from(files));
  }

  function handleKeyDown(event: React.KeyboardEvent): void {
    if (event.key === 'Enter') {
      event.preventDefault();
      event.stopPropagation();
      openFilePicker();
    }
  }

  function addDropzoneStyle(e: React.DragEvent<HTMLButtonElement>): void {
    e.currentTarget.style.backgroundColor = 'rgba(255, 255, 255, 0.1)';
  }

  function removeDropzoneStyle(e: React.DragEvent<HTMLButtonElement>): void {
    e.currentTarget.style.removeProperty('background-color');
  }

  const isUploadDisabled = !destinationPath;

  return (
    <Form>
      <PathInput
        label="Upload Destination"
        value={destinationPath}
        autoFocus
        onChange={e => setDestinationPath(e.target.value)}
        onKeyDown={handleKeyDown}
      />
      <input
        ref={fileSelectorRef}
        disabled={isUploadDisabled}
        type="file"
        data-testid="file-input"
        multiple
        css={`
          display: none;
        `}
        accept="*.*"
        onChange={onFileSelected}
      />
      <Dropzone
        disabled={isUploadDisabled}
        ref={dropzoneRef}
        onDragOver={e => {
          e.preventDefault();
          addDropzoneStyle(e);
        }}
        onDragLeave={removeDropzoneStyle}
        onDrop={handleDrop}
        onClick={e => {
          e.preventDefault();
          openFilePicker();
        }}
      >
        <NoteAdded size="extra-large" mb={2} />
        <Text typography="body2" bold>
          Drag your files here
        </Text>
        <Text typography="body3">
          or Browse your computer to start uploading
        </Text>
      </Dropzone>
    </Form>
  );
}

const Dropzone = styled.button`
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 100%;
  color: inherit;
  background-color: ${props => props.theme.colors.spotBackground[0]};
  margin-top: ${props => props.theme.space[3]}px;
  border: 1px dashed ${props => props.theme.colors.text.muted};
  height: 128px;
  text-align: center;
  cursor: pointer;
  opacity: ${props => (props.disabled ? 0.7 : 1)};
  pointer-events: ${props => (props.disabled ? 'none' : 'unset')};
  border-radius: ${props => props.theme.radii[2]}px;
  font-family: inherit;

  &:focus {
    border-color: ${props => props.theme.colors.spotBackground[1]};
  }
`;
