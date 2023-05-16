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

import React, { useState } from 'react';
import { Flex } from 'design';
import { ButtonPrimary } from 'design/Button';

import { Form, PathInput } from '../CommonElements';

interface DownloadFormProps {
  onAddDownload(sourcePath: string): void;
}

export function DownloadForm(props: DownloadFormProps) {
  const [sourcePath, setSourcePath] = useState('~/');
  const isSourcePathValid = !sourcePath.endsWith('/');

  function download(): void {
    props.onAddDownload(sourcePath);
  }

  return (
    <Form
      onSubmit={e => {
        e.preventDefault();
        download();
      }}
    >
      <Flex alignItems="end">
        <PathInput
          label="File Path"
          autoFocus
          onChange={e => setSourcePath(e.target.value)}
          value={sourcePath}
        />
        <ButtonPrimary
          ml={2}
          px={3}
          size="medium"
          title="Download"
          disabled={!isSourcePathValid}
          type="submit"
        >
          Download
        </ButtonPrimary>
      </Flex>
    </Form>
  );
}
