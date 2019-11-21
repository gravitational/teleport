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
import { useStore } from 'shared/libs/stores';
import { Uploader, Downloader } from 'teleport/console/services/fileTransfer';
import StoreFiles from './storeFiles';

export class Scp {
  storeFiles = new StoreFiles();

  init({ clusterId, serverId, login }) {
    this.storeFiles = new StoreFiles({
      clusterId,
      serverId,
      login,
    });
  }

  removeFile(id) {
    this.storeFiles.remove(id);
  }

  updateFile(json) {
    this.storeFiles.update(json);
  }

  addDownload(location) {
    this.storeFiles.add({
      location,
      name: location,
      isUpload: false,
      blob: [],
    });
  }

  addUpload(location, filename, blob) {
    this.storeFiles.add({
      location,
      name: filename,
      isUpload: true,
      blob,
    });
  }

  isTransfering() {
    return this.storeFiles.state.files.some(
      f => f.status === FileStateEnum.PROCESSING
    );
  }

  createUploader() {
    return new Uploader();
  }

  createDownloader() {
    return new Downloader();
  }
}

export const ScpContext = React.createContext(new Scp());

export default function useScpContext() {
  const value = React.useContext(ScpContext);

  if (!value) {
    throw new Error('ScpContext is missing a value');
  }

  return value;
}

export function useStoreFiles() {
  const scpContext = useScpContext();
  return useStore(scpContext.storeFiles);
}

export const FileStateEnum = {
  PROCESSING: 'processing',
  COMPLETED: 'completed',
  ERROR: 'error',
};
