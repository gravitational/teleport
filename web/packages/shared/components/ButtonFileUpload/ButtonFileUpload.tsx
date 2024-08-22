import React, { useEffect, useState, useRef } from 'react';
import styled from 'styled-components';
import { Upload } from 'design/Icon';
import { ButtonBorder, Flex, LabelInput } from 'design';

import type { ButtonSize } from 'design/Button';
import type { IconSize } from 'design/Icon/Icon';

/**
 * ButtonFileUpload let the user choose a file from local filesystem.
 * Only single file can be selected. To support multiple file uploads,
 * update this component with `multiple` attribute.
 * @param text button text.
 * @param errorMessage error message to display when no file is selected.
 * @param accept whitelist file extension for the file picker.
 * @param disabled enable or disable button.
 * @param setFile callback function to process selected file.
 * @param buttonSize button display size.
 * @param buttonIconSize upload icon display size.
 */
export function ButtonFileUpload({
  text,
  errorMessage,
  accept,
  disabled,
  setSelectedFile,
  buttonSize = 'medium',
  buttonIconSize = 'medium',
}: {
  text: string;
  errorMessage: string;
  accept: string;
  disabled: boolean;
  setSelectedFile: (file: File) => void;
  buttonSize?: ButtonSize;
  buttonIconSize?: IconSize;
}) {
  const fileInputRef = useRef<HTMLInputElement>();
  const [fileInputError, setFileInputError] = useState<boolean>(false);
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
    setSelectedFile(file);
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
