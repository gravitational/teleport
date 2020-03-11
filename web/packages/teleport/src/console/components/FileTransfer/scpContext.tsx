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

import { Uploader, Downloader } from 'teleport/console/services/fileTransfer';
import StoreFiles from './storeFiles';

export class Scp {
  store = new StoreFiles();

  constructor({ clusterId, serverId, login }: ScpParams) {
    this.store = new StoreFiles({
      clusterId,
      serverId,
      login,
    });
  }

  removeFile(id: number) {
    this.store.remove(id);
  }

  updateFile(partial: Partial<File>) {
    this.store.update(partial);
  }

  addDownload(location: string) {
    this.store.add({
      location,
      name: location,
      isUpload: false,
      blob: [],
    });
  }

  addUpload(location, filename, blob) {
    this.store.add({
      location,
      name: filename,
      isUpload: true,
      blob,
    });
  }

  isTransfering() {
    return this.store.state.files.some(f => f.status === 'processing');
  }

  createUploader() {
    return new Uploader();
  }

  createDownloader() {
    return new Downloader();
  }
}

type ScpParams = {
  clusterId: string;
  serverId: string;
  login: string;
};
