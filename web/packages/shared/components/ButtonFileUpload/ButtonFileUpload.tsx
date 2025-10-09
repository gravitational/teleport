/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React, { useEffect, useRef, useState } from 'react';
import styled from 'styled-components';

import { ButtonBorder, Flex, LabelInput } from 'design';
import type { ButtonSize } from 'design/Button';
import { Upload } from 'design/Icon';
import type { IconSize } from 'design/Icon/Icon';

/**
 * ButtonFileUpload let the user choose a file from local filesystem.
 * Only single file can be selected. To support multiple file uploads,
 * update this component with `multiple` attribute.
 * @param text button text.
 * @param errorMessage error message to display when a file is not selected.
 * @param showValidationError display errorMessage on file validation error.
 * @param accept whitelist file extension for the file picker.
 * @param disabled enable or disable button.
 * @param onFileSelect callback function to process selected file.
 * @param buttonSize button display size.
 * @param buttonIconSize upload icon display size.
 */
export function ButtonFileUpload({
  text,
  errorMessage,
  accept,
  disabled,
  onFileSelect,
  showValidationError,
  buttonSize = 'medium',
  buttonIconSize = 'medium',
}: {
  text: string;
  errorMessage: string;
  accept: string;
  disabled: boolean;
  onFileSelect: (file: File) => void;
  showValidationError?: boolean;
  buttonSize?: ButtonSize;
  buttonIconSize?: IconSize;
}) {
  const fileInputRef = useRef<HTMLInputElement>();
  const [fileInputError, setFileInputError] =
    useState<boolean>(showValidationError);
  if (showValidationError !== fileInputError) {
    if (!fileInputError) {
      setFileInputError(showValidationError);
    }
  }
  const [fileName, setFileName] = useState<string>(null);

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    e.stopPropagation();
    e.preventDefault();
    setFileInputError(false);

    if (e.target.files.length === 0) {
      if (!fileName) {
        setFileInputError(true);
      }
      return;
    }
    const file = e.target.files[0];
    setFileName(file.name);
    onFileSelect(file);
  }

  useEffect(() => {
    /**
     * cancel event listener is added to show error message
     * on situation where user cancels file picker without
     * selecting a file.
     */
    fileInputRef.current.addEventListener('cancel', () => {
      if (!fileName) {
        setFileInputError(true);
      }
    });
  }, [fileName, fileInputRef]);

  return (
    <Flex alignItems="center" gap={2}>
      <ButtonBorder
        gap={2}
        onClick={() => fileInputRef.current.click()}
        size={buttonSize}
        textTransform="none"
        px={3}
        disabled={disabled}
      >
        {text}
        <Upload size={buttonIconSize} />
      </ButtonBorder>
      <FileLabel hasError={fileInputError}>
        {fileInputError ? errorMessage : fileName}
      </FileLabel>
      <input
        disabled={disabled}
        accept={accept}
        style={{ display: 'none' }}
        type="file"
        onChange={handleFileChange}
        ref={fileInputRef}
        data-testid="button-file-upload"
      />
    </Flex>
  );
}

const FileLabel = styled(LabelInput)`
  width: auto;
  margin-bottom: 0;
`;
