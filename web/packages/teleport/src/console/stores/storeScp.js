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

import { merge } from 'lodash';
import { Store } from 'shared/libs/stores';
import cfg from 'teleport/config';

const defaultState = {
  isOpen: false,
  isUpload: false,
  clusterId: '',
  serverId: '',
  login: '',
  files: [],
};

export default class StoreScp extends Store {
  state = {
    ...defaultState,
  };

  constructor(json) {
    super();
    json && this.setState(json);
  }

  open({ isUpload, clusterId, serverId, login }) {
    return this.setState({
      isOpen: true,
      isUpload,
      clusterId,
      serverId,
      login,
    });
  }

  close() {
    this.setState({
      ...defaultState,
    });
  }

  makeUrl(location, filename) {
    const { clusterId, serverId, login } = this.state;
    return cfg.getScpUrl({
      clusterId,
      serverId,
      login,
      location,
      filename,
    });
  }

  removeFile(id) {
    const files = this.state.files.filter(f => f.id !== id);
    return this.setState({ files });
  }

  addFile({ location, name, blob, isUpload }) {
    const url = this.makeUrl(location, name);
    const file = makeFile({
      url,
      name,
      isUpload,
      blob,
    });

    return this.setState({
      files: [...this.state.files, file],
    });
  }

  update({ id, ...json }) {
    const file = this.state.files.find(f => f.id === id);
    merge(file, defaultFileStatus, json);
    this.setState({
      files: [...this.state.files],
    });
  }

  isTransfering() {
    return this.state.files.some(f => f.isProcessing === true);
  }

  openUpload({ clusterId, serverId, login }) {
    this.open({
      clusterId,
      serverId,
      login,
      isUpload: true,
    });
  }

  openDownload({ clusterId, serverId, login }) {
    this.open({
      clusterId,
      serverId,
      login,
      isUpload: false,
    });
  }
}

const defaultFileStatus = {
  isFailed: false,
  isProcessing: false,
  isCompleted: false,
  error: '',
};

export function makeFile(json) {
  const { url, name, isUpload, blob } = json;
  return {
    id: new Date().getTime() + name,
    url,
    name,
    isUpload,
    blob,
    ...defaultFileStatus,
  };
}
