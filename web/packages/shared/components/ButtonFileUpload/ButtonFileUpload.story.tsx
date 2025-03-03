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

import { useEffect, useState } from 'react';

import { Box } from 'design';

import { ButtonFileUpload as ButtonFileUploadComponent } from './ButtonFileUpload';

export default {
  title: 'Shared/ButtonFileUpload',
};

export const ButtonFileUpload = () => {
  const [selectedFile, setSelectedFile] = useState<File>();
  const [content, setContent] = useState<string>();
  useEffect(() => {
    if (selectedFile) {
      selectedFile.text().then(data => setContent(data));
    }
  }, [selectedFile]);
  return (
    <>
      <Box mb={4}>
        <ButtonFileUploadComponent
          onFileSelect={setSelectedFile}
          text="Click me to upload a file "
          errorMessage="No files selected."
          accept=".txt"
          disabled={false}
        />
        {content && (
          <>
            <br />
            <p>
              <b>File content:</b> <br />
              <code>{content}</code>
            </p>
          </>
        )}
      </Box>
      <Box mb={4}>
        <ButtonFileUploadComponent
          onFileSelect={setSelectedFile}
          text="Click me to upload a file (disabled)"
          errorMessage=""
          accept=".txt"
          disabled={true}
        />
      </Box>
      <Box>
        <ButtonFileUploadComponent
          onFileSelect={setSelectedFile}
          text="Click me to upload a file (error)"
          showValidationError={true}
          errorMessage="Error rendered"
          accept=".txt"
          disabled={false}
        />
      </Box>
    </>
  );
};
