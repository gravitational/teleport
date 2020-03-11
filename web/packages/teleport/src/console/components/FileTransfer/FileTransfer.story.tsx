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
import { FileTransfer } from './FileTransfer';
import { ScpContext } from './scpContextProvider';
import { Scp } from './scpContext';
import { Uploader, Downloader } from 'teleport/console/services/fileTransfer';

export default {
  title: 'TeleportConsole/FileTransfer',
};

const props = {
  onClose: () => null,
};

export const DownloadError = () => {
  const context = makeContext({
    ...defaultFile,
    status: 'error',
    error: 'stat /root/test: no such file or directory',
  });

  return (
    <ScpContext.Provider value={context}>
      <FileTransfer {...props} isDownloadOpen={true} />
    </ScpContext.Provider>
  );
};

export const DownloadProgress = () => {
  const context = makeContext({
    ...defaultFile,
    status: 'processing',
  });

  return (
    <ScpContext.Provider value={context}>
      <FileTransfer {...props} isDownloadOpen={true} />
    </ScpContext.Provider>
  );
};
export const DownloadCompleted = () => {
  const context = makeContext({
    ...defaultFile,
    status: 'completed',
  });

  return (
    <ScpContext.Provider value={context}>
      <FileTransfer {...props} isDownloadOpen={true} />
    </ScpContext.Provider>
  );
};

export const Upload = () => {
  const context = makeContext({
    ...defaultFile,
    status: 'completed',
    fileName: 'test',
  });

  return (
    <ScpContext.Provider value={context}>
      <FileTransfer {...props} isUploadOpen={true} />
    </ScpContext.Provider>
  );
};

function makeContext(file) {
  const context = new Scp({} as any);

  context.updateFile = () => null;
  context.store.state.files = [file];
  context.createUploader = () => {
    const uploader = new Uploader();
    uploader.do = () => null;
    return uploader;
  };

  context.createDownloader = () => {
    const downloader = new Downloader();
    downloader.do = () => null;
    return downloader;
  };

  return context;
}

const defaultFile = {
  location: '~test',
  id: '1547581437406~/test',
  url: '/v1/webapi/sites/one/nodes/',
  name:
    '~/test~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf',
  blob: [],
};
