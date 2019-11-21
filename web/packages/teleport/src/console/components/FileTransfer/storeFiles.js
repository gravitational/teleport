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

import { Store } from 'shared/libs/stores';
import cfg from 'teleport/config';

const defaultState = {
  files: [],
  clusterId: null,
  serverId: null,
  login: null,
};

export default class StoreFiles extends Store {
  state = {
    ...defaultState,
  };

  constructor(json) {
    super();
    json && this.setState(json);
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

  remove(id) {
    const files = this.state.files.filter(f => f.id !== id);
    return this.setState({ files });
  }

  add({ location, name, blob, isUpload }) {
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
    const index = this.state.files.findIndex(f => f.id === id);
    const file = this.state.files[index];
    this.state.files[index] = {
      ...file,
      ...json,
    };

    this.setState({
      files: [...this.state.files],
    });
  }
}

export function makeFile(json) {
  const { url, name, isUpload, blob } = json;
  return {
    id: new Date().getTime() + name,
    url,
    name,
    isUpload,
    blob,
    status: '',
    error: '',
  };
}
