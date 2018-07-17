/*
Copyright 2015 Gravitational, Inc.

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

import { Store } from 'nuclear-js';
import { Record, OrderedMap } from 'immutable';
import cfg from 'app/config';
import * as AT from './actionTypes';

const defaultStatus = {
  isFailed: false,
  isProcessing: false,
  isCompleted: false,
  error: "",
}

class File extends Record({
  id: '',
  url: '',
  name: '',
  isUpload: '',
  blob: null,
  ...defaultStatus
}) {

  constructor(props) {
    props = {
      ...props,
      id: new Date().getTime() + props.name
    }
    super(props)
  }
}

export class FileTransferStore extends Record({
  isOpen: false,
  isUpload: false,
  siteId: '',
  serverId: '',
  login: '',
  files: new OrderedMap()
}){

  constructor(params){
    super(params);
  }

  open({isUpload, siteId, serverId, login}) {
    return this.merge({
      isOpen: true,
      isUpload,
      siteId,
      serverId,
      login
    })
  }

  close() {
    return new FileTransferStore();
  }

  makeUrl(fileName) {
    const {
      siteId,
      serverId,
      login } = this;

    let url = cfg.api.getScpUrl({
      siteId,
      serverId,
      login
    });

    if (fileName.indexOf('/') === 0) {
      url = `${url}/absolute${fileName}`
    } else {
      url = `${url}/relative/${fileName}`
    }

    return url;
  }

  removeFile(id) {
    const files = this.files.delete(id);
    return this.set('files', files);
  }

  addFile({ name, blob, isUpload }) {
    const url = this.makeUrl(name);
    const file = new File({
      url,
      name,
      isUpload,
      blob
    })

    return this.update('files', files => files.set(file.id, file))
  }

  updateStatus({ id, ...rest }) {
    let file = this.files.get(id);
    let status = {
      ...defaultStatus,
      ...rest
    }

    file = file.merge(status);
    return this.update('files', files => files.set(id, file));
  }

  isTransfering() {
    return this.files.some(f => f.isProcessing === true);
  }

}

export default Store({
  getInitialState() {
    return new FileTransferStore();
  },

  initialize() {
    this.on(AT.OPEN, (state, json) => state.open(json));
    this.on(AT.CLOSE, state => state.close());
    this.on(AT.ADD, (state, json) => state.addFile(json));
    this.on(AT.REMOVE, (state, id) => state.removeFile(id));
    this.on(AT.UPDATE_STATUS, (state, json) => state.updateStatus(json));
  }
})