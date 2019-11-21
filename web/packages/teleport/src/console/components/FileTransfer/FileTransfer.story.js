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
import FileTransferDialog from './FileTransfer';
import { ScpContext } from './useScpContext';
import { Uploader, Downloader } from 'teleport/console/services/fileTransfer';

storiesOf('TeleportConsole/FileTransfer', module)
  .add('with download error', () => {
    const props = createProps();
    const file = {
      ...defaultFile,
      status: 'error',
      error: 'stat /root/test: no such file or directory',
    };

    props.isDownloadOpen = true;
    props.files = [file];

    return (
      <ScpContext.Provider value={mocked}>
        <FileTransferDialog {...props} />
      </ScpContext.Provider>
    );
  })
  .add('with download progress', () => {
    const props = createProps();
    const file = {
      ...defaultFile,
      status: 'processing',
    };

    props.isDownloadOpen = true;
    props.files = [file];

    return (
      <ScpContext.Provider value={mocked}>
        <FileTransferDialog {...props} />
      </ScpContext.Provider>
    );
  })
  .add('with download completed', () => {
    const props = createProps();
    const file = {
      ...defaultFile,
      status: 'completed',
    };

    props.isDownloadOpen = true;
    props.files = [file];

    return (
      <ScpContext.Provider value={mocked}>
        <FileTransferDialog {...props} />
      </ScpContext.Provider>
    );
  })
  .add('with upload', () => {
    const props = createProps();
    const file = {
      ...defaultFile,
      status: 'completed',
      fileName: 'test',
    };

    props.isUploadOpen = true;
    props.files = [file];

    return (
      <ScpContext.Provider value={mocked}>
        <FileTransferDialog {...props} />
      </ScpContext.Provider>
    );
  });

const mocked = {
  createUploader() {
    const uploader = new Uploader();
    uploader.do = () => null;
    return uploader;
  },

  createDownloader() {
    const downloader = new Downloader();
    downloader.do = () => null;
    return downloader;
  },
};

function createProps() {
  return {
    isDownloadOpen: false,
    isUploadOpen: false,
    files: [],
    onDownload: () => null,
    onUpload: () => null,
    onRemove: () => null,
    onUpdate: () => null,
    onClose: () => null,
  };
}

const defaultFile = {
  location: '~test',
  id: '1547581437406~/test',
  url: '/v1/webapi/sites/one/nodes/',
  name:
    '~/test~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf',
  blob: [],
};
