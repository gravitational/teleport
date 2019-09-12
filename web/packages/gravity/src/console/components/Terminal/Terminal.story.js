/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { storiesOf } from '@storybook/react';
import { Terminal } from './Terminal';
import { TermRec } from 'gravity/console/flux/terminal/store';
import { FileTransferStore } from 'gravity/console/flux/scp/store';

const defaultProps = {
  onAddFile: () => null,
  onCloseFileTransfer: () => null,
  onClose: () => null,
  onOpenPlayer: () => null,
  updateRoute: () => null,
  onTransferRemove: () => null,
  onTransferStart: () => null,
  onTransferUpdate: () => null,
  onOpenUploadDialog: () => null,
  onOpenDownloadDialog: () => null,
  onJoin: () => null,
  termStore: new TermRec(),
  fileStore: new FileTransferStore(),
};

storiesOf('GravityConsole/Terminal', module)
  .add('Loading', () => {
    const props = {
      ...defaultProps,
      termStore: new TermRec(termJson).setStatus({ isLoading: true }),
    };

    return <Terminal {...props} />;
  })
  .add('Error', () => {
    const props = {
      ...defaultProps,
      termStore: new TermRec(termJson).setStatus({
        isError: true,
        errorText: 'system error with long text',
      }),
    };

    return <Terminal {...props} />;
  });

const termJson = {
  hostname: 'localhost',
  login: 'support',
};
