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

const defaultState = {
  isDownloadOpen: false,
  isUploadOpen: false,
};

export default class StoreDialogs extends Store {
  state = {};

  constructor(json) {
    super();
    json && this.setState(json);
  }

  openUpload(tabId) {
    return this.setState({
      [tabId]: {
        ...defaultState,
        isUploadOpen: true,
      },
    });
  }

  openDownload(tabId) {
    return this.setState({
      [tabId]: {
        ...defaultState,
        isDownloadOpen: true,
      },
    });
  }

  getState(tabId) {
    return this.state[tabId] || defaultState;
  }

  close(tabId) {
    this.setState({
      [tabId]: null,
    });
  }
}
