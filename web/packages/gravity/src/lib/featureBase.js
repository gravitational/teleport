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

import { Store } from './stores';
import Logger from 'shared/libs/logger';

const logger = Logger.create('featureBase');

const StatusEnum = {
  READY: 'ready',
  PROCESSING: 'processing',
  FAILED: 'failed',
  UNINITIALIZED :'uninitialized',
  DISABLED: 'disabled'
}

export default class FeatureBase extends Store {

  state = {
    statusText: '',
    status: StatusEnum.UNINITIALIZED
  }

  onload() {
  }

  setProcessing() {
    this.setState({ status:  StatusEnum.PROCESSING})
  }

  setReady() {
    this.setState({ status: StatusEnum.READY })
  }

  setDisabled(){
    this.setState({ status: StatusEnum.DISABLED });
  }

  setFailed(err) {
    logger.error(err);
    this.setState({
      status: StatusEnum.FAILED,
      statusText: err.message
    })
  }

  isReady(){
    return this.state.status === StatusEnum.READY;
  }

  isProcessing() {
    return this.state.status === StatusEnum.PROCESSING;
  }

  isFailed() {
    return this.state.status === StatusEnum.FAILED;
  }

  isDisabled() {
    return this.state.status === StatusEnum.DISABLED;
  }
}

// Activator invokes methods on a group of features.
export class Activator {

  constructor(features) {
    this.features = features || [];
  }

  onload(context) {
    this.features.forEach(f => {
      this._invokeOnload(f, context);
    });
  }

  _invokeOnload(f, ...props) {
    try {
      f.onload(...props);
    } catch(err) {
      logger.error('failed to invoke feature onload()', err);
    }
  }
}